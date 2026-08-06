-- Längste Anwesenheits- bzw. Absage-Serie je User (gaps-and-islands):
-- pro User und Zustand (anwesend/abwesend) die längste zusammenhängende
-- Donnerstags-Serie mit Start-/Enddatum. Donnerstage zählen ab dem
-- geklemmten Start des Users; bei Gleichstand gewinnt die frühere Serie.
-- $1 = Periodenstart (Domänen-Minimum), $2 = Periodenende (an current_date gekappt).
WITH startdates AS (
    SELECT
        u."userId",
        GREATEST(COALESCE(u."startDate", $1::date)::date, $1::date) AS start
    FROM public.users u
),
days AS (
    SELECT s."userId", d.day::date AS day
    FROM startdates s
    CROSS JOIN LATERAL generate_series(
        s.start, LEAST($2::date, current_date), interval '1 day'
    ) d(day)
    WHERE EXTRACT(ISODOW FROM d.day) = 4
      AND d.day::date NOT IN (SELECT date FROM excluded_days)
),
marked AS (
    SELECT dy."userId", dy.day,
           (a."userId" IS NOT NULL) AS is_absent
    FROM days dy
    LEFT JOIN public.stammtisch_abwesenheit a
        ON a."userId" = dy."userId" AND a.date = dy.day
),
islands AS (
    SELECT *,
           ROW_NUMBER() OVER (PARTITION BY "userId" ORDER BY day)
         - ROW_NUMBER() OVER (PARTITION BY "userId", is_absent ORDER BY day) AS grp
    FROM marked
),
runs AS (
    SELECT "userId", is_absent,
           COUNT(*)::int AS len,
           MIN(day) AS start_day,
           MAX(day) AS end_day
    FROM islands
    GROUP BY "userId", is_absent, grp
)
SELECT DISTINCT ON ("userId", is_absent)
    "userId", is_absent, len, start_day, end_day
FROM runs
ORDER BY "userId", is_absent, len DESC, start_day
