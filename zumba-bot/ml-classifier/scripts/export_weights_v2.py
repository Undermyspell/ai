"""Exportiert das NB-LogReg-Endmodell für die pure-Go-Inferenz (classifier-service).

Eingabe: models/v2/final/nb_logreg.joblib (train_final_v2.py)
Ausgabe (Default ../classifier-service/model/):

- model.json.gz – Format 2, alles für die Inferenz: je Merkmal (Zeichen- und
  Wort-n-Gramm) ein fertiges Gewicht pro Klasse. NB-LogReg ist linear: das
  Naive-Bayes-Verhältnis r_c wird in den Koeffizienten eingerechnet
  (w_c = r_c · coef_c), der Go-Service summiert nur die Gewichte der
  vorkommenden Merkmale, nimmt je Klasse das Sigmoid und normiert auf 1.
  Dazu Schwelle (darunter → invalid) und Metadaten für /info.
- golden.json – Paritäts-Testfälle (Text → sklearn-Wahrscheinlichkeiten und
  Label nach Schwelle). Enthält echte Nachrichten → nicht im Repo (gitignored).
"""

import argparse
import datetime
import gzip
import json
import sys
from pathlib import Path

import joblib
import numpy as np

from evaluate_v2 import apply_threshold
from models_v2 import DATA, FINAL_START, MODELS, WORD_PATTERN

OUT_DIR = Path(__file__).resolve().parent.parent.parent / "classifier-service" / "model"

GOLDEN_EXTRA = [
    "Muss mi heut abmelden ❌",
    "Ich meld mich mal vorsichtshalber ab. I komm aber wahrscheinlich später no",
    "Bei mir wirds bissi später ✌🏻",
    "bin doch dabei 🍻",
    "Melde mich wieder an",
    "mal schauen ob ich komme",
    "Sogi doch!",
    "Ab 19⁰⁰Uhr gibt's kühle Getränke",
    "obmeldn muas i mi heid",
    "ÄÖÜ ß straße İstanbul",
    "a_b c-d 3² x",
    "",
    "   ",
    "🍻🍻🍻",
    "❌",
    "✅ wieder dabei",
]


def load(name):
    return [json.loads(line) for line in (DATA / f"{name}.jsonl").open(encoding="utf-8")]


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", type=Path, default=OUT_DIR)
    ap.add_argument("--version", required=True, help="Version des classifier-service, z. B. 0.2.0")
    args = ap.parse_args()

    pipe = joblib.load(MODELS / "final" / "nb_logreg.joblib")
    final_report = json.loads((MODELS / "final" / "report.json").read_text())
    cv_real = json.loads((MODELS / "crossval_real.json").read_text())
    res = final_report["models"]["nb_logreg"]
    tau = float(res["schwelle"])

    union, clf = pipe.named_steps["vec"], pipe.named_steps["clf"]
    (_, char_vec), (_, word_vec) = union.transformer_list
    assert char_vec.analyzer == "char_wb" and word_vec.token_pattern == WORD_PATTERN
    classes = [str(c) for c in clf.classes_]
    W = np.vstack([r * lr.coef_[0] for r, lr in zip(clf.r_, clf.clfs_)])  # Klassen × Merkmale
    offset = len(char_vec.vocabulary_)

    def weights(vocab, off):
        return {term: [float(x) for x in W[:, off + idx]] for term, idx in vocab.items()}

    ok = sum(c["erwartet"] == c["vorhersage"] for c in res["manuell"])
    cv = cv_real["nb_logreg"]["mit_echten"]
    model = {
        "format": 2,
        "kind": "nb_logreg",
        "classes": classes,
        "lowercase": True,
        "char": {"ngram_min": char_vec.ngram_range[0], "ngram_max": char_vec.ngram_range[1],
                 "weights": weights(char_vec.vocabulary_, 0)},
        "word": {"ngram_min": word_vec.ngram_range[0], "ngram_max": word_vec.ngram_range[1],
                 "token_pattern": WORD_PATTERN, "weights": weights(word_vec.vocabulary_, offset)},
        "intercept": [float(lr.intercept_[0]) for lr in clf.clfs_],
        "threshold": tau,
        "meta": {
            "name": "NB-LogReg",
            "version": args.version,
            "trainiert_am": datetime.date.today().isoformat(),
            "merkmale": f"Zeichen-n-Gramme {char_vec.ngram_range[0]}–{char_vec.ngram_range[1]} "
                        f"+ Wort-n-Gramme {word_vec.ngram_range[0]}–{word_vec.ngram_range[1]}, binär",
            "gewichtung": "Naive-Bayes-Log-Verhältnis je Klasse",
            "kopf": "logistische Regression je Klasse (one-vs-rest), Sigmoid, auf Summe 1 normiert",
            "schwelle": tau,
            "anzahl_merkmale": {"zeichen": len(char_vec.vocabulary_), "woerter": len(word_vec.vocabulary_)},
            "training": {"synthetics": final_report["training"]["synthetics"],
                         "whatsapp": final_report["training"]["whatsapp"],
                         "whatsapp_bis": FINAL_START},
            "test": {"macro_f1": cv["macro_f1"], "macro_f1_bereich": cv["macro_f1_ci"],
                     "basis": "87 eindeutige Whatsapp-Nachrichten, 5-fach mit echten im Training",
                     "pruefaelle": f"{ok}/{len(res['manuell'])}"},
        },
    }
    args.out.mkdir(parents=True, exist_ok=True)
    with gzip.open(args.out / "model.json.gz", "wt", encoding="utf-8") as f:
        json.dump(model, f, ensure_ascii=False)

    texts = sorted({r["text"] for r in load("whatsapp")} | {r["text"] for r in load("manuell")}) \
        + [r["text"] for r in load("synthetics")][::10] + GOLDEN_EXTRA
    P = pipe.predict_proba(texts)
    labels = apply_threshold(P, classes, tau)
    golden = [{"text": t, "proba": {c: float(p) for c, p in zip(classes, row)}, "label": str(l)}
              for t, row, l in zip(texts, P, labels)]
    (args.out / "golden.json").write_text(json.dumps(golden, ensure_ascii=False, indent=1), encoding="utf-8")

    print(f"model.json.gz: {len(char_vec.vocabulary_)} Zeichen- + {len(word_vec.vocabulary_)} "
          f"Wort-Merkmale, Klassen {classes}, Schwelle {tau}", file=sys.stderr)
    print(f"golden.json: {len(golden)} Testfälle → {args.out}", file=sys.stderr)


if __name__ == "__main__":
    main()
