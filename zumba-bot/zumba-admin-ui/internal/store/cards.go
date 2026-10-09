package store

import (
	"context"
	"time"

	sharedstore "github.com/michael/zumba-shared/store"
)

// CardSettings sind die Bild-Design-Einstellungen des Bots (shared-Typ).
type CardSettings = sharedstore.CardSettings

// EnsureCardSchema legt die Einstellungs-Tabelle idempotent an (geteilte
// DDL; der Bot ruft dieselbe Funktion und seedet CARD_STYLES).
func (s *Postgres) EnsureCardSchema(ctx context.Context) error {
	return sharedstore.EnsureCardSchema(ctx, s.db)
}

func (s *Postgres) CardSettings(ctx context.Context) (CardSettings, error) {
	return sharedstore.GetCardSettings(ctx, s.db)
}

func (s *Postgres) SetCardRotation(ctx context.Context, rotation []string) error {
	return sharedstore.SetCardRotation(ctx, s.db, rotation)
}

func (s *Postgres) SetNextCard(ctx context.Context, tag time.Time, style string) error {
	return sharedstore.SetNextCard(ctx, s.db, tag, style)
}
