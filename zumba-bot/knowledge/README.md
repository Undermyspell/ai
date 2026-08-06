# Stammtisch-System — fachlicher Überblick

Digitale Verwaltung eines wöchentlichen Stammtischs (donnerstags). Die Gruppe
organisiert sich über WhatsApp; das System liest Zu-/Absagen mit, führt
Statistik, verhängt Strafen und produziert einen Jahresrückblick im
Spotify-Wrapped-Stil.

## Teilbereiche

| Doku | Bereich | Kurzbeschreibung |
|---|---|---|
| [whatsapp-bot.md](whatsapp-bot.md) | `whatsapp-bot/` | Liest die WhatsApp-Gruppe, klassifiziert Absagen per LLM, beantwortet Statistik-Anfragen, verschickt den Wochenreport |
| [admin-ui.md](admin-ui.md) | `zumba-admin-ui/` | Pflege-Oberfläche: Anwesenheiten, Sperrtage, Strafen, Bot-Test |
| [wrapped.md](wrapped.md) | `wrapped/` | Jahresrückblick als Slide-Show (25 Slides, Stand 08/2026) |
| [strafen.md](strafen.md) | quer | Strafen-Fachlogik (in drei Services dupliziert!) |
| [deployment.md](deployment.md) | `deployment/` | GitOps auf k3s (Raspberry Pi 5) via ArgoCD |

## Das Domänenmodell (gilt überall)

**Anwesenheit per Default**: Wer nicht explizit absagt, war da. Es gibt keine
"Anwesend"-Tabelle — nur Absagen werden gespeichert. Alle Auswertungen leiten
Anwesenheit aus dem Fehlen einer Absage ab.

**Nur Donnerstage zählen** (`EXTRACT(DOW) = 4` / ISODOW 4). Sperrtage
(`excluded_days`, z. B. Feiertage) werden überall herausgefiltert.

**Zeitrechnung**: Auswertungszeitraum "Wrapped 2026" = 01.12.2025–30.11.2026.
Startdaten von Mitgliedern werden auf frühestens 01.12.2025 geklemmt
(`ClampStart`). Zukunft zählt nie mit — Enddatum wird auf "heute" gekappt.

### Tabellen (Postgres, DB `zumba`, Schema `public`)

- `users` — `userId` (WhatsApp-JID, Format `<nummer>@s.whatsapp.net`), `userName`,
  `startDate` (nullable). 15 Mitglieder (Stand 08/2026).
- `stammtisch_abwesenheit` — eine Zeile pro Absage: `userId`, `date`,
  `message` (nullable — viele Absagen kommen ohne Text), `created_at`
  (TIMESTAMPTZ, seit 08/2026; Altbestand NULL. Für Wrapped 2027:
  "kurzfristigste Absage"). PK (`userId`, `date`).
- `excluded_days` — Donnerstage, die nicht zählen.
- `strafen` — siehe [strafen.md](strafen.md).

Zwei Datenbanken auf einer Postgres-Instanz: `n8n` (n8n-State + Evolution-API
im Schema `evolution`) und `zumba` (Domänendaten).

## Konventionen

- **Alle nutzerseitigen Texte deutsch** (UI, Reports, Logs). Datumsformat
  DD.MM., Woche beginnt Montag.
- **Keep-in-sync-Duplikate**: bewusst kopierte Logik statt Shared Library,
  weil jeder Service ein eigenes Go-Modul ist und Docker-Builds nur das
  Service-Verzeichnis kopieren. Änderungen immer in allen Kopien nachziehen:
  - `internal/penalty/` in whatsapp-bot, zumba-admin-ui **und** wrapped
  - Klassifikator-Prompt + Statistik-SQL zwischen Bot und n8n-Workflow
  - Strafen-DDL (`EnsureStrafenSchema`) in Bot und Admin-UI
- **LLM-Klassifikator-Vertrag**: Antwort ist exakt `true` / `false` /
  `invalid` — nie ändern ohne alle Konsumenten.

## Historie / Kontext

- Ursprung: n8n-Workflow "Zumba" (`HG0zPlWsmPI3Mt7z`, Instanz
  `zumba-staging`); der Go-Bot ist dessen 1:1-Port und ist produktiv.
- Die n8n-Code-Node-Snippets (`whatsapp-statistic/`) und n8n-SQL-Dateien
  (`whatsapp-statistic.sql`, `absagen.sql`) wurden entfernt — n8n wird nicht
  mehr benutzt; die Rangliste-Query lebt in
  `shared/store/queries/leaderboard.sql`.
- Geplant: Personalisierung im Wrapped (`?wer=…`) als nächster größerer
  Schritt; created_at-Auswertungen ab Wrapped 2027.
