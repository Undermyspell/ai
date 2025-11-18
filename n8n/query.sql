WITH user_thursdays AS (
    SELECT 
        u."userId",
        COUNT(*) AS thursday_count
    FROM public.users u
    CROSS JOIN LATERAL generate_series(
        /* start date */
        COALESCE(u."startDate", date_trunc('year', current_date))::date,
        /* end date */
        current_date,
        interval '1 day'
    ) AS d(day)
    WHERE EXTRACT(ISODOW FROM d.day) = 4
    GROUP BY u."userId"
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
	u."startDate"::text AS start_date
FROM public.users u
LEFT JOIN public.stammtisch_abwesenheit a 
    ON u."userId" = a."userId"
JOIN user_thursdays ut
    ON ut."userId" = u."userId"
GROUP BY u."userId", u."userName", ut.thursday_count
ORDER BY attendance_count DESC, attend_percentage DESC;
