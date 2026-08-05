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

## Test-Modus

Ein `/test`-Endpoint führt die komplette Verarbeitung einer Beispielnachricht
aus (Klassifikation, DB-Wirkung, Antworttext), aber ohne Tages-/Gruppen-
Sperren. Das Admin-UI nutzt ihn für die Bot-Test-Seite.

## Betriebsverhalten

- Preview-Modus: Antworten gehen an eine einzelne Testnummer statt in die
  Gruppe (Staging-Standard).
- Alle Antworttexte deutsch, WhatsApp-Formatierung (Fettdruck etc.).
- Nachrichtenverarbeitung wird als Trace aufgezeichnet (21 Tage Retention)
  zur Fehlersuche.
