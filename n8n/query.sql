WITH startdates AS (
    SELECT
        u."userId",
        GREATEST(
            COALESCE(u."startDate", DATE '2025-12-01')::date,
            DATE '2025-12-01'
        ) AS effective_start_date
    FROM public.users u
),

-- Count all valid Thursdays for each user (excluding excluded_days)
user_thursdays AS (
    SELECT 
        s."userId",
        s.effective_start_date,
        COUNT(*) AS thursday_count
    FROM startdates s
    CROSS JOIN LATERAL generate_series(
        s.effective_start_date,
        current_date,
        interval '1 day'
    ) d(day)
    LEFT JOIN excluded_days ed
        ON ed.date = d.day
    WHERE EXTRACT(ISODOW FROM d.day) = 4
      AND ed.date IS NULL
    GROUP BY s."userId", s.effective_start_date
),

-- Build per-Thursday attendance status (0 = attended, 1 = absent)
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
            current_date,
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

-- Compute streak: compare each row with the first row in descending order
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

-- Collapse to first break and compute streak count
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
    COUNT(a."userId") AS away_count,
    ut.thursday_count - COUNT(a."userId") AS attendance_count,
    ROUND(
        (ut.thursday_count - COUNT(a."userId")::numeric) 
        / ut.thursday_count * 100,
        2
    ) AS attend_percentage,
    u."userId" AS user_id,
    u."userName" AS user_name,
    u."startDate",
    ut.effective_start_date,
    us.streak
FROM public.users u
JOIN user_thursdays ut 
    ON ut."userId" = u."userId"
LEFT JOIN public.stammtisch_abwesenheit a 
    ON a."userId" = u."userId"
    AND a.date >= ut.effective_start_date
LEFT JOIN excluded_days ed
    ON ed.date = a.date
LEFT JOIN user_streak us 
    ON us."userId" = u."userId"
WHERE ed.date IS NULL
GROUP BY 
    u."userId", 
    u."userName", 
    ut.thursday_count, 
    ut.effective_start_date,
    us.streak
ORDER BY attendance_count DESC, attend_percentage DESC;
