# classifier-service

Pure-Go-Inferenz des in `ml-classifier/` trainierten Nachrichten-Klassifikators
(`tfidf_logreg`). Läuft im **Shadow-Modus** neben Gemini: der `whatsapp-bot` ruft
diesen Service zusätzlich auf und protokolliert beide Ergebnisse in `ml_messages` —
Gemini entscheidet weiterhin allein.

## API

```
POST /classify   {"text": "Muss mi heut abmelden ❌"}
→ {"label": "false", "confidence": 0.97, "probs": {"false": 0.97, "invalid": 0.02, "true": 0.01}}

GET /healthz     → 200 ok
```

## Modell-Artefakte

`model/model.json.gz` (Gewichte) und `model/golden.json` (Paritäts-Testfälle) werden
von `ml-classifier/scripts/export_weights.py` erzeugt und per `go:embed` ins Binary
eingebettet. Nach jedem Retraining:

```bash
cd ../ml-classifier && uv run scripts/export_weights.py
cd ../classifier-service && go test ./...
```

`TestGoldenParity` vergleicht die Go-Inferenz mit den sklearn-Wahrscheinlichkeiten
(Toleranz 1e-9) über 122 echte + synthetische Nachrichten — schlägt der Test fehl,
weichen Go- und Python-Implementierung voneinander ab und der Service darf nicht
deployed werden.

## Implementierungsnotiz

`internal/model` repliziert sklearn exakt: `char_wb`-Zeichen-n-Gramme (2–4,
wortweise mit Leerzeichen gepolstert, **Runen statt Bytes** — Emojis!), TF-IDF mit
`sublinear_tf` + L2-Norm, multinomiale LogReg via Softmax.
