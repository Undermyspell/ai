package store

import (
	"context"
	"time"
)

// Stat ist eine Zeile der Per-User-Statistik (entspricht den Spalten von
// stats.sql bzw. dem n8n-Node "Get Per user stats").
type Stat struct {
	UserID         string
	Name           string
	StartDate      *time.Time // nullable ("startDate")
	EffectiveStart time.Time
	Attendance     int
	Away           int
	Percent        float64
	Streak         int
}

// Store kapselt die DB-Operationen des Workflows.
type Store interface {
	// UserStats liefert die Rangliste (n8n: "Get Per user stats").
	UserStats(ctx context.Context) ([]Stat, error)
	// MarkAbsent trägt eine Absage ein (n8n: "Insert or update rows", UPSERT).
	MarkAbsent(ctx context.Context, userID string, date time.Time, message string) error
	// MarkPresent entfernt eine Absage (n8n: "Delete table or rows").
	MarkPresent(ctx context.Context, userID string, date time.Time) error
}
