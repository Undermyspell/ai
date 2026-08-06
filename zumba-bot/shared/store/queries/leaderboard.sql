-- Rangliste je User: Donnerstage ab effektivem Start (GREATEST(startDate,
-- Periodenstart)), Anwesenheit = Donnerstage - Absagen (attendance-by-default),
-- Streak vorzeichenbehaftet über gaps-and-islands.
-- $1 = Periodenstart, $2 = Stichtag/Periodenende (wird an current_date gekappt).
-- Einzige Kopie dieser Query; früher dupliziert als whatsapp-bot stats.sql,
-- zumba-admin-ui leaderboardQ und n8n whatsapp-statistic.sql.
WITH startdates AS (
    SELECT
        u."userId",
        GREATEST(
            COALESCE(u."startDate", $1::date)::date,
            $1::date
        ) AS effective_start_date
    FROM public.users u
),
user_thursdays AS (
    SELECT
        s."userId",
        s.effective_start_date,
        COUNT(*) AS thursday_count
    FROM startdates s
    CROSS JOIN LATERAL generate_series(
        s.effective_start_date,
        LEAST($2::date, current_date),
        interval '1 day'
    ) d(day)
    LEFT JOIN excluded_days ed
        ON ed.date = d.day
    WHERE EXTRACT(ISODOW FROM d.day) = 4
      AND ed.date IS NULL
    GROUP BY s."userId", s.effective_start_date
),
per_thursday AS (
    SELECT
        s."userId",
        d.day AS thursday,
        CASE WHEN a."userId" IS NOT NULL THEN 1 ELSE 0 END AS is_absent
    FROM startdates s
    CROSS JOIN LATERAL (
        SELECT day
        FROM generate_series(
            s.effective_start_date,
            LEAST($2::date, current_date),
            interval '1 day'
        ) day
        LEFT JOIN excluded_days ed
            ON ed.date = day
        WHERE EXTRACT(ISODOW FROM day) = 4
          AND ed.date IS NULL
    ) d
    LEFT JOIN public.stammtisch_abwesenheit a
        ON a."userId" = s."userId"
        AND a.date = d.day
),
streak_calc AS (
    SELECT
        p."userId",
        p.thursday,
        p.is_absent,
        CASE
            WHEN p.is_absent = first_value(p.is_absent)
                OVER (PARTITION BY p."userId" ORDER BY p.thursday DESC)
            THEN 0
            ELSE 1
        END AS break_flag
    FROM per_thursday p
),
user_streak AS (
    SELECT
        "userId",
        CASE
            WHEN is_absent = 1 THEN -COUNT(*)
            ELSE COUNT(*)
        END AS streak
    FROM (
        SELECT
            sc.*,
            SUM(break_flag) OVER (
                PARTITION BY "userId"
                ORDER BY thursday DESC
                ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
            ) AS grp
        FROM streak_calc sc
    ) x
    WHERE grp = 0
    GROUP BY "userId", is_absent
)
SELECT
    u."userId",
    u."userName",
    u."startDate",
    ut.effective_start_date,
    ut.thursday_count,
    (ut.thursday_count - COUNT(a."userId"))::int AS attendance_count,
    COUNT(a."userId")::int AS away_count,
    CASE WHEN ut.thursday_count = 0 THEN 0
         ELSE ROUND(
             (ut.thursday_count - COUNT(a."userId")::numeric)
             / ut.thursday_count * 100, 2)
    END AS attend_percentage,
    COALESCE(us.streak, 0) AS streak
FROM public.users u
JOIN user_thursdays ut ON ut."userId" = u."userId"
LEFT JOIN public.stammtisch_abwesenheit a
    ON a."userId" = u."userId"
    AND a.date >= ut.effective_start_date
    AND a.date <= LEAST($2::date, current_date)
    AND a.date NOT IN (SELECT date FROM excluded_days)
LEFT JOIN user_streak us ON us."userId" = u."userId"
GROUP BY
    u."userId", u."userName", u."startDate",
    ut.thursday_count, ut.effective_start_date, us.streak
ORDER BY attendance_count DESC, attend_percentage DESC, u."userName"
