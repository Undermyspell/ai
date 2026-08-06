package eval2026

import (
	"github.com/michael/stammtisch-wrapped/pkg/models"
)

// calculateMonthStats counts cancellations per month
func (e *Evaluator) calculateMonthStats(cancellations []models.Cancellation) models.MonthStats {
	stats := make(models.MonthStats)

	for _, c := range cancellations {
		monthKey := c.Date.Format("2006-01")
		stats[monthKey]++
	}

	return stats
}

// calculateMonthlyAttendanceStats mittelt die Tagesraten aus den
// SQL-ThursdayStats pro Monat. Die Raten berücksichtigen damit den
// geklemmten Start jedes Users (vorher: fixer Nenner totalUsers — Monate
// vor dem Eintritt eines Users wurden falsch gerechnet).
func (e *Evaluator) calculateMonthlyAttendanceStats(thursdayStats []models.ThursdayStat) models.MonthlyAttendanceStats {
	stats := make(models.MonthlyAttendanceStats)

	type monthData struct {
		rateSum       int
		thursdayCount int
	}
	monthlyData := make(map[string]*monthData)

	for _, t := range thursdayStats {
		monthKey := t.Date.Format("2006-01")
		if monthlyData[monthKey] == nil {
			monthlyData[monthKey] = &monthData{}
		}
		monthlyData[monthKey].rateSum += t.Rate
		monthlyData[monthKey].thursdayCount++
	}

	for monthKey, data := range monthlyData {
		if data.thursdayCount > 0 {
			stats[monthKey] = data.rateSum / data.thursdayCount
		}
	}

	return stats
}
