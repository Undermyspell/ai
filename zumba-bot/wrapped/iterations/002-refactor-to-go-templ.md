# Iteration 002: Refactoring zu Go + templ Part2

# Ziel 
Refactoring der Anwendung sodass nicht nur ein index.html verwendet wird sondern für jeden Slide eine separate Seite. Dies soll ein bessere Strukturierung gewährleisten und erweiterbar werden. Es soll möglich sein die Wrapped Jahre separat aufzurufen. Das bedeutet, wenn man die url mit /2025 aufruft bekommt man das wrapped für 2025. Für weitere Jahre funktioniert das analag.

# Vorgehen
- Analyziere die bestehended Applikation
- Mache einen Plan wie du indext.html und die app.js sowie das styles.css so aufteilst, dass zum einen wiederverwendbare styles enstehen für Seiten und auch Jahre übergreifend. Zudemn sollten Styles und Animationen, die Seiten spezifisch sind auch auf dieser Seite gekapselt sein (wie Komponenten eben).
- Präsentiere mir die Schritte und erwarte vor jedem Schritte meine Bestätigung
- Überprüfe am Ende ob die User Experience am Ende exakt genauso ist wie vor dem Umbau. Hierfür nutze die playwright tools.

# Vorgaben
- Führe templ ein, um einzelne Components und Pages abzubilden (https://templ.guide/). Für aktuelle Dokumentation nutze die Tools von context7
- Für styles sollte weiterhin tailwindcss genutzt werden
- (Optional) Nutze alpine.js oder htmx falls es Sinn macht, aber forciere es nicht.

# Mögliche Ordnerstrukt Beispiel für Web
web/
└── templates/
    ├── layout.templ             
    └── years/
        ├── 2024/
        │   ├── layout.templ      
        │   ├── slides/
        │   │   ├── intro.templ
        │   │   ├── overview.templ
        │   │   ├── ranking.templ
        │   │   ├── streaks.templ
        │   │   ├── excuses.templ
        │   │   └── awards.templ
        │   └── components/
        │       ├── navigation.templ
        │       ├── progress-bar.templ
        │       └── ranking-card.templ
        └── 2025/
            ├── layout.templ
            ├── slides/
            │   ├── intro.templ
            │   ├── overview.templ
            │   ├── ranking.templ
            │   ├── streaks.templ
            │   ├── excuses.templ
            │   └── awards.templ
            └── components/
                ├── navigation.templ
                ├── progress-bar.templ
                └── ranking-card.templ
- Die aktuelle Anwendung bildet ein Jahr 2025 und du sollst auch nur das integrieren