"""Evaluiert trainierte Modelle auf dem goldenen Testset (nur echte Nachrichten).

Achtung bei der Interpretation: Die true-Klasse hat aktuell fast keine echten
Beispiele — ihr Recall ist statistisch wertlos, bis ml_messages gesammelt hat.

Für Modelle mit predict_proba läuft zusätzlich ein Schwellen-Sweep: Vorhersagen
unter der Konfidenz-Schwelle werden auf "invalid" gesetzt (sicherer Default,
gleiche Semantik wie der bisherige Gemini-Prompt).
"""

import joblib
import numpy as np
from sklearn.metrics import classification_report, confusion_matrix, f1_score

from common import DATA, LABELS, MODELS, load_jsonl


def show_confusion(y_true, y_pred) -> None:
    cm = confusion_matrix(y_true, y_pred, labels=LABELS)
    width = max(len(l) for l in LABELS) + 2
    print("  Confusion (Zeile=wahr, Spalte=vorhergesagt):")
    print("  " + " " * width + "".join(f"{l:>9}" for l in LABELS))
    for label, row in zip(LABELS, cm):
        print(f"  {label:<{width}}" + "".join(f"{n:>9}" for n in row))


def threshold_sweep(pipe, X, y_true) -> None:
    proba = pipe.predict_proba(X)
    classes = list(pipe.classes_)
    best = np.max(proba, axis=1)
    argbest = [classes[i] for i in np.argmax(proba, axis=1)]
    print("  Schwellen-Sweep (unter Schwelle -> invalid):")
    print("  Schwelle  Macro-F1  überschrieben")
    for t in (0.0, 0.4, 0.5, 0.6, 0.7, 0.8):
        y_pred = [p if c >= t else "invalid" for p, c in zip(argbest, best)]
        overridden = sum(1 for p, c in zip(argbest, best) if c < t and p != "invalid")
        f1 = f1_score(y_true, y_pred, labels=LABELS, average="macro", zero_division=0)
        print(f"  {t:>7.1f}  {f1:>8.3f}  {overridden:>4}")


def main() -> None:
    X, y = load_jsonl(DATA / "real.jsonl")
    print(f"Goldenes Testset: {len(X)} echte Nachrichten "
          f"({ {l: y.count(l) for l in LABELS} })\n")

    for path in sorted(MODELS.glob("*.joblib")):
        pipe = joblib.load(path)
        y_pred = list(pipe.predict(X))
        print(f"=== {path.stem} ===")
        print(classification_report(y, y_pred, labels=LABELS, zero_division=0))
        show_confusion(y, y_pred)
        if hasattr(pipe, "predict_proba"):
            threshold_sweep(pipe, X, y)

        wrong = [(t, yt, yp) for t, yt, yp in zip(X, y, y_pred) if yt != yp]
        print(f"  Fehlklassifikationen ({len(wrong)}):")
        for text, yt, yp in wrong:
            one_line = text.replace("\n", " ⏎ ")[:90]
            print(f"    wahr={yt:<7} pred={yp:<7} | {one_line}")
        print()


if __name__ == "__main__":
    main()
