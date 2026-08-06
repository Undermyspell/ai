package eval2026

import (
	"sort"
	"time"

	"github.com/michael/zumba-shared/penalty"
	"github.com/michael/stammtisch-wrapped/pkg/models"
)

// calculateThursdayStats computes attendance numbers per Thursday.
// A user counts as active on a Thursday once their (clamped) start date is
// reached; attendance-by-default applies, so attendees = active - absences.
func (e *Evaluator) calculateThursdayStats() []models.ThursdayStat {
	if len(e.rawData.Thursdays) == 0 {
		return nil
	}

	// Effective start per userId (clamped to the domain minimum 2025-12-01)
	startByUser := make(map[string]time.Time, len(e.rawData.Users))
	for _, u := range e.rawData.Users {
		startByUser[u.UserID] = penalty.ClampStart(u.StartDate)
	}

	// Absences per date, only counting users active on that date
	absencesByDate := make(map[string]int)
	for _, r := range e.rawData.Rejections {
		start, ok := startByUser[r.UserID]
		if !ok || r.Date.Before(start) {
			continue
		}
		absencesByDate[r.Date.Format("2006-01-02")]++
	}

	stats := make([]models.ThursdayStat, 0, len(e.rawData.Thursdays))
	for _, thursday := range e.rawData.Thursdays {
		active := 0
		for _, start := range startByUser {
			if !thursday.Before(start) {
				active++
			}
		}
		if active == 0 {
			continue
		}

		absent := absencesByDate[thursday.Format("2006-01-02")]
		attendees := active - absent
		if attendees < 0 {
			attendees = 0
		}

		stats = append(stats, models.ThursdayStat{
			Date:      thursday,
			Attendees: attendees,
			Total:     active,
			Rate:      (attendees * 100) / active,
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Date.Before(stats[j].Date)
	})

	return stats
}
