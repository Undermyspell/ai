package store

import (
	"context"
	"time"

	"github.com/michael/zumba-whatsapp-bot/internal/penalty"
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

// AutoStrafe ist der Marker einer erkannten Fehltage-Strafe.
type AutoStrafe struct {
	UserID string
	Datum  time.Time
}

// Store kapselt die DB-Operationen des Workflows.
type Store interface {
	// UserStats liefert die Rangliste zum Stichtag asOf (n8n: "Get Per user
	// stats"; im Original bis current_date – asOf=heute ist identisch).
	UserStats(ctx context.Context, asOf time.Time) ([]Stat, error)
	// MarkAbsent trägt eine Absage ein (n8n: "Insert or update rows", UPSERT).
	MarkAbsent(ctx context.Context, userID string, date time.Time, message string) error
	// MarkPresent entfernt eine Absage (n8n: "Delete table or rows").
	MarkPresent(ctx context.Context, userID string, date time.Time) error

	// PenaltyInputs liefert alles, was penalty.Assess zum Stichtag asOf
	// braucht (User mit Abwesenheiten, Sperrtage, strafen-Zeilen). Die
	// Queries sind auf [2025-12-01, asOf] begrenzt – außerhalb liegende
	// Zeilen können das Ergebnis von Assess nicht beeinflussen.
	PenaltyInputs(ctx context.Context, asOf time.Time) (penalty.Input, error)
	// InsertAutoStrafen persistiert die Marker erkannter Fehltage-Strafen in
	// einem Statement (idempotent: userId + erster Fehltag der Serie).
	InsertAutoStrafen(ctx context.Context, marks []AutoStrafe) error
}
