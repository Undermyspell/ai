WITH thursdays AS (
    SELECT COUNT(*) AS thursday_count
    FROM generate_series(
        date_trunc('year', current_date)::date,
        current_date,
        interval '1 day'
    ) AS d(day)
    WHERE EXTRACT(ISODOW FROM d.day) = 4
)
SELECT 
	Count(*) as away_count,
	t.thursday_count - COUNT(*) as attendance_count,
	ROUND(COUNT(*)::numeric / t.thursday_count * 100, 2) AS attend_percentage,
	a."userId" as user_id, 
	a."userName" as user_name
FROM public.stammtisch_abwesenheit a
CROSS JOIN thursdays t 
group by a."userId", a."userName", t.thursday_count;