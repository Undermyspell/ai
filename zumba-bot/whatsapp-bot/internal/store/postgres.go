package store

import (
	"context"
	"fmt"
	"time"

	"github.com/michael/zumba-shared/domain"
	"github.com/michael/zumba-shared/penalty"
	sharedstore "github.com/michael/zumba-shared/store"

	"github.com/michael/zumba-whatsapp-bot/internal/db"
)

type Postgres struct {
	db *db.Postgres
}

func NewPostgres(p *db.Postgres) *Postgres {
	return &Postgres{db: p}
}

// UserStats nutzt die geteilte Leaderboard-Query (shared/store/queries/
// leaderboard.sql – früher eigene stats.sql-Kopie): Periodenstart ist der
// Domänen-Mindeststart 2025-12-01, Ende der Stichtag asOf.
func (s *Postgres) UserStats(ctx context.Context, asOf time.Time) ([]Stat, error) {
	period := domain.Period{Start: penalty.ClampStart(nil), End: asOf}
	rows, err := sharedstore.Leaderboard(ctx, s.db, period)
	if err != nil {
		return nil, fmt.Errorf("UserStats: %w", err)
	}

	out := make([]Stat, 0, len(rows))
	for _, r := range rows {
		out = append(out, Stat{
			UserID:         r.UserID,
			Name:           r.UserName,
			StartDate:      r.StartDate,
			EffectiveStart: r.EffectiveStart,
			Attendance:     r.AttendanceCount,
			Away:           r.AwayCount,
			Percent:        r.AttendPercent,
			Streak:         r.Streak,
		})
	}
	return out, nil
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
