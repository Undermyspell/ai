package store

import (
	"context"
	"time"

	"github.com/michael/zumba-shared/domain"
	"github.com/michael/zumba-shared/penalty"
	sharedstore "github.com/michael/zumba-shared/store"
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

// AutoStrafe ist der Marker einer erkannten Fehltage-Strafe (shared-Typ).
type AutoStrafe = sharedstore.AutoStrafe

// Season ist ein Stammtischjahr (shared-Typ).
type Season = domain.Season

// Store kapselt die DB-Operationen des Workflows.
type Store interface {
	// SeasonAt liefert das Stammtischjahr, in das t fällt. Ist keines
	// gepflegt, kommt domain.ErrNoSeason – der Report entfällt dann bewusst,
	// statt eine leere Rangliste zu senden.
	SeasonAt(ctx context.Context, t time.Time) (Season, error)
	// UserStats liefert die Rangliste des Stammtischjahres season zum
	// Stichtag asOf (n8n: "Get Per user stats"). Mit dem Jahreswechsel am
	// Jahresstart beginnt die Rangliste wieder bei null.
	UserStats(ctx context.Context, season Season, asOf time.Time) ([]Stat, error)
	// MarkAbsent trägt eine Absage ein (n8n: "Insert or update rows", UPSERT).
	MarkAbsent(ctx context.Context, userID string, date time.Time, message string) error
	// MarkPresent entfernt eine Absage (n8n: "Delete table or rows").
	MarkPresent(ctx context.Context, userID string, date time.Time) error

	// PenaltyInputs liefert alles, was penalty.Assess im Stammtischjahr
	// season zum Stichtag asOf braucht (User mit Abwesenheiten, Sperrtage,
	// strafen-Zeilen). Die Queries sind auf [season.Start, asOf] begrenzt –
	// außerhalb liegende Zeilen können das Ergebnis von Assess nicht
	// beeinflussen, Fehltage-Serien enden an der Jahresgrenze.
	PenaltyInputs(ctx context.Context, season Season, asOf time.Time) (penalty.Input, error)
	// InsertAutoStrafen persistiert die Marker erkannter Fehltage-Strafen in
	// einem Statement (idempotent: userId + erster Fehltag der Serie).
	InsertAutoStrafen(ctx context.Context, marks []AutoStrafe) error
}
