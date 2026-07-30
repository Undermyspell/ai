"""Gemeinsame Bausteine für train.py / evaluate.py.

MiniLMEncoder muss hier (importierbares Modul) liegen, nicht in einem
__main__-Skript — sonst können die joblib-Artefakte nicht wieder geladen werden.
"""

import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DATA = ROOT / "data"
MODELS = ROOT / "models"

LABELS = ["true", "false", "invalid"]

# Anteil (in Prozent), der ins Testset geht — je nach Herkunft der Nachricht.
# Verifizierte sind das wertvollste Signal und die einzige echte Quelle für
# true/invalid, deshalb geht der Großteil ins Training und nur 30% in den Holdout.
TEST_PCT_VERIFIED = 30
TEST_PCT_ABWESENHEIT = 20


def bucket(norm_text: str) -> int:
    """Stabiler Bucket 0-99 aus dem normalisierten Text.

    Bewusst ein Hash und kein Zufalls-Split: eine Nachricht landet dadurch immer
    auf derselben Seite, auch wenn der Datensatz wächst — sonst wären Metriken
    über die Zeit nicht vergleichbar. Und weil der Split eine reine Funktion des
    Textes ist, kann derselbe Satz nie gleichzeitig in Training und Test landen,
    selbst wenn die Dedup irgendwo durchrutscht. Leakage ist damit strukturell
    ausgeschlossen, nicht nur prozedural vermieden.
    """
    return int(hashlib.sha1(norm_text.encode("utf-8")).hexdigest(), 16) % 100


def split_for(norm_text: str, origin: str, verified: bool) -> str:
    """Ordnet einen Record "train", "test" oder "monitor" zu.

    monitor = unverifizierte Gemini-/Bot-Label. Die gehören weder ins Training
    (sonst lernt das Modell Geminis Fehler) noch in die Hauptmetrik (sonst misst
    man Übereinstimmung mit Gemini statt Korrektheit) — sie werden nur separat
    ausgewiesen, bis sie im Admin-UI bewertet wurden.
    """
    if verified:
        return "test" if bucket(norm_text) < TEST_PCT_VERIFIED else "train"
    if origin == "stammtisch_abwesenheit":
        # Strukturell verlässlich (jede Zeile ist eine persistierte Absage),
        # aber label-degeneriert: ausschließlich "false".
        return "test" if bucket(norm_text) < TEST_PCT_ABWESENHEIT else "train"
    return "monitor"


def load_jsonl(path: Path, splits: set[str] | None = None
               ) -> tuple[list[str], list[str]]:
    """Lädt Texte + Label. splits filtert auf das split-Feld (None = alles).

    synthetic.jsonl hat kein split-Feld; dort greift der Filter nie.
    """
    texts, labels = [], []
    with path.open(encoding="utf-8") as f:
        for line in f:
            rec = json.loads(line)
            if splits is not None and rec.get("split") not in splits:
                continue
            texts.append(rec["text"])
            labels.append(rec["label"])
    return texts, labels


class FinetunedMiniLM:
    """Finetuntes MiniLM (models/minilm_ft/, siehe finetune_minilm.py) hinter der
    sklearn-Pipeline-API, damit evaluate.py es wie die joblib-Kandidaten behandelt.

    Gleicher Trick wie MiniLMEncoder: das joblib-Artefakt speichert keine Gewichte,
    beim Laden wird aus dem safetensors-Verzeichnis neu geladen.
    """

    DIR = MODELS / "minilm_ft"
    BATCH = 64
    MAX_LEN = 64

    def __init__(self):
        self._load()

    def _load(self):
        import torch
        from transformers import AutoModelForSequenceClassification, AutoTokenizer

        self.tok = AutoTokenizer.from_pretrained(self.DIR)
        self.model = AutoModelForSequenceClassification.from_pretrained(self.DIR).eval()
        self.device = "cuda" if torch.cuda.is_available() else "cpu"
        self.model.to(self.device)
        cfg = self.model.config
        self.classes_ = [cfg.id2label[i] for i in range(cfg.num_labels)]

    def predict_proba(self, X):
        import numpy as np
        import torch

        out = []
        with torch.no_grad():
            for i in range(0, len(X), self.BATCH):
                enc = self.tok(list(X[i:i + self.BATCH]), truncation=True,
                               max_length=self.MAX_LEN, padding=True,
                               return_tensors="pt").to(self.device)
                probs = torch.softmax(self.model(**enc).logits, dim=-1)
                out.append(probs.cpu().numpy())
        return np.concatenate(out)

    def predict(self, X):
        import numpy as np

        return [self.classes_[i] for i in np.argmax(self.predict_proba(X), axis=1)]

    def __getstate__(self):
        return {}

    def __setstate__(self, state):
        self._load()


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
