## Kontext
Du bist ein Senior-Frontend-Entwickler und Designer und hilfst dabei, eine kleine Webanwendung zu bauen, die ein „Wrapped“-Feature anbietet – ähnlich wie Spotify oder andere Musik-Streaming-Plattformen es am Ende des Jahres tun.

Was du bauen sollst

Die Anwendung, in der Daten gesammelt werden, bezieht sich auf die Teilnahme oder Nicht-Teilnahme am „Stammtisch“. Jede Woche am Donnerstag geben die Nutzer an, ob sie an diesem Tag teilnehmen oder nicht. Standardmäßig wird davon ausgegangen, dass der Nutzer teilnimmt, daher müssen Nutzer explizit eine Nachricht senden, wenn sie nicht teilnehmen.

Was ich haben möchte, ist eine kleine Webanwendung, die den Nutzern ein „Wrapped“-Erlebnis bietet. Die gesammelten Daten sind recht einfach: Eine Nicht-Teilnahme-Nachricht wird gespeichert, wenn der Nutzer eine solche sendet. Teilnahmen werden nicht explizit gespeichert, da sie der Standardfall sind.

Eine Nicht-Teilnahme-Nachricht speichert:
- das Datum
- die ursprüngliche Nachricht des Nutzers
- den Namen des Nutzers

## Vorgehen
- Zu bauen ist eine simple web Anwendung, die wrapped features nacheinander abspielt
- Analysiere hierfür die unten spezifizierten Wrapped feature Ideen
- Implementiere Hauptsächlich die von mir spezifierten wrapped features + 2 eigene von dir
- Es gibt insgesamt 15 user, versuche die Daten bei den pro-User wrapped features immer etwas anders darzustellen, damit es sich nicht wiederholt.
- Bitte arbeite mit dummy Daten, es geht hierbei nur um das Frontend
- Erstelle zunächst einen Plan mit Teilschritten und beschreibe was du tun würdest
- Warte für jeden Teilschritt auf meine Genehmigung

## Wrapped feature Ideen

Folgendes sind Ideen für wrapped pages, und sind nicht vollständig. Gerne auch erweitern und ein bisschen kreativ werden.

🟢 1. Das große Stammtisch-Jahr in Zahlen
Eine Übersichts-Slide:
🍺 X Donnerstage
👥 X Stammtischler
✅ Y Zusagen
❌ Z Absagen
📈 Durchschnittliche Teilnahmequote: 68 %

🏆 2. Der Zuverlässigkeits-Score (Haupt-Ranking)

Für jede Person:
Teilnahmequote in %
Platzierung (1–N)
Spitzname:
„Fels in der Brandung“
„Kommt, wenn’s nicht regnet“
„Mystische Erscheinung“
Beispiel:
Max – 87 % Teilnahme
Titel: Der Wirtshaus-Veteran

🔥 3. Streaks & Serien für jede Person:
Spotify-Style:
🔥 Längste Zusage-Serie
🧊 Längste Absage-Serie
🎯 Nie abgesagt (wenn vorhanden)
💔 Meist nach Zusage nicht erschienen (falls trackbar)

Text:
„3 Wochen in Folge da – das ist Hingabe.“

🤡 4. Ausreden Wrapped
Sehr beliebt, sehr lustig:
🥇 Beste Ausrede des Jahres
🤯 Kreativste Ausrede
💤 Meistgenutzte Ausrede-Kategorie
(müde, Arbeit, Familie, Wetter, „keine Lust“)
📊 Absage-Heatmap (z. B. besonders viele Absagen im Februar)

🍻 5. Der typische Stammtisch
AI-Zusammenfassung:
📆 Beliebtester Donnerstag im Monat
🌧️ Schlechtester Monat für Teilnahme
usw...

Text:
„Im Sommer motiviert, im Winter selektiv.“

🧠 6. Persönliche Wrapped-Slides (pro User)

Wie Spotify „Dein Jahr“:
Dein Teilnahme-Prozent
Dein häufigster Status (Zusage / Absage)
Dein Titel:
„Der Spontane“
„Der Planer“
„Der Vielleicht-Typ“
Persönliche AI-Message:
„2025 war dein Jahr für 37 Maßkrüge – Respekt!“

🏅 7. Die Stammtisch-Awards
Abschluss-Slide:
🏆 Stammtisch-König
🥈 Fast immer da
🥉 Potenzial nach oben
🎭 Ausreden-Legende
👻 Der Unsichtbare

8. Der große Abschluss
Letzte Slide:
„Danke für ein legendäres Stammtisch-Jahr 🍻
2026 wird stärker – donnerstags.“
Optional:
Voting: „Wer holt sich 2026 den Titel?“
Meme oder Emoji-Overkill 😄

## Richtlinien und Guidelines
- Benutze Tailwind
- Halte die Webanwendung simple, kein Framework wenn möglich
- Webpage muss mobile first sein, um auf Smartphones perfekt zu funktionieren
- Einzelne wrapped pages sollten nach einer gewissen Zeit von alleine navigieren
- Benutze Animation, es darf ruhig etwas ausgeflippt sein
- Achte auf eine coole Farbgestaltung
- Bitte orientiere dich an der Nicht-Teilnahme Nachricht um eventuelle weitere wrapped features abzuleiten