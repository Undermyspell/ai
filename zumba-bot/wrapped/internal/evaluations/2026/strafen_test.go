package eval2026

import (
	"testing"
	"time"

	"github.com/michael/zumba-shared/domain"

	"github.com/michael/stammtisch-wrapped/internal/repository"
)

// testSeason ist das Stammtischjahr der Tests (wie 2026 in public.seasons).
// Ohne Season hätten die User keinen Startpunkt und die Serien würden ab dem
// Nulldatum gerechnet.
func testSeason() domain.Season {
	s := domain.Season{Label: "2026"}
	s.Start, s.End = day(2025, 12, 1), day(2026, 11, 30)
	return s
}

// consecutiveThursdays returns n consecutive Thursdays starting 04.12.2025
func consecutiveThursdays(n int) []time.Time {
	out := make([]time.Time, 0, n)
	d := day(2025, 12, 4)
	for range n {
		out = append(out, d)
		d = d.AddDate(0, 0, 7)
	}
	return out
}

func TestCalculateStrafenStats(t *testing.T) {
	thursdays := consecutiveThursdays(8)
	noShowBetrag := 50

	rawData := &repository.RawData{
		Season: testSeason(),
		Users: []repository.RawUser{
			{UserID: "a", UserName: "Anna"},
			{UserID: "b", UserName: "Ben"},
		},
		Thursdays: thursdays,
		// Anna misses 6 Thursdays in a row -> 25 € + 1×5 € = 30 €
		Rejections: []repository.RawRejection{
			{UserID: "a", Date: thursdays[0]},
			{UserID: "a", Date: thursdays[1]},
			{UserID: "a", Date: thursdays[2]},
			{UserID: "a", Date: thursdays[3]},
			{UserID: "a", Date: thursdays[4]},
			{UserID: "a", Date: thursdays[5]},
		},
		StrafenRows: []repository.StrafenRow{
			{ID: 1, UserID: "b", Art: "noshow", Datum: thursdays[2], Betrag: &noShowBetrag, Status: "offen"},
		},
	}

	stats := NewEvaluator(rawData).calculateStrafenStats()

	if stats.TotalCount != 2 {
		t.Fatalf("expected 2 penalties, got %d", stats.TotalCount)
	}
	if stats.TotalSum != 80 {
		t.Errorf("expected total 80 €, got %d", stats.TotalSum)
	}

	// Sorted descending: Ben (50 €) before Anna (30 €)
	if stats.UserTotals[0].UserName != "Ben" || stats.UserTotals[0].Total != 50 {
		t.Errorf("top payer: expected Ben/50, got %s/%d", stats.UserTotals[0].UserName, stats.UserTotals[0].Total)
	}
	anna := stats.UserTotals[1]
	if anna.UserName != "Anna" || anna.Total != 30 || anna.FehltageCount != 1 {
		t.Errorf("expected Anna/30/1 fehltage, got %s/%d/%d", anna.UserName, anna.Total, anna.FehltageCount)
	}

	entry := anna.Entries[0]
	if !entry.Start.Equal(thursdays[0]) {
		t.Errorf("expected streak start %v, got %v", thursdays[0], entry.Start)
	}
	if !entry.End.Equal(thursdays[5]) {
		t.Errorf("expected streak end %v, got %v", thursdays[5], entry.End)
	}
	if entry.Tage != 6 {
		t.Errorf("expected 6 Tage, got %d", entry.Tage)
	}
}

func TestCalculateStrafenStatsExcludesDeleted(t *testing.T) {
	thursdays := consecutiveThursdays(3)
	betrag := 50
	deletedAt := thursdays[2]

	rawData := &repository.RawData{
		Season:    testSeason(),
		Users:     []repository.RawUser{{UserID: "a", UserName: "Anna"}},
		Thursdays: thursdays,
		StrafenRows: []repository.StrafenRow{
			{ID: 1, UserID: "a", Art: "noshow", Datum: thursdays[0], Betrag: &betrag, Status: "geloescht", GeloeschtAm: &deletedAt},
		},
	}

	stats := NewEvaluator(rawData).calculateStrafenStats()
	if stats.TotalCount != 0 {
		t.Errorf("deleted penalties must not appear, got %d entries", stats.TotalCount)
	}
}
