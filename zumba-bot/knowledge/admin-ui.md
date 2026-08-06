# Admin-UI — fachliche Beschreibung

Die Pflege-Oberfläche für die Stammtisch-Daten. Zielgruppe: der Organisator
(eine Person). Erreichbar im Heimnetz, keine Authentifizierung.

## Was man damit macht

### Anwesenheiten korrigieren
Dashboard mit Mitgliedern und Donnerstagen. Anwesenheit lässt sich pro
Person/Tag umschalten — fachlich heißt das: eine Absage-Zeile anlegen oder
löschen (Anwesenheit per Default, es gibt nur Absagen). Typische Fälle:
jemand hat mündlich abgesagt, jemand stand trotz Absage plötzlich da,
Bot hat eine Nachricht falsch klassifiziert.

Manuell angelegte Absagen tragen seit 08/2026 ebenfalls einen
`created_at`-Zeitpunkt (den Klick-Zeitpunkt — nicht den einer echten
WhatsApp-Absage; für Timing-Auswertungen entsprechend mit Vorsicht genießen).

### Sperrtage pflegen
Donnerstage, an denen kein Stammtisch stattfindet (Feiertage, Sommerpause).
Nur Donnerstage sind zulässig — die Eingabe validiert das. Gesperrte Tage
verschwinden aus sämtlichen Auswertungen (Statistik, Strafen, Wrapped).

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
Spielwiese gegen den echten Bot ohne WhatsApp — ein Formular in vier
Schritten:

1. **Szenario** — Statistik, Absage, Zusage oder Wochenreport. Die ersten
   drei schicken eine Beispielnachricht durch die komplette Verarbeitung,
   der Wochenreport löst den Donnerstagsreport aus.
2. **Beispiel-Nachricht** — das Webhook-JSON, frei editierbar (entfällt beim
   Wochenreport).
3. **Ausgabe** — „💬 Nachricht" (Text, inkl. alternativer Statistik-Designs)
   oder „🖼️ Bild" (PNG-Karte mit Design-Auswahl). Entfällt bei
   Absage/Zusage, weil dort nur klassifiziert wird.
4. **Versand** — Dry-Run oder Vorschau an die eigene Nummer, dazu ein
   optionaler Stichtag.

Danach zeigt das UI das strukturierte Ergebnis (Klassifikation, DB-Wirkung,
Antworttext bzw. Bild). Nicht zutreffende Schritte blendet die Seite aus,
die Nummerierung bleibt lückenlos. Der Modus „Vorschau
an meine Nummer“ verschickt entsprechend Text oder Bild an die Testnummer —
nie an die Gruppe.

### ML-Testdaten
Tabelle `ml_test_messages`: gesammelte Beispielnachrichten für den
Classifier-Vergleich (LLM vs. eigenes Modell); manueller Klassifikations-Test
gegen den Classifier-Service.

## Verhalten ohne Datenbank

Ist die DB nicht erreichbar, läuft das UI mit Mock-Daten weiter (nur
Ansicht, sinnvoll für UI-Entwicklung). Eine grün aussehende Seite beweist
also keine funktionierende DB-Anbindung.
