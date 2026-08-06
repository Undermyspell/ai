package eval2026

import (
	"testing"
	"time"

	"github.com/michael/stammtisch-wrapped/internal/repository"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Die Anwesenheits-Berechnung (aktiv ab geklemmtem Start, Absagen nur
// aktiver User) lebt in thursday_stats.sql — hier wird nur das Mapping
// auf das Modell getestet.
func TestCalculateThursdayStats(t *testing.T) {
	rawData := &repository.RawData{
		ThursdayStats: []repository.ThursdayAttendance{
			{Day: day(2025, 12, 4), Active: 2, Attendees: 0},
			{Day: day(2025, 12, 11), Active: 2, Attendees: 2},
			{Day: day(2026, 1, 8), Active: 3, Attendees: 2},
			{Day: day(2025, 12, 18), Active: 0, Attendees: 0}, // niemand aktiv -> entfällt
		},
	}

	stats := NewEvaluator(rawData).calculateThursdayStats()

	if len(stats) != 3 {
		t.Fatalf("expected 3 thursdays, got %d", len(stats))
	}
	if stats[0].Attendees != 0 || stats[0].Total != 2 || stats[0].Rate != 0 {
		t.Errorf("04.12.: expected 0/2 (0%%), got %d/%d (%d%%)", stats[0].Attendees, stats[0].Total, stats[0].Rate)
	}
	if stats[1].Attendees != 2 || stats[1].Total != 2 || stats[1].Rate != 100 {
		t.Errorf("11.12.: expected 2/2 (100%%), got %d/%d (%d%%)", stats[1].Attendees, stats[1].Total, stats[1].Rate)
	}
	if stats[2].Attendees != 2 || stats[2].Total != 3 || stats[2].Rate != 66 {
		t.Errorf("08.01.: expected 2/3 (66%%), got %d/%d (%d%%)", stats[2].Attendees, stats[2].Total, stats[2].Rate)
	}
}
