package eval2026

import (
	"sort"
	"time"
)

// calculateStreaks calculates max attendance and cancellation streaks for a user
func (e *Evaluator) calculateStreaks(userID int, cancellationDates []time.Time) (maxAttendanceStreak, maxCancellationStreak int) {
	if len(e.rawData.Thursdays) == 0 {
		return 0, 0
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

	currentAttendanceStreak := 0
	currentCancellationStreak := 0

	for _, thursday := range thursdays {
		dateKey := thursday.Format("2006-01-02")
		wasCancelled := cancellationSet[dateKey]

		if wasCancelled {
			// User was absent
			currentCancellationStreak++
			if currentCancellationStreak > maxCancellationStreak {
				maxCancellationStreak = currentCancellationStreak
			}
			currentAttendanceStreak = 0
		} else {
			// User was present
			currentAttendanceStreak++
			if currentAttendanceStreak > maxAttendanceStreak {
				maxAttendanceStreak = currentAttendanceStreak
			}
			currentCancellationStreak = 0
		}
	}

	return maxAttendanceStreak, maxCancellationStreak
}
