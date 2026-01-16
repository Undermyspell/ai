package eval2026

import (
	"sort"
	"time"
)

// StreakResult contains the streak count and date range
type StreakResult struct {
	Count int
	Start time.Time
	End   time.Time
}

// calculateStreaks calculates max attendance and cancellation streaks for a user
func (e *Evaluator) calculateStreaks(userID int, cancellationDates []time.Time) (attendance StreakResult, cancellation StreakResult) {
	if len(e.rawData.Thursdays) == 0 {
		return StreakResult{}, StreakResult{}
	}

	// Create a set of cancellation dates for O(1) lookup
	cancellationSet := make(map[string]bool)
	for _, d := range cancellationDates {
		cancellationSet[d.Format("2006-01-02")] = true
	}

	// Sort thursdays to ensure chronological order
	thursdays := make([]time.Time, len(e.rawData.Thursdays))
	copy(thursdays, e.rawData.Thursdays)
	sort.Slice(thursdays, func(i, j int) bool {
		return thursdays[i].Before(thursdays[j])
	})

	var maxAttendance, maxCancellation StreakResult
	var currentAttendanceStart, currentCancellationStart time.Time
	currentAttendanceCount := 0
	currentCancellationCount := 0

	for _, thursday := range thursdays {
		dateKey := thursday.Format("2006-01-02")
		wasCancelled := cancellationSet[dateKey]

		if wasCancelled {
			// User was absent
			if currentCancellationCount == 0 {
				currentCancellationStart = thursday
			}
			currentCancellationCount++
			if currentCancellationCount > maxCancellation.Count {
				maxCancellation = StreakResult{
					Count: currentCancellationCount,
					Start: currentCancellationStart,
					End:   thursday,
				}
			}
			currentAttendanceCount = 0
		} else {
			// User was present
			if currentAttendanceCount == 0 {
				currentAttendanceStart = thursday
			}
			currentAttendanceCount++
			if currentAttendanceCount > maxAttendance.Count {
				maxAttendance = StreakResult{
					Count: currentAttendanceCount,
					Start: currentAttendanceStart,
					End:   thursday,
				}
			}
			currentCancellationCount = 0
		}
	}

	return maxAttendance, maxCancellation
}
