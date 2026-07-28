"""Gemeinsame Bausteine für train.py / evaluate.py.

MiniLMEncoder muss hier (importierbares Modul) liegen, nicht in einem
__main__-Skript — sonst können die joblib-Artefakte nicht wieder geladen werden.
"""

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DATA = ROOT / "data"
MODELS = ROOT / "models"

LABELS = ["true", "false", "invalid"]


def load_jsonl(path: Path) -> tuple[list[str], list[str]]:
    texts, labels = [], []
    with path.open(encoding="utf-8") as f:
        for line in f:
            rec = json.loads(line)
            texts.append(rec["text"])
            labels.append(rec["label"])
    return texts, labels


class MiniLMEncoder:
    """Frozen-Sentence-Embeddings als sklearn-Transformer."""

    MODEL = "paraphrase-multilingual-MiniLM-L12-v2"

    def __init__(self):
        from sentence_transformers import SentenceTransformer

        self.model = SentenceTransformer(self.MODEL)

    def fit(self, X, y=None):
        return self

    def transform(self, X):
        return self.model.encode(list(X), show_progress_bar=False,
                                 normalize_embeddings=True)

    def __getstate__(self):  # Modell nicht mit-picklen, beim Laden neu holen
        return {}

    def __setstate__(self, state):
        from sentence_transformers import SentenceTransformer

        self.model = SentenceTransformer(self.MODEL)
