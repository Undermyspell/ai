# Iteration 001: Refactoring zu Go + templ

## Ziel
Refactoring der bestehenden HTML/JavaScript Webanwendung zu einer vollständigen Go-Applikation mit templ als Template-Engine.

## Ausgangslage
- Bestehende Single-Page-Application (SPA) mit:
  - `index.html` - Hauptseite mit TailwindCSS und Animationen
  - `app.js` - JavaScript Logik für Slide-Navigation und Animationen
  - `data.js` - Mock-Daten für Stammtisch-Statistiken
  - `PRD.md` - Produktanforderungen

## Zielarchitektur

### Tech Stack
- **Backend**: Go (Golang)
- **Template Engine**: [templ](https://templ.guide/)
- **Styling**: TailwindCSS (integriert über CDN oder Build-Prozess)
- **Web Server**: Standard Go `net/http` Package
- **Hot Reload** Air für Development

### Projektstruktur
```
wrapped/
├── cmd/
│   └── server/
│       └── main.go           # Einstiegspunkt
├── internal/
│   └── handlers/
│       ├── wrapped.go        # HTTP Handler
│       └── static.go         # Static Asset Handler
├── pkg/
│   └── models/
│       ├── user.go           # User Datenmodelle
│       ├── stats.go          # Statistik-Modelle
│       └── cancellation.go   # Absagen-Modelle
├── data/
│   └── mock.go               # Mock-Daten (Migration von data.js)
├── web/
│   └── templates/
│       ├── layout.templ      # Basis-Layout
│       ├── slides/
│       │   ├── intro.templ
│       │   ├── overview.templ
│       │   ├── ranking.templ
│       │   ├── streaks.templ
│       │   ├── excuses.templ
│       │   └── awards.templ
│       └── components/
│           ├── navigation.templ
│           ├── progress-bar.templ
│           └── ranking-card.templ
├── static/
│   ├── css/
│   │   └── styles.css        # Custom CSS + Animationen
│   └── js/
│       └── app.js            # Client-seitige Logik
├── go.mod
├── go.sum
└── README.md
```

## Implementierungsschritte

### Phase 0: Analyze
- [] Analyze der bestehenden Applikation, die sich direkt im Order 'wrapped' befindet und über die index.html gestartet werden kann
- [] Für den Endnutzer soll die Anwendung sich EXAKT so verhalten wie bisher
- [] Es müssen die gleichen Features, Seiten etc. implementiert sein
- [] Es können Mock Daten verwendet werden wie bisher auch schon

### Phase 1: Projekt-Setup
- [ ] Go Module initialisieren (`go mod init`)
- [ ] Dependencies installieren:
  - `github.com/a-h/templ` - Template Engine
  - `github.com/cosmtrek/air` - Hot Reload
- [ ] Basis-Projektstruktur erstellen
- [ ] `.gitignore` für Go-Projekte anpassen

### Phase 2: Datenmodelle
- [ ] User-Model erstellen (`pkg/models/user.go`)
  - ID, Name, Emoji
  - Berechnung von Statistiken (Teilnahmequote, Streaks)
- [ ] Cancellation-Model (`pkg/models/cancellation.go`)
  - Datum, UserID, Message, Category
- [ ] Stats-Model (`pkg/models/stats.go`)
  - Globale Statistiken
  - User-Statistiken
  - Kategorie-Auswertungen
- [ ] Mock-Daten-Generator (`data/mock.go`)
  - Migration der JavaScript-Logik aus `data.js`
  - Donnerstags-Berechnung für 2025
  - Generierung von realistischen Absagen

### Phase 3: templ Templates
- [ ] Basis-Layout (`web/templates/layout.templ`)
  - HTML Head mit TailwindCSS
  - Body-Struktur
  - Progress Bar
  - Navigation Dots
- [ ] Slide-Templates erstellen:
  - Intro-Slide mit Animation
  - Overview-Slide (Jahresstatistiken)
  - Ranking-Slides (Top 5, Mid, Bottom)
  - Streaks-Slide
  - Excuses-Slides
  - Personal-Slides
  - Awards-Slide
  - Outro-Slide
- [ ] Wiederverwendbare Components:
  - Ranking-Card Component
  - Stat-Box Component
  - Excuse-Card Component

### Phase 4: Backend-Logik
- [ ] HTTP Server Setup (`cmd/server/main.go`)
  - Router einrichten
  - Port-Konfiguration
  - Graceful Shutdown
- [ ] Handlers (`internal/handlers/`)
  - Wrapped-Handler für Haupt-Route
  - Static-Assets-Handler
  - API-Endpoints für dynamische Daten (optional)
- [ ] Statistik-Berechnungen
  - Rankings berechnen
  - Streaks ermitteln
  - Ausreden kategorisieren
  - Heatmap-Daten generieren

### Phase 5: Frontend-Integration
- [ ] CSS-Animationen portieren
  - Keyframe-Animationen aus `index.html`
  - Custom Tailwind-Konfiguration
  - Gradient-Backgrounds
- [ ] JavaScript-Logik anpassen
  - Slide-Navigation
  - Counter-Animationen
  - Auto-Play Funktionalität
  - Touch/Swipe-Events
- [ ] Progressive Enhancement
  - Funktionalität ohne JavaScript sicherstellen
  - Server-seitiges Rendering optimieren
- [ ] Für Frontend Interaktion alpine.js verwenden https://alpinejs.dev/
- [ ] Für das laden von Inhalten, welche über das navigieren von Seiten hinausgeht soll HTMX eingesetz werden https://htmx.org/

### Phase 6: Build & Deployment
- [ ] templ Build-Prozess einrichten
  - `templ generate` in Build-Pipeline
  - Watch-Mode für Development
- [ ] Static Assets bundeln
  - TailwindCSS Build (optional)
  - Asset-Minifizierung
- [ ] Docker Setup (optional)
  - Multi-stage Build
  - Kleines Production Image
- [ ] Deployment-Dokumentation
  - Environment Variables
  - Port-Konfiguration
  - Systemd Service (optional)

## Vorteile der Migration

### Performance
- **Server-Side Rendering**: Schnellere Initial-Page-Load
- **Kompilierte Templates**: templ generiert Go-Code (type-safe)
- **Single Binary**: Einfaches Deployment ohne Node.js

### Entwicklung
- **Type Safety**: Go's Type-System verhindert Runtime-Errors
- **Hot Reload**: Schnelle Entwicklungszyklen mit Air
- **Bessere IDE-Unterstützung**: Autocomplete für Templates

### Wartbarkeit
- **Modularität**: Klare Trennung von Models, Handlers, Templates
- **Testbarkeit**: Go's Testing-Framework für Backend-Logik
- **Skalierbarkeit**: Leichter zu erweitern mit API-Endpoints

## Technische Entscheidungen

### Warum templ?
- Type-safe Templates (compile-time errors)
- Komponenten-basiert (wie React)
- Keine String-Interpolation wie in `html/template`
- Hot-Reload während Development
- Generiert effizienten Go-Code

### Warum TailwindCSS beibehalten?
- Bestehende Styles können größtenteils übernommen werden
- Rapid Prototyping
- Mobile-First Design
- Konsistente Design-Language

### JavaScript-Anteil minimieren?
- Core-Navigation kann serverseitig bleiben
- Animationen und Interaktionen benötigen JS
- Progressive Enhancement für bessere UX

## Offene Fragen

1. **Datenbank-Integration**: Sollen echte Daten aus einer DB kommen?
   - SQLite für lokale Entwicklung
   - PostgreSQL für Production
   - Vorerst Mock-Daten ausreichend

2. **API-Endpoints**: REST API für externe Clients?
   - Optional für zukünftige Mobile-App
   - Zunächst nur Web-UI

3. **Authentication**: User-Login erforderlich?
   - Zunächst öffentlich zugänglich
   - Später: Basic Auth oder OAuth

4. **Real-time Updates**: WebSockets für Live-Daten?
   - Nicht notwendig für statische Wrapped-Daten
   - Evtl. für zukünftige Features

1. Bestätigung des Ansatzes
2. Detaillierung der ersten Phase
3. Beginn mit Projekt-Setup
4. Iterative Implementierung pro Phase

## Referenzen

- [templ Documentation](https://templ.guide/)
- [Go Standard Library](https://pkg.go.dev/std)
- [TailwindCSS](https://tailwindcss.com/)
- [Go Project Layout](https://github.com/golang-standards/project-layout)
- [Alpine Js](https://alpinejs.dev/)
- [Htmx](https://htmx.org/)

## MCP Tools
- Playwright zum aufrufen und analysieren der Seite
- Context7, um aktuelle Docs abzurufen