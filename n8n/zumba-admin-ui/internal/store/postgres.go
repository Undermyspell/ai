package store

import (
	"context"
	"fmt"
	"time"

	"github.com/michael/zumba-admin-ui/internal/db"
	"github.com/michael/zumba-admin-ui/internal/timeutil"
)

type Postgres struct {
	db *db.Postgres
}

func NewPostgres(p *db.Postgres) *Postgres {
	return &Postgres{db: p}
}

func (s *Postgres) ListUsers(ctx context.Context) ([]User, error) {
	const q = `
		SELECT "userId", "userName", "startDate"
		FROM users
		ORDER BY "userName"
	`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("ListUsers: %w", err)
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.StartDate); err != nil {
			return nil, fmt.Errorf("ListUsers scan: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Postgres) ListThursdays(ctx context.Context, p timeutil.Period) ([]time.Time, error) {
	const q = `
		WITH all_thursdays AS (
			SELECT d::date AS thursday
			FROM generate_series($1::date, $2::date, interval '1 day') AS d
			WHERE EXTRACT(DOW FROM d) = 4
		)
		SELECT thursday FROM all_thursdays
		WHERE thursday NOT IN (SELECT date FROM excluded_days WHERE date >= $1 AND date <= $2)
		ORDER BY thursday DESC
	`
	rows, err := s.db.QueryContext(ctx, q, p.Start, p.EffectiveEnd())
	if err != nil {
		return nil, fmt.Errorf("ListThursdays: %w", err)
	}
	defer rows.Close()

	var out []time.Time
	for rows.Next() {
		var t time.Time
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("ListThursdays scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Postgres) ListExcludedDays(ctx context.Context, p timeutil.Period) ([]time.Time, error) {
	const q = `
		SELECT date FROM excluded_days
		WHERE date >= $1 AND date <= $2
		ORDER BY date DESC
	`
	rows, err := s.db.QueryContext(ctx, q, p.Start, p.End)
	if err != nil {
		return nil, fmt.Errorf("ListExcludedDays: %w", err)
	}
	defer rows.Close()

	var out []time.Time
	for rows.Next() {
		var t time.Time
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("ListExcludedDays scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Postgres) ListAbsences(ctx context.Context, p timeutil.Period) ([]Absence, error) {
	const q = `
		SELECT "userId", date, message
		FROM stammtisch_abwesenheit
		WHERE date >= $1 AND date <= $2
		  AND EXTRACT(DOW FROM date) = 4
		  AND date NOT IN (SELECT date FROM excluded_days)
		ORDER BY date DESC, "userId"
	`
	rows, err := s.db.QueryContext(ctx, q, p.Start, p.EffectiveEnd())
	if err != nil {
		return nil, fmt.Errorf("ListAbsences: %w", err)
	}
	defer rows.Close()

	var out []Absence
	for rows.Next() {
		var a Absence
		if err := rows.Scan(&a.UserID, &a.Date, &a.Message); err != nil {
			return nil, fmt.Errorf("ListAbsences scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Leaderboard ports whatsapp-statistic.sql verbatim, parameterized over the
// evaluation period. Streak is signed (positive = current attendance run,
// negative = current absence run).
func (s *Postgres) Leaderboard(ctx context.Context, p timeutil.Period) ([]LeaderboardRow, error) {
	const q = `
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
`
	rows, err := s.db.QueryContext(ctx, q, p.Start, p.End)
	if err != nil {
		return nil, fmt.Errorf("Leaderboard: %w", err)
	}
	defer rows.Close()

	var out []LeaderboardRow
	for rows.Next() {
		var r LeaderboardRow
		if err := rows.Scan(
			&r.UserID, &r.UserName, &r.StartDate,
			&r.EffectiveStart, &r.ThursdayCount,
			&r.AttendanceCount, &r.AwayCount,
			&r.AttendPercent, &r.Streak,
		); err != nil {
			return nil, fmt.Errorf("Leaderboard scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
