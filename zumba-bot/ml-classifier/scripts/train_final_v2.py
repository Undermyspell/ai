"""Trainiert die Kandidaten für den Probebetrieb: Synthetics PLUS alle echten
Whatsapp-Nachrichten vor dem Final-Set (FINAL_START).

Hyperparameter und Schwelle kommen aus dem reinen Synthetics-Lauf (cv.json,
report.json) – an echten Nachrichten wird nichts mehr eingestellt. Gemessen
werden diese Modelle nur noch auf dem Final-Set (evaluate_final_v2 bzw. hier
am Ende, sobald es Nachrichten enthält) und den Prüffällen.

Ausgabe: models/v2/final/<name>.joblib (+ Verzeichnis für Transformer),
models/v2/final/report.json
"""

import json
import sys

import joblib
from sklearn.base import clone

from evaluate_v2 import apply_threshold, metrics
from models_v2 import DATA, FINAL_START, LABELS, MODELS, FinetunedModel, candidates, fit_vocab_from

NAMES = ["tfidf_logreg", "tfidf_charword", "nb_logreg", "minilm_ft"]
OUT = MODELS / "final"


def load(name):
    return [json.loads(line) for line in (DATA / f"{name}.jsonl").open(encoding="utf-8")]


def main() -> None:
    names = sys.argv[1:] or NAMES
    cv = json.loads((MODELS / "cv.json").read_text())
    report = json.loads((MODELS / "report.json").read_text())
    syn, wa_all, ma = load("synthetics"), load("whatsapp"), load("manuell")
    wa = [r for r in wa_all if r["created_at"] < FINAL_START]
    final = [r for r in wa_all if r["created_at"] >= FINAL_START]
    X = [r["text"] for r in syn] + [r["text"] for r in wa]
    y = [r["label"] for r in syn] + [r["label"] for r in wa]
    print(f"Training: {len(syn)} Synthetics + {len(wa)} Whatsapp = {len(X)}; "
          f"Final-Set: {len(final)}", file=sys.stderr)
    OUT.mkdir(parents=True, exist_ok=True)
    out = {"training": {"synthetics": len(syn), "whatsapp": len(wa)}, "final_n": len(final), "models": {}}

    cands = candidates()
    for name in names:
        tau = report["models"][name]["schwelle"]
        if name == "minilm_ft":
            from finetune_v2 import BASES, train
            tok, model = train(BASES[name], X, y)
            model.save_pretrained(OUT / name)
            tok.save_pretrained(OUT / name)
            est = FinetunedModel(f"final/{name}")
        else:
            pipe, _ = cands[name]
            params = {k: (tuple(v) if isinstance(v, list) else v) for k, v in cv[name]["params"].items()}
            # Vokabular nur aus den Synthetics: keine echten Texte in der Modelldatei
            est = fit_vocab_from(clone(pipe).set_params(**params), [r["text"] for r in syn], X, y)
        joblib.dump(est, OUT / f"{name}.joblib")
        classes = [str(c) for c in est.classes_]
        res = {"schwelle": tau, "params": cv[name]["params"]}
        P = est.predict_proba([r["text"] for r in ma])
        pred = apply_threshold(P, classes, tau)
        res["manuell"] = [{"text": r["text"], "erwartet": r["label"], "vorhersage": str(p)}
                          for r, p in zip(ma, pred)]
        ok = sum(c["erwartet"] == c["vorhersage"] for c in res["manuell"])
        if final:
            P = est.predict_proba([r["text"] for r in final])
            res["final"] = metrics([r["label"] for r in final], apply_threshold(P, classes, tau))
        out["models"][name] = res
        print(f"{name}: Prüffälle {ok}/{len(ma)}"
              + (f", Final-Set Macro-F1 {res['final']['macro_f1']}" if final else ""), flush=True)
    (OUT / "report.json").write_text(json.dumps(out, indent=1, ensure_ascii=False))


if __name__ == "__main__":
    main()
