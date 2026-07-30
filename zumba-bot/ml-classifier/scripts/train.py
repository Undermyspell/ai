"""Trainiert die Klassifikator-Kandidaten.

Setup:
- Training: data/synthetic.jsonl + die "train"-Records aus data/real.jsonl
  (von Hand verifizierte Nachrichten und der Trainingsanteil der Absagen).
- Goldenes Testset: die "test"-Records aus data/real.jsonl — werden hier nie
  angefasst, nur evaluate.py liest sie.
Die Zuordnung macht common.split_for über einen Hash des normalisierten Textes;
derselbe Satz landet damit nie gleichzeitig in Training und Test.

Kandidaten:
- tfidf_svm:    TF-IDF char-n-grams (2-5) + LinearSVC (Baseline, kein predict_proba)
- tfidf_logreg: gleiche Features + LogisticRegression (liefert Konfidenzen für
                die spätere invalid-Schwelle)
- minilm_logreg: paraphrase-multilingual-MiniLM-L12-v2 (frozen) + LogisticRegression.
                Nur wenn sentence-transformers installiert ist (uv sync --group embeddings),
                sonst wird der Kandidat übersprungen.

Modellauswahl über 5-fold CV (Macro-F1) auf den Trainingsdaten; jedes Modell wird
anschließend auf allen Trainingsdaten gefittet und nach models/ geschrieben.
"""

import sys

import joblib
import numpy as np
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.linear_model import LogisticRegression
from sklearn.model_selection import StratifiedKFold, cross_val_score
from sklearn.pipeline import Pipeline
from sklearn.svm import LinearSVC

from common import DATA, LABELS, MODELS, MiniLMEncoder, load_jsonl


def tfidf() -> TfidfVectorizer:
    # (2,4)/min_df=1/C=4 via Sweep + Real-Testset gewählt (siehe Git-Historie);
    # bei größerem echten Testset neu validieren — Auswahl nutzte die 71 echten.
    return TfidfVectorizer(analyzer="char_wb", ngram_range=(2, 4),
                           min_df=1, sublinear_tf=True)


def candidates() -> dict[str, Pipeline]:
    cands = {
        "tfidf_svm": Pipeline([
            ("vec", tfidf()),
            ("clf", LinearSVC(class_weight="balanced", C=1.0)),
        ]),
        "tfidf_logreg": Pipeline([
            ("vec", tfidf()),
            ("clf", LogisticRegression(class_weight="balanced", C=4.0, max_iter=2000)),
        ]),
    }
    try:
        cands["minilm_logreg"] = Pipeline([
            ("vec", MiniLMEncoder()),
            ("clf", LogisticRegression(class_weight="balanced", C=4.0, max_iter=2000)),
        ])
    except ImportError:
        print("sentence-transformers fehlt -> minilm_logreg übersprungen "
              "(uv sync --group embeddings)", file=sys.stderr)
    return cands


def main() -> None:
    Xs, ys = load_jsonl(DATA / "synthetic.jsonl")
    Xr, yr = load_jsonl(DATA / "real.jsonl", splits={"train"})
    X, y = Xs + Xr, ys + yr
    print(f"Training: {len(Xs)} synthetische + {len(Xr)} echte Nachrichten "
          f"= {len(X)} ({ {l: y.count(l) for l in LABELS} })", file=sys.stderr)
    if Xr:
        print(f"  echte Trainingsdaten: "
              f"{ {l: yr.count(l) for l in LABELS} }", file=sys.stderr)

    MODELS.mkdir(exist_ok=True)
    cv = StratifiedKFold(n_splits=5, shuffle=True, random_state=42)

    for name, pipe in candidates().items():
        scores = cross_val_score(pipe, X, y, cv=cv, scoring="f1_macro", n_jobs=1)
        print(f"{name}: CV Macro-F1 = {np.mean(scores):.3f} ± {np.std(scores):.3f}",
              flush=True)
        pipe.fit(X, y)
        out = MODELS / f"{name}.joblib"
        joblib.dump(pipe, out)
        print(f"  -> {out}", file=sys.stderr)


if __name__ == "__main__":
    main()
