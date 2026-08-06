# WhatsApp-Bot — fachliche Beschreibung

Der Bot ist das Ohr und der Mund des Systems in der WhatsApp-Gruppe. Er läuft
rund um die Uhr und macht drei Dinge: Absagen/Zusagen erkennen, Statistik
beantworten, Wochenreport verschicken.

## Absage-Erkennung

**Auslöser**: Jede Nachricht in der Stammtisch-Gruppe (via Evolution-API
Webhook). Nachrichten außerhalb der Gruppe oder von unbekannten Nummern
werden ignoriert.

**Klassifikation**: Ein LLM (Google Gemini) beurteilt jede Nachricht mit
genau drei möglichen Ergebnissen:

| Ergebnis | Bedeutung | Wirkung |
|---|---|---|
| `true` | Zusage bzw. Rücknahme einer Absage | Absage-Zeile für den Tag wird gelöscht |
| `false` | Absage für den kommenden Donnerstag | Zeile in `stammtisch_abwesenheit` wird angelegt (Upsert: erneute Absage aktualisiert nur den Text) |
| `invalid` | Normale Konversation, keine An-/Abmeldung | Nichts passiert |

Fachliche Regeln:
- Eine Absage gilt immer für den **nächsten regulären Donnerstag** (Sperrtage
  werden übersprungen).
- Nur Donnerstage sind gültige Ziele — die Tabelle akzeptiert fachlich nur
  Donnerstags-Daten.
- Seit 08/2026 wird der **Absage-Zeitpunkt** (`created_at`) mitgeschrieben.
  Bei mehrfacher Absage fürs selbe Datum bleibt der Zeitpunkt der ersten.
- Ein ML-Schattenmodell (eigener Classifier-Service) klassifiziert parallel
  mit, ohne Wirkung — dient dem Vergleich LLM vs. eigenes Modell.

## Statistik auf Zuruf

Schreibt jemand „statistik" in die Gruppe, antwortet der Bot mit der
Rangliste: alle Mitglieder sortiert nach Anwesenheitsquote im laufenden
Auswertungszeitraum, mit Anwesenheits-/Fehlzahlen. Anwesenheit = Donnerstage
ohne Absage (Anwesenheit per Default, Sperrtage zählen nicht, Startdatum wird
geklemmt).

## Wochenreport (automatisch)

Jeden **Donnerstag um 21:00** (Europe/Berlin) postet der Bot den Report in
die Gruppe:

1. **Rangliste** (wie bei „statistik", mit Header „Automatischer
   Wochenreport")
2. **STRAFEN-Block** — offene und frisch beglichene Strafen
   (Sichtbarkeitsregeln siehe [strafen.md](strafen.md))
3. Footer

Beim echten Lauf (kein Dry-Run) persistiert der Bot dabei neu erkannte
Fehltage-Strafen als Marker. Der Report ist über das Admin-UI als Dry-Run
testbar („Wochenreport testen"), inklusive simuliertem Stichtag — simulierte
Läufe schreiben nie.

## Statistik als Bild-Karte

Die Statistik gibt es außer als Text auch als **PNG-Karte** im Wrapped-Look
(dunkles Holz/Biergold): Rangliste mit Medaillen, Prozent-Balken und
Streak-Symbolen, Highlights (GOAT, heißeste Serie, längste Pause) und
STRAFEN-Block. Der Bot baut dafür HTML und lässt es vom eigenen
**renderer-service** (headless Chromium) zu einem Bild schießen; verschickt
wird es als WhatsApp-Bild mit kurzer Caption.

Aktiv nur, wenn der Renderer konfiguriert ist (`RENDERER_URL`). Zwei
unabhängige Schalter steuern den Live-Betrieb (beide auf Staging seit
08/2026 auf `image`):

- **Gruppen-„statistik“**: Env `STATS_FORMAT=text|image`
  (Helm: `whatsappBot.env.STATS_FORMAT`)
- **Wochenreport**: Helm `whatsappBot.weeklyReport.format: text|image`
  (hängt `?format=image` an die CronJob-URL)

Neben dem Live-Design („Wrapped") gibt es drei weitere Bild-Designs, die
**nur im Bot-Test** wählbar sind (Auswahl „Bild-Design", `?cardStyle=`) —
Spielwiese, um Alternativen auszuprobieren:

| Design | Idee |
|---|---|
| `wrapped` | Live: dunkle Karte im Wrapped-Look, Medaillen und Quotenbalken |
| `bierdeckel` | Wirtshaus-Deckel aus heller Pappe: Strichliste in Fünferbündeln, Handschrift, Stempel, „offene Rechnung" |
| `bierdeckel-dunkel` | Derselbe Deckel in den Live-Farben (Holz/Biergold/Schaum) |
| `tafel` | Kreidetafel im Holzrahmen: Tageskarte mit Kreidebalken |
| `masskrug` | Schankbrett: je Mitglied ein Krug, gefüllt nach Quote, mit Schaumkante |
| `zeitung` | Sportteil des „Zumba-Anzeigers": Schlagzeile, Vorspann, Tabelle, Strafregister — ohne Emojis |
| `arena` | LED-Anzeigetafel: ein leuchtendes Segment je Stammtisch, Strafenbank |

In der Strichliste der Bierdeckel-Designs sind die Striche der **laufenden
Serie** farbig abgesetzt (sie sind definitionsgemäß die zuletzt gemachten),
eine laufende Pause erscheint als Reihe von ✕.

Live läuft immer „Wrapped"; unbekannte Werte fallen darauf zurück.

Schlägt Rendern oder Bild-Versand fehl, geht der Report **als Text** raus
(Fallback — er muss immer ankommen). Der Wochenreport trägt auf der Karte
einen Badge „📅 Automatischer Wochenreport“; zusätzlich unterscheidet die
WhatsApp-Caption die beiden Fälle. Im Admin-UI Bot-Test ist das Format pro
Request per Ausgabe-Wahl „Nachricht/Bild“ wählbar.

## Test-Modus

Ein `/test`-Endpoint führt die komplette Verarbeitung einer Beispielnachricht
aus (Klassifikation, DB-Wirkung, Antworttext), aber ohne Tages-/Gruppen-
Sperren. Das Admin-UI nutzt ihn für die Bot-Test-Seite (dort wählbar:
Nachricht oder Bild-Karte).

## Betriebsverhalten

- Preview-Modus: Antworten gehen an eine einzelne Testnummer statt in die
  Gruppe (Staging-Standard).
- Alle Antworttexte deutsch, WhatsApp-Formatierung (Fettdruck etc.).
- Nachrichtenverarbeitung wird als Trace aufgezeichnet (21 Tage Retention)
  zur Fehlersuche.
