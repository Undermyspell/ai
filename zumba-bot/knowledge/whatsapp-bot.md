# WhatsApp-Bot — fachliche Beschreibung

Der Bot ist das Ohr und der Mund des Systems in der WhatsApp-Gruppe. Er läuft
rund um die Uhr und macht drei Dinge: Absagen/Zusagen erkennen, Statistik
beantworten, Wochenreport verschicken.

## Absage-Erkennung

**Auslöser**: Jede Nachricht in der Stammtisch-Gruppe (via Evolution-API
Webhook). Nachrichten außerhalb der Gruppe oder von unbekannten Nummern
werden ignoriert.

**Klassifikation**: Ein LLM über die Gemini-API beurteilt jede Nachricht mit
genau drei möglichen Ergebnissen. Welches Modell das ist (Haupt- und
Fallback-Modell), stellt das Admin-UI auf der Seite *KI-Modell* um; der Bot
liest die Wahl vor jeder Nachricht aus `classifier_models`, die Werte aus dem
Deployment sind nur der Startwert. Liefert das Hauptmodell einen Fehler oder
antwortet nicht innerhalb von 30 Sekunden, fragt der Bot das Fallback; scheitert
auch das, gilt die Nachricht als `invalid`.

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

Es gibt fünfzehn Bild-Designs (Katalog: `shared/cards`). Welche davon im
Umlauf sind und welche Karte der nächste Wochenreport bekommt, stellt das
Admin-UI ein (Seite „Bild-Designs", siehe unten); im Bot-Test sind immer alle
wählbar (Auswahl „Bild-Design", `?cardStyle=`).

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
| `abfahrtstafel` | Schwarze Bahnhofs-Anzeigetafel „Zumba Hbf", Fallblatt-Zeilen, Strafen als Störungsmeldungen | „37 da · 6 weg" je Zeile, Serie als Hinweis („pünktlich +4" / „verspätet −2" / „fällt aus −5") |
| `kassenbon` | Thermo-Kassenbon auf dem Holztisch, Strafen als Nachberechnung, „OFFEN"-Summe und Strichcode | Menge „37x 6w" plus Quote je Posten, Serie als `[+4]`/`[−2]` |
| `gipfelbuch` | Hüttenbuch der Zumba-Alm: Bergprofil (ein Gipfel je Mitglied, Höhe = Quote) über den Einträgen | Fahne je Mitglied (rot = laufende Pause), „37/43" im Eintrag |
| `wetterbericht` | Stammtisch-Wetter: Großwetterlage, Hitzewelle/Kältefront, Wetterkachel je Mitglied | Quote als Temperatur, Wettersymbol nach Serie (Tropennacht … Dauerfrost) |
| `hochrechnung` | Wahlabend-Hochrechnung: Balken je Mitglied mit Ø-Marke, Sitzverteilung als Halbkreis | Balken = Quote, Halbkreis = alle Anwesenheiten vs. Absagen, Serie als Gewinn/Verlust |

In der Strichliste der Bierdeckel-Designs sind die Striche der **laufenden
Serie** farbig abgesetzt (sie sind definitionsgemäß die zuletzt gemachten).

Die fünf jüngeren Designs (Abfahrtstafel bis Hochrechnung) zeigen die Quote
gerundet und bringen eigene Schriften mit (Barlow Condensed, Space Mono als
eingebettete latin-Subsets). Gipfelbuch, Kassenbon und Hochrechnung nennen das
Stammtischjahr („Saison 2026") — es kommt aus `seasons`, ohne gepflegtes Jahr
steht dort das Kalenderjahr.

### Design-Warteschlange (Admin-UI „Bild-Designs", Tabelle `card_settings`)

Welche Designs im Umlauf sind, steht als **Warteschlange** in `card_settings`
(eine Zeile) und wird im Admin-UI unter `/bild-designs` per Drag & Drop
gepflegt. Der Bot liest die Tabelle vor jeder Karte – eine Änderung greift
ohne Neustart oder Deployment.

- **Wochenreport:** es kommt das **oberste** Design der Schlange. Nach dem
  echten Versand (nicht bei Dry-Run/Vorschau) rückt der Bot die Schlange
  weiter: das gesendete Design wandert ans Ende (`AdvanceCardQueue`). So
  stimmt „oben kommt als nächstes" jede Woche. Fällt ein Donnerstag aus (Bot
  weg, kein Versand), rückt nichts weiter – das Design kommt dann eben eine
  Woche später.
- **Wiederholungslauf** am selben Tag (`zuletzt_tag`/`zuletzt_style`) nimmt
  das schon gesendete Design und rückt nicht ein zweites Mal weiter; auch ein
  Dry-Run danach zeigt die gesendete Karte.
- **Einmal-Auswahl:** für den nächsten Wochenreport lässt sich ein Design fest
  wählen (`naechster_tag` + `naechster_style`). Sie ersetzt genau diesen
  Donnerstag und bewegt die Schlange nicht – das oberste Design kommt dann
  eine Woche später.
- **„statistik" auf Zuruf** zieht je Aufruf zufällig aus den Designs der
  Schlange (die Reihenfolge spielt dort keine Rolle).
- **`CARD_STYLES`** (Helm: `whatsappBot.env.CARD_STYLES`, Komma-Liste) ist nur
  der **Startwert**: der Bot trägt ihn beim Start ein, solange noch keine
  Schlange gespeichert ist (`rotation IS NULL`); eine leere `CARD_STYLES`
  schreibt nichts. Ohne gespeicherte Schlange oder wenn die DB nicht
  antwortet, rechnet der Bot wie früher über `CARD_STYLES`: Position =
  Wochenindex (Tage seit der Unix-Epoche / 7) modulo Listenlänge, ohne
  gespeicherten Zustand. Unbekannte IDs in `CARD_STYLES` beenden den Bot beim
  Start; unbekannte IDs in der Tabelle (etwa nach dem Entfernen eines Designs)
  überspringt er.
- Der Bot-Test wählt weiterhin selbst: ein ausdrückliches `?cardStyle=`
  schlägt Schlange und Einmal-Auswahl; ohne (Option „Wie live") wählt der Bot
  wie im Betrieb.

Startwert auf Staging (`CARD_STYLES`): `arena,formular,wrapped,bierdeckel,zeitung`.

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

## Notiz an die eigene Nummer

Ein `/notify`-Endpoint schickt einen übergebenen Text per WhatsApp — und zwar
ausschließlich an die Vorschau-Nummer aus der Konfiguration. Der Empfänger
kommt **nie** aus dem Request, sonst wäre das ein offener Versandweg in die
Stammtisch-Gruppe. Genutzt wird er vom Admin-UI, um sich die Adresse eines
offenen Tunnels aufs Handy zu schicken; ohne gepflegte Vorschau-Nummer
antwortet er mit einem Fehler und sendet nichts.

## Betriebsverhalten

- Preview-Modus: Antworten gehen an eine einzelne Testnummer statt in die
  Gruppe (Staging-Standard).
- Alle Antworttexte deutsch, WhatsApp-Formatierung (Fettdruck etc.).
- Nachrichtenverarbeitung wird als Trace aufgezeichnet (21 Tage Retention)
  zur Fehlersuche.
