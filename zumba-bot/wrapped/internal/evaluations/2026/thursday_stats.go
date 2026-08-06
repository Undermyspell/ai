package eval2026

import (
	"github.com/michael/stammtisch-wrapped/pkg/models"
)

// calculateThursdayStats mappt die SQL-berechnete Anwesenheit je Donnerstag
// (thursday_stats.sql: aktiv ab geklemmtem Start, Absagen nur aktiver User)
// auf das Modell. Zeilen ohne aktive User entfallen.
func (e *Evaluator) calculateThursdayStats() []models.ThursdayStat {
	stats := make([]models.ThursdayStat, 0, len(e.rawData.ThursdayStats))
	for _, t := range e.rawData.ThursdayStats {
		if t.Active == 0 {
			continue
		}
		stats = append(stats, models.ThursdayStat{
			Date:      t.Day,
			Attendees: t.Attendees,
			Total:     t.Active,
			Rate:      (t.Attendees * 100) / t.Active,
		})
	}
	return stats
}
