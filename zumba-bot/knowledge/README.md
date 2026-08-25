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
| [strafen.md](strafen.md) | quer | Strafen-Fachlogik (einzige Kopie in `shared/penalty/`) |
| [deployment.md](deployment.md) | `deployment/` | GitOps auf k3s (Raspberry Pi 5) via ArgoCD |

## Das Domänenmodell (gilt überall)

**Anwesenheit per Default**: Wer nicht explizit absagt, war da. Es gibt keine
"Anwesend"-Tabelle — nur Absagen werden gespeichert. Alle Auswertungen leiten
Anwesenheit aus dem Fehlen einer Absage ab.

**Nur Donnerstage zählen** (`EXTRACT(DOW) = 4` / ISODOW 4). Sperrtage
(`excluded_days`, z. B. Feiertage) werden überall herausgefiltert.

**Zeitrechnung**: Die Auswertung läuft in **Stammtischjahren** — benannte
Zeiträume mit gepflegtem Start- und Enddatum, keine Kalenderjahre. Sie stehen
in `seasons` (siehe unten), überlappen nicht und lösen den Jahreswechsel
automatisch auf: maßgeblich ist das Jahr, in das der Stichtag fällt. "2026" =
01.12.2025–30.11.2026, "2027" beginnt am 01.12.2026.

Startdaten von Mitgliedern werden auf frühestens den Jahresstart geklemmt
(`domain.Season.ClampStart`) — mit jedem neuen Jahr beginnen alle wieder bei
null. Zukunft zählt nie mit: das Enddatum wird auf "heute" gekappt, bei
abgelaufenen Jahren auf ihr Jahresende (Archiv zeigt den Endstand).
Fehltage-Serien enden an der Jahresgrenze und beginnen im neuen Jahr neu.

### Tabellen (Postgres, DB `zumba`, Schema `public`)

- `users` — `userId` (WhatsApp-JID, Format `<nummer>@s.whatsapp.net`), `userName`,
  `startDate` (nullable). 15 Mitglieder (Stand 08/2026).
- `stammtisch_abwesenheit` — eine Zeile pro Absage: `userId`, `date`,
  `message` (nullable — viele Absagen kommen ohne Text), `created_at`
  (TIMESTAMPTZ, seit 08/2026; Altbestand NULL. Für Wrapped 2027:
  "kurzfristigste Absage"). PK (`userId`, `date`).
- `excluded_days` — Donnerstage, die nicht zählen.
- `seasons` — die Stammtischjahre: `label` ("2026"), `start_date`, `end_date`
  (beide inklusive). Ein EXCLUDE-Constraint auf `daterange` verbietet
  Überlappungen, damit "das Jahr zum Zeitpunkt t" eindeutig ist. Bot und
  Admin-UI legen die Tabelle idempotent an und seeden sie einmalig
  (`shared/store.EnsureSeasonsSchema`, Seed: 2025/2026/2027, je 1.12.–30.11.);
  danach wird sie von Hand gepflegt. Ist für einen Zeitpunkt kein Jahr
  gepflegt, gibt es `domain.ErrNoSeason` — bewusst ein Fehler statt einer
  stillen Null-Auswertung.

  **Datenlage 2025**: Absagen sind erst ab 13.10.2025 erfasst. Die ~45
  Donnerstage davor zählen wegen attendance-by-default als anwesend — das
  Archiv-Jahr 2025 liest sich deshalb deutlich besser als die Realität.
  Zusätzlich liegen 285 Altzeilen (13.10.–16.11.2025) auf Nicht-Donnerstagen;
  alle Auswertungen filtern `ISODOW = 4`, sie sind damit wirkungslos, aber
  noch in der Tabelle.
- `strafen` — siehe [strafen.md](strafen.md).

Zwei Datenbanken auf einer Postgres-Instanz: `n8n` (n8n-State + Evolution-API
im Schema `evolution`) und `zumba` (Domänendaten).

## Konventionen

- **Alle nutzerseitigen Texte deutsch** (UI, Reports, Logs). Datumsformat
  DD.MM., Woche beginnt Montag.
- **Shared-Modul statt Duplikate** (seit 08/2026): gemeinsame Domänen-Logik
  lebt einmal in `shared/` (`penalty/`, `domain/`, `store/` inkl.
  Rangliste-Query `leaderboard.sql` und Strafen-DDL) und wird von Bot,
  Admin-UI und Wrapped per `replace`-Directive eingebunden. Deshalb ist der
  Docker-Build-Kontext `zumba-bot/` (Ausnahme: `renderer-service/`,
  eigenständig). Verbleibendes Keep-in-sync: der Klassifikator-Prompt
  (`system-prompt.txt` ↔ `whatsapp-bot/internal/classifier/system-prompt.txt`).
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
