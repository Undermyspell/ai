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
Spielwiese gegen den echten Bot ohne WhatsApp: Beispielnachricht wählen
(Statistik / Absage / Zusage), JSON anpassen, abschicken — das UI zeigt das
strukturierte Ergebnis (Klassifikation, DB-Wirkung, Antworttext). Zusätzlich
„Wochenreport testen": Dry-Run des kompletten Donnerstagsreports, optional
mit simuliertem Datum.

### ML-Testdaten
Tabelle `ml_test_messages`: gesammelte Beispielnachrichten für den
Classifier-Vergleich (LLM vs. eigenes Modell); manueller Klassifikations-Test
gegen den Classifier-Service.

## Verhalten ohne Datenbank

Ist die DB nicht erreichbar, läuft das UI mit Mock-Daten weiter (nur
Ansicht, sinnvoll für UI-Entwicklung). Eine grün aussehende Seite beweist
also keine funktionierende DB-Anbindung.
