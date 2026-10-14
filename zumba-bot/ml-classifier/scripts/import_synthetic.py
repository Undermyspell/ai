"""Bereitet generierte Synthetics für die Tabelle ml_synthetic_messages auf.

Eingaben:
- Kandidaten (JSONL, eine Nachricht pro Zeile):
  {"text", "generated_label" (true|false|invalid|null), "art", "sprache", "batch_id"}
  generated_label null = Grenzfall, das Label entscheidet erst der Mensch.
- --ref: Referenztexte aus der DB (JSONL: {"kind": "real"|"synthetic", "text"}),
  siehe import_synthetic.sh.

Verworfen werden
- exakte Duplikate (normalisierter Text, Emojis zählen mit – sonst wären
  „Ich auch ❌" und „Ich auch 👍" dasselbe und reine Emoji-Nachrichten leer),
- Fast-Duplikate echter Nachrichten (Kosinus über Zeichen-n-Gramme ≥ --real):
  die echten sind das Testset, eine Paraphrase davon im Training wäre Leakage,
- Fast-Duplikate untereinander und gegen vorhandene Synthetics (≥ --syn).

Ausgabe: INSERT-SQL auf stdout (ON CONFLICT (text_hash) DO NOTHING – die DB
blockt exakte Duplikate ein zweites Mal), Bericht auf stderr.
"""

import argparse
import hashlib
import json
import re
import sys
import unicodedata
from pathlib import Path

from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.metrics.pairwise import cosine_similarity

LABELS = {"true", "false", "invalid"}


def text_key(text: str) -> str:
    """Dedup-Schlüssel: Unicode-normalisiert, kleingeschrieben, ohne
    Satzzeichen und Emoji-Variantenwähler, Leerraum zusammengefasst –
    Emojis selbst bleiben drin."""
    t = unicodedata.normalize("NFKC", text).casefold().replace("️", "")
    t = "".join(c for c in t if not unicodedata.category(c).startswith("P"))
    return re.sub(r"\s+", " ", t).strip()


def text_hash(text: str) -> str:
    return hashlib.sha1(text_key(text).encode("utf-8")).hexdigest()


def load_jsonl(paths: list[Path]) -> list[dict]:
    out = []
    for p in paths:
        with p.open(encoding="utf-8") as f:
            out.extend(json.loads(line) for line in f if line.strip())
    return out


def sql_str(v: str | None) -> str:
    return "NULL" if v is None or v == "" else "'" + v.replace("'", "''") + "'"


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("candidates", nargs="+", type=Path)
    ap.add_argument("--ref", type=Path, required=True)
    ap.add_argument("--real", type=float, default=0.9, help="Schwelle gegen echte Nachrichten")
    ap.add_argument("--syn", type=float, default=0.9, help="Schwelle gegen Synthetics")
    args = ap.parse_args()

    cands = load_jsonl(args.candidates)
    refs = load_jsonl([args.ref])
    real = [r["text"] for r in refs if r["kind"] == "real"]
    synth = [r["text"] for r in refs if r["kind"] == "synthetic"]

    for c in cands:
        lab = c.get("generated_label")
        if lab is not None and lab not in LABELS:
            sys.exit(f"ungültiges generated_label {lab!r}: {c['text']}")

    vec = TfidfVectorizer(analyzer="char_wb", ngram_range=(2, 4), sublinear_tf=True)
    vec.fit(real + synth + [c["text"] for c in cands])
    real_m = vec.transform(real) if real else None

    seen_keys = {text_key(t): ("echt", t) for t in real}
    seen_keys.update({text_key(t): ("synthetic", t) for t in synth})
    kept: list[dict] = []
    kept_texts = list(synth)
    dropped: list[tuple[str, str]] = []

    for c in cands:
        text = c["text"].strip()
        key = text_key(text)
        if not key:
            dropped.append((text, "leer nach Normalisierung"))
            continue
        if key in seen_keys:
            kind, other = seen_keys[key]
            dropped.append((text, f"exakt wie {kind}: {other}"))
            continue
        v = vec.transform([text])
        if real_m is not None:
            sims = cosine_similarity(v, real_m)[0]
            i = sims.argmax()
            if sims[i] >= args.real:
                dropped.append((text, f"{sims[i]:.2f} zu echter Nachricht: {real[i]}"))
                continue
        if kept_texts:
            sims = cosine_similarity(v, vec.transform(kept_texts))[0]
            i = sims.argmax()
            if sims[i] >= args.syn:
                dropped.append((text, f"{sims[i]:.2f} zu Synthetic: {kept_texts[i]}"))
                continue
        seen_keys[key] = ("synthetic", text)
        kept_texts.append(text)
        kept.append({**c, "text": text})

    print("BEGIN;")
    for c in kept:
        print("INSERT INTO ml_synthetic_messages "
              "(message, text_hash, generated_label, art, sprache, batch_id) VALUES ("
              f"{sql_str(c['text'])}, {sql_str(text_hash(c['text']))}, "
              f"{sql_str(c.get('generated_label'))}, {sql_str(c.get('art'))}, "
              f"{sql_str(c.get('sprache'))}, {sql_str(c.get('batch_id'))}) "
              "ON CONFLICT (text_hash) DO NOTHING;")
    print("COMMIT;")

    print(f"{len(cands)} Kandidaten → {len(kept)} übernommen, {len(dropped)} verworfen "
          f"(Referenz: {len(real)} echte, {len(synth)} vorhandene Synthetics)", file=sys.stderr)
    for text, why in dropped:
        print(f"  verworfen: {text!r} – {why}", file=sys.stderr)


if __name__ == "__main__":
    main()
