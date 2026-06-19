# Zumba Admin UI

Admin-Weboberfläche für Stammtisch-Daten. Sister-App zu `../wrapped/`, teilt sich die `zumba` Postgres-Datenbank.

## Stack
- Go 1.25 + a-h/templ + lib/pq
- HTMX (vendored, Phase 2) für inline Edits
- Plain CSS mit Custom Properties (light/dark Theme)
- Embedded static assets (`go:embed`)

## Lokal starten
```bash
make install   # einmalig: deps + air + templ
DB_HOST=192.168.178.46 DB_PORT=5433 DB_NAME=zumba DB_USER=n8n DB_PASSWORD=n8n_password make dev
```

Ohne DB-Env-Vars: App läuft mit Mock-Daten (siehe Banner im UI).

App läuft auf http://localhost:8080.

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
| `DB_PASSWORD` | `n8n_password` | (Secret `postgres-secrets`) |
| `DB_NAME` | `zumba` | `zumba` |
| `DB_SSLMODE` | `disable` | `disable` |
| `PORT` | `8080` | `8080` |
| `EVAL_PERIOD_START` | `2025-12-01` | `2025-12-01` |
| `EVAL_PERIOD_END` | `2026-11-30` | `2026-11-30` |
| `BOT_URL` | `http://localhost:8080` | `http://zumba-whatsapp-bot:8080` |

## Phase 2: schreibende Operationen

- **An-/Abwesenheit umschalten** (Tages- und Mitgliederdetail): Klick auf den Toggle pro
  Donnerstag legt eine Absage an bzw. löscht sie (HTMX, Toast-Feedback).
- **Sperrtage verwalten** (`/excluded`): Donnerstag anlegen (serverseitig validiert) oder löschen.

## Bot-Test-Seite (`/bot-test`)

UI für die Webhook-Requests gegen den `whatsapp-bot`. Funktion wählen (Statistik / Absage /
Zusage) → Beispiel-JSON erscheint editierbar → „An Bot senden" proxyt serverseitig an
`BOT_URL/test` und zeigt die Antwort (Ranglisten-Bubble bzw. Klassifizierung/Aktion).
Der Bot führt den Test **echt** aus (DB-Write bzw. Versand) und **umgeht** die
Donnerstag-/Gruppen-Guards. Setzt einen laufenden, erreichbaren `whatsapp-bot` voraus
(`BOT_URL`); für Absage/Zusage braucht der Bot einen `GEMINI_API_KEY`.
