"""Feintuning von Transformern für Generation v2 – nur auf Synthetics.

Kandidaten:
- minilm_ft  paraphrase-multilingual-MiniLM-L12-v2 (118 M Parameter, mehrsprachig)
- gbert_ft   deepset/gbert-base (110 M, deutsches BERT)

Wie bei train_v2.py: 5-fache stratifizierte CV (Out-of-fold-Wahrscheinlichkeiten
für Schwelle und CV-Wert), danach Training auf allen Synthetics. Keine Raster-
suche – eine feste, übliche Konfiguration je Modell. Klassen-Gewichte in der
Loss analog class_weight="balanced".

Ausgabe: models/v2/<name>/ (Gewichte + Tokenizer), <name>.joblib (sklearn-API),
<name>.oof.npy, Eintrag in cv.json
"""

import json
import sys
import time

import joblib
import numpy as np
import torch
from sklearn.metrics import f1_score
from sklearn.model_selection import StratifiedKFold
from transformers import AutoModelForSequenceClassification, AutoTokenizer

from models_v2 import DATA, LABELS, MODELS, FinetunedModel

SEED = 42
BASES = {
    "minilm_ft": {"base": "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2",
                  "epochs": 6, "lr": 5e-5},
    "gbert_ft": {"base": "deepset/gbert-base", "epochs": 4, "lr": 3e-5},
}
BATCH = 16
MAX_LEN = 64
DEVICE = "cuda" if torch.cuda.is_available() else "cpu"


def load(name):
    rows = [json.loads(line) for line in (DATA / f"{name}.jsonl").open(encoding="utf-8")]
    return [r["text"] for r in rows], [r["label"] for r in rows]


def load_tokenizer(base):
    try:
        return AutoTokenizer.from_pretrained(base)
    except ValueError:
        # Repos nur mit vocab.txt (deepset/gbert-base): Transformers 5 baut daraus
        # keinen Fast-Tokenizer mehr – direkt über die tokenizers-Bibliothek.
        from huggingface_hub import hf_hub_download
        from tokenizers import BertWordPieceTokenizer
        from transformers import PreTrainedTokenizerFast

        wp = BertWordPieceTokenizer(hf_hub_download(base, "vocab.txt"),
                                    lowercase=False, strip_accents=False)
        return PreTrainedTokenizerFast(tokenizer_object=wp._tokenizer, unk_token="[UNK]",
                                       sep_token="[SEP]", cls_token="[CLS]",
                                       pad_token="[PAD]", mask_token="[MASK]")


def train(cfg, texts, labels):
    torch.manual_seed(SEED)
    tok = load_tokenizer(cfg["base"])
    kw = dict(num_labels=len(LABELS), id2label=dict(enumerate(LABELS)),
              label2id={l: i for i, l in enumerate(LABELS)})
    try:
        model = AutoModelForSequenceClassification.from_pretrained(cfg["base"], **kw)
    except ValueError:
        # Ältere Repos ohne model_type in config.json (deepset/gbert-base) sind BERT.
        from transformers import BertForSequenceClassification

        model = BertForSequenceClassification.from_pretrained(cfg["base"], **kw)
    model = model.to(DEVICE)
    y = torch.tensor([LABELS.index(l) for l in labels])
    counts = np.bincount(y.numpy(), minlength=len(LABELS))
    weights = torch.tensor(len(texts) / (len(LABELS) * counts), dtype=torch.float32, device=DEVICE)
    loss_fn = torch.nn.CrossEntropyLoss(weight=weights)
    opt = torch.optim.AdamW(model.parameters(), lr=cfg["lr"], weight_decay=0.01)
    steps = cfg["epochs"] * ((len(texts) + BATCH - 1) // BATCH)
    sched = torch.optim.lr_scheduler.LambdaLR(
        opt, lambda s: min(1.0, (s + 1) / (0.1 * steps)) * max(0.0, (steps - s) / steps))
    gen = torch.Generator().manual_seed(SEED)
    model.train()
    for _ in range(cfg["epochs"]):
        perm = torch.randperm(len(texts), generator=gen)
        for start in range(0, len(perm), BATCH):
            idx = perm[start:start + BATCH]
            enc = tok([texts[i] for i in idx], truncation=True, max_length=MAX_LEN,
                      padding=True, return_tensors="pt").to(DEVICE)
            loss = loss_fn(model(**enc).logits, y[idx].to(DEVICE))
            loss.backward()
            opt.step()
            sched.step()
            opt.zero_grad()
    return tok, model.eval()


def predict(tok, model, texts):
    out = []
    with torch.no_grad():
        for i in range(0, len(texts), 64):
            enc = tok(texts[i:i + 64], truncation=True, max_length=MAX_LEN,
                      padding=True, return_tensors="pt").to(DEVICE)
            out.append(torch.softmax(model(**enc).logits, dim=-1).cpu().numpy())
    return np.concatenate(out)


def main() -> None:
    only = set(sys.argv[1:])
    X, y = load("synthetics")
    MODELS.mkdir(parents=True, exist_ok=True)
    cv_path = MODELS / "cv.json"
    results = json.loads(cv_path.read_text()) if cv_path.exists() else {}
    folds = StratifiedKFold(n_splits=5, shuffle=True, random_state=SEED)

    for name, cfg in BASES.items():
        if only and name not in only:
            continue
        t0 = time.time()
        oof = np.zeros((len(X), len(LABELS)))
        scores = []
        for tr, va in folds.split(X, y):
            tok, model = train(cfg, [X[i] for i in tr], [y[i] for i in tr])
            P = predict(tok, model, [X[i] for i in va])
            oof[va] = P
            scores.append(f1_score([y[i] for i in va], [LABELS[j] for j in P.argmax(1)],
                                   average="macro"))
            del model
            torch.cuda.empty_cache()
        tok, model = train(cfg, X, y)
        model.save_pretrained(MODELS / name)
        tok.save_pretrained(MODELS / name)
        joblib.dump(FinetunedModel(name), MODELS / f"{name}.joblib")
        np.save(MODELS / f"{name}.oof.npy", oof)
        results[name] = {"cv_macro_f1": round(float(np.mean(scores)), 4),
                         "cv_std": round(float(np.std(scores)), 4),
                         "params": {"base": cfg["base"], "epochs": cfg["epochs"], "lr": cfg["lr"]},
                         "classes": LABELS, "seconds": round(time.time() - t0, 1)}
        print(f"{name}: CV Macro-F1 {np.mean(scores):.3f} ± {np.std(scores):.3f} "
              f"({time.time() - t0:.0f} s)", flush=True)
        cv_path.write_text(json.dumps(results, indent=1))


if __name__ == "__main__":
    main()
