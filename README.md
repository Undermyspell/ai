# Zumba-Bot

Ein WhatsApp-Bot für einen wöchentlichen Donnerstags-Stammtisch: er erkennt Ab- und Zusagen
in der Gruppe, schreibt sie nach Postgres und beantwortet `statistik`-Anfragen mit einer
Rangliste. Dazu ein Admin-UI, ein Jahresrückblick im Wrapped-Stil und ein eigener, lokal
trainierter ML-Klassifikator. Läuft komplett selbst gehostet (k3s auf einem Raspberry Pi,
GitOps via ArgoCD).

**Das Datenmodell in einem Satz:** Anwesenheit ist der Default — wer nichts schreibt, war da.
Es gibt nur eine Tabelle für *Absagen* (`stammtisch_abwesenheit`), keine für Anwesenheit.

## Der Weg einer Nachricht

```
WhatsApp-Gruppe → Evolution API → POST /webhook/whatsapp (whatsapp-bot)
   │
   ├─ Nachricht == "statistik"  → Rangliste aus Postgres → zurück in den Chat
   │
   └─ sonst: Guards — conversation-Event? Zumba-Gruppe? heute Donnerstag?
        └─ Klassifikator → true | false | invalid
             false (Absage)  → Zeile für heute anlegen
             true  (Zusage)  → Zeile für heute löschen
             invalid         → nichts tun
```

Der Wochentag ist hart verdrahtet: klassifiziert wird **nur donnerstags** in der Zumba-Gruppe
(`statistik` funktioniert immer). Fällt ein Guard, passiert gar nichts — der Klassifikator
wird nicht einmal aufgerufen.

## Klassifikation

Drei Klassen, ein verbindlicher Kontrakt: **`true`** (Zusage) · **`false`** (Absage) ·
**`invalid`** (Smalltalk, „mal schauen“, Orga). Die Schwierigkeit sind die echten Nachrichten:
Bairisch, Emojis, Insider — *„obmeldn muas i mi heid“*, *„Sogi doch!“*.

**1. Google Gemini — produktiv.** Prompt in `zumba-bot/system-prompt.txt`, gibt exakt eines
der drei Labels zurück. Kein Training, dafür Cloud-Abhängigkeit, Latenz und keine
belastbaren Konfidenzen.

**2. Eigenes ML-Modell — im Shadow-Modus.** Läuft parallel mit, wird nur protokolliert
(Admin-UI `/ml-shadow`); Gemini entscheidet weiterhin allein. Trainiert wird in
`zumba-bot/ml-classifier/` auf synthetischen Nachrichten, getestet ausschließlich auf 71
echten — die Eval misst also den Transfer Synthetik → Realwelt.

Jeder Kandidat ist **Featurizer** (Text → Zahlenvektor) + **Head** (Vektor → Klasse). Wie viel
davon aus den eigenen Daten gelernt wird, unterscheidet die drei Ansätze —
`[trainiert]` = aus unseren Daten gelernt, `(frozen)` = vortrainiert, unverändert:

```
A · Feature-Extraktion, klassisch          → tfidf_logreg · tfidf_svm
   Text ─→ [TF-IDF] ─────────────→ 6758-dim sparse ─→ [LogReg/SVM] ─→ Klasse
            vocabulary + idf                            coef + intercept

B · Linear Probing / frozen Encoder        → minilm_logreg
   Text ─→ (MiniLM-Encoder) ─────→ 384-dim dense  ─→ [LogReg] ─────→ Klasse
            117M Param. unberührt                     ~1 200 Param.

C · Full Fine-Tuning                       → minilm_ft
   Text ─→ [MiniLM-Encoder] ─────────────────────────→ [Head] ─────→ Klasse
            alle 117M Param. mittrainiert               Linear 384→3
```

| Kandidat | Ansatz | Macro-F1 |
|---|---|---|
| **`tfidf_logreg`** ✅ | TF-IDF über Zeichen-n-Gramme (2–4) + logistische Regression | **0.987** |
| `minilm_ft` | MiniLM (BERT-Familie) komplett finegetunt | 0.974 |
| `tfidf_svm` | gleiche Features + LinearSVC | 0.860 |
| `minilm_logreg` | MiniLM **frozen** als Featurizer + LogReg | 0.570 |

Zeichen-n-Gramme gewinnen, weil sie zur Eigenart der Daten passen („obmeldn“ und „abmelden“
teilen viele Gramme), während MiniLM auf Standardsprache vortrainiert ist. Der Sprung von
0.57 auf 0.974 durch Fine-Tuning zeigt, woran es lag: nicht am Modell, sondern daran, dass
der eingefrorene Encoder sich nicht anpassen durfte. Zum Überholen reicht es nicht — und der
Sieger deployed als **165 KB JSON in pure Go** statt 466 MB PyTorch.

Der 0.99 ist trotzdem nicht zu trauen: nur **eine** echte Zusage im Testset (Anwesenheit ist
ja der Default), und das Testset floss in die Modellentwicklung ein. Der ehrliche Abnahmetest
ist der Shadow-Modus.

📊 **Details:** `zumba-bot/ml-classifier/eval-ui/index.html` im Browser öffnen — Confusion-Matrizen,
Metriken je Klasse, alle Fehlklassifikationen im Wortlaut, Glossar.

## Komponenten

| Verzeichnis | Was |
|---|---|
| [`whatsapp-bot/`](zumba-bot/whatsapp-bot/) | Go · Webhook, Klassifikation, DB-Writes, Wochenreport |
| [`classifier-service/`](zumba-bot/classifier-service/) | Go · pure-Go-Inferenz des ML-Modells |
| [`ml-classifier/`](zumba-bot/ml-classifier/) | Python · Trainingsdaten, Training, Evaluation, Export |
| [`zumba-admin-ui/`](zumba-bot/zumba-admin-ui/) | Go · Adminoberfläche (templ + HTMX) |
| [`wrapped/`](zumba-bot/wrapped/) | Go · Jahresrückblick `/2026` |
| [`deployment/`](zumba-bot/deployment/) | ArgoCD ApplicationSet · Helm · SealedSecrets |

Jeder Go-Service: `make dev` (Hot-Reload) · `make build` · `make test`.
Details in den jeweiligen READMEs.
