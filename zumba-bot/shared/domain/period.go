// Package domain bündelt service-übergreifende Stammtisch-Konventionen.
package domain

import "time"

// Period ist der Auswertungszeitraum (z. B. Wrapped 2026:
// 01.12.2025 – 30.11.2026).
type Period struct {
	Start time.Time
	End   time.Time
}

// EffectiveEnd kappt das Ende am heutigen Tag, damit zukünftige Donnerstage
// nicht als verpasst zählen.
func (p Period) EffectiveEnd() time.Time {
	today := time.Now().Truncate(24 * time.Hour)
	if p.End.After(today) {
		return today
	}
	return p.End
}
