package store

import (
	"context"
	"time"

	"github.com/michael/zumba-shared/penalty"
	sharedstore "github.com/michael/zumba-shared/store"
)

// EnsureStrafenSchema legt die Strafen-Tabelle idempotent an (geteilte DDL im
// shared-Modul; Admin-UI ruft dieselbe Funktion, Deploy-Reihenfolge offen).
func (s *Postgres) EnsureStrafenSchema(ctx context.Context) error {
	return sharedstore.EnsureStrafenSchema(ctx, s.db)
}

// PenaltyInputs delegiert an das shared-Modul (Queries auf
// [season.Start, asOf] begrenzt, siehe dort).
func (s *Postgres) PenaltyInputs(ctx context.Context, season Season, asOf time.Time) (penalty.Input, error) {
	return sharedstore.PenaltyInputs(ctx, s.db, season, asOf)
}

// InsertAutoStrafen persistiert alle Marker in einem Statement (shared).
func (s *Postgres) InsertAutoStrafen(ctx context.Context, marks []AutoStrafe) error {
	return sharedstore.InsertAutoStrafen(ctx, s.db, marks)
}
