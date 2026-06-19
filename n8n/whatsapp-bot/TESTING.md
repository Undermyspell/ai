# Lokales Testen

## 0. Unit-/Integrationstests (kein Setup nötig)

```bash
make test
```

Enthält u.a. `TestWebhookStatistikEndToEnd`: schickt
`reference/example-requests/statistik.json` durch den echten HTTP-Handler und prüft den
gerenderten Ranglisten-Text – komplett ohne DB, Gemini oder Evolution.

## 1. Statistik-Request lokal – ohne Evolution API

Der `statistik`-Pfad ist **read-only** (nur ein SELECT, keine DB-Writes) und braucht weder
Gemini noch die Evolution API. Das Ergebnis geht je nach `OUTPUT_MODE` nach stdout oder in eine Datei.

**Setup** (`.env` aus `.env.example`):

```bash
cp .env.example .env
```

Dann in `.env`:

```ini
OUTPUT_MODE=stdout          # oder: file  (dann landet es in OUTPUT_FILE)
# Statistik liest aus Postgres – z.B. die staging-DB (extern erreichbar, read-only-Pfad):
DB_HOST=192.168.178.46
DB_PORT=5433
DB_NAME=zumba
DB_USER=n8n
DB_PASSWORD=<staging-postgres-passwort>
```

> Alternativ gegen eine lokale Postgres mit Schema/Daten testen (siehe `../docker-compose.yml`).
> Gemini-/Evolution-Variablen können für den Statistik-Test leer bleiben.

**Starten:**

```bash
make run
# Log: "📤 Output-Modus: stdout"  und  "✅ Connected to PostgreSQL 'zumba' ..."
```

**Request abschicken** (zweites Terminal) – exakt das Event-Format, das auch von der Evolution
API kommt:

```bash
curl -X POST http://localhost:8080/webhook/whatsapp \
  -H 'Content-Type: application/json' \
  --data @reference/example-requests/statistik.json
```

Im Server-Log erscheint dann der gerenderte Text:

```
===== WhatsApp → 000000000000-0000000000@g.us =====
🍻 *ZUMBA STATS*
_Weihnachtsfeier → Weihnachtsfeier_
...
===== (Ende) =====
```

Bei `OUTPUT_MODE=file` steht dasselbe in `OUTPUT_FILE` (Default `output.txt`).

## 2. Ergebnis an Evolution (staging) schicken statt stdout

Um die Nachricht real per WhatsApp rauszuschicken, in `.env`:

```ini
OUTPUT_MODE=evolution
EVOLUTION_URL=http://evolution-stage.pi.home
EVOLUTION_API_KEY=<staging AUTHENTICATION_API_KEY>
EVOLUTION_INSTANCE=whatsapp
```

> ⚠️ Das verschickt eine **echte** WhatsApp-Nachricht an den `remoteJid` aus dem Request.
> Zum gefahrlosen Test den `remoteJid` in einer Kopie der JSON auf eine eigene Nummer
> (`<nummer>@s.whatsapp.net`) ändern.

## 3. Absage/Zusage-Pfad (Classifier)

`reference/example-requests/absage.json` bzw. `zusage.json`. **Achtung:** dieser Pfad

- feuert nur, wenn **heute Donnerstag** ist (TZ `Europe/Berlin`) **und** `remoteJid == ZUMBA_GROUP_JID`,
- braucht `GEMINI_API_KEY`,
- **schreibt** in `stammtisch_abwesenheit` (UPSERT bzw. DELETE für das heutige Datum).

Daher am besten gegen eine lokale Wegwerf-DB testen, nicht gegen staging. (Falls ein Test an
einem Nicht-Donnerstag gebraucht wird, kann ich eine Test-Override-Env für den Wochentag ergänzen.)
