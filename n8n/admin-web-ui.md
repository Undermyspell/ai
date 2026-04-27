# Admin Web UI app

## Ziel 
Ich möchte eine Admin Weboberfläche haben mit der ich mein Stammtischdaten außerhalb des automatisierten workflows über n8n verwalten kann.

## Anwendungsfälle
- Nachtragen von Ab- und Abwesenheiten von Mitgliedern bzw Korrekturen
- Eintragen und löschen von excluded days, Tagen an denen kein Stammtisch ist und somit in für die Auswertung ignoriert werden.

## Features 

## Nachtragen und Korrekturen von An- und Abwesenheiten von Mitgliedern
- Absagen werden als Dateneintrag pro Mitglied in einer postgres tabelle festgehalten. Ist an einem Donnerstag (welcher nicht in der excluded days tabelle glistet ist) kein Eintrag für den Benutzer in der Datenbank zählt der Tag als Zusage
- Als Admin soll es mir möglich sein alle meine Mitglieder einsehen zu können sowie die Stammtischtage zu sehen. Es sind dabei immer nur Donnerstage relevant. Ich möchte auf eine einfache Art für die Mitglieder Abwesenheiten einpflegen können für einen bestimmten Stammtischtag. Ich möchte diese aber auch leicht wieder löschen können
- Ich will die Stammtische Tage einsehen können und es soll leicht und gut ersichtlich sein welche Mitglieder an oder abwesend waren.

## Anlegen und Löschen und exkludierten Stammtischtagen
- Ich möchte als Admin exkludierte Donnerstage anlegen, die nicht in der Statistik mit einberechnet werden
- Hierbei brauche ich keinen Mitgliederbezug
- Ich möchte auch eine Übersicht der exkludierten Tage
- Das Anlegen eines exkludierten Tages soll nur für Donnerstage möglich sein

## Dashboard mit Statistik
- Ich möchte ein hübsches Dashboard mit Statistik wo ich sehe wie oft Mitglieder an- und abwesend waren bisher, was ihre Quote und Streak ist, das Leaderboard möchte ich sehen. Dabei kannst du dich an whatsapp-statistic.sql bzw dashboard.js orientieren welche Informationen interessant sind

## Tech Stack und Vorgaben
- Bitte orientiere dich am Techstack, den wir für das wrapped feature verwendet haben (go, templ, air etc...). 
- Die Anwendung soll als Docker image gebaut werden können
- Die Anwendung soll zumba-admin-ui heißen
- Die Anwendung soll ebenfalls in k3s deployed werden (siehe deployment directory)
- WICHTIG: Verwende immer den k8s context rpi5, niemals einen anderen
- Analysiere die bestehenden Anwendungen im Repo
- Analysiere das PostgresDB Schema
- In Phase 1 sind alle Featues aber erstmal nur lesend zu imlementieren. Phase 2 dann die schreibenden. Phase 3 kümmert sich um das Deployment.
- Eigene IngressRoute mit hostname
- Darkmode und lightmode unterstützung
- WICHTIG: Anwendung muss mobile first sein, soll aber auch auf größeren Bildschirmen top aussehen und bedienbar sein

## Vorgehen
- Analyse bestehnder Services und Code sowie DB Schema, verstehen der Fachlichkeit  
- Entwurf eines Plans
- WICHTIG: Verwende den frontend-design skill, um eine schöne UI und UX zu entwerfen