-- Anwesenheit je Donnerstag: aktiv = User, deren geklemmter Start erreicht
-- ist; Absagen zählen nur für zu dem Zeitpunkt aktive User
-- (attendance-by-default: anwesend = aktiv - abgemeldet).
-- $1 = Periodenstart (Domänen-Minimum), $2 = Periodenende (an current_date gekappt).
WITH startdates AS (
    SELECT
        u."userId",
        GREATEST(COALESCE(u."startDate", $1::date)::date, $1::date) AS start
    FROM public.users u
),
days AS (
    SELECT d.day::date AS day
    FROM generate_series($1::date, LEAST($2::date, current_date), interval '1 day') d(day)
    WHERE EXTRACT(ISODOW FROM d.day) = 4
      AND d.day::date NOT IN (SELECT date FROM excluded_days)
),
active AS (
    SELECT dy.day, COUNT(*)::int AS active
    FROM days dy
    JOIN startdates s ON s.start <= dy.day
    GROUP BY dy.day
),
absent AS (
    SELECT a.date AS day, COUNT(*)::int AS absent
    FROM public.stammtisch_abwesenheit a
    JOIN startdates s ON s."userId" = a."userId" AND a.date >= s.start
    WHERE a.date >= $1 AND a.date <= LEAST($2::date, current_date)
      AND EXTRACT(ISODOW FROM a.date) = 4
      AND a.date NOT IN (SELECT date FROM excluded_days)
    GROUP BY a.date
)
SELECT act.day,
       act.active,
       GREATEST(act.active - COALESCE(ab.absent, 0), 0) AS attendees
FROM active act
LEFT JOIN absent ab ON ab.day = act.day
ORDER BY act.day
