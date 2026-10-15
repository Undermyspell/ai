# classifier-service

Pure-Go-Inferenz des in `ml-classifier/` trainierten Nachrichten-Klassifikators
(**NB-LogReg**, Modellgeneration v2). Wer entscheidet – Gemini oder dieses
Modell –, stellt das Admin-UI ein (`/ki-modell`, Standard Gemini). Der
`whatsapp-bot` fragt den Service so oder so und protokolliert beide Label in
`ml_messages`.

## API

```
POST /classify   {"text": "Muss mi heut abmelden ❌"}
→ {"label": "false", "confidence": 0.99, "probs": {"false": 0.99, "invalid": 0.00, "true": 0.01},
   "model": "NB-LogReg 0.2.0"}

GET /info        → Modellname, Klassen, Schwelle und Metadaten (Merkmale, Kopf,
                   Training, Testwerte) – angezeigt auf /ki-modell im Admin-UI
GET /healthz     → 200 ok
```

`label` gilt schon nach der Schwelle: Liegt die höchste Wahrscheinlichkeit
darunter (derzeit 0,45), lautet es `invalid` – der Bot tut dann nichts.

## Modell-Artefakte

`model/model.json.gz` (Format 2: je Zeichen- und Wort-n-Gramm ein fertiges
Gewicht pro Klasse, Intercepts, Schwelle, Metadaten) wird von
`ml-classifier/scripts/export_weights_v2.py` erzeugt und per `go:embed` ins
Binary eingebettet; daneben entsteht `model/golden.json` (Paritäts-Testfälle,
enthält echte Nachrichten → gitignored). Nach jedem Retraining:

```bash
cd ../ml-classifier
uv run --group embeddings scripts/train_final_v2.py nb_logreg
uv run scripts/export_weights_v2.py --version <neue Version>
cd ../classifier-service && go test ./...
```

`TestGoldenParity` vergleicht die Go-Inferenz mit den sklearn-Wahrscheinlichkeiten
(Toleranz 1e-9) und den Labels nach Schwelle – schlägt der Test fehl, weichen
Go- und Python-Implementierung voneinander ab und der Service darf nicht
deployed werden.

**Das Repo ist öffentlich.** Das Endmodell lernt auch von echten
Gruppennachrichten, sein Vokabular (welche n-Gramme es kennt) kommt aber nur
aus den Synthetics (`models_v2.fit_vocab_from`). So steht in `model.json.gz`
kein Name und kein Wortpaar, das es nur in echten Nachrichten gibt.

## Implementierungsnotiz

`internal/model` repliziert sklearn: binäre `char_wb`-Zeichen-n-Gramme (2–4,
wortweise mit Leerzeichen gepolstert, **Runen statt Bytes** — Emojis!) und
Wort-n-Gramme (1–2) mit dem Token-Muster `(?u)\b\w+\b|[❌✅]` (in Go als
`[\p{L}\p{N}_]+|[❌✅]`, weil RE2s `\w` nur ASCII kennt). Kleinschreibung wie
Pythons `str.lower()` (inkl. „İ" → „i̇"). Score je Klasse = Intercept + Summe
der Gewichte, Sigmoid, auf Summe 1 normiert (NB-LogReg one-vs-rest).

Das Dockerfile baut quer (Go kompiliert für `TARGETARCH`, die Endstufe hat
kein `RUN`): `docker buildx build --platform linux/arm64 -t zumba-classifier:<tag> --load .`
