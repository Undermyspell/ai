package eval2026

import (
	"slices"
	"testing"

	"github.com/michael/stammtisch-wrapped/pkg/models"
)

// Plätze wie im Sport: gleiche exakte Quote und gleich viele Anwesenheiten =
// gleicher Platz, danach entfallen Plätze. Die gerundete Quote allein reicht
// nicht für Gleichstand.
func TestSortAndRankWieImSport(t *testing.T) {
	u := func(name string, pct float64, att int) models.UserStats {
		return models.UserStats{User: models.User{Name: name}, AttendancePercent: pct, AttendanceRate: int(pct), AttendanceCount: att}
	}
	stats := []models.UserStats{
		u("Didi", 70, 28), u("Anna", 85, 34), u("Bert", 77.5, 31), u("Carl", 77.5, 31), u("Emil", 77.9, 30),
	}
	sortAndRank(stats)

	var namen []string
	var plaetze []int
	for _, s := range stats {
		namen = append(namen, s.Name)
		plaetze = append(plaetze, s.Rank)
	}
	if !slices.Equal(namen, []string{"Anna", "Emil", "Bert", "Carl", "Didi"}) {
		t.Errorf("Reihenfolge = %v", namen)
	}
	if !slices.Equal(plaetze, []int{1, 2, 3, 3, 5}) {
		t.Errorf("Plätze = %v, will [1 2 3 3 5]", plaetze)
	}
}
