# Wrapped — fachliche Beschreibung

Der Jahresrückblick des Stammtischs, inspiriert von Spotify Wrapped: eine
mobile-first Slide-Show, die das Stammtisch-Jahr in Zahlen, Rankings, Awards
und Gossip erzählt. Zielgruppe: die Gruppe selbst, geteilt via WhatsApp.
Läuft im Cluster unter eigenem Hostnamen (siehe Staging-Values), Route `/2026`.

## Auswertungszeitraum

„Wrapped 2026" = **01.12.2025 – 30.11.2026**. Zukünftige Donnerstage zählen
nie mit (Kappung auf „heute") — die Seite ist also unterjährig jederzeit
aufrufbar und wächst mit. Sperrtage sind überall herausgerechnet, Startdaten
geklemmt. Jeder Jahrgang ist als eigenständiges Paket gedacht (2027 kommt
neben 2026, ersetzt es nicht).

## Bedienung (Story-Mechanik)

- Slides laufen automatisch weiter (pro Slide eigene Anzeigedauer),
  Fortschrittsbalken oben, Punkte-Navigation unten.
- Tippen/Klick = weiter, Swipe = vor/zurück, Pfeiltasten am Desktop.
- **Gedrückt halten = Pause** (wie Instagram-Stories), Loslassen läuft
  weiter; ein Hold zählt nicht als Weiter-Tipp.
- Reduzierte Bewegung des Betriebssystems wird respektiert (Animationen aus).

## Die Slides (Stand 08/2026, 25 Stück)

1. **Intro** — Titel
2. **Jahreszahlen** — Donnerstage, Mitglieder, Zusagen, Absagen, Ø-Quote
   (animierte Zähler)
3. **Ranking** (Intro + 3 Slides) — alle 15 nach Anwesenheitsquote in
   Fünfergruppen, mit Titel (z. B. „Stammtisch-König", „Der Spontane"),
   FunFact und Spruch pro Person
4. **Streaks** — Top 3 längste Anwesenheits- und Absage-Serien, mit Zeitraum
5. **Ausreden nach Kategorie** — Balkenstatistik (Arbeit, Familie,
   Gesundheit, Müdigkeit, Wetter, Freizeit, Kreativ, Keine Lust)
6. **Kreativste Ausreden** — die besten Original-Nachrichten, plus
   **Ausreden-Recycling** (wortgleich wiederholte Ausreden derselben Person)
7. **Ausreden-Forensik** — 📖 Romanautor (längste Absage), 🪨 Minimalist
   (kürzeste), 😂 Emoji-König (inkl. Lieblings-Emoji der Gruppe)
8. **Jahreskalender** — Heatmap: Anwesenheitsquote pro Monat (Farbe) +
   Absagenzahl, bester/schlechtester Monat
9. **Muffel des Jahres** — ☀️ Sommermuffel (meiste Absagen Jun–Aug),
   ❄️ Wintermuffel (Dez–Feb)
10. **Legendärste / einsamste Donnerstage** — Top 3 und Flop 3 nach
    Anwesenden, mit Datum
11. **Volle Hütte** — wie oft waren ALLE da (mit Datumsliste; Empty-State,
    falls nie)
12. **Dynamische Duos** — 🤜🤛 Unzertrennliche (meiste gemeinsame
    Anwesenheiten), 👯 Absage-Zwillinge (meiste gemeinsame Fehltage),
    🔄 Wachablösung (viele Absagen, nie am selben Tag), 🏓 Ping-Pong-Duo
    (längste strikte Wechsel-Serie)
13. **Verdächtige Duos („Ermittlungsakte")** — 🧲 Magnet & Schreck (B fehlt
    auffällig oft, wenn A da ist), 🕵️ Alibi-Duo (gleicher Tag, gleiche
    Ausreden-Kategorie), 📋 Copy-Paste-Duo (wortgleiche Ausrede zweier
    Personen), ☠️ Todesduo (fehlen beide, bricht der Abend messbar ein)
14. **Einsatz-Typen** — 🏛️ Dreamteam (bestes Trio), 🦸 Retter in der Not
    (kommt bei leerer Bude), 🐑 Mitläufer (kommt nur bei voller)
15. **KI-Zusammenfassung** — einer von drei vorformulierten Jahres-Absätzen
16. **Stammtisch-Typen** — Persönlichkeits-Cluster (Der Fels, Der Spontane,
    Der Kreative, Das Phantom)
17. **Quiz** — interaktive Frage mit Auflösen-Button (z. B. „Wer hat nie
    abgesagt?")
18. **Strafenkasse** (Intro + Slide) — Kassenstand mit Zähler,
    Maß-Umrechnung (5 €/Maß), Top-3-Zahler mit Einzelstrafen und Zeiträumen
19. **Awards** (Intro + Slide) — 👑 Stammtisch-König, 🔥 Streak-Meister,
    🎨 Kreativster Absager, 🦅 Comeback des Jahres (längste beendete
    Absage-Serie), 🌟 Rising Star (größte Quoten-Steigerung 2. vs.
    1. Halbjahr, mindestens +10 Punkte)
20. **Finale** — Konfetti + **„Als Bild teilen"**: rendert die Kernzahlen
    als Bild für die WhatsApp-Gruppe (am Handy direkt über den
    System-Share-Dialog, am Desktop als Download)

## Signal-Gates statt erfundener Stories

Jede „Gossip"-Karte hat eine Mindestschwelle (z. B. Ping-Pong erst ab 4
Wochen Wechsel, Todesduo nur wenn der Einbruch über den mechanischen
Zwei-Personen-Effekt hinausgeht, Rising Star nur ab +10 Punkten). Gibt es
kein echtes Signal, entfällt die Karte — und eine Slide ganz ohne Karten
wird gar nicht gezeigt. Lieber weniger Slides als erfundene Muster.

## Datenquelle & Fallback

Rechnet live auf der `zumba`-Datenbank (read-only). Ohne DB-Verbindung
fällt die App **stillschweigend auf Mock-Daten** zurück (15 fiktive
Mitglieder, deterministisch) — gut für Entwicklung, aber: eine hübsche Seite
beweist keine echten Daten. Kontrolle über das Log („Connected to
PostgreSQL" vs. „Using mock data").

## Geplant

- **Personalisierung** (`?wer=…`): eigene „DEIN Jahr"-Sequenz pro Person —
  nächster größerer Schritt.
- **Wrapped 2027**: Timing-Auswertungen dank `created_at` (kurzfristigste
  Absage, Frühwarner vs. Last-Minute, Domino-Effekt).
