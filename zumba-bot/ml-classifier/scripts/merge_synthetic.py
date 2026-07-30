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
    test_recs: list[dict] = []
    with (DATA / "real.jsonl").open(encoding="utf-8") as f:
        for line in f:
            rec = json.loads(line)
            real_keys.add(normalize(rec["text"]))
            if rec.get("split") == "test":
                test_recs.append(rec)

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
    warn_near_duplicates(list(rows.values()), test_recs)


def warn_near_duplicates(syn: list[dict], test: list[dict], thresh: float = 0.9) -> None:
    """Stolperdraht: synthetischer Trainingssatz ~ Testsatz, aber anderes Label.

    Exakte Kollisionen fängt der Dedup oben. Fast-Duplikate nicht — über
    Zeichen-n-Gramme sind sie aber praktisch derselbe Vektor. Gemeldet wird nur
    der Fall mit abweichendem Label: der bringt dem Modell im Training das
    Gegenteil dessen bei, was im Testset als richtig gilt. Gleiches Label ist
    unkritisch (und aktuell der einzige auftretende Fall).
    """
    if not syn or not test:
        return
    try:
        from sklearn.feature_extraction.text import TfidfVectorizer
        from sklearn.metrics.pairwise import cosine_similarity
    except ImportError:
        return

    st, tt = [r["text"] for r in syn], [r["text"] for r in test]
    vec = TfidfVectorizer(analyzer="char_wb", ngram_range=(2, 4),
                          min_df=1, sublinear_tf=True).fit(st + tt)
    sim = cosine_similarity(vec.transform(tt), vec.transform(st))
    hits = 0
    for i, row in enumerate(sim):
        j = int(row.argmax())
        if row[j] > thresh and test[i]["label"] != syn[j]["label"]:
            hits += 1
            print(f"  ⚠ Near-Duplicate {row[j]:.2f} mit anderem Label: "
                  f"test[{test[i]['label']}] {test[i]['text'][:34]!r} <-> "
                  f"syn[{syn[j]['label']}] {syn[j]['text'][:34]!r}", file=sys.stderr)
    if not hits:
        print(f"Near-Duplicate-Check: keine Test-Nachricht über {thresh} "
              f"Ähnlichkeit bei abweichendem Label.", file=sys.stderr)


if __name__ == "__main__":
    main()
