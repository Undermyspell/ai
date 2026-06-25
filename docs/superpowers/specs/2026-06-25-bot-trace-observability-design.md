# Bot-Trace-Observability — Design

**Datum:** 2026-06-25
**Status:** Design abgenommen, bereit für Implementierungsplan

## Problem / Motivation

An n8n geschätzt: die **Trigger-Historie** (welche WhatsApp-Nachrichten reinkamen) und der **visuelle Editor**, in dem man sah, **wo der Workflow bei welchen IFs abgebogen ist**. Seit der Ablösung durch den Go-`whatsapp-bot` fehlt das. Ziel: nachvollziehen können, **was mit einer eingehenden Nachricht passiert ist** — speziell, falls eine Nachricht **nicht korrekt verarbeitet** wurde (z.B. eine Absage, die nicht eingetragen wurde).

## Scope & Entscheidungen (mit User abgestimmt)

- **Detailtiefe:** Schritt-für-Schritt-Trace jedes Entscheidungspunkts (nicht nur das Endergebnis).
- **Aufzeichnungs-Umfang:** **nur Nachrichten in der Zumba-Gruppe** (`remoteJid == ZUMBA_GROUP_JID`) **und nur donnerstags** (`isThursday()` in `Europe/Berlin`). Andere Tage / fremde Chats werden nicht gespeichert. Das ist exakt das Event-Set, das die Absage/Zusage-Logik betrifft.
- **Retention:** nur die **letzten 3 Wochen** (21 Tage).
- **Speicher:** die **vorhandene `zumba`-Postgres** (kein neuer Datastore).
- **Darstellung Detailansicht:** **visueller Flow-Graph wie n8n** — und er soll **visuell schön** sein (explizites Designziel, nicht generischer Default-Look).
- **Auto-Refresh:** **nein** — stattdessen ein manueller „Aktualisieren"-Button in der Liste (kein Dauer-Polling).
- **Tests:** keine neuen automatisierten Tests (stehende User-Präferenz); Verifikation manuell. Bestehende `report`/`web`-Tests bleiben grün.

## Architektur & Datenfluss

**Erzeuger = Bot, Leser = Admin-UI** (beide hängen bereits an der `zumba`-DB).

```
Evolution → POST /webhook/whatsapp (Bot)
   │  Request-Bytes puffern (für Roh-Payload), dann decoden
   │  shouldRecord = remoteJid == ZUMBA_GROUP_JID && isThursday()
   │  run() führt aus UND befüllt einen *trace.Recorder
   │  wenn shouldRecord: tracestore.Save(trace)  (best-effort)
   ▼
zumba-DB: Tabelle bot_trace  ◄── Admin-UI liest: GET /trace, GET /trace/{id}
```

- **Best-effort & nicht-blockierend:** Schlägt der Trace-Insert fehl, nur Warn-Log; der Webhook antwortet trotzdem `200`. Aufzeichnung synchron nach `run()` (Volumen winzig; kein Trace-Verlust bei Crash).
- **Konsequenz des Filters:** In jedem gespeicherten Event sind die Knoten „Gruppe?" und „Donnerstag?" immer ✓ (nur Bestätigung). Reale Verzweigungen im aufgezeichneten Set: *„statistik"? · messageType==conversation? · Classifier (false/true/invalid)*.

## Datenmodell

Neue Tabelle in `zumba` (schema `public`); der Bot legt sie idempotent beim Start an (kein Migrations-Framework im Repo):

```sql
CREATE TABLE IF NOT EXISTS bot_trace (
  id             BIGSERIAL PRIMARY KEY,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  remote_jid     TEXT,
  user_id        TEXT,
  user_name      TEXT,
  message        TEXT,              -- roher Nachrichtentext
  message_type   TEXT,
  path           TEXT,              -- statistik | classify | ignored
  classification TEXT,              -- true | false | invalid | ''
  action         TEXT,              -- marked_absent | marked_present | none | would_*
  has_error      BOOLEAN NOT NULL DEFAULT false,
  raw_payload    JSONB,             -- komplettes Evolution-Event (exakte Request-Bytes)
  trace          JSONB NOT NULL     -- geordnete Schritt-Liste
);
CREATE INDEX IF NOT EXISTS bot_trace_created_idx ON bot_trace (created_at DESC);
```

Ein Schritt im `trace`-Array:

```json
{ "node": "guard_thursday", "outcome": "pass", "label": "Donnerstag?", "detail": "Do, 2026-06-25" }
```

- `node` — fester Bezeichner, die UI mappt ihn auf den Graphen (s.u.).
- `outcome ∈ {pass, fail, info, error}` — steuert die Farbcodierung.
- `detail` — menschenlesbarer Kontext, z.B. beim `classify`-Knoten die **Gemini-Roh-Antwort + Modell** (`"\"false\" · gemini-2.5-flash"`), beim `send_stats`/`mark_absent`-Knoten Empfänger bzw. DB-Ergebnis oder Fehlertext.

## Fester Graph (Knoten-Topologie)

| node | Label | Typ/Icon | Bedeutung |
|---|---|---|---|
| `received` | Webhook empfangen | 📨 | immer; zeigt von / pushName / messageType |
| `check_statistik` | „statistik"? | ❓ | Verzweigung: ja → statistik-Pfad, nein → runter |
| `build_stats` | Statistik berechnen | 📊 | statistik-Pfad |
| `send_stats` | An Gruppe senden | 📤 | statistik-Pfad (Empfänger + Erfolg/Fehler) |
| `guard_type` | messageType == conversation? | 🛡️ | nein → `ignored` |
| `guard_group` | Gruppe? | 🛡️ | im aufgezeichneten Set immer ✓ |
| `guard_thursday` | Donnerstag? | 🛡️ | im aufgezeichneten Set immer ✓ |
| `classify` | Classifier (Gemini) | 🤖 | Roh-Antwort + Modell; fan-out false/true/invalid |
| `mark_absent` | Absage: DB-Insert | 📝 | classification == false |
| `mark_present` | Zusage: DB-Delete | 📝 | classification == true |
| `no_action` | keine Aktion | ➖ | classification == invalid |
| `ignored` | Ignoriert | 🚫 | guard_type fehlgeschlagen |

Layout (top-down, statistik-Zweig rechts):

```
              [received]
                  │
            [check_statistik]
        ja ───┘            └─── nein
   [build_stats]      [guard_type]
        │          ja ┘          └ nein
   [send_stats]   [guard_group ✓]   [ignored]
                  [guard_thursday ✓]
                       │
                   [classify]
         false ──┬──── true ────┬── invalid
        [mark_absent]   [mark_present]   [no_action]
```

## UI (Admin-UI)

- **Nav:** neuer Eintrag `{Key:"trace", Href:"/trace", Icon:"📜", Label:"Verlauf"}`.
- **`GET /trace`** — Liste der letzten 21 Tage, neueste zuerst. Tabelle: Zeit · User · Nachricht (gekürzt) · Pfad-Badge · Ergebnis-Badge · ⚠ bei Fehler. Zeile verlinkt zum Detail. Manueller „Aktualisieren"-Button (HTMX `hx-get` auf die Liste).
- **`GET /trace/{id}`** — Detail: der **Flow-Graph** + Kopf mit Metadaten + aufklappbares Roh-Payload-`<pre>`.
- **Store:** `ListTraces(ctx, limit)` und `GetTrace(ctx, id)` in `postgres.go` + `mock.go` (Mock liefert Beispieldaten).
- **Templates:** `web/templates/trace/` (`list.templ`, `detail.templ`, `graph.templ`, ggf. `embed.go`).

### Rendering & visueller Anspruch (Flow-Graph)

- **Technik:** statisches SVG mit **`foreignObject`-Knotenkarten** (sauberer Textumbruch im Karten-Look) + **geschwungene Bézier-Verbindungen mit Pfeilspitzen**. Feste Koordinaten — kein Layout-Algorithmus, kein JS-Graph-Lib.
- Aus dem Trace wird eine `map[node]→Step` gebaut; ein Helper setzt pro Karte/Kante die CSS-Klasse **`taken` / `skipped` / `error`** und füllt den `detail`-Text.
- **Zweig-Labels** als kleine Pills an den Kanten („ja/nein", „false/true/invalid").
- **Zustands-Farbcodierung** über bestehende CSS-Variablen: genommener Pfad `--accent` (kräftige/leuchtende Kante, volle Deckung), übersprungen ausgegraut (`opacity`), Fehler `--danger` (roter Rahmen).
- **Politur:** dezente Schatten, sanfte Eingangs-Animation (bestehende `.enter`-Klasse), Whitespace, sauberes Raster — konsistent mit dem bestehenden Designsystem.
- Umsetzung mit dem **frontend-design**-Skill, damit die Optik wirklich poliert wird.

## Bot-Instrumentierung

- **`internal/classifier`:** `Classify` gibt künftig zusätzlich die **Gemini-Roh-Antwort** und das **tatsächlich genutzte Modell** (Primär vs. Fallback) zurück (z.B. `Classification{Result, Raw, Model}`). Interface `web.Classifier` + Fake in den Tests ziehen nach.
- **`internal/web/server.go`:** `run()` befüllt einen `*trace.Recorder` (pro Entscheidungspunkt `recorder.Step(node, outcome, label, detail)`). Im `/test`-Pfad ein Wegwerf-Recorder (nicht persistiert); im Webhook-Pfad bei `shouldRecord` persistiert. `Server` erhält eine optionale `tracer`-Abhängigkeit (nil-sicher → ohne DB kein Recording).
- **`internal/tracestore` (neu):** `EnsureSchema(ctx)` (CREATE TABLE IF NOT EXISTS beim Start, in `cmd/server/main.go` aufgerufen), `Save(ctx, Trace)`. Roh-Payload = exakte Request-Bytes (im Handler vor dem Decode gepuffert via `io.ReadAll` + `bytes.NewReader`).
- **Retention:** in `Save` zusätzlich `DELETE FROM bot_trace WHERE created_at < now() - interval '21 days'`. Winziges Volumen → kein CronJob.

## Fehlerbehandlung

- Trace-Aufzeichnung best-effort: DB-Fehler → Warn-Log, Webhook bleibt `200`.
- Classifier-Fehler → roter `classify`-Knoten (`outcome: error`, `detail`: Fehlertext); `has_error = true`.
- Roh-Payload nicht marshallbar → `raw_payload = NULL`, Trace bleibt erhalten.

## Deployment

- Keine neuen Secrets/Config (Bot hat DB-Zugang + `ZUMBA_GROUP_JID` bereits; Admin-UI hat DB-Zugang).
- Images → **0.1.3** (Bot + Admin-UI), auf dem Pi nativ bauen (`pi@192.168.178.46`, `docker build`), via `docker save | sudo k3s ctr images import -` importieren, Tags in `environments/staging/values.yaml` bumpen, commit + push → ArgoCD synct.

## Verifikation (manuell, keine neuen Tests)

1. Lokal Bot bauen + mit `.env` starten.
2. Beispiel-Payloads (jeweils Gruppe + Donnerstag) an `POST /webhook/whatsapp`: `statistik`, eine Absage, eine Zusage, ein Nicht-`conversation`-Event → prüfen, dass je eine `bot_trace`-Zeile mit korrektem Pfad/Trace entsteht.
3. Admin-UI lokal: Liste zeigt die Events, Detailseite rendert den Graphen mit korrekt eingefärbtem Pfad und Annotationen (inkl. Gemini-Roh-Antwort).
4. Retention: Zeile mit `created_at` > 21 Tage künstlich einfügen → nach nächstem `Save` verschwunden.
5. Nach Deployment: Liste in staging füllt sich; am Donnerstag echtes Event prüfen.
6. Bestehende Tests (`report`, `web`) bleiben grün.

## Offene Punkte / bewusst ausgeklammert (YAGNI)

- Kein Auto-Refresh/Polling (manueller Button reicht).
- Keine Aufzeichnung von Nicht-Gruppe / Nicht-Donnerstag (inkl. on-demand `statistik` aus Privatchats) — bewusst außerhalb des Scopes.
- Keine Volltextsuche/Filter über Traces in v1 (kann später kommen).
