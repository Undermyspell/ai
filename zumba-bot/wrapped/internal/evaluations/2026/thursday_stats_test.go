package eval2026

import (
	"testing"
	"time"

	"github.com/michael/stammtisch-wrapped/internal/repository"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestCalculateThursdayStats(t *testing.T) {
	lateStart := day(2026, 1, 1)
	rawData := &repository.RawData{
		Users: []repository.RawUser{
			{UserID: "a", UserName: "Anna"},
			{UserID: "b", UserName: "Ben"},
			{UserID: "c", UserName: "Carl", StartDate: &lateStart}, // joins later
		},
		Thursdays: []time.Time{
			day(2025, 12, 4),
			day(2025, 12, 11),
			day(2026, 1, 8),
		},
		Rejections: []repository.RawRejection{
			{UserID: "a", Date: day(2025, 12, 4)},
			{UserID: "b", Date: day(2025, 12, 4)},
			{UserID: "c", Date: day(2025, 12, 11)}, // before Carl's start — must not count
			{UserID: "a", Date: day(2026, 1, 8)},
		},
	}

	stats := NewEvaluator(rawData).calculateThursdayStats()

	if len(stats) != 3 {
		t.Fatalf("expected 3 thursdays, got %d", len(stats))
	}

	// 04.12.: 2 active (Carl not yet), 2 absent -> 0 attendees
	if stats[0].Attendees != 0 || stats[0].Total != 2 {
		t.Errorf("04.12.: expected 0/2, got %d/%d", stats[0].Attendees, stats[0].Total)
	}
	// 11.12.: 2 active, Carl's absence ignored -> 2 attendees
	if stats[1].Attendees != 2 || stats[1].Total != 2 {
		t.Errorf("11.12.: expected 2/2, got %d/%d", stats[1].Attendees, stats[1].Total)
	}
	// 08.01.: 3 active, 1 absent -> 2 attendees
	if stats[2].Attendees != 2 || stats[2].Total != 3 {
		t.Errorf("08.01.: expected 2/3, got %d/%d", stats[2].Attendees, stats[2].Total)
	}
	if stats[2].Rate != 66 {
		t.Errorf("08.01.: expected rate 66, got %d", stats[2].Rate)
	}
}
