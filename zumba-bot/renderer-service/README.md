# zumba-renderer

Winziger HTTP-Dienst: rendert selbst mitgebrachtes HTML per headless Chromium
(chromedp) zu einem PNG. Der whatsapp-bot nutzt ihn, um die Statistik als
Bild-Karte zu verschicken (`internal/report/card.go` baut das HTML, der Bot
schickt das PNG via Evolution `sendMedia`).

## API

| Route          | Beschreibung                                                        |
| -------------- | ------------------------------------------------------------------- |
| `POST /render` | Body `{"html": "...", "width": 720, "scale": 2}` → `image/png`      |
| `GET /healthz` | Liveness                                                             |

`width` = CSS-Pixel des Viewports (Default 720), `scale` = deviceScaleFactor
(Default 2, Retina-Schärfe). Die Bildhöhe folgt der Dokumenthöhe
(Full-Page-Screenshot).

## Design-Entscheidungen

- **Chromium pro Request**: Prozess startet und endet je Aufruf — kein
  Idle-RAM auf dem Pi, keine Zombie-Tabs. 1–2 s Startzeit sind für
  Wochenreport/Bot-Test irrelevant. Aufrufe sind serialisiert (Mutex).
- **Debian-Basisimage**: `chromium` + `fonts-noto-color-emoji` sind auf
  amd64 und arm64 verlässlich paketiert (🥇🔥❄️ im Ranking brauchen die
  Color-Emoji-Fonts).
- Kein Zugriff auf externe Ressourcen nötig: das HTML muss self-contained
  sein (Inline-CSS, System-Fonts).

## Lokal

```bash
go run ./cmd/server                # nutzt lokal installiertes chromium/chrome
CHROME_PATH=/usr/bin/google-chrome go run ./cmd/server
```

Docker (Port 8091, wie in ../docker-compose.yml):

```bash
docker build -f Dockerfile -t zumba-renderer:dev .
docker run --rm -p 8091:8080 zumba-renderer:dev
curl -s localhost:8091/render -d '{"html":"<h1 style=\"font-size:80px\">🍻 Test</h1>"}' -o /tmp/test.png
```
