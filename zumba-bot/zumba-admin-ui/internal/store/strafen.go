package store

import (
	"context"
	"time"

	"github.com/michael/zumba-shared/penalty"
	sharedstore "github.com/michael/zumba-shared/store"
)

// EnsureStrafenSchema legt die Strafen-Tabelle idempotent an (geteilte DDL im
// shared-Modul; der whatsapp-bot ruft dieselbe Funktion, Deploy-Reihenfolge
// offen).
func (s *Postgres) EnsureStrafenSchema(ctx context.Context) error {
	return sharedstore.EnsureStrafenSchema(ctx, s.db)
}

func (s *Postgres) ListStrafen(ctx context.Context) ([]penalty.Row, error) {
	return sharedstore.ListStrafen(ctx, s.db)
}

func (s *Postgres) InsertAutoStrafe(ctx context.Context, userID string, datum time.Time) error {
	return sharedstore.InsertAutoStrafen(ctx, s.db, []sharedstore.AutoStrafe{{UserID: userID, Datum: datum}})
}

func (s *Postgres) InsertNoShowStrafe(ctx context.Context, userID string, datum time.Time, betrag int) error {
	return sharedstore.InsertNoShowStrafe(ctx, s.db, userID, datum, betrag)
}

func (s *Postgres) BegleicheStrafe(ctx context.Context, id int64) error {
	return sharedstore.BegleicheStrafe(ctx, s.db, id)
}

func (s *Postgres) LoescheStrafe(ctx context.Context, id int64) error {
	return sharedstore.LoescheStrafe(ctx, s.db, id)
}
