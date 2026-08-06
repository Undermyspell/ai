package store

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/michael/zumba-shared/domain"
)

//go:embed queries/leaderboard.sql
var leaderboardQ string

// LeaderboardRow ist eine Zeile der Rangliste.
type LeaderboardRow struct {
	UserID          string
	UserName        string
	StartDate       *time.Time
	EffectiveStart  time.Time
	ThursdayCount   int
	AttendanceCount int
	AwayCount       int
	AttendPercent   float64
	// Streak ist vorzeichenbehaftet: >0 = aktuelle Anwesenheits-Serie,
	// <0 = aktuelle Abwesenheits-Serie, 0 = noch keine Donnerstage.
	Streak int
}

func scanLeaderboardRows(ctx context.Context, q Queryer, query string, args ...any) ([]LeaderboardRow, error) {
	rows, err := q.QueryContext(ctx, query, args...)
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

// Leaderboard liefert die Rangliste für den Zeitraum p (Ende wird in SQL an
// current_date gekappt), sortiert nach Anwesenheit, Prozent, Name.
func Leaderboard(ctx context.Context, q Queryer, p domain.Period) ([]LeaderboardRow, error) {
	return scanLeaderboardRows(ctx, q, leaderboardQ, p.Start, p.End)
}

// UserLeaderboardRow filtert die Rangliste in SQL auf einen einzelnen User.
func UserLeaderboardRow(ctx context.Context, q Queryer, p domain.Period, userID string) (LeaderboardRow, error) {
	query := `SELECT * FROM (` + leaderboardQ + `) b WHERE b."userId" = $3`
	rows, err := scanLeaderboardRows(ctx, q, query, p.Start, p.End, userID)
	if err != nil {
		return LeaderboardRow{}, err
	}
	if len(rows) == 0 {
		return LeaderboardRow{}, nil
	}
	return rows[0], nil
}
