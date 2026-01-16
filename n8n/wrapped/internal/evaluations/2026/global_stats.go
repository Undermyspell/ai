package eval2026

import "github.com/michael/stammtisch-wrapped/pkg/models"

// calculateGlobalStats computes overall statistics
func (e *Evaluator) calculateGlobalStats(userStats []models.UserStats) models.GlobalStats {
	totalThursdays := len(e.rawData.Thursdays)
	totalUsers := len(e.rawData.Users)

	totalCancellations := 0
	totalAttendances := 0
	totalRates := 0

	for _, user := range userStats {
		totalCancellations += user.CancellationCount
		totalAttendances += user.AttendanceCount
		totalRates += user.AttendanceRate
	}

	averageAttendanceRate := 0
	if totalUsers > 0 {
		averageAttendanceRate = totalRates / totalUsers
	}

	return models.GlobalStats{
		TotalThursdays:        totalThursdays,
		TotalUsers:            totalUsers,
		TotalCancellations:    totalCancellations,
		TotalAttendances:      totalAttendances,
		AverageAttendanceRate: averageAttendanceRate,
	}
}
