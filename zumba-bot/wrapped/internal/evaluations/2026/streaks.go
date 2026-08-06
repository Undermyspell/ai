package eval2026

import "time"

// streakOf ist eine Serie aus max_streaks.sql (Länge + Zeitraum). Die
// eigentliche Serien-Berechnung passiert in SQL (gaps-and-islands).
type streakOf struct {
	Len   int
	Start time.Time
	End   time.Time
}
