package eval2026

import "github.com/michael/stammtisch-wrapped/pkg/models"

// calculateMonthStats counts cancellations per month
func (e *Evaluator) calculateMonthStats(cancellations []models.Cancellation) models.MonthStats {
	stats := make(models.MonthStats)

	for _, c := range cancellations {
		monthKey := c.Date.Format("2006-01")
		stats[monthKey]++
	}

	return stats
}
