"""Versuch: Was bringen echte Nachrichten im Training?

Nicht der Weg für das Produktivmodell (dort bleibt Whatsapp vorerst reiner
Test), sondern eine Abschätzung für die Entscheidung, wann echte Nachrichten
ins Training wandern.

5-fache Cross-Validation über die eindeutigen Whatsapp-Nachrichten, gruppiert
nach Text (Duplikate bleiben auf einer Seite): trainiert wird auf allen
Synthetics plus den Whatsapp-Nachrichten der anderen vier Teile, vorhergesagt
der fünfte. Hyperparameter und Schwelle wie im reinen Synthetics-Lauf.

Ausgabe: models/v2/crossval_real.json + Übersicht auf stdout.
"""

import json

import numpy as np
from sklearn.base import clone
from sklearn.model_selection import StratifiedKFold

from evaluate_v2 import apply_threshold, metrics
from import_synthetic import text_key
from models_v2 import DATA, FINAL_START, LABELS, MODELS, candidates

SEED = 42
MODELS_TO_TRY = ["tfidf_logreg", "tfidf_charword", "nb_logreg", "minilm_ft"]


def load(name):
    return [json.loads(line) for line in (DATA / f"{name}.jsonl").open(encoding="utf-8")]


def main() -> None:
    cv = json.loads((MODELS / "cv.json").read_text())
    report = json.loads((MODELS / "report.json").read_text())
    syn = load("synthetics")
    wa = [r for r in load("whatsapp") if r["created_at"] < FINAL_START]
    Xs, ys = [r["text"] for r in syn], [r["label"] for r in syn]

    groups = {}
    for r in wa:
        groups.setdefault(text_key(r["text"]), []).append(r)
    keys = list(groups)
    key_labels = [groups[k][0]["label"] for k in keys]
    folds = list(StratifiedKFold(n_splits=5, shuffle=True, random_state=SEED).split(keys, key_labels))

    cands = candidates()
    out = {}
    for name in MODELS_TO_TRY:
        tau = report["models"][name]["schwelle"]
        pred = {}
        for tr, te in folds:
            train_rows = [r for i in tr for r in groups[keys[i]]]
            X = Xs + [r["text"] for r in train_rows]
            y = ys + [r["label"] for r in train_rows]
            test = [groups[keys[i]][0] for i in te]
            if name == "minilm_ft":
                import torch
                from finetune_v2 import BASES, predict, train
                tok, model = train(BASES[name], X, y)
                P = predict(tok, model, [r["text"] for r in test])
                classes = LABELS
                del model
                torch.cuda.empty_cache()
            else:
                pipe, _ = cands[name]
                params = {k: (tuple(v) if isinstance(v, list) else v) for k, v in cv[name]["params"].items()}
                model = clone(pipe).set_params(**params).fit(X, y)
                P = model.predict_proba([r["text"] for r in test])
                classes = [str(c) for c in model.classes_]
            for r, p_ in zip(test, apply_threshold(P, classes, tau)):
                pred[text_key(r["text"])] = str(p_)
        t = [groups[k][0]["label"] for k in keys]
        p = [pred[k] for k in keys]
        nur_syn = report["models"][name]["whatsapp_eindeutig"]["mit_schwelle"]
        out[name] = {"mit_echten": metrics(t, p, boot=True), "nur_synthetics": nur_syn}
        m = out[name]["mit_echten"]
        print(f"{name:16s} nur Synthetics {nur_syn['macro_f1']:.3f} → mit echten {m['macro_f1']:.3f} "
              f"[{m['macro_f1_ci'][0]}–{m['macro_f1_ci'][1]}]  Wirkung {m['wirkung']}", flush=True)
    (MODELS / "crossval_real.json").write_text(json.dumps(out, indent=1, ensure_ascii=False))


if __name__ == "__main__":
    main()
