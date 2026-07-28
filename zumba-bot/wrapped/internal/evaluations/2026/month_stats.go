package eval2026

import (
	"time"

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

// calculateMonthlyAttendanceStats calculates average attendance rate per month
// For each Thursday in a month, calculates (totalUsers - cancellations) / totalUsers
// Then averages across all Thursdays in that month
func (e *Evaluator) calculateMonthlyAttendanceStats(cancellations []models.Cancellation) models.MonthlyAttendanceStats {
	stats := make(models.MonthlyAttendanceStats)
	totalUsers := len(e.rawData.Users)

	if totalUsers == 0 {
		return stats
	}

	// Build a map of cancellations per Thursday
	cancellationsPerDay := make(map[string]int)
	for _, c := range cancellations {
		dateKey := c.Date.Format("2006-01-02")
		cancellationsPerDay[dateKey]++
	}

	// Group Thursdays by month and calculate attendance for each
	type monthData struct {
		totalAttendance int
		thursdayCount   int
	}
	monthlyData := make(map[string]*monthData)

	for _, thursday := range e.rawData.Thursdays {
		monthKey := thursday.Format("2006-01")
		dateKey := thursday.Format("2006-01-02")

		if monthlyData[monthKey] == nil {
			monthlyData[monthKey] = &monthData{}
		}

		// Calculate attendance rate for this Thursday
		cancellationsOnDay := cancellationsPerDay[dateKey]
		attendees := totalUsers - cancellationsOnDay
		attendanceRate := (attendees * 100) / totalUsers

		monthlyData[monthKey].totalAttendance += attendanceRate
		monthlyData[monthKey].thursdayCount++
	}

	// Calculate average attendance rate per month
	for monthKey, data := range monthlyData {
		if data.thursdayCount > 0 {
			stats[monthKey] = data.totalAttendance / data.thursdayCount
		}
	}

	return stats
}

// getThursdaysInMonth returns all Thursdays from rawData that fall in the given year/month
func (e *Evaluator) getThursdaysInMonth(year int, month time.Month) []time.Time {
	var result []time.Time
	for _, t := range e.rawData.Thursdays {
		if t.Year() == year && t.Month() == month {
			result = append(result, t)
		}
	}
	return result
}
