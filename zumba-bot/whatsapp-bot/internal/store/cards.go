package store

import (
	"context"
	"time"

	sharedstore "github.com/michael/zumba-shared/store"
)

// EnsureCardSchema legt die Tabelle der Bild-Design-Einstellungen an
// (geteilte DDL; Admin-UI ruft dieselbe Funktion) und trägt CARD_STYLES als
// Startrotation ein – nur solange es noch keine Einstellungen gibt.
func (s *Postgres) EnsureCardSchema(ctx context.Context, seed []string) error {
	if err := sharedstore.EnsureCardSchema(ctx, s.db); err != nil {
		return err
	}
	return sharedstore.SeedCardRotation(ctx, s.db, seed)
}

// CardSettings liefert die Bild-Design-Einstellungen aus dem Admin-UI
// (erfüllt web.CardSettingsSource).
func (s *Postgres) CardSettings(ctx context.Context) (sharedstore.CardSettings, error) {
	return sharedstore.GetCardSettings(ctx, s.db)
}

// AdvanceCardQueue hält den gesendeten Wochenreport fest und rückt die
// Warteschlange weiter (erfüllt web.CardSettingsSource).
func (s *Postgres) AdvanceCardQueue(ctx context.Context, tag time.Time, style string, fest bool) error {
	return sharedstore.AdvanceCardQueue(ctx, s.db, tag, style, fest)
}
