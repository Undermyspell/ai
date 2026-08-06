# Zumba-Bot

Ein WhatsApp-Bot für einen wöchentlichen Donnerstags-Stammtisch: er erkennt Ab- und Zusagen
in der Gruppe, schreibt sie nach Postgres, beantwortet `statistik`-Anfragen mit einer
Rangliste und postet donnerstags um 21:00 den Wochenreport samt Strafenkasse. Dazu ein
Admin-UI, ein Jahresrückblick im Wrapped-Stil (`/2026`, läuft auf Staging) und ein eigener,
lokal trainierter ML-Klassifikator. Läuft komplett selbst gehostet (k3s auf einem Raspberry
Pi 5, GitOps via ArgoCD, k3s-Upgrades ebenfalls per GitOps).

**Das Datenmodell in einem Satz:** Anwesenheit ist der Default — wer nichts schreibt, war da.
Es gibt nur eine Tabelle für *Absagen* (`stammtisch_abwesenheit`), keine für Anwesenheit.
Seit 08/2026 wird zu jeder Absage auch der Zeitpunkt (`created_at`) mitgeschrieben — Futter
für Wrapped 2027 („kurzfristigste Absage“).

## Der Weg einer Nachricht

```
WhatsApp-Gruppe → Evolution API → POST /webhook/whatsapp (whatsapp-bot)
   │
   ├─ Nachricht == "statistik"  → Rangliste aus Postgres → zurück in den Chat
   │
   └─ sonst: Guards — conversation-Event? Zumba-Gruppe? heute Donnerstag?
        └─ Klassifikator → true | false | invalid
             false (Absage)  → Zeile für heute anlegen (+ created_at)
             true  (Zusage)  → Zeile für heute löschen
             invalid         → nichts tun
```

Der Wochentag ist hart verdrahtet: klassifiziert wird **nur donnerstags** in der Zumba-Gruppe
(`statistik` funktioniert immer). Fällt ein Guard, passiert gar nichts — der Klassifikator
wird nicht einmal aufgerufen.

Unabhängig davon läuft der **Wochenreport** (CronJob, Do 21:00): Rangliste plus
Strafen-Block in die Gruppe, im Admin-UI als Dry-Run mit simuliertem Stichtag testbar.

Statistik und Wochenreport gehen wahlweise als Text oder als **PNG-Bild-Karte** im
Wrapped-Look raus (headless Chromium im `renderer-service/`, Schalter
`STATS_FORMAT` bzw. `weeklyReport.format`, Fallback immer Text — auf Staging steht
beides auf `image`).

## Strafen

Zwei Strafarten: **Fehltage** (automatisch — 5 abgemeldete Donnerstage in Folge = 25 €,
jede weitere Woche +5 €) und **No-Show** (manuell im Admin-UI, Default 50 €). Beträge werden
immer live aus den Anwesenheiten berechnet, persistiert wird nur ein Serien-Marker;
Begleichen oder Löschen setzt den Serienzähler zurück. Die Fachlogik lebt einmal im
shared-Modul (`shared/penalty/`) und wird von Bot, Admin-UI und Wrapped importiert —
Details und Regeln in [`knowledge/strafen.md`](zumba-bot/knowledge/strafen.md).

## Klassifikation

Drei Klassen, ein verbindlicher Kontrakt: **`true`** (Zusage) · **`false`** (Absage) ·
**`invalid`** (Smalltalk, „mal schauen“, Orga). Die Schwierigkeit sind die echten Nachrichten:
Bairisch, Emojis, Insider — *„obmeldn muas i mi heid“*, *„Sogi doch!“*.

**1. Google Gemini — produktiv.** Prompt in `zumba-bot/system-prompt.txt`, gibt exakt eines
der drei Labels zurück. Kein Training, dafür Cloud-Abhängigkeit, Latenz und keine
belastbaren Konfidenzen.

**2. Eigenes ML-Modell — im Shadow-Modus.** Läuft parallel mit, wird nur protokolliert
(Admin-UI `/ml-shadow`); Gemini entscheidet weiterhin allein. Trainiert wird in
`zumba-bot/ml-classifier/` auf 1034 Nachrichten (973 synthetische + 61 verifizierte echte).
Ein Hash über den normalisierten Text — kein Zufall — teilt jede echte Nachricht dauerhaft
in Training, Holdout oder Monitor ein: dieselbe Nachricht landet nie auf beiden Seiten,
Leakage ist strukturell ausgeschlossen. Unverifizierte Gemini-Labels bleiben im
Monitor-Topf — sonst lernte das Modell Geminis Fehler mit.

Jeder Kandidat ist **Featurizer** (Text → Zahlenvektor) + **Head** (Vektor → Klasse). Wie viel
davon aus den eigenen Daten gelernt wird, unterscheidet die drei Ansätze —
`[trainiert]` = aus unseren Daten gelernt, `(frozen)` = vortrainiert, unverändert:

```
A · Feature-Extraktion, klassisch          → tfidf_logreg · tfidf_svm
   Text ─→ [TF-IDF] ─────────────→ sparse Vektor ─→ [LogReg/SVM] ─→ Klasse
            vocabulary + idf                         coef + intercept

B · Linear Probing / frozen Encoder        → minilm_logreg
   Text ─→ (MiniLM-Encoder) ─────→ 384-dim dense ─→ [LogReg] ─────→ Klasse
            117M Param. unberührt                    ~1 200 Param.

C · Full Fine-Tuning                       → minilm_ft
   Text ─→ [MiniLM-Encoder] ────────────────────────→ [Head] ─────→ Klasse
            alle 117M Param. mittrainiert             Linear 384→3
```

Leitmetrik ist die **5-fold-Kreuzvalidierung** über die 1034 Trainingsdaten — der Holdout
ist nach dem neuen Split auf 12 Nachrichten geschrumpft und taugt nur noch als Rauchtest
(drei von vier Kandidaten treffen 12/12):

| Kandidat | Ansatz | CV-Macro-F1 |
|---|---|---|
| **`tfidf_logreg`** ✅ | TF-IDF über Zeichen-n-Gramme (2–4) + logistische Regression | **0.960 ±0.014** |
| `tfidf_svm` | gleiche Features + LinearSVC | 0.963 ±0.008 |
| `minilm_ft` | MiniLM (BERT-Familie) komplett finegetunt | — (kein k-Fold im Skript) |
| `minilm_logreg` | MiniLM **frozen** als Featurizer + LogReg | 0.864 ±0.017 |

Zeichen-n-Gramme gewinnen, weil sie zur Eigenart der Daten passen („obmeldn“ und „abmelden“
teilen viele Gramme), während MiniLM auf Standardsprache vortrainiert ist. `tfidf_svm` liegt
nominell vorn, aber innerhalb der Streuung beider — und LinearSVC liefert keine Konfidenzen,
die für die invalid-Schwelle gebraucht werden. Deployed bleibt deshalb **`tfidf_logreg`**:
**165 KB JSON in pure Go** statt 466 MB PyTorch.

Ganz trauen darf man auch der CV-Zahl nicht: im gesamten echten Korpus existieren nur zwei
Zusagen, und beide liegen im Training — die `true`-Klasse ist real schlicht ungemessen.
Der ehrliche Abnahmetest ist der Shadow-Modus.

📊 **Details:** `zumba-bot/ml-classifier/eval-ui/index.html` im Browser öffnen — Confusion-Matrizen,
Metriken je Klasse, Schwellen-Sweep, alle Fehlklassifikationen im Wortlaut, Glossar.

## Komponenten

| Verzeichnis | Was |
|---|---|
| [`whatsapp-bot/`](zumba-bot/whatsapp-bot/) | Go · Webhook, Klassifikation, DB-Writes, Wochenreport, Strafen-Erkennung |
| [`classifier-service/`](zumba-bot/classifier-service/) | Go · pure-Go-Inferenz des ML-Modells |
| [`ml-classifier/`](zumba-bot/ml-classifier/) | Python · Trainingsdaten, Training, Evaluation, Export |
| [`zumba-admin-ui/`](zumba-bot/zumba-admin-ui/) | Go · Adminoberfläche (templ + HTMX): Anwesenheiten, Sperrtage, Strafen, Bot-Test |
| [`wrapped/`](zumba-bot/wrapped/) | Go · Jahresrückblick `/2026` (25 Slides, auf Staging) |
| [`renderer-service/`](zumba-bot/renderer-service/) | Go · HTML → PNG via headless Chromium (Statistik-Bild-Karte) |
| [`shared/`](zumba-bot/shared/) | Go · gemeinsames Modul: Strafen-Logik, Rangliste-Query, DB-Zugriffe |
| [`deployment/`](zumba-bot/deployment/) | ArgoCD ApplicationSet · Helm · SealedSecrets · system-upgrade-controller |
| [`knowledge/`](zumba-bot/knowledge/) | Fachliche Doku: Domänenmodell, Strafregeln, Deployment |

Jeder Go-Service: `make dev` (Hot-Reload) · `make build` · `make test`.
Details in den jeweiligen READMEs.
