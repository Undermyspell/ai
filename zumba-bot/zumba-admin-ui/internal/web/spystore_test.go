package web

import (
	"context"
	"time"

	"github.com/michael/zumba-admin-ui/internal/penalty"
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

	strafen          []penalty.Row
	nextStrafeID     int64
	beglichenStrafe  int64
	geloeschteStrafe int64
}

func newSpyStore() *spyStore {
	return &spyStore{users: []store.User{{ID: "u01", Name: "Max"}}}
}

func (s *spyStore) ListUsers(context.Context) ([]store.User, error) { return s.users, nil }
func (s *spyStore) GetUser(_ context.Context, userID string) (*store.User, error) {
	for i := range s.users {
		if s.users[i].ID == userID {
			u := s.users[i]
			return &u, nil
		}
	}
	return nil, nil
}
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
func (s *spyStore) UserLeaderboardRow(_ context.Context, _ timeutil.Period, _ string) (store.LeaderboardRow, error) {
	return store.LeaderboardRow{}, nil
}
func (s *spyStore) ListUserAbsences(_ context.Context, _ timeutil.Period, userID string) ([]store.Absence, error) {
	var out []store.Absence
	for _, a := range s.absences {
		if a.UserID == userID {
			out = append(out, a)
		}
	}
	return out, nil
}
func (s *spyStore) AbsencesOn(_ context.Context, date time.Time) ([]store.Absence, error) {
	var out []store.Absence
	for _, a := range s.absences {
		if timeutil.FormatISO(a.Date) == timeutil.FormatISO(date) {
			out = append(out, a)
		}
	}
	return out, nil
}
func (s *spyStore) IsExcludedDay(_ context.Context, _ time.Time) (bool, error) { return false, nil }
func (s *spyStore) ThursdayStrip(_ context.Context, _ timeutil.Period, _ int) ([]store.StripDay, error) {
	return nil, nil
}
func (s *spyStore) ListDayAbsences(_ context.Context, _ timeutil.Period) ([]store.DayAbsences, error) {
	return nil, nil
}
func (s *spyStore) ToggleAbsence(ctx context.Context, userID string, date time.Time) (bool, error) {
	for _, a := range s.absences {
		if a.UserID == userID && timeutil.FormatISO(a.Date) == timeutil.FormatISO(date) {
			return false, s.DeleteAbsence(ctx, userID, date)
		}
	}
	return true, s.InsertAbsence(ctx, userID, date, nil)
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
func (s *spyStore) GetTrace(_ context.Context, _ int64) (*store.Trace, error)  { return nil, nil }

func mustDate(s string) time.Time {
	d, err := timeutil.ParseISO(s)
	if err != nil {
		panic(err)
	}
	return d
}

func (s *spyStore) ListMLMessages(_ context.Context, _ bool, _ int) ([]store.MLMessage, error) {
	return nil, nil
}
func (s *spyStore) MLShadowStats(_ context.Context) (store.MLShadowStats, error) {
	return store.MLShadowStats{}, nil
}
func (s *spyStore) VerifyMLMessage(_ context.Context, id int64, correctedLabel *string) (*store.MLMessage, error) {
	return &store.MLMessage{ID: id, Verified: true, CorrectedLabel: correctedLabel}, nil
}

func (s *spyStore) InsertMLTest(_ context.Context, _, _ string, _ float64) (int64, error) {
	return 1, nil
}
func (s *spyStore) ListMLTests(_ context.Context, _ int) ([]store.MLTestMessage, error) {
	return nil, nil
}
func (s *spyStore) JudgeMLTest(_ context.Context, id int64, expectedLabel string) (*store.MLTestMessage, error) {
	return &store.MLTestMessage{ID: id, ExpectedLabel: &expectedLabel}, nil
}

func (s *spyStore) DeleteMLTest(_ context.Context, _ int64) error { return nil }

func (s *spyStore) ListStrafen(_ context.Context) ([]penalty.Row, error) { return s.strafen, nil }
func (s *spyStore) InsertAutoStrafe(_ context.Context, userID string, datum time.Time) error {
	for _, r := range s.strafen {
		if r.Art == penalty.ArtFehltage && r.UserID == userID && timeutil.FormatISO(r.Datum) == timeutil.FormatISO(datum) {
			return nil
		}
	}
	s.nextStrafeID++
	s.strafen = append(s.strafen, penalty.Row{
		ID: s.nextStrafeID, UserID: userID, Art: penalty.ArtFehltage,
		Datum: datum, Status: penalty.StatusOffen,
	})
	return nil
}
func (s *spyStore) InsertNoShowStrafe(_ context.Context, userID string, datum time.Time, betrag int) error {
	s.nextStrafeID++
	s.strafen = append(s.strafen, penalty.Row{
		ID: s.nextStrafeID, UserID: userID, Art: penalty.ArtNoShow,
		Datum: datum, Betrag: betrag, Status: penalty.StatusOffen,
	})
	return nil
}
func (s *spyStore) BegleicheStrafe(_ context.Context, id int64) error {
	s.beglichenStrafe = id
	now := time.Now()
	for i := range s.strafen {
		if s.strafen[i].ID == id {
			s.strafen[i].Status = penalty.StatusBeglichen
			s.strafen[i].BeglichenAm = &now
		}
	}
	return nil
}
func (s *spyStore) LoescheStrafe(_ context.Context, id int64) error {
	s.geloeschteStrafe = id
	now := time.Now()
	for i := range s.strafen {
		if s.strafen[i].ID == id {
			s.strafen[i].Status = penalty.StatusGeloescht
			s.strafen[i].GeloeschtAm = &now
		}
	}
	return nil
}
