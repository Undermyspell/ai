"""Exportiert das Sieger-Modell (tfidf_logreg) für die pure-Go-Inferenz.

Erzeugt zwei Dateien im Zielverzeichnis (Default: ../classifier-service/model/):

- model.json.gz  — alles, was der Go-Service für die Inferenz braucht:
    vocabulary (n-Gramm -> Spaltenindex), idf, LogReg-Koeffizienten/-Intercepts,
    Klassen, Vectorizer-Config (ngram_range, lowercase, sublinear_tf)
- golden.json    — Testfälle (Nachricht -> erwartete Wahrscheinlichkeiten aus
    sklearn) für Paritätstests der Go-Implementierung. Enthält echte und
    synthetische Nachrichten inkl. der bekannten harten Fälle.
"""

import gzip
import json
import sys
from pathlib import Path

import joblib

from common import DATA, MODELS, load_jsonl

OUT_DIR = Path(sys.argv[1]) if len(sys.argv) > 1 else (
    Path(__file__).resolve().parent.parent.parent / "classifier-service" / "model"
)

GOLDEN_EXTRA = [
    "Muss mi heut abmelden ❌",
    "Ich meld mich mal vorsichtshalber ab. I komm aber wahrscheinlich später no",
    "Bei mir wirds bissi später ✌🏻",
    "bin doch dabei 🍻",
    "Melde mich wieder an",
    "mal schauen ob ich komme",
    "Weiß no ned",
    "Sogi doch!",
    "Was sagt die Statistik?",
    "obmeldn muas i mi heid",
    "",
    "🍻🍻🍻",
]


def main() -> None:
    pipe = joblib.load(MODELS / "tfidf_logreg.joblib")
    vec = pipe.named_steps["vec"]
    clf = pipe.named_steps["clf"]

    model = {
        "classes": [str(c) for c in clf.classes_],
        "ngram_min": vec.ngram_range[0],
        "ngram_max": vec.ngram_range[1],
        "sublinear_tf": bool(vec.sublinear_tf),
        "lowercase": bool(vec.lowercase),
        # analyzer ist fest char_wb — die Go-Seite implementiert genau den
        "vocabulary": {term: int(idx) for term, idx in vec.vocabulary_.items()},
        "idf": [float(x) for x in vec.idf_],
        "coef": [[float(x) for x in row] for row in clf.coef_],
        "intercept": [float(x) for x in clf.intercept_],
    }

    OUT_DIR.mkdir(parents=True, exist_ok=True)
    with gzip.open(OUT_DIR / "model.json.gz", "wt", encoding="utf-8") as f:
        json.dump(model, f, ensure_ascii=False)

    # Golden-Testfälle: alle echten + Stichprobe Synthetik + Spezialfälle
    Xr, _ = load_jsonl(DATA / "real.jsonl")
    Xs, _ = load_jsonl(DATA / "synthetic.jsonl")
    texts = Xr + Xs[::25] + GOLDEN_EXTRA
    proba = pipe.predict_proba(texts)
    classes = [str(c) for c in clf.classes_]
    golden = [
        {"text": t, "proba": {c: float(p) for c, p in zip(classes, row)}}
        for t, row in zip(texts, proba)
    ]
    with (OUT_DIR / "golden.json").open("w", encoding="utf-8") as f:
        json.dump(golden, f, ensure_ascii=False, indent=1)

    print(f"model.json.gz: {len(model['vocabulary'])} n-Gramme, "
          f"Klassen {model['classes']}", file=sys.stderr)
    print(f"golden.json: {len(golden)} Testfälle -> {OUT_DIR}", file=sys.stderr)


if __name__ == "__main__":
    main()
