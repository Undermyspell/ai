"""Kandidaten und Bausteine der Modellgeneration v2.

Liegt in einem eigenen, importierbaren Modul (nicht in einem __main__-Skript),
damit joblib die Artefakte wieder laden kann.

Kandidaten (sklearn), jeweils mit einem kleinen Raster für die Cross-Validation:
- tfidf_logreg       Zeichen-n-Gramme (char_wb) + LogReg – das heutige Modell
- tfidf_svm          dieselben Merkmale + LinearSVC, per Platt-Skalierung
                     kalibriert (liefert so Wahrscheinlichkeiten für die Schwelle)
- tfidf_charword     Zeichen- UND Wort-n-Gramme + LogReg – „doch", „wieder",
                     „ned" als ganze Wörter und als Paare
- nb_logreg          NB-LogReg (Wang & Manning 2012): binäre Zeichen-+Wort-
                     n-Gramme, je Klasse mit dem Naive-Bayes-Log-Verhältnis
                     skaliert, darauf je Klasse eine LogReg (one-vs-rest)
- minilm_logreg      eingefrorene MiniLM-Satz-Embeddings + LogReg
Feingetunte Transformer (minilm_ft, gbert_ft) trainiert finetune_v2.py; sie
hängen über FinetunedModel an derselben sklearn-API.
"""

from pathlib import Path

import numpy as np
from scipy import sparse
from sklearn.base import BaseEstimator, ClassifierMixin
from sklearn.calibration import CalibratedClassifierCV
from sklearn.feature_extraction.text import CountVectorizer, TfidfVectorizer
from sklearn.linear_model import LogisticRegression
from sklearn.pipeline import FeatureUnion, Pipeline
from sklearn.svm import LinearSVC

ROOT = Path(__file__).resolve().parent.parent
DATA = ROOT / "data" / "v2"
MODELS = ROOT / "models" / "v2"
LABELS = ["true", "false", "invalid"]

# Wort-Token: Buchstaben/Ziffern-Folgen ab einem Zeichen („i", „a" sind im
# Dialekt Wörter: „i a ned"), plus ❌/✅ als eigene Token.
WORD_PATTERN = r"(?u)\b\w+\b|[❌✅]"


def char_tfidf(ngram=(2, 4)):
    return TfidfVectorizer(analyzer="char_wb", ngram_range=ngram, sublinear_tf=True)


def word_tfidf(ngram=(1, 2)):
    return TfidfVectorizer(analyzer="word", token_pattern=WORD_PATTERN,
                           ngram_range=ngram, sublinear_tf=True)


class NBLogReg(BaseEstimator, ClassifierMixin):
    """NB-LogReg für mehrere Klassen (one-vs-rest).

    Je Klasse c: r_c = log((p/|p|₁) / (q/|q|₁)) mit p, q = alpha + Summe der
    binären Merkmale in bzw. außerhalb von c. Die LogReg für c sieht X ∘ r_c.
    Wahrscheinlichkeiten = Sigmoid je Klasse, auf Summe 1 normiert.
    """

    def __init__(self, C=4.0, alpha=1.0):
        self.C = C
        self.alpha = alpha

    def fit(self, X, y):
        X = sparse.csr_matrix(X)
        X.data[:] = 1.0
        y = np.asarray(y)
        self.classes_ = np.array(sorted(set(y)))
        self.r_, self.clfs_ = [], []
        for c in self.classes_:
            pos = y == c
            p = self.alpha + np.asarray(X[pos].sum(axis=0)).ravel()
            q = self.alpha + np.asarray(X[~pos].sum(axis=0)).ravel()
            r = np.log((p / p.sum()) / (q / q.sum()))
            clf = LogisticRegression(C=self.C, class_weight="balanced", max_iter=3000)
            clf.fit(X.multiply(r).tocsr(), pos)
            self.r_.append(r)
            self.clfs_.append(clf)
        return self

    def predict_proba(self, X):
        X = sparse.csr_matrix(X)
        X.data[:] = 1.0
        cols = [clf.predict_proba(X.multiply(r).tocsr())[:, 1]
                for r, clf in zip(self.r_, self.clfs_)]
        P = np.column_stack(cols)
        return P / P.sum(axis=1, keepdims=True)

    def predict(self, X):
        return self.classes_[self.predict_proba(X).argmax(axis=1)]


def candidates():
    """Name → (Pipeline, Raster für GridSearchCV)."""
    cands = {
        "tfidf_logreg": (
            Pipeline([("vec", char_tfidf()),
                      ("clf", LogisticRegression(class_weight="balanced", max_iter=3000))]),
            {"vec__ngram_range": [(2, 4), (2, 5), (1, 5)], "clf__C": [1, 4, 10]},
        ),
        "tfidf_svm": (
            Pipeline([("vec", char_tfidf()),
                      ("clf", CalibratedClassifierCV(
                          LinearSVC(class_weight="balanced"), method="sigmoid", cv=5))]),
            {"vec__ngram_range": [(2, 4), (2, 5), (1, 5)], "clf__estimator__C": [0.3, 1, 3]},
        ),
        "tfidf_charword": (
            Pipeline([("vec", FeatureUnion([("char", char_tfidf((2, 5))), ("word", word_tfidf())])),
                      ("clf", LogisticRegression(class_weight="balanced", max_iter=3000))]),
            {"vec__char__ngram_range": [(2, 4), (2, 5)], "clf__C": [1, 4, 10]},
        ),
        "nb_logreg": (
            Pipeline([("vec", FeatureUnion([
                          ("char", CountVectorizer(analyzer="char_wb", ngram_range=(2, 5), binary=True)),
                          ("word", CountVectorizer(analyzer="word", token_pattern=WORD_PATTERN,
                                                   ngram_range=(1, 2), binary=True))])),
                      ("clf", NBLogReg())]),
            {"vec__char__ngram_range": [(2, 4), (2, 5)], "clf__C": [1, 4, 10]},
        ),
    }
    try:
        from common import MiniLMEncoder
        cands["minilm_logreg"] = (
            Pipeline([("vec", MiniLMEncoder()),
                      ("clf", LogisticRegression(class_weight="balanced", max_iter=3000))]),
            {"clf__C": [1, 4, 10]},
        )
    except ImportError:
        pass
    return cands


class FinetunedModel:
    """Feingetunter Transformer (Verzeichnis unter models/v2/) hinter der
    sklearn-API. Das joblib-Artefakt speichert nur den Pfad, beim Laden werden
    Gewichte und Tokenizer aus dem Verzeichnis geholt."""

    BATCH = 64
    MAX_LEN = 64

    def __init__(self, name: str):
        self.name = name
        self._load()

    def _load(self):
        import torch
        from transformers import AutoModelForSequenceClassification, AutoTokenizer

        path = MODELS / self.name
        self.tok = AutoTokenizer.from_pretrained(path)
        self.model = AutoModelForSequenceClassification.from_pretrained(path).eval()
        self.device = "cuda" if torch.cuda.is_available() else "cpu"
        self.model.to(self.device)
        cfg = self.model.config
        self.classes_ = np.array([cfg.id2label[i] for i in range(cfg.num_labels)])

    def predict_proba(self, X):
        import torch

        out = []
        with torch.no_grad():
            for i in range(0, len(X), self.BATCH):
                enc = self.tok(list(X[i:i + self.BATCH]), truncation=True, max_length=self.MAX_LEN,
                               padding=True, return_tensors="pt").to(self.device)
                out.append(torch.softmax(self.model(**enc).logits, dim=-1).cpu().numpy())
        return np.concatenate(out)

    def predict(self, X):
        return self.classes_[self.predict_proba(X).argmax(axis=1)]

    def __getstate__(self):
        return {"name": self.name}

    def __setstate__(self, state):
        self.name = state["name"]
        self._load()
