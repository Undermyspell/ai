# Iteration 003: Echte PostgresDB anbinden und echtes Datenmodell auswerten

# Ziel 
Aus dem echten Datenmodell und den daraus resultierenden Daten sollen die existierenden Auswertungen und Statistiken ermittelt werden. Das Rohdatenmodell soll dabei immer der Ausgangspunkt sein.

# Vorgehen
- Analysiere die bestehende Applikation
- Analysiere das PostgresSql Schema um ein Rohdatenmodell abzuleiten
- Analysiere die bisherigen Slides + Auswertungen und ermittle wie aus den Rohdaten diese berechnet werden
- Die Auswertungs und Berechnungslogiken sollen dabei modular aufgebaut werden. Dabei orientiere dich an den bestehenden Slides
- Strukturiere dabei auch alles in einen Ordner 2025, da im Folgejahr ganz andere Auswertungen kommen können. Das Rohdatenschema bleibt allerdings immer gleich und wird zwischen den Jahrern wiederverwendet.
- Bewerte bei manchen Auswertungen, ob diese überhaupt aus dem Rohdatenmodell ermittelt werden können. Ein Beispiel könnte das Gruppieren der Absagennachrichten in Kategorieren. Wenn dies in Go möglich ist wäre das gut aber nicht zwingend notwendig und falls nicht möglich kann weiterhin mit Mockdaten befüllt werden.
- Erstelle einen Plan, den ich Schritt für Schritt approve
- Die letztendliche User Experience muss gleich bleiben

# Vorgaben
- Implementiere einen Adapter Service, um die PostgresDb anzubinden
- Implemntiere ein Repository 'RejectionRepository' dass immer die Daten der gesamten Tabelle liest. Als Filter soll dabei immer ein Jahr genommen werden. In unserem Fall aktuell 2025 also alle Einträge aus dem Jahr 2025.
- Der Handler des Endpunkts nutzt das Repository und orchestriert die Auswertungskomponenten um das Gesamtergebnis an die Page zurückzugeben
- Der Datenbankname ist 'zumba'
- WICHTIG: Es ist nicht zwingend, dass jede Statistik oder Auswertung die Rohdaten in Memory im Go Code auswertet. Falls die gewünschte Statistik oder Auswertung mittels SQL Query erledigt werden kann ist das sehr gut. Ein direkte SQL Abfrage soll präferiert werden, falls nicht möglich dann können die Rohdaten im Code ausgewertet werden.

# Existierendes PostgresDb Tabelle und Schema
- In der Datenbanktabelle werden Absagen erfasst
- Es sind nur Donnerstage relevant, alle anderen Tage sollen ignoriert werden
- Wenn ein User keine Zeile an einem Donnerstag hat, dann ist der User anwesend
- Jede Zeile repräsentiert dabei eine Absage eines Users mit Datum, gesendeter Nachricht

## Haupttabelle
CREATE TABLE IF NOT EXISTS public.stammtisch_abwesenheit
(
    "userId" character varying COLLATE pg_catalog."default" NOT NULL,
    date date NOT NULL,
    message character varying COLLATE pg_catalog."default",
    CONSTRAINT stammtisch_abwesenheit_pkey PRIMARY KEY ("userId", date)
)

## Usertabelle
CREATE TABLE IF NOT EXISTS public.users
(
    "userId" character varying COLLATE pg_catalog."default" NOT NULL,
    "userName" character varying COLLATE pg_catalog."default" NOT NULL,
    "startDate" date,
    CONSTRAINT users_pkey PRIMARY KEY ("userId")
)

## Exkludierte Tage Tabelle
CREATE TABLE IF NOT EXISTS public.excluded_days
(
    date date NOT NULL
)
Diese Tabelle beinhaltet Tage, die von jeglicher Auswertung IMMER exkludiert sind. Absagen sowie Zusagen sind für diese Tage irrelevant