"""Führt die drei Generator-Outputs zu data/synthetic.jsonl zusammen.

- Dedup über normalisierten Text (gleiche Normalisierung wie export_real.py)
- Wirft Synthetik raus, die mit echten Nachrichten kollidiert (Test-Leakage!)
"""

import json
import sys
from pathlib import Path

from export_real import normalize

DATA = Path(__file__).resolve().parent.parent / "data"


def parts() -> list[Path]:
    """Alle Generator-Outputs (synthetic_*.jsonl), nicht der Merge-Output selbst."""
    return sorted(DATA.glob("synthetic_*.jsonl"))


def main() -> None:
    real_keys = set()
    with (DATA / "real.jsonl").open(encoding="utf-8") as f:
        for line in f:
            real_keys.add(normalize(json.loads(line)["text"]))

    rows: dict[str, dict] = {}
    dropped_dup = dropped_leak = 0
    for part in parts():
        with part.open(encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                rec = json.loads(line)
                key = normalize(rec["text"])
                if not key:
                    continue
                if key in real_keys:
                    dropped_leak += 1
                    continue
                if key in rows:
                    dropped_dup += 1
                    continue
                rows[key] = rec

    out = DATA / "synthetic.jsonl"
    with out.open("w", encoding="utf-8") as f:
        for rec in rows.values():
            f.write(json.dumps(rec, ensure_ascii=False) + "\n")

    counts: dict[str, int] = {}
    for rec in rows.values():
        counts[rec["label"]] = counts.get(rec["label"], 0) + 1
    print(f"{len(rows)} synthetische Nachrichten -> {out}", file=sys.stderr)
    print(f"Labels: {counts} | Duplikate: {dropped_dup} | Real-Kollisionen: {dropped_leak}",
          file=sys.stderr)


if __name__ == "__main__":
    main()
