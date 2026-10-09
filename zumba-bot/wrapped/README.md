# Stammtisch Wrapped 2025 🍺

Eine Wrapped-Style Web-Applikation für Stammtisch-Statistiken, gebaut mit Go.

## Features

- **19 interaktive Slides** mit Animationen
- **Automatische Navigation** mit Progress Bar
- **Touch/Swipe Support** für mobile Geräte
- **Ranking-System** für Teilnehmer
- **Streak-Tracking** (Anwesenheit & Absagen)
- **Ausreden-Analyse** mit Kategorisierung
- **Awards & Persönlichkeitstypen**
- **Responsive Design** (Mobile First)

## Tech Stack

- **Backend**: Go (Golang)
- **Frontend**: templ, eigenes Stylesheet im Look des Admin-Portals (`assets/static/css/styles.css`), Plus Jakarta Sans + JetBrains Mono, wenig Vanilla-JS (Story-Navigation im Layout)
- **Hot Reload**: Air (Development)
- **Server**: Standard Go `net/http`

## Installation

### Voraussetzungen

- Go 1.21 oder höher
- Air (optional, für Hot Reload)

### Setup

```bash
# Dependencies installieren
go mod download

# Air installieren (optional)
go install github.com/air-verse/air@latest
```

## Entwicklung

### Mit Air (Hot Reload)

```bash
air
```

Die Applikation ist dann verfügbar unter: `http://localhost:8080`

### Ohne Air

```bash
# Server bauen
go build -o ./tmp/server ./cmd/server

# Server starten
./tmp/server
```

## Produktiv-Build

```bash
# Binary erstellen
go build -o wrapped ./cmd/server

# Server starten
./wrapped
```

Die Applikation läuft standardmäßig auf Port `8080`. Der Port kann über die Environment Variable `PORT` angepasst werden:

```bash
PORT=3000 ./wrapped
```

## Projekt-Struktur

```
wrapped/
├── cmd/
│   └── server/          # HTTP Server
│       └── main.go
├── data/                # Mock-Daten Generator
│   └── mock.go
├── pkg/
│   └── models/          # Datenmodelle
│       ├── user.go
│       └── stats.go
├── static/
│   ├── css/
│   │   └── styles.css   # Animationen & Custom CSS
│   └── js/
│       ├── app.js       # Frontend-Logik
│       └── data.js      # Mock-Daten
├── legacy/              # Original HTML/JS Dateien
│   ├── index.html       # Haupt-Template
│   ├── app.js           # Original JavaScript
│   ├── data.js          # Original Daten
│   └── README.md
├── web/
│   └── templates/       # templ Templates (in Entwicklung)
├── go.mod
└── .air.toml           # Air Konfiguration
```

## Features im Detail

### Slides

1. **Intro** - Willkommensseite
2. **Jahresübersicht** - Gesamtstatistiken
3. **Ranking Intro**
4. **Top 5 Ranking** - Die Zuverlässigsten
5. **Plätze 6-10** - Mittelfeld
6. **Plätze 11-15** - Aufsteiger
7. **Streaks Intro**
8. **Längste Streaks** - Anwesenheit & Absagen
9. **Ausreden Intro**
10. **Ausreden-Kategorien** - Statistik
11. **Beste Ausreden** - Highlights
12. **Heatmap** - Absagen nach Monat
13. **AI-Zusammenfassung** - Jahresrückblick
14-16. **Persönliche Statistiken** - Pro User
17. **Persönlichkeitstypen** - Stammtisch-Archetypen
18. **Awards Intro**
19. **Awards** - Schmähpreise, dann Ehrenpreise (Gewinner + Wert, Gleichstand teilt)
20. **Outro** - Abschluss & Danke

### Animationen

- Fade In/Out
- Scale In
- Slide Left/Right
- Bounce
- Float
- Glow
- Fire Effect
- Counter Animationen

### Interaktion

- **Klick/Tap** - Nächste Slide
- **Pfeiltasten** - Navigation
- **Swipe** - Mobile Gesten
- **Dots** - Direkte Slide-Auswahl
- **ESC** - Pause/Resume

## Konfiguration

### Slide-Duration

Jede Slide hat ein `data-duration` Attribut (in Millisekunden):

```html
<div class="slide" data-duration="7000">
  <!-- Slide bleibt 7 Sekunden sichtbar -->
</div>
```

### Gestaltung

Farben, Schriften und Glas-Flächen kommen 1:1 aus dem Admin-Portal (dieselben
CSS-Variablen), hell und dunkel mit Umschalter (folgt sonst der
Systemeinstellung, Wahl wird gemerkt). Jede Slide trägt einen Ton
(`data-tone="blue|cyan|green|amber|red|neutral"`), der Glow, Akzente und
Zahlenfarbe der Szene bestimmt. Der Go-Code liefert nur semantische Werte
(`Tone`, `BarTone` = `is-good|is-ok|is-warn|is-bad`, Heatmap-`Level`
`lv-0…lv-4`, Rangliste-`Tier`), keine CSS-Framework-Klassen. Gestaffelte
Auftritte über `delay-0 … delay-3000` (50-ms-Schritte). Design-Kontext:
`.impeccable.md` im Repo-Root.

Bedienung wie bei Stories: Tippen rechts = weiter, links = zurück, Wischen,
Halten = Pause, Esc = Pause an/aus.

## Mock-Daten

Die Applikation verwendet Mock-Daten für 15 Stammtisch-Teilnehmer über 51 Donnerstage in 2025.

### Datenstruktur

- **User**: ID, Name, Emoji
- **Cancellation**: Datum, UserID, Message, Category
- **Stats**: Teilnahmequote, Streaks, Rankings

### Kategorien

- Arbeit 💼
- Familie 👨‍👩‍👧
- Gesundheit 🤒
- Müdigkeit 😴
- Wetter 🌧️
- Freizeit 🎉
- Kreativ 🎨
- Keine Lust 😬

## Deployment

### Docker (TODO)

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o wrapped ./cmd/server

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/wrapped .
COPY --from=builder /app/static ./static
COPY --from=builder /app/index.html .
EXPOSE 8080
CMD ["./wrapped"]
```

### Systemd Service (TODO)

```ini
[Unit]
Description=Stammtisch Wrapped
After=network.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/wrapped
ExecStart=/opt/wrapped/wrapped
Restart=always

[Install]
WantedBy=multi-user.target
```

## TODO / Roadmap

- [ ] templ Integration vollständig umsetzen
- [ ] Alpine.js für Interaktivität
- [ ] HTMX für dynamisches Content-Laden
- [ ] PostgreSQL Integration
- [ ] User Authentication
- [ ] Admin Dashboard
- [ ] Export als PDF/Video
- [ ] Mehrsprachigkeit

## Lizenz

MIT

## Credits

Entwickelt mit ❤️ und 🍺

## Credits

Die großen Emoji-Grafiken der Kapitel-Öffner (`assets/static/img/emoji/*.svg`)
sind [Noto Emoji](https://github.com/googlefonts/noto-emoji) von Google,
lizenziert unter [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/)
(bezogen über `fonts.gstatic.com/s/e/notoemoji/latest/<codepoint>/emoji.svg`).
Als SVG bleiben sie auf hochauflösenden Displays scharf – die Emoji-Schrift
selbst ist ein Pixelformat und wird beim Hochskalieren unscharf.
