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

**Jedes** Design trägt das offizielle Stammtisch-Emblem (kreisrund
freigestellt, `internal/report/assets/logo.png`, als Data-URL eingebettet —
der Renderer hat keinen Netzzugriff): als Wappen neben dem Titel, als
Untersetzer, Wirtshausschild, Zeitungssignet oder Dienstsiegel, je nach
Bildwelt. Designs, die es vielfach zeigen (Treuekarte, Sammelalbum), legen es
einmal in eine CSS-Variable, statt die Data-URL je Element zu wiederholen.

Ebenso zeigt jedes Design **An- und Abwesenheit** als eigene Form und nicht
nur als Quote — geteilter Balken, Kreuz, Leerfeld, roter Ring, dunkelrotes
LED-Segment —, dazu immer beide Zahlen im Klartext.

Aktiv nur, wenn der Renderer konfiguriert ist (`RENDERER_URL`). Zwei
unabhängige Schalter steuern den Live-Betrieb (beide auf Staging seit
08/2026 auf `image`):

- **Gruppen-„statistik“**: Env `STATS_FORMAT=text|image`
  (Helm: `whatsappBot.env.STATS_FORMAT`)
- **Wochenreport**: Helm `whatsappBot.weeklyReport.format: text|image`
  (hängt `?format=image` an die CronJob-URL)

Es gibt zehn Bild-Designs. Welche davon im Umlauf sind, steuert
`CARD_STYLES` (siehe unten); im Bot-Test sind immer alle wählbar
(Auswahl „Bild-Design", `?cardStyle=`).

| Design | Idee | An-/Abwesenheit |
|---|---|---|
| `wrapped` | Live-Look: dunkle Karte, Medaillen, Quotenbalken | Balken gold/schraffiert, „29 da“ / „2 gefehlt“ darunter |
| `bierdeckel` | Wirtshaus-Deckel aus heller Pappe, Handschrift, Stempel, „offene Rechnung" | Strichliste in Fünferbündeln, dahinter ein ✕ je Fehltermin |
| `bierdeckel-dunkel` | Derselbe Deckel in den Live-Farben (Holz/Biergold/Schaum) | wie `bierdeckel` |
| `tafel` | Kreidetafel im Holzrahmen: Tageskarte | Kreidepunkt je Besuch, rotes Kreidekreuz je Fehltermin |
| `masskrug` | Schankbrett: je Mitglied ein Krug, gefüllt nach Quote | „29 DA“ im Bier, schraffierter Rest, „2 gefehlt“ daneben |
| `zeitung` | Sportteil des „Zumba-Anzeigers": Schlagzeile, Vorspann, Tabelle — ohne Emojis | Balken schwarz/schraffiert plus Spalten „Da“ und „Fehlt“ |
| `arena` | LED-Anzeigetafel, Strafenbank | ein Segment je Stammtisch: leuchtend = da, dunkelrot = gefehlt |
| `stempelkarte` | Treuekarte aus Karton mit Perforationsrand | ein Feld je Stammtisch: gestempelt (das Emblem) vs. durchgestrichenes Leerfeld |
| `sammelkarten` | Sammelalbum im Panini-Raster statt Tabelle | Punkt je Stammtisch: gold gefüllt = da, roter Ring = gefehlt |
| `formular` | Amtliches Formblatt ZU-4, Schreibmaschinensatz, Gebührenbescheid | Kästchenmatrix: ausgefüllt = anwesend, leer/rot = gefehlt |

In der Strichliste der Bierdeckel-Designs sind die Striche der **laufenden
Serie** farbig abgesetzt (sie sind definitionsgemäß die zuletzt gemachten).

### Design-Rotation (`CARD_STYLES`)

`CARD_STYLES` ist eine Komma-Liste von Design-IDs (Helm:
`whatsappBot.env.CARD_STYLES`), leer = immer das Live-Design. Unbekannte IDs
beendet der Bot beim Start mit Fehler.

- **„statistik" auf Zuruf** zieht je Aufruf zufällig aus der Liste — sie kann
  mehrmals am Tag kommen, ein fester Durchlauf wäre da nur berechenbar.
- **Wochenreport** geht die Liste der Reihe nach durch: Position =
  Wochenindex (Tage seit der Unix-Epoche / 7) modulo Listenlänge. Ein Design
  ist damit erst wieder dran, wenn alle anderen einmal dran waren. Ein je
  Durchlauf neu gewürfelter Zufall kann das nicht garantieren (dort kommt
  dasselbe Design an der Durchlauf-Grenze schnell wieder), deshalb der feste
  Umlauf — die Reihenfolge bestimmt `CARD_STYLES`.
- Das braucht **keinen gespeicherten Zustand**: Neustart, Wiederholungslauf
  und Dry-Run im Admin-UI liefern dasselbe Design wie der echte Versand. Tag 0
  der Epoche war ein Donnerstag, die Wochen wechseln also im Takt des Reports
  — ein Nachhol-Lauf am Freitag bleibt beim Design des Vortags.
- Der Bot-Test wählt weiterhin selbst: ein ausdrückliches `?cardStyle=`
  schlägt die Rotation.

Auf Staging seit 08/2026 im Umlauf: `wrapped,bierdeckel,zeitung,arena,formular`.

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
