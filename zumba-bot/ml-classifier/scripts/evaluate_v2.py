"""Bewertet die Kandidaten der Generation v2 auf echten Nachrichten.

Testsets (export_data.sh):
- whatsapp eindeutig  – echte Gruppennachrichten, Duplikate (gleicher Text bis
                        auf Satzzeichen/Groß-Klein) nur einmal: Hauptmetrik
- whatsapp alle       – wie im Betrieb, jede Nachricht so oft, wie sie kam
- whatsapp ab August  – nur Nachrichten, die das alte Modell 0.1.2 (trainiert am
                        30.07.) nie gesehen hat: fairer Vergleich mit ihm
- manuell             – Prüffälle, Fall für Fall

Vergleich: Gemini (Label aus dem Probebetrieb) und das alte Modell 0.1.2
(dessen Label aus dem Probebetrieb) auf denselben Nachrichten.

Je Modell: Accuracy, Macro-F1 (mit 95-%-Bootstrap-Bereich), F1 je Klasse,
Fehler mit Wirkung, Fehlerliste. Die invalid-Schwelle (zu unsicher → invalid)
wird auf den Out-of-fold-Vorhersagen der Synthetics gewählt, nie am Test.

Ausgabe: models/v2/report.json + Übersicht auf stdout.
"""

import json
import sys

import joblib
import numpy as np
from sklearn.metrics import accuracy_score, confusion_matrix, f1_score

from import_synthetic import text_key
from models_v2 import DATA, FINAL_START, LABELS, MODELS

AFTER = "2026-07-31"
THRESHOLDS = [0.0] + [round(x, 2) for x in np.arange(0.35, 0.91, 0.05)]
BOOT = 2000
SEED = 42


def load(name):
    return [json.loads(line) for line in (DATA / f"{name}.jsonl").open(encoding="utf-8")]


def apply_threshold(P, classes, tau):
    pred = np.array(classes)[P.argmax(1)]
    if tau > 0:
        pred = np.where(P.max(1) < tau, "invalid", pred)
    return pred


def wirkung(t, p):
    t, p = np.asarray(t), np.asarray(p)
    return {
        "faelschlich_abgemeldet": int(((p == "false") & (t != "false")).sum()),
        "absage_verpasst": int(((t == "false") & (p != "false")).sum()),
        "zusage_verpasst": int(((t == "true") & (p != "true")).sum()),
        "zusage_absage_vertauscht": int((((t == "true") & (p == "false")) |
                                         ((t == "false") & (p == "true"))).sum()),
    }


def metrics(t, p, boot=False):
    m = {
        "n": len(t),
        "accuracy": round(accuracy_score(t, p), 4),
        "macro_f1": round(f1_score(t, p, labels=LABELS, average="macro", zero_division=0), 4),
        "f1": {l: round(v, 4) for l, v in zip(
            LABELS, f1_score(t, p, labels=LABELS, average=None, zero_division=0))},
        "confusion": confusion_matrix(t, p, labels=LABELS).tolist(),
        "wirkung": wirkung(t, p),
    }
    if boot:
        rng = np.random.default_rng(SEED)
        t, p = np.asarray(t), np.asarray(p)
        vals = []
        for _ in range(BOOT):
            i = rng.integers(0, len(t), len(t))
            vals.append(f1_score(t[i], p[i], labels=LABELS, average="macro", zero_division=0))
        m["macro_f1_ci"] = [round(float(np.percentile(vals, 2.5)), 4),
                            round(float(np.percentile(vals, 97.5)), 4)]
    return m


def pick_threshold(name, classes):
    oof_path = MODELS / f"{name}.oof.npy"
    if not oof_path.exists():
        return 0.0, None
    P = np.load(oof_path)
    y = json.load((MODELS / "oof_labels.json").open())
    scores = {tau: f1_score(y, apply_threshold(P, classes, tau), average="macro")
              for tau in THRESHOLDS}
    best = max(scores.values())
    tau = min(t for t, s in scores.items() if s >= best - 1e-9)
    return tau, round(best, 4)


def main() -> None:
    cv = json.loads((MODELS / "cv.json").read_text())
    wa_all, ma = load("whatsapp"), load("manuell")
    wa = [r for r in wa_all if r["created_at"] < FINAL_START]
    final = [r for r in wa_all if r["created_at"] >= FINAL_START]

    # eindeutige Whatsapp-Nachrichten: erster Eintrag je Schlüssel
    seen, uniq, conflicts = {}, [], []
    for r in wa:
        k = text_key(r["text"])
        if k in seen:
            if seen[k]["label"] != r["label"]:
                conflicts.append((seen[k]["text"], seen[k]["label"], r["label"]))
            continue
        seen[k] = r
        uniq.append(r)
    after = [r for r in wa if r["created_at"] >= AFTER]
    sets = {"whatsapp_eindeutig": uniq, "whatsapp_alle": wa, "whatsapp_ab_august": after}
    # Fair gegen 0.1.2: eindeutige Nachrichten, die in dessen Trainings-Export
    # (data/real.jsonl vom 30.07., v1) nicht vorkamen.
    old_export = DATA.parent / "real.jsonl"
    if old_export.exists():
        old_keys = {text_key(json.loads(line)["text"]) for line in old_export.open(encoding="utf-8")}
        sets["whatsapp_ungesehen_012"] = [r for r in uniq if text_key(r["text"]) not in old_keys]
    if final:
        sets["final"] = final
    print(f"Whatsapp: {len(wa)} Nachrichten, {len(uniq)} eindeutig, {len(after)} ab August; "
          f"Label-Konflikte unter Duplikaten: {len(conflicts)}", file=sys.stderr)

    report = {"sets": {k: {"n": len(v), "labels": {l: sum(r["label"] == l for r in v) for l in LABELS}}
                       for k, v in sets.items()},
              "conflicts": conflicts, "models": {}, "baselines": {}}

    # Vergleichsgrößen aus dem Probebetrieb
    for base, field in (("gemini", "gemini"), ("modell_0.1.2", "model_012")):
        b = {}
        for sname, rows in sets.items():
            rows = [r for r in rows if r.get(field)]
            b[sname] = metrics([r["label"] for r in rows], [r[field] for r in rows],
                               boot=sname in ("whatsapp_eindeutig", "whatsapp_ungesehen_012"))
        b["manuell"] = [{"text": r["text"], "erwartet": r["label"], "vorhersage": r.get(field)}
                        for r in ma if field in r]
        report["baselines"][base] = b

    for name in cv:
        model = joblib.load(MODELS / f"{name}.joblib")
        classes = [str(c) for c in model.classes_]
        tau, oof_f1 = pick_threshold(name, classes)
        res = {"cv": cv[name], "schwelle": tau, "oof_macro_f1_mit_schwelle": oof_f1}
        for sname, rows in sets.items():
            P = model.predict_proba([r["text"] for r in rows])
            t = [r["label"] for r in rows]
            res[sname] = {"ohne_schwelle": metrics(t, apply_threshold(P, classes, 0.0),
                                                   boot=sname in ("whatsapp_eindeutig", "whatsapp_ungesehen_012")),
                          "mit_schwelle": metrics(t, apply_threshold(P, classes, tau),
                                                  boot=sname in ("whatsapp_eindeutig", "whatsapp_ungesehen_012"))}
            if sname == "whatsapp_eindeutig":
                pred = apply_threshold(P, classes, tau)
                res["fehler"] = [{"text": r["text"], "richtig": r["label"], "modell": str(p_),
                                  "konfidenz": round(float(P[i].max()), 3)}
                                 for i, (r, p_) in enumerate(zip(rows, pred)) if p_ != r["label"]]
                res["schwellen_kurve"] = {
                    str(x): metrics(t, apply_threshold(P, classes, x))["macro_f1"] for x in THRESHOLDS}
        P = model.predict_proba([r["text"] for r in ma])
        pred = apply_threshold(P, classes, tau)
        res["manuell"] = [{"text": r["text"], "erwartet": r["label"], "vorhersage": str(p_),
                           "konfidenz": round(float(P[i].max()), 3)}
                          for i, (r, p_) in enumerate(zip(ma, pred))]
        report["models"][name] = res

    (MODELS / "report.json").write_text(json.dumps(report, indent=1, ensure_ascii=False))

    # Übersicht
    def row(label, m):
        w = m["wirkung"]
        ci = m.get("macro_f1_ci", ["", ""])
        return (f"{label:22s} {m['macro_f1']:.3f} [{ci[0]}–{ci[1]}]  acc {m['accuracy']:.3f}  "
                f"F1 t/f/i {m['f1']['true']:.2f}/{m['f1']['false']:.2f}/{m['f1']['invalid']:.2f}  "
                f"falsch-ab {w['faelschlich_abgemeldet']}  abs-verp {w['absage_verpasst']}  "
                f"zus-verp {w['zusage_verpasst']}  t↔f {w['zusage_absage_vertauscht']}")
    print("\n== Whatsapp eindeutig (Hauptmetrik) ==")
    for b in report["baselines"]:
        print(row(b, report["baselines"][b]["whatsapp_eindeutig"]))
    for name, res in report["models"].items():
        print(row(f"{name} τ={res['schwelle']}", res["whatsapp_eindeutig"]["mit_schwelle"]),
              f" | CV {res['cv']['cv_macro_f1']}")
    fair = "whatsapp_ungesehen_012" if "whatsapp_ungesehen_012" in sets else "whatsapp_ab_august"
    print(f"\n== {fair} (fair gegen 0.1.2) ==")
    for b in report["baselines"]:
        print(row(b, report["baselines"][b][fair]))
    for name, res in report["models"].items():
        print(row(name, res[fair]["mit_schwelle"]))
    print("\n== Prüffälle (Manuell) bestanden ==")
    for name, res in report["models"].items():
        ok = sum(c["erwartet"] == c["vorhersage"] for c in res["manuell"])
        print(f"{name:22s} {ok}/{len(res['manuell'])}")
    for b in report["baselines"]:
        cases = report["baselines"][b]["manuell"]
        if cases:
            print(f"{b:22s} {sum(c['erwartet'] == c['vorhersage'] for c in cases)}/{len(cases)}")


if __name__ == "__main__":
    main()
