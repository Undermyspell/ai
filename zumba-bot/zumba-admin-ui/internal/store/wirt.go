package store

import (
	"context"
	"time"

	sharedstore "github.com/michael/zumba-shared/store"
)

// wirt.go: Tisch-Erinnerung an den Wirt (Seite /tisch-erinnerung). Das
// Admin-UI schreibt die Einstellungen, lehnt den vorgenerierten KI-Text ab
// und übernimmt einen im Bot-Test erzeugten; Text und Versand schreibt der Bot.

// WirtSettings: Versand an/aus und das Modell für den KI-Text (shared-Typ).
type WirtSettings = sharedstore.WirtSettings

// WirtNachricht ist der Stand der Erinnerung eines Donnerstags (shared-Typ).
type WirtNachricht = sharedstore.WirtNachricht

// EnsureWirtSchema legt die Tabellen idempotent an (geteilte DDL; der Bot
// ruft dieselbe Funktion und seedet das Startmodell).
func (s *Postgres) EnsureWirtSchema(ctx context.Context) error {
	return sharedstore.EnsureWirtSchema(ctx, s.db)
}

func (s *Postgres) WirtSettings(ctx context.Context) (WirtSettings, error) {
	return sharedstore.GetWirtSettings(ctx, s.db)
}

func (s *Postgres) SetWirtVersand(ctx context.Context, versand bool) error {
	return sharedstore.SetWirtVersand(ctx, s.db, versand)
}

func (s *Postgres) SetWirtModel(ctx context.Context, model string) error {
	return sharedstore.SetWirtModel(ctx, s.db, model)
}

func (s *Postgres) WirtNachricht(ctx context.Context, tag time.Time) (*WirtNachricht, error) {
	return sharedstore.GetWirtNachricht(ctx, s.db, tag)
}

func (s *Postgres) ListWirtNachrichten(ctx context.Context, limit int) ([]WirtNachricht, error) {
	return sharedstore.ListWirtNachrichten(ctx, s.db, limit)
}

func (s *Postgres) AdoptWirtText(ctx context.Context, tag time.Time, text, model string) (bool, error) {
	return sharedstore.SaveWirtEntwurf(ctx, s.db, tag, text, model, sharedstore.WirtQuelleTest, true)
}

func (s *Postgres) SetWirtAbgelehnt(ctx context.Context, tag time.Time, abgelehnt bool) (bool, error) {
	return sharedstore.SetWirtAbgelehnt(ctx, s.db, tag, abgelehnt)
}
