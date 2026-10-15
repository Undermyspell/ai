"""Trainiert die sklearn-Kandidaten der Generation v2 – nur auf Synthetics.

- Training: data/v2/synthetics.jsonl (gelabelt, nicht verworfen; export_data.sh)
- Modellwahl: 5-fache, stratifizierte Cross-Validation (Macro-F1) über ein
  kleines Raster je Kandidat. Der CV-Wert ist optimistisch (ähnliche
  Formulierungen landen in Training und Validierung) – er wählt nur die
  Hyperparameter, die Qualität misst evaluate_v2.py auf Whatsapp.
- Zusätzlich die Out-of-fold-Wahrscheinlichkeiten des besten Rasters: daraus
  bestimmt evaluate_v2.py die invalid-Schwelle, ohne den Test anzufassen.

Ausgabe: models/v2/<name>.joblib, <name>.oof.npy, cv.json
"""

import json
import sys
import time

import joblib
import numpy as np
from sklearn.model_selection import GridSearchCV, StratifiedKFold, cross_val_predict

from models_v2 import DATA, LABELS, MODELS, candidates

SEED = 42


def load(name):
    rows = [json.loads(line) for line in (DATA / f"{name}.jsonl").open(encoding="utf-8")]
    return [r["text"] for r in rows], [r["label"] for r in rows]


def main() -> None:
    only = set(sys.argv[1:])
    X, y = load("synthetics")
    print(f"Training: {len(X)} Synthetics {{{', '.join(f'{l}: {y.count(l)}' for l in LABELS)}}}",
          file=sys.stderr)
    MODELS.mkdir(parents=True, exist_ok=True)
    cv_path = MODELS / "cv.json"
    results = json.loads(cv_path.read_text()) if cv_path.exists() else {}
    cv = StratifiedKFold(n_splits=5, shuffle=True, random_state=SEED)

    for name, (pipe, grid) in candidates().items():
        if only and name not in only:
            continue
        t0 = time.time()
        gs = GridSearchCV(pipe, grid, cv=cv, scoring="f1_macro", n_jobs=1, refit=True)
        gs.fit(X, y)
        i = gs.best_index_
        mean, std = gs.cv_results_["mean_test_score"][i], gs.cv_results_["std_test_score"][i]
        # OOF-Wahrscheinlichkeiten mit den besten Parametern (gleiche Folds)
        best = pipe.set_params(**gs.best_params_)
        oof = cross_val_predict(best, X, y, cv=cv, method="predict_proba", n_jobs=1)
        classes = [str(c) for c in gs.best_estimator_.classes_]
        np.save(MODELS / f"{name}.oof.npy", oof)
        joblib.dump(gs.best_estimator_, MODELS / f"{name}.joblib")
        results[name] = {
            "cv_macro_f1": round(float(mean), 4), "cv_std": round(float(std), 4),
            "params": {k: (list(v) if isinstance(v, tuple) else v) for k, v in gs.best_params_.items()},
            "classes": classes, "seconds": round(time.time() - t0, 1),
        }
        print(f"{name}: CV Macro-F1 {mean:.3f} ± {std:.3f}  {gs.best_params_}  "
              f"({time.time() - t0:.0f} s)", flush=True)

    cv_path.write_text(json.dumps(results, indent=1))
    json.dump(y, (MODELS / "oof_labels.json").open("w"))


if __name__ == "__main__":
    main()
