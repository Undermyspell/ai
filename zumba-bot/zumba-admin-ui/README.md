# Zumba Admin UI

Admin-Weboberfläche für Stammtisch-Daten. Sister-App zu `../wrapped/`, teilt sich die `zumba` Postgres-Datenbank.

## Stack
- Go 1.27 + a-h/templ + lib/pq
- HTMX 2.0.10 (vendored unter `assets/static/js/htmx.min.js`) für inline Edits
- Plain CSS mit Custom Properties: dunkler Glas-Look mit blauem Akzent, helles
  Theme per `data-theme` (Tokens oben in `assets/static/css/styles.css`)
- Wenig JavaScript (`assets/static/js/ui.js`): Bottom-Sheet, „Mehr"-Menü,
  Matrix-Spaltenauswahl, Bot-Test-Sprechblase ↔ Webhook-JSON
- Embedded static assets (`go:embed`)

## Lokal starten
```bash
make install   # einmalig: deps + air + templ
DB_HOST=192.168.178.46 DB_PORT=5433 DB_NAME=zumba DB_USER=n8n DB_PASSWORD=<postgres-passwort> make dev
```

Ohne DB-Env-Vars: App läuft mit Mock-Daten (siehe Banner im UI).

App läuft auf http://localhost:8080.

`make dev` startet **nur** das Admin-UI. Die ML-Test-Seite (`/ml-test`) braucht
zusätzlich den `classifier-service` und meldet sonst „Kein classifier-service
konfiguriert" (mit Mock-Daten fällt das nicht auf — dort ist die Seite ohne
Klassifikator freigeschaltet). Für den vollen lokalen Stack — Renderer, Bot,
Admin-UI, wrapped und Klassifikator, alle korrekt verdrahtet:

```bash
make stack     # geht aus jedem Unterverzeichnis, delegiert ans Repo-Root
```

## Build & Deploy zu k3s `rpi5` (Phase 3)
```bash
docker build -t zumba-admin-ui:0.1.0 .
docker save zumba-admin-ui:0.1.0 | sudo k3s ctr images import -
# Tag in deployment/helm-charts/zumba/values.yaml bumpen, committen
# ArgoCD sync ~3min
```

## Konfiguration (Env-Variablen)
| Var | Default (lokal) | Default (in-cluster, via ConfigMap) |
|---|---|---|
| `DB_HOST` | `192.168.178.46` | `zumba-postgres` |
| `DB_PORT` | `5433` | `5432` |
| `DB_USER` | `n8n` | `n8n` |
| `DB_PASSWORD` | *(kein Default)* | (Secret `postgres-secrets`) |
| `DB_NAME` | `zumba` | `zumba` |
| `DB_SSLMODE` | `disable` | `disable` |
| `PORT` | `8080` | `8080` |
| `BOT_URL` | `http://localhost:8080` | `http://zumba-whatsapp-bot:8080` |

Der Auswertungszeitraum ist **keine** Env-Variable mehr: Stammtischjahre stehen in
`public.seasons` (`label`, `start_date`, `end_date`, überlappungsfrei per
EXCLUDE-Constraint). Ohne `?jahr=` gilt das heute laufende Jahr, `?jahr=2025`
öffnet ein Archiv. Abgeschlossene Jahre sind read-only – der Server lehnt
Schreibzugriffe auf sie ab (HTTP 409), unabhängig vom Parameter.

## Seiten und schreibende Operationen

- **Dashboard**: Kennzahlen, „Zuletzt am Stammtisch" (‹ › blättert per `?tag=`),
  Teilnahme-Gauge, laufende Absage-Serien, Saisonverlauf, Rangliste.
- **Donnerstage** (`/days`): Kacheln aller Donnerstage und die Anwesenheits-Matrix
  Mitglieder × Donnerstage (Zelle = umschalten).
- **Bottom-Sheet „Donnerstag bearbeiten"**: `GET /days/{date}?sheet=1` lädt es in
  `#sheet` – von Kacheln, Balken, der Seitenleiste („Letzten Do. eintragen") und
  dem Dashboard aus. Ohne `?sheet` ist `/days/{date}` eine eigene Seite.
- **Mitglieder** (`/members`, Suche oben per `?q=`, ein Treffer springt direkt
  zum Mitglied) und Mitglied-Detail mit Verlauf.
- **An-/Abwesenheit umschalten**: `POST /toggle-absence` antwortet ohne HTML, nur
  mit Toast und dem HTMX-Ereignis `absenceChanged`. Darauf lädt sich `#page`
  (Seiten mit `RefreshURL`) und ein offenes Bottom-Sheet selbst neu – so bleibt
  alles stimmig, egal wo geklickt wurde.
- **Sperrtage** (`/excluded`): Jahreskalender aller Donnerstage, Klick sperrt bzw.
  gibt frei; dazu Feiertags-Vorschläge (Feiertage, die auf einen Donnerstag fallen)
  und Datumseingabe (serverseitig auf Donnerstag validiert).

## Bot-Test-Seite (`/bot-test`)

Drei Spalten: Einstellungen (Szenario, Ausgabe, Versand, Stichtag), Chat-Simulation
und Verarbeitungspfad. Das Formular proxyt serverseitig an den `whatsapp-bot`: Die
Szenarien Statistik/Absage/Zusage gehen an `BOT_URL/test`, das Szenario Wochenreport
an `BOT_URL/weekly-report`. Die eingehende Nachricht steht als editierbare
Sprechblase im Chat; `ui.js` spiegelt sie in `message.conversation` des
Webhook-JSON (das JSON ist das abgeschickte Feld und hat Vorrang). Die Antwort des
Bots erscheint als Bot-Blase, den Verarbeitungspfad schickt der Server per
`hx-swap-oob` mit. Nicht zutreffende Optionen blendet die Seite per CSS aus (`:has()`).

Der Bot **umgeht** dabei die Donnerstag-/Gruppen-Guards, läuft aber immer als Dry-Run —
DB-Writes und Gruppen-Versand passieren nie; der Modus „Vorschau an meine Nummer“
schickt die erzeugte Nachricht zusätzlich an `PREVIEW_JID`. Die Ausgabe ist wählbar:
„💬 Nachricht“ (Text, inkl. alternativer Statistik-Designs) oder „🖼️ Bild“ (PNG-Karte
über den renderer-service, mit Design-Auswahl). Setzt einen laufenden, erreichbaren
`whatsapp-bot` voraus (`BOT_URL`); für Absage/Zusage braucht der Bot einen
`GEMINI_API_KEY`, fürs Bild `RENDERER_URL`.
