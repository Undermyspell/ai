package web

import (
	"context"
	"time"

	"github.com/michael/zumba-admin-ui/internal/store"
	"github.com/michael/zumba-admin-ui/internal/timeutil"
)

// spyStore implements store.Store: reads return empty/minimal data, writes are recorded.
type spyStore struct {
	insertedAbsence  string // "userId@date"
	deletedAbsence   string
	insertedExcluded string // "YYYY-MM-DD"
	deletedExcluded  string
	users            []store.User
	absences         []store.Absence
}

func newSpyStore() *spyStore {
	return &spyStore{users: []store.User{{ID: "u01", Name: "Max"}}}
}

func (s *spyStore) ListUsers(context.Context) ([]store.User, error) { return s.users, nil }
func (s *spyStore) ListThursdays(_ context.Context, _ timeutil.Period) ([]time.Time, error) {
	return []time.Time{mustDate("2026-01-01")}, nil
}
func (s *spyStore) ListExcludedDays(_ context.Context, _ timeutil.Period) ([]time.Time, error) {
	return nil, nil
}
func (s *spyStore) ListAbsences(_ context.Context, _ timeutil.Period) ([]store.Absence, error) {
	return s.absences, nil
}
func (s *spyStore) Leaderboard(_ context.Context, _ timeutil.Period) ([]store.LeaderboardRow, error) {
	return nil, nil
}
func (s *spyStore) InsertAbsence(_ context.Context, userID string, date time.Time, _ *string) error {
	s.insertedAbsence = userID + "@" + timeutil.FormatISO(date)
	s.absences = append(s.absences, store.Absence{UserID: userID, Date: date})
	return nil
}
func (s *spyStore) DeleteAbsence(_ context.Context, userID string, date time.Time) error {
	s.deletedAbsence = userID + "@" + timeutil.FormatISO(date)
	return nil
}
func (s *spyStore) InsertExcludedDay(_ context.Context, date time.Time) error {
	s.insertedExcluded = timeutil.FormatISO(date)
	return nil
}
func (s *spyStore) DeleteExcludedDay(_ context.Context, date time.Time) error {
	s.deletedExcluded = timeutil.FormatISO(date)
	return nil
}

func (s *spyStore) ListTraces(_ context.Context, _ int) ([]store.Trace, error) { return nil, nil }
func (s *spyStore) GetTrace(_ context.Context, _ int64) (*store.Trace, error)   { return nil, nil }

func mustDate(s string) time.Time {
	d, err := timeutil.ParseISO(s)
	if err != nil {
		panic(err)
	}
	return d
}
