"""Finetuned MiniLM end-to-end (Kandidat minilm_ft, Architektur-Muster C).

Gleiches Basismodell wie minilm_logreg (paraphrase-multilingual-MiniLM-L12-v2),
aber statt frozen Encoder + LogReg werden alle Gewichte mittrainiert; der
Klassifikations-Head (Linear 384 -> 3) startet zufällig. Direkter A/B-Vergleich:
rettet Domänen-Anpassung den Dialekt-Nachteil des vortrainierten Netzes?

Setup wie bei den anderen Kandidaten: Training auf data/synthetic.jsonl plus dem
"train"-Anteil aus data/real.jsonl,
das goldene Testset (real.jsonl) wird hier nie angefasst. Klassen-Gewichte in
der Loss analog zu class_weight="balanced" bei den sklearn-Kandidaten.

Output:
- models/minilm_ft/         safetensors + Tokenizer (das eigentliche Modell)
- models/minilm_ft.joblib   sklearn-API-Wrapper (common.FinetunedMiniLM), damit
                            evaluate.py den Kandidaten automatisch mitnimmt

Bewusst ein plain-PyTorch-Loop statt transformers.Trainer: kein accelerate
nötig, und bei 973 Beispielen ist der Loop übersichtlicher als die Config.
"""

import joblib
import numpy as np
import torch
from transformers import AutoModelForSequenceClassification, AutoTokenizer

from common import DATA, LABELS, MODELS, FinetunedMiniLM, load_jsonl

BASE = "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2"
EPOCHS = 4
BATCH = 16
LR = 2e-5
MAX_LEN = 64
SEED = 42


def main() -> None:
    torch.manual_seed(SEED)
    device = "cuda" if torch.cuda.is_available() else "cpu"

    # Gleiche Trainingsmenge wie train.py: Synthetik + der "train"-Anteil der
    # echten Nachrichten. Sonst waeren die Kandidaten nicht vergleichbar.
    Xs, ys = load_jsonl(DATA / "synthetic.jsonl")
    Xr, yr = load_jsonl(DATA / "real.jsonl", splits={"train"})
    texts, labels = Xs + Xr, ys + yr
    y = torch.tensor([LABELS.index(l) for l in labels])
    print(f"Training: {len(texts)} synthetische Nachrichten auf {device} "
          f"({ {l: labels.count(l) for l in LABELS} })")

    tok = AutoTokenizer.from_pretrained(BASE)
    model = AutoModelForSequenceClassification.from_pretrained(
        BASE, num_labels=len(LABELS),
        id2label=dict(enumerate(LABELS)),
        label2id={l: i for i, l in enumerate(LABELS)},
    ).to(device)

    # analog class_weight="balanced": n / (k * Klassenhäufigkeit)
    counts = np.bincount(y.numpy(), minlength=len(LABELS))
    weights = torch.tensor(len(texts) / (len(LABELS) * counts),
                           dtype=torch.float32, device=device)
    loss_fn = torch.nn.CrossEntropyLoss(weight=weights)
    opt = torch.optim.AdamW(model.parameters(), lr=LR, weight_decay=0.01)

    gen = torch.Generator().manual_seed(SEED)
    model.train()
    for epoch in range(1, EPOCHS + 1):
        perm = torch.randperm(len(texts), generator=gen)
        total = 0.0
        for start in range(0, len(perm), BATCH):
            idx = perm[start:start + BATCH]
            enc = tok([texts[i] for i in idx], truncation=True, max_length=MAX_LEN,
                      padding=True, return_tensors="pt").to(device)
            loss = loss_fn(model(**enc).logits, y[idx].to(device))
            loss.backward()
            opt.step()
            opt.zero_grad()
            total += loss.item() * len(idx)
        print(f"Epoche {epoch}/{EPOCHS}  loss={total / len(texts):.4f}")

    out = MODELS / "minilm_ft"
    model.save_pretrained(out)
    tok.save_pretrained(out)
    joblib.dump(FinetunedMiniLM(), MODELS / "minilm_ft.joblib")
    print(f"Gespeichert: {out}/ + minilm_ft.joblib")


if __name__ == "__main__":
    main()
