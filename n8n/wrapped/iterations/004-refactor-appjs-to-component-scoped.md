# Iteration 004: App.js javascript in die Komponenten verteilen

# Ziel 
Die app.js beinhaltet jeglichen Clientseitigen Javascript Code und ist sehr unübersichtlich und schwer zu warten. Der Javascript Code soll auf die jeweiligen Components (.templ) files aufgeteilt werden und zwar wo der Code jeweils gebraucht wird. Dadurch erreichen wir eine besser Modulalisierung und verbessern die Wartbarkeit der Anwendung. Ebenso sollte ein .templ File nur genau eine Component enthalten.

# Vorgehen
- Analysiere die bisherigen Komponenten (Slides, Pages)
- Refactor jedes .templ file und lege für jede Komponente ein eigens file an
- Refactor die app.js und verteile den Code auf die Komponenten
- Für templ Dokumentation und wie man javascript integriert verwende tools von Context7 oder https://templ.guide/syntax-and-usage/script-templates/

# Akzeptanz Kriterien
- Die app.js wie sie jetzt existiert sollte nicht mehr gebraucht werden
- Die User Experience muss exakt gleich bleiben