package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	"github.com/michael/zumba-whatsapp-bot/internal/db"
)

//go:embed stats.sql
var statsQuery string

type Postgres struct {
	db *db.Postgres
}

func NewPostgres(p *db.Postgres) *Postgres {
	return &Postgres{db: p}
}

func (s *Postgres) UserStats(ctx context.Context) ([]Stat, error) {
	rows, err := s.db.QueryContext(ctx, statsQuery)
	if err != nil {
		return nil, fmt.Errorf("UserStats: %w", err)
	}
	defer rows.Close()

	var out []Stat
	for rows.Next() {
		var (
			st        Stat
			startDate sql.NullTime
		)
		// Spaltenreihenfolge identisch zu stats.sql:
		// away_count, attendance_count, attend_percentage, user_id,
		// user_name, "startDate", effective_start_date, streak
		if err := rows.Scan(
			&st.Away,
			&st.Attendance,
			&st.Percent,
			&st.UserID,
			&st.Name,
			&startDate,
			&st.EffectiveStart,
			&st.Streak,
		); err != nil {
			return nil, fmt.Errorf("UserStats scan: %w", err)
		}
		if startDate.Valid {
			t := startDate.Time
			st.StartDate = &t
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Postgres) MarkAbsent(ctx context.Context, userID string, date time.Time, message string) error {
	// UPSERT auf (userId, date) – entspricht dem n8n-Node mit matchingColumns
	// userId+date. Setzt eine eindeutige Constraint/Index auf ("userId", date) voraus.
	const q = `
		INSERT INTO public.stammtisch_abwesenheit ("userId", date, message)
		VALUES ($1, $2, $3)
		ON CONFLICT ("userId", date)
		DO UPDATE SET message = EXCLUDED.message
	`
	if _, err := s.db.ExecContext(ctx, q, userID, date, message); err != nil {
		return fmt.Errorf("MarkAbsent: %w", err)
	}
	return nil
}

func (s *Postgres) MarkPresent(ctx context.Context, userID string, date time.Time) error {
	const q = `
		DELETE FROM public.stammtisch_abwesenheit
		WHERE "userId" = $1 AND date = $2
	`
	if _, err := s.db.ExecContext(ctx, q, userID, date); err != nil {
		return fmt.Errorf("MarkPresent: %w", err)
	}
	return nil
}
