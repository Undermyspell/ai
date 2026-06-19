# Zumba WhatsApp-Bot

Go-Service, der den n8n-Workflow **„Zumba"** (`HG0zPlWsmPI3Mt7z`) ablöst. Er empfängt
WhatsApp-Nachrichten über die Evolution API, klassifiziert Zu-/Absagen per Google Gemini,
schreibt sie nach Postgres und beantwortet `statistik`-Anfragen mit der Rangliste.

Konventionen wie die Schwester-Services `wrapped/` und `zumba-admin-ui/`: vanilla
`net/http`, `lib/pq`, `godotenv`, Air für Hot-Reload, Multi-Stage-Docker-Build.

## Verhalten (1:1 aus dem n8n-Workflow)

`POST /webhook/whatsapp` empfängt ein Evolution-`messages.upsert`-Event:

- **`message == "statistik"`** (getrimmt, case-insensitive) → Per-User-Stats aus Postgres
  (`internal/store/stats.sql`) → Ranglisten-Text (`internal/report`) → zurück an den Absender
  (Evolution `sendText`).
- **sonst**, wenn **alle** gelten: `messageType == "conversation"`, `remoteJid == ZUMBA_GROUP_JID`,
  **heute ist Donnerstag** (TZ `Europe/Berlin`):
  - Gemini-Classifier → `true` / `false` / `invalid`
  - `false` (Absage) → UPSERT in `stammtisch_abwesenheit (userId, date=heute, message)`
  - `true` (Zusage) → DELETE der heutigen Zeile
  - `invalid` → keine Aktion
- sonst: keine Aktion. Antwort ist immer `200 OK`.

`GET /healthz` → `200 ok` (Liveness/Readiness).

### Bekannte 1:1-Eigenheit
Der Original-Code zeigte das Startdatum `(d.m.)` nie an (er las `start_date`, die Spalte heißt
`startDate`). Das ist über `showStartDate` in `internal/report/report.go` nachgebildet und
standardmäßig aus. Auf `true` setzen, um die ursprünglich beabsichtigte Anzeige zu aktivieren.

## Commands

```bash
make dev    # Hot-Reload via Air
make build  # Build nach ./tmp/server
make run    # build + run
make test   # go test -v ./...
```

## Konfiguration (Env / `.env`)

Siehe `.env.example`. Wichtig:

| Variable | Bedeutung |
|---|---|
| `DB_*` | Postgres (Domänendaten in DB `zumba`, User `n8n`) |
| `GEMINI_API_KEY` | Google-AI-Studio-Key für den Classifier |
| `GEMINI_MODEL` / `GEMINI_FALLBACK_MODEL` | `gemini-2.5-flash` (primär) / `gemini-3-flash-preview` (Fallback) |
| `OUTPUT_MODE` | Ziel ausgehender Nachrichten: `evolution` (default) / `stdout` / `file` |
| `OUTPUT_FILE` | Pfad bei `OUTPUT_MODE=file` (default `output.txt`) |
| `EVOLUTION_URL` / `EVOLUTION_API_KEY` / `EVOLUTION_INSTANCE` | Evolution-API-Endpunkt, `apikey`, Instanzname (`whatsapp`) – nur bei `OUTPUT_MODE=evolution` |
| `ZUMBA_GROUP_JID` | remoteJid der Zumba-Gruppe |
| `TZ` | Zeitzone für Donnerstag-Prüfung + Tagesdatum |

Lokales Testen (Statistik ohne Evolution, Beispiel-Requests): siehe **`TESTING.md`**.

Anders als die Schwester-Services gibt es **keinen Mock-Fallback**: ohne erreichbare DB
beendet sich der Service (ein Bot ohne DB ist sinnlos).

## Deployment

Helm-Templates unter `../deployment/helm-charts/zumba/templates/whatsapp-bot/` (hinter
`whatsappBot.enabled`). Cut-over: Webhook der Evolution-Instanz auf
`http://<release>-whatsapp-bot:8080/webhook/whatsapp` setzen, dann den n8n-Workflow deaktivieren.

## Referenz

`reference/zumba-workflow.json` — Export des abgelösten n8n-Workflows (Rollback/Vergleich).
