// Package store is the data-access layer for the admin UI.
// Exposes a Store interface implemented by both Postgres and the mock backend
// so handlers don't care which one is wired up.
package store

import (
	"context"
	"time"

	"github.com/michael/zumba-admin-ui/internal/timeutil"
)

type User struct {
	ID        string
	Name      string
	Emoji     string
	StartDate *time.Time
}

type Absence struct {
	UserID  string
	Date    time.Time
	Message *string
}

type LeaderboardRow struct {
	UserID          string
	UserName        string
	StartDate       *time.Time
	EffectiveStart  time.Time
	ThursdayCount   int
	AttendanceCount int
	AwayCount       int
	AttendPercent   float64
	// Streak is signed: >0 = current attendance run, <0 = current absence run, 0 = no Thursdays yet.
	Streak int
}

// Store is the interface used by handlers. Phase 2 adds the write methods.
type Store interface {
	ListUsers(ctx context.Context) ([]User, error)
	ListThursdays(ctx context.Context, p timeutil.Period) ([]time.Time, error)
	ListExcludedDays(ctx context.Context, p timeutil.Period) ([]time.Time, error)
	ListAbsences(ctx context.Context, p timeutil.Period) ([]Absence, error)
	Leaderboard(ctx context.Context, p timeutil.Period) ([]LeaderboardRow, error)

	InsertAbsence(ctx context.Context, userID string, date time.Time, message *string) error
	DeleteAbsence(ctx context.Context, userID string, date time.Time) error
	InsertExcludedDay(ctx context.Context, date time.Time) error
	DeleteExcludedDay(ctx context.Context, date time.Time) error
}
