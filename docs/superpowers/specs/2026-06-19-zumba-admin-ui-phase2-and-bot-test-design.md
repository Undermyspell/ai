# zumba-admin-ui: Phase 2 (CRUD) + Bot-Test-Seite — Design

Datum: 2026-06-19

## Kontext & Problem

`n8n/zumba-admin-ui/` ist die Admin-Oberfläche für die Stammtisch-Daten (Anwesenheits-Modell:
„anwesend, außer es gibt eine Absage"). **Phase 1 (nur lesend)** ist fertig: Dashboard/Leaderboard,
Mitglieder-Liste + -Detail, Tage-Liste + -Detail, Excluded-Days-Liste, Thursday-Strip, Dark/Light,
mobile-first, Mock-Fallback. Anforderungen: `n8n/admin-web-ui.md`.

Es fehlt **Phase 2 (schreibend)**: Korrekturen an An-/Abwesenheiten und Verwaltung der
Excluded-Days. Zusätzlich soll eine **Bot-Test-Seite** entstehen — eine UI für die curl-Requests
gegen den `whatsapp-bot` (Funktionen Zusage / Absage / Statistik), die das Beispiel-JSON editierbar
anzeigt und die Antwort darstellt. Außerdem **Phase 3 (Deployment)** als Dateien (Helm/IngressRoute),
ohne Cluster-Apply.

## Entscheidungen (mit User abgestimmt)

- **Bot-Test erreicht den Bot über einen serverseitigen Proxy** im admin-ui auf einen neuen
  `/test`-Endpunkt des Bots (Logik bleibt im Bot, kein CORS, Bot-URL serverseitig).
- **Bot-Test führt echt aus** (DB-Write bei Absage/Zusage, Versand via Sink bei Statistik).
- **Bot-Test umgeht** die Donnerstag-/Gruppen-Guards (jederzeit testbar).
- **Scope:** Phase 2 CRUD + Bot-Test-Seite + Phase-3-Deployment-Dateien (kein Cluster-Apply).
- UI/UX mit dem `frontend-design`-Skill umsetzen (mobile-first, Dark/Light, bestehender Stil).

## Domänen-/Datenmodell (Bestand)

Postgres-DB `zumba`, Schema `public` (User n8n). Relevant:
- `stammtisch_abwesenheit (userId, date, message)` — eine Zeile je Absage; nur Donnerstage zählen,
  Unique-Constraint auf (`userId`, `date`) vorausgesetzt (vom n8n-UPSERT genutzt).
- `excluded_days (date)` — Donnerstage, die nicht zählen.
- `users (userId, userName, startDate)`.

---

## Teil A — whatsapp-bot: Test-Endpunkt

**Refactor `internal/web/server.go`:** aus `process(ctx, ev)` wird

```go
type Outcome struct {
    Path           string `json:"path"`           // "statistik" | "classify" | "ignored"
    Classification string `json:"classification"` // "true"|"false"|"invalid" (nur classify)
    Action         string `json:"action"`         // "marked_absent"|"marked_present"|"none"
    Message        string `json:"message"`        // generierter Statistik-Text bzw. Eingabe-Text
    Recipient      string `json:"recipient"`       // remoteJid
    Date           string `json:"date"`            // YYYY-MM-DD (bei classify-Write)
    UserID         string `json:"userId"`
    Reason         string `json:"reason"`          // wenn ignored: warum (nur ohne Bypass)
}

func (s *Server) run(ctx context.Context, ev evolution.WebhookEvent, bypassGuards bool) Outcome
```

`run` enthält die bisherige Logik, liefert aber ein `Outcome` und akzeptiert `bypassGuards`:
- `bypassGuards == false` (Webhook): Guard `messageType==conversation && remoteJid==group && Donnerstag`
  wird geprüft; bei Nichterfüllung `Path="ignored"`, `Reason` gesetzt.
- `bypassGuards == true` (Test): Guard wird übersprungen; bei Nicht-`statistik`-Nachricht wird immer
  klassifiziert und die DB-Aktion ausgeführt.

**Routen** (`Routes()`):
- `POST /webhook/whatsapp` — unverändertes Verhalten: `run(ev, false)`, Rückgabe `200` ohne Body.
- `POST /test` (neu) — `run(ev, true)`, Rückgabe `200` mit `Outcome` als JSON. Bei ungültigem
  JSON-Body `400`.
- `GET /healthz` — unverändert.

Der Statistik-Pfad sendet weiterhin über den konfigurierten Sender (stdout/file/evolution) **und**
gibt den Text im `Outcome.Message` zurück (für die UI). Klassifizierung schreibt via
`store.MarkAbsent`/`MarkPresent` für `today` (TZ-Datum) — Action entsprechend gesetzt.

**Tests:** `run(ev, true)` für statistik (Text im Outcome, kein Guard nötig) und classify
(Absage→`marked_absent`, Zusage→`marked_present`, invalid→`none`) mit Fake-Store/Classifier/Sender;
`run(ev, false)` weiterhin durch die bestehenden Guard-Tests abgedeckt (auf `Outcome.Path` umgestellt).

---

## Teil B — admin-ui: Phase 2 (schreibende CRUD)

### Store

`internal/store/store.go` Interface erweitern; Mock + Postgres implementieren:

```go
InsertAbsence(ctx, userID string, date time.Time, message *string) error // UPSERT (userId,date)
DeleteAbsence(ctx, userID string, date time.Time) error
InsertExcludedDay(ctx, date time.Time) error                              // ON CONFLICT DO NOTHING
DeleteExcludedDay(ctx, date time.Time) error
```

Postgres-SQL analog zum bot (`stats.sql`/UPSERT): `INSERT … ON CONFLICT ("userId", date) DO UPDATE
SET message`, `DELETE … WHERE "userId"=$1 AND date=$2`, `INSERT INTO excluded_days(date) VALUES($1)
ON CONFLICT (date) DO NOTHING`, `DELETE FROM excluded_days WHERE date=$1`. Mock mutiert seine
In-Memory-Slices (damit die UI auch im Mock-Modus reagiert).

### Validierung

- Absence + Excluded-Day nur für **Donnerstage** (`timeutil.IsThursday`) — sonst `422` + Fehler-Toast.
- Datum via `timeutil.ParseISO`. Bei DB-Fehler `500` + Fehler-Toast (+ Log).

### Routen (HTMX) + Handler

Ein wiederverwendbares **Toggle-Control** (Partial) für An-/Abwesenheit, genutzt von Tagesdetail
und Mitgliederdetail:

- `POST /toggle-absence` (Form: `userId`, `date`) — flippt Zustand (anwesend↔abwesend): existiert eine
  Absage → `DeleteAbsence` (→ anwesend); sonst → `InsertAbsence` (→ abwesend). Antwort: neu
  gerendertes Toggle-Control-Partial (gleiche Markup-Komponente) + Toast.
- `POST /excluded` (Form: `date`) — Donnerstag-validiert, `InsertExcludedDay`. Antwort: aktualisierte
  Excluded-Liste (bzw. neue Zeile) + Toast.
- `DELETE /excluded/{date}` — `DeleteExcludedDay`. Antwort: Zeile entfernen + Toast.

### Templates

- Neues Partial `partials/absence_toggle.templ` (Button mit `hx-post="/toggle-absence"`, Zustand
  als CSS-Klasse/Icon), eingebaut in `days/detail.templ` (pro User) und `members/detail.templ`
  (pro Donnerstag).
- `excluded/list.templ`: Formular (Datumsauswahl, nur Do) + Löschen-Button je Zeile (`hx-delete`).
- **Toast verdrahten:** Handler setzen `HX-Trigger`-Header (`{"showToast":{"msg":…,"level":…}}`);
  kleines JS in `assets/static/js/` hört darauf und hängt `partials/toast.templ`-Markup an
  `#toast-stack` (Auto-Dismiss). Damit wird die vorhandene, bislang ungenutzte Toast-Komponente aktiv.

---

## Teil C — admin-ui: Bot-Test-Seite

### Config

`internal/config/config.go`: neues `BOT_URL` (Default `http://localhost:8080`).

### Beispiel-Requests

Drei JSON-Fixtures in admin-ui via `go:embed` (`web/bottest/examples/{zusage,absage,statistik}.json`),
inhaltlich identisch zu `whatsapp-bot/reference/example-requests/`.

### Routen + Handler

- `GET /bot-test` — Seite: 3er-Auswahl (Zusage/Absage/Statistik), editierbares `<textarea id="bot-json">`,
  „Senden"-Button (`hx-post="/bot-test/run"`, `hx-target="#bot-response"`), Antwort-Panel `#bot-response`.
- `GET /bot-test/example/{kind}` (`kind ∈ zusage|absage|statistik`) — liefert das Beispiel-JSON als
  Text; `hx-target="#bot-json"` füllt das Textarea. Beim Laden der Seite ist „Statistik" vorausgewählt.
- `POST /bot-test/run` (Form: `payload`) — serverseitiger Proxy: `POST {BOT_URL}/test` mit dem
  (ggf. editierten) JSON. Antwort-`Outcome` wird als Partial gerendert:
  - `path=statistik` → WhatsApp-artige Bubble mit `Message` (monospace/preserve-newlines).
  - `path=classify` → Badges: Klassifizierung (`true/false/invalid`) + Aktion (`marked_absent/…`) + userId/date.
  - Fehler (Bot nicht erreichbar / `4xx`/`5xx`) → Fehler-Panel mit Statuscode/Text.

### Nav

Eintrag „Bot-Test" in `partials/nav.templ`.

---

## Teil D — Deployment (Phase 3, nur Dateien, kein Apply)

Analog zu `whatsapp-bot` und n8n unter `deployment/helm-charts/zumba/templates/admin-ui/`:
- `deployment.yaml` (initContainer wait-for-postgres, `envFrom` ConfigMap, `DB_PASSWORD` aus
  `postgres-secrets/DB_POSTGRESDB_PASSWORD`, Liveness/Readiness `/healthz`),
- `service.yaml` (ClusterIP 8080), `configmap.yaml` (DB→`zumba`, `BOT_URL=http://<fullname>-whatsapp-bot:8080`,
  `EVAL_PERIOD_*`, `TZ`), `ingress-route.yaml` (eigener Host, z.B. `zumba-admin-stage.pi.home` /
  prod-Pendant).
- `_helpers.tpl`: `zumba.adminUi.labels/selectorLabels`. `values.yaml`: `adminUi`-Block
  (`enabled: false`, image-Platzhalter, resources, env, service, ingress host).
- Validierung lokal via `helm template`. **Kein** `kubectl apply`, keine echten Secrets.

---

## Daten- & Kontrollfluss

1. **CRUD:** Browser (HTMX) → admin-ui-Handler → Store (Postgres/Mock) → Partial + `HX-Trigger`-Toast.
2. **Bot-Test:** Browser → admin-ui `/bot-test/run` → (Proxy) Bot `/test` → `run(ev, bypass=true)`
   → Store/Sender im Bot → `Outcome`-JSON → admin-ui rendert Partial → Browser.

## Fehlerbehandlung

- Ungültiges Datum / Nicht-Donnerstag → `422` + Fehler-Toast, keine DB-Aktion.
- DB-Fehler → `500` + Fehler-Toast + Log; UI bleibt konsistent (kein optimistisches Update ohne Erfolg).
- Bot nicht erreichbar / Fehlerstatus → Fehler-Panel auf der Bot-Test-Seite (Status + Text).
- Mock-Modus: Writes mutieren In-Memory; Banner weist (wie bisher) auf Mock hin.

## Tests

- **bot:** `run(ev, true)`-Tests (statistik/classify/invalid) mit Fakes; bestehende Guard-Tests auf
  `Outcome` umgestellt.
- **admin-ui store:** Mock-Writes (Insert/Delete Absence + Excluded) verändern Lese-Ergebnisse.
- **admin-ui handlers:** Toggle (insert↔delete), Excluded add/delete inkl. Nicht-Donnerstag→422,
  jeweils mit Fake-Store; `/bot-test/run` gegen einen `httptest`-Fake-Bot, der ein `Outcome` liefert
  (prüft Proxy + Rendering + Fehlerpfad).

## Out of Scope

- Authentifizierung (in den Anforderungen nicht gefordert).
- Schedule-Trigger des Bots (bewusst nicht portiert).
- Tatsächliches Cluster-Deployment / SealedSecrets-Erzeugung (nur Dateien).
