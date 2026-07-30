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


def present_labels(y_true) -> list[str]:
    """Nur Klassen mit mindestens einem Testfall.

    Ohne diesen Filter wertet sklearn eine leere Klasse mit 0.00 und mittelt sie
    in den Macro-F1 ein — bei 3 Klassen deckelt eine unbelegte das Ergebnis auf
    0.67, egal wie gut das Modell ist. Das liest sich wie ein Modellproblem, ist
    aber ein Datenproblem.
    """
    return [l for l in LABELS if y_true.count(l) > 0]


def threshold_sweep(pipe, X, y_true) -> None:
    proba = pipe.predict_proba(X)
    classes = list(pipe.classes_)
    best = np.max(proba, axis=1)
    argbest = [classes[i] for i in np.argmax(proba, axis=1)]
    labels = present_labels(y_true)
    print("  Schwellen-Sweep (unter Schwelle -> invalid):")
    print("  Schwelle  Macro-F1  überschrieben")
    for t in (0.0, 0.4, 0.5, 0.6, 0.7, 0.8):
        y_pred = [p if c >= t else "invalid" for p, c in zip(argbest, best)]
        overridden = sum(1 for p, c in zip(argbest, best) if c < t and p != "invalid")
        f1 = f1_score(y_true, y_pred, labels=labels, average="macro", zero_division=0)
        print(f"  {t:>7.1f}  {f1:>8.3f}  {overridden:>4}")


def secondary(pipe, X, y, title: str, note: str) -> None:
    """Nebenblock: eine Zahl plus Einordnung, bewusst ohne vollen Report —
    diese Mengen sind keine Güte-Metrik und sollen nicht so gelesen werden."""
    if not X:
        return
    y_pred = list(pipe.predict(X))
    acc = sum(1 for a, b in zip(y, y_pred) if a == b) / len(y)
    print(f"  {title} (n={len(X)}): Trefferquote {acc:.2f} — {note}")


def main() -> None:
    X, y = load_jsonl(DATA / "real.jsonl", splits={"test"})
    Xm, ym = load_jsonl(DATA / "real.jsonl", splits={"monitor"})
    Xt, yt_ = load_jsonl(DATA / "real.jsonl", splits={"train"})

    print(f"Goldenes Testset: {len(X)} echte Nachrichten "
          f"({ {l: y.count(l) for l in LABELS} })")
    thin = [l for l in LABELS if y.count(l) < 5]
    if thin:
        print(f"WARNUNG: Klasse(n) {', '.join(thin)} mit <5 Testfällen — "
              f"Macro-F1 ist hier noch Rauschen. Bis genug verifizierte Daten "
              f"da sind, ist die CV-Zahl aus train.py die belastbarere Größe.")
    print()

    for path in sorted(MODELS.glob("*.joblib")):
        pipe = joblib.load(path)
        y_pred = list(pipe.predict(X))
        print(f"=== {path.stem} ===")
        print(classification_report(y, y_pred, labels=LABELS, zero_division=0))
        labels = present_labels(y)
        if len(labels) < len(LABELS):
            missing = [l for l in LABELS if l not in labels]
            f1 = f1_score(y, y_pred, labels=labels, average="macro", zero_division=0)
            print(f"  Macro-F1 nur über belegte Klassen ({', '.join(labels)}): "
                  f"{f1:.3f}")
            print(f"  ('macro avg' oben zählt {', '.join(missing)} mit 0.00 mit, "
                  f"obwohl dafür kein Testfall existiert — nicht als Güte lesen.)")
        show_confusion(y, y_pred)
        if hasattr(pipe, "predict_proba"):
            threshold_sweep(pipe, X, y)

        wrong = [(t, a, b) for t, a, b in zip(X, y, y_pred) if a != b]
        print(f"  Fehlklassifikationen ({len(wrong)}):")
        for text, a, b in wrong:
            one_line = text.replace("\n", " ⏎ ")[:90]
            print(f"    wahr={a:<7} pred={b:<7} | {one_line}")

        secondary(pipe, Xt, yt_, "Im Training gesehen",
                  "Sanity-Check, keine Güte")
        secondary(pipe, Xm, ym, "Monitor (unverifiziert)",
                  "misst Übereinstimmung mit Gemini, nicht Korrektheit")
        print()


if __name__ == "__main__":
    main()
