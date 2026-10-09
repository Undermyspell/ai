# Admin-UI — fachliche Beschreibung

Die Pflege-Oberfläche für die Stammtisch-Daten. Zielgruppe: der Organisator
(eine Person). Erreichbar im Heimnetz, hinter einem Login.

## Login

Ein Benutzer (`admin`), ein Passwort. Ohne gültige Sitzung führt jeder Pfad
außer Login, Healthcheck und statischen Dateien auf die Anmeldeseite; nach dem
Anmelden geht es an der ursprünglich gewünschten Stelle weiter.

Die Sitzung steckt in einem signierten Cookie — HttpOnly (kein Zugriff aus
JavaScript), SameSite=Strict (wird bei Aufrufen von fremden Seiten nicht
mitgeschickt) und Secure (nur über https). Der Server hält keinen
Sitzungsspeicher; die Signatur macht das Cookie fälschungssicher, die
Laufzeit endet nach sieben Tagen. Weil das Cookie Secure ist, läuft die Seite
im Cluster über https (Traefik-Default-Zertifikat, der Browser warnt einmalig)
und http leitet dorthin um.

Passwort und Signatur-Schlüssel liegen als SealedSecret `admin-ui-secrets` im
Repo. Fehlt das Secret, startet der Pod nicht — lieber kein Admin-UI als
eines ohne Anmeldung. Lokal ist der Login aus, solange kein Passwort gesetzt
ist.

## Was man damit macht

### Anwesenheiten korrigieren
Dashboard, Donnerstage (Kacheln + Matrix Mitglieder × Donnerstage) und
Mitglieder. Anwesenheit lässt sich überall pro Person/Tag umschalten – in der
Matrix, im Mitglieder-Verlauf und im Bottom-Sheet „Donnerstag bearbeiten", das
sich von jedem Donnerstag aus öffnet (Seitenleiste: „Letzten Do. eintragen").
Fachlich heißt das: eine Absage-Zeile anlegen oder löschen (Anwesenheit per
Default, es gibt nur Absagen). Typische Fälle:
jemand hat mündlich abgesagt, jemand stand trotz Absage plötzlich da,
Bot hat eine Nachricht falsch klassifiziert.

Manuell angelegte Absagen tragen seit 08/2026 ebenfalls einen
`created_at`-Zeitpunkt (den Klick-Zeitpunkt — nicht den einer echten
WhatsApp-Absage; für Timing-Auswertungen entsprechend mit Vorsicht genießen).

### Sperrtage pflegen
Donnerstage, an denen kein Stammtisch stattfindet (Feiertage, Sommerpause).
Ein Jahreskalender zeigt alle Donnerstage des Stammtischjahres; Klick sperrt
bzw. gibt frei. Feiertage, die auf einen Donnerstag fallen, schlägt die Seite
vor. Nur Donnerstage sind zulässig — die Eingabe validiert das. Gesperrte Tage
verschwinden aus sämtlichen Auswertungen (Statistik, Strafen, Wrapped).

### Stammtischjahre pflegen (`/stammtischjahre`)
Zeigt alle Stammtischjahre mit Zeitraum, Zahl der Donnerstage und Status
(läuft / kommt / abgeschlossen) und ersetzt das Pflegen von `seasons` per SQL:

- **Beginn verschieben** — nur für ein Jahr, das noch nicht begonnen hat, und
  nur auf ein Datum in der Zukunft. Das Vorjahr endet automatisch am Tag
  davor, es entsteht weder Lücke noch Überlappung. Vergangene Grenzen sind
  fest, sonst änderten sich abgeschlossene Auswertungen und Strafen-Serien.
- **Nächstes Jahr anlegen** — schließt lückenlos an das letzte an und dauert
  ein Jahr. Erst möglich, wenn das letzte gepflegte Jahr läuft: so steht
  höchstens ein Jahr im Voraus fest. Ist das laufende Jahr das letzte, warnt
  die Seite, dass danach Bot und Admin-UI ohne Jahr dastehen.

Die Admin-UI zeigt Änderungen sofort, der Bot nach spätestens 10 Minuten
(Jahres-Cache). Löschen gibt es bewusst nicht.

### Strafen verwalten (`/strafen`)
Vollständige Strafenverwaltung, Regeln siehe [strafen.md](strafen.md):

- **No-Show-Strafen anlegen** (nicht abgemeldet und nicht erschienen,
  Default 50 €, Betrag frei wählbar).
- **Begleichen** — Strafe ist bezahlt. Wirkt zugleich als Reset-Punkt für
  laufende Fehltage-Serien.
- **Löschen** (soft) — Strafe war unberechtigt. Verschwindet aus allen
  Reports, bleibt aber als Reset-Marker bestehen.
- **Simulierter Stichtag** (`?stichtag=`) — zeigt die Strafenlage, wie sie
  an einem beliebigen Datum aussähe. Für „was passiert nächsten Donnerstag?"

Wichtig: Fehltage-Strafen entstehen nie im Admin-UI — sie werden automatisch
vom Bot erkannt. Das UI zeigt auch erkannte, noch nicht persistierte
Kandidaten an.

### Bot-Test (`/bot-test`)
Spielwiese gegen den echten Bot ohne WhatsApp, als Chat-Simulation:

- **Szenario** — Statistik, Absage, Zusage oder Wochenreport. Die ersten
  drei schicken eine Beispielnachricht durch die komplette Verarbeitung,
  der Wochenreport löst den Donnerstagsreport aus.
- **Nachricht** — steht als Sprechblase im Chat und lässt sich direkt
  bearbeiten; darunter liegt das vollständige Webhook-JSON, ebenfalls frei
  editierbar (entfällt beim Wochenreport).
- **Ausgabe** — „💬 Nachricht" (Text, inkl. alternativer Statistik-Designs)
  oder „🖼️ Bild" (PNG-Karte mit Design-Auswahl). Entfällt bei
  Absage/Zusage, weil dort nur klassifiziert wird.
- **Versand** — Dry-Run oder Vorschau an die eigene Nummer, dazu ein
  optionaler Stichtag.

Danach antwortet der Bot im Chat (Antworttext bzw. Bild, bei Absage/Zusage
die Klassifikation samt DB-Wirkung), daneben zeigt ein Verarbeitungspfad,
welche Schritte gelaufen sind. Der Modus „Vorschau an meine Nummer“
verschickt entsprechend Text oder Bild an die Testnummer — nie an die
Gruppe.

### KI-Modell (`/ki-modell`)

Welches Modell die Gruppen-Nachrichten klassifiziert: oben der Weg einer
Nachricht (Hauptmodell → bei Fehler Fallback → Label), darunter je eine Karte
für Gemma 4 31B, Gemini 3.8 Flash und Gemini 3.5 Flash Lite mit den Knöpfen
„Als Hauptmodell" / „Als Fallback". Ein Klick schaltet sofort um (Tabelle
`classifier_models`), der Bot nimmt die Wahl ab der nächsten Nachricht. Das
Modell der jeweils anderen Rolle ist gesperrt – gleich gesetzt gäbe es keinen
Fallback. Jede Karte zeigt die Grenzen des kostenlosen Kontingents, die
Seitenspalte rechnet vor, ob es reicht: ein Donnerstag braucht etwa zehn
Aufrufe.

### Bild-Designs (`/bild-designs`)

Welche Karte der Bot als Bild schickt. Oben der **nächste Wochenreport**
(Datum, welches Design kommt) mit einer Auswahl „Rotation" oder ein
bestimmtes Design – das gilt nur für diesen einen Donnerstag, die
Warteschlange rückt dann eine Woche nach hinten. Darunter die
**Warteschlange**: oben kommt am nächsten Donnerstag, jede Zeile zeigt ihr
Datum. Sortiert wird am Griff ⠿ per Drag & Drop (Maus und Finger) oder mit
↑/↓ auf dem Griff; Häkchen weg nimmt ein Design raus, ein Häkchen unter
„Nicht im Umlauf" stellt es hinten an. Jede Änderung speichert sofort. Nach
jedem Wochenreport wandert das gesendete Design ans Ende. Die Seitenspalte
zeigt den zuletzt gesendeten Report und die nächsten acht Donnerstage.
Gespeichert wird in `card_settings`; der Bot liest die Tabelle vor jeder
Karte. Wie beim KI-Modell gesperrt ist die Seite bei offenem Tunnel nicht –
Speichern verschickt nichts.

### ML-Testdaten
Tabelle `ml_test_messages`: gesammelte Beispielnachrichten für den
Classifier-Vergleich (LLM vs. eigenes Modell); manueller Klassifikations-Test
gegen den Classifier-Service.

### Öffentlich (`/public`)

Schaltet Wrapped und das Admin-UI über ngrok ins Internet — für den Moment, in
dem man den Wrapped-Link in die Gruppe schickt. Je Ziel eine Karte mit Zustand
(*nicht erreichbar · baut auf · öffentlich erreichbar · baut ab · Fehler*),
Adresse samt Kopierknopf und Laufzeit. Beim Einschalten wählt man
2, 8 oder 24 Stunden; danach schließt der Tunnel von selbst, spätestens nachts
um drei.

Neben „Kopieren" liegt **„📱 Aufs Handy"**: schickt die Adresse per WhatsApp an
die eigene Nummer — praktisch, wenn man am Rechner freischaltet und den Link
vom Telefon aus weitergeben will. Den Versand macht der whatsapp-bot
(`/notify`), dessen Empfänger fest auf der Vorschau-Nummer steht; aus dem
Admin-UI heraus ist die Gruppe darüber nicht erreichbar. Zwei Klicks
hintereinander werden gebremst (5 Sekunden), damit ein Doppelklick nicht zwei
Nachrichten schickt.

Die Seite fragt den Zustand laufend nach — alle 2 Sekunden, solange etwas auf-
oder abbaut, sonst gemächlicher. Der Aufbau dauert ein paar Sekunden, deshalb
gibt es die Zwischenzustände überhaupt.

Zwei Dinge, die dranhängen:

- **Bot-Test und ML-Test sind gesperrt, solange etwas offen ist** (im Menü
  ausgeblendet, Aufruf gibt 403). Der Preview-Modus des Bot-Tests schickt eine
  echte WhatsApp-Nachricht — das darf niemand auslösen, der zufällig auf der
  öffentlichen Adresse landet.
- **Der Login bremst Fehlversuche exponentiell** (0,5 s, 1 s, 2 s … bis 30 s,
  Rücksetzung nach erfolgreicher Anmeldung oder 15 Minuten Ruhe). Bewusst keine
  Sperre: die ließe sich von außen auslösen, um den Besitzer auszusperren.

Ohne `TUNNEL_URL` (kein ngrok im Cluster) zeigt die Seite nur einen Hinweis.

## Verhalten ohne Datenbank

Ist die DB nicht erreichbar, läuft das UI mit Mock-Daten weiter (nur
Ansicht, sinnvoll für UI-Entwicklung). Eine grün aussehende Seite beweist
also keine funktionierende DB-Anbindung.
