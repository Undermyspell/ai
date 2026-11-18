# WhatsApp-Bot — fachliche Beschreibung

Der Bot ist das Ohr und der Mund des Systems in der WhatsApp-Gruppe. Er läuft
rund um die Uhr und macht vier Dinge: Absagen/Zusagen erkennen, Statistik
beantworten, Wochenreport verschicken, den Wirt an den Tisch erinnern.

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
- **Wer entscheidet**, stellt das Admin-UI ein (`/ki-modell`): Gemini oder
  das eigene Modell (classifier-service). Standard ist Gemini. Der jeweils
  andere klassifiziert mit, ohne Wirkung; beide Label landen mit
  „entschieden von" und Modellversion in `ml_messages`. Entscheidet das eigene
  Modell und antwortet es nicht, springt für die Nachricht Gemini ein.

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

## Tisch-Erinnerung an den Wirt

Jeden **Donnerstag um 16:00** (Europe/Berlin) bittet der Bot den Wirt, den
Stammtisch-Tisch für den Abend freizuhalten. Die Nachricht geht in die
Stammtisch-Gruppe, der Wirt wird darin nur markiert – einen Direktchat mit ihm
gibt es nicht. Der Wirt ist kein Mitglied der Runde (steht nicht in `users`),
aber in der WhatsApp-Gruppe; seine Nummer liegt wie die der Gruppe im
SealedSecret `whatsapp-bot-secrets` (`WIRT_JID`, optional).

### Aufbau: Rahmen fest, Text jede Woche neu

Die Nachricht besteht aus drei Teilen (`shared/wirt`):

| Teil | Inhalt | Woher |
|---|---|---|
| Kopf | „🍻 *Stammtisch heute Abend*" | fest, vom Bot |
| Text | Anrede mit Markierung, Bitte um den Tisch, Dank | **Sprachmodell** – sonst der feste Standardtext |
| Fußzeile | „🤖🍺 *Automatische Erinnerung vom Zumba-Bot*" | fest, vom Bot |

Das Modell schreibt nur den Mittelteil und setzt für den Wirt den Platzhalter
`{wirt}`. Der Bot ersetzt ihn beim Versand durch `@<nummer>`; dieselbe Nummer
geht als `mentioned` an die Evolution API, WhatsApp zeigt daraus in Gruppen
den Kontaktnamen (im 1:1-Chat bleibt es bei `@<nummer>`). Markierung und
Fußzeile können so nie fehlen, und die Nummer des Wirts steht weder in der DB
noch im Admin-UI. Die Nachricht spricht von „heute Abend" – eine Ankündigung,
dass wir kommen, braucht es nicht.

### Ablauf am Donnerstag

1. **10:00 – vorgenerieren.** Ein CronJob
   (`templates/whatsapp-bot/wirt-reminder-generate-cronjob.yaml`, `0 10 * * 4`,
   Schalter `whatsappBot.wirtReminder.generate.enabled`) ruft
   `POST /wirt-reminder/generate` auf. Der Bot lässt das Modell den Text
   schreiben (`internal/wirttext`) und legt ihn als Entwurf des Tages ab
   (`wirt_nachrichten`).
2. **Bis 16:00 – ansehen, ablehnen.** Das Admin-UI zeigt den Entwurf auf der
   Seite *Tisch-Erinnerung*. Wer nichts tut, lässt ihn durch; „Ablehnen"
   schaltet für diesen Tag auf die Standardnachricht (und lässt sich
   zurücknehmen).
3. **16:00 – senden.** Der zweite CronJob
   (`wirt-reminder-cronjob.yaml`, `0 16 * * 4`, Schalter
   `whatsappBot.wirtReminder.enabled`) ruft `POST /wirt-reminder?send=true`
   auf. Raus geht der Entwurf – oder die **Standardnachricht**, wenn es
   keinen gibt, er abgelehnt wurde oder die feste Prüfung nicht (mehr)
   besteht. Die Standardnachricht ist der bisherige feste Text; sie muss
   immer möglich sein.

Was rausging (KI-Text oder Standard), hält der Bot in `wirt_nachrichten` fest.

### Was ein KI-Text erfüllen muss

Leicht lustig darf er sein, höflich und nett muss er sein. Dafür sorgen drei
Stufen, alle vor dem Speichern:

- **Prompt** (`internal/wirttext/prompt.txt`): herzlich, per Du, ein
  Augenzwinkern; nie frech, spöttisch, fordernd oder auf Kosten des Wirts;
  nur die Bitte um den Tisch, nichts dazuerfunden (keine Uhrzeit,
  Personenzahl, Versprechen). Zugeschnitten auf einen Bier-Stammtisch: Bier,
  Durst, die erste Halbe und Anstoßen dürfen vorkommen, vom Essen ist nicht
  die Rede – und Bier ja, Saufen nein (keine Witze über Rausch oder Mengen). Jede Woche bekommt das Modell einen anderen
  Aufhänger und die letzten fünf Texte mit der Ansage, sich nicht zu
  wiederholen.
- **Feste Prüfung** (`shared/wirt.Validate`): genau eine Markierung, keine
  weiteren `@` oder Platzhalter, das Wort „Tisch" kommt vor, 30–400 Zeichen,
  höchstens 8 Zeilen, kein Link.
- **Tonprüfung**: ein zweiter Modell-Aufruf (`pruef-prompt.txt`) beurteilt den
  Text streng mit `ok` oder `nein`; nur ein eindeutiges `ok` zählt.

Fällt ein Text durch, schreibt das Modell einmal neu. Bleibt es dabei oder
antwortet das Modell nicht, ist das nächste Modell dran (siehe
Einstellungen). Liefert keines, gibt es keinen Entwurf: der Endpoint meldet
502 (der CronJob versucht es bis zu zweimal erneut), der Grund steht im
Admin-UI, und nachmittags geht die Standardnachricht raus. Ein Text kostet im
Normalfall zwei, je Modell höchstens vier Aufrufe.

### Einstellungen aus dem Admin-UI (`wirt_settings`)

Der Bot liest sie vor jedem Lauf – eine Änderung greift ohne Neustart.

- **Automatischer Versand an/aus.** Aus heißt: donnerstags geht nichts an den
  Wirt (weder KI-Text noch Standard) und es wird nichts vorgeneriert.
  Standard ist an. Testläufe bleiben möglich.
- **Modell für den Text** (Seite *KI-Modell*, Karte „Tisch-Erinnerung an den
  Wirt"). Dieselbe Auswahl wie beim Klassifizieren, aber ohne Gemma.
  Startwert ist das Hauptmodell des Klassifikators, solange nichts gewählt
  ist. Antwortet das gewählte Modell nicht oder liefert es nichts
  Brauchbares, versuchen es **Haupt- und Fallback-Modell des Klassifikators**
  (in dieser Reihenfolge, jedes nur einmal); erst danach bleibt die
  Standardnachricht.

### Wann nichts rausgeht

Der Bot sendet nur, wenn

- der automatische Versand eingeschaltet ist,
- heute Donnerstag ist,
- der Tag **kein Sperrtag** ist (`excluded_days`),
- `WIRT_JID` gesetzt ist,
- heute noch nicht gesendet wurde.

Sonst entfällt der Versand, der Grund steht in der Antwort und im Log.
Antwortet die DB bei Schalter oder Sperrtag nicht, geht die Bitte trotzdem
raus – ein unnötig freigehaltener Tisch ist das kleinere Übel. Der
16-Uhr-CronJob wiederholt einen fehlgeschlagenen Aufruf nicht, damit die
Bitte nie doppelt in der Gruppe steht. Unter denselben Bedingungen entsteht
vormittags auch kein Entwurf; außerdem nicht, wenn es für den Tag schon einen
gibt – ein Wiederholungslauf hebt eine Ablehnung nicht auf.

### Testen

Admin-UI, Bot-Test → „Tisch-Erinnerung": `?dryRun=true` berechnet nur den
Text, `?preview=true` schickt ihn an die Vorschau-Nummer bzw. mit
`&previewTarget=group` in die Testgruppe (`PREVIEW_GROUP_JID`) und markiert
dort die Vorschau-Nummer statt des Wirts. Ein Aufruf ohne Parameter ist ein
Dry-Run. `?nachricht=` wählt den Text des Testlaufs: ohne Angabe das, was
beim nächsten Versand rausginge („wie live" – am Mittwoch also schon der
Text für Donnerstag), `standard` die feste Nachricht, `neu` einen jetzt
geschriebenen KI-Text. Der wird nicht gespeichert – übernommen wird er
im Admin-UI (dort landet er als Entwurf des nächsten Versands). Weder
Empfänger noch Text einer Nachricht kommen je aus dem Request.

## Statistik als Bild-Karte

Die Statistik gibt es außer als Text auch als **PNG-Karte** im Wrapped-Look
(dunkles Holz/Biergold): Rangliste mit Medaillen, Prozent-Balken und
Streak-Symbolen, Highlights (GOAT, heißeste Serie, längste Pause) und
STRAFEN-Block. Der Bot baut dafür HTML und lässt es vom eigenen
**renderer-service** (headless Chromium) zu einem Bild schießen; verschickt
wird es als WhatsApp-Bild mit kurzer Caption.

**Plätze wie im Sport:** Gleichauf ist, wer gleich oft da war und dieselbe
Quote hat; Gleichplatzierte teilen sich den Platz, die folgenden entfallen
(1, 2, 2, **4** – nach zwei Zweiten gibt es keine Bronze). Dieselbe Regel gilt
in Text, Bild-Karten und im Admin-UI (Dashboard, Mitglied-Detail).

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
