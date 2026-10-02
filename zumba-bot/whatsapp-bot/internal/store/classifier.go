package store

import (
	"context"

	sharedstore "github.com/michael/zumba-shared/store"
)

// EnsureClassifierSchema legt die Tabelle der Modellwahl an (geteilte DDL;
// Admin-UI ruft dieselbe Funktion) und trägt die Modelle aus der
// Konfiguration als Startwert ein – nur für Rollen ohne Wahl, eine Umstellung
// im Admin-UI überlebt so jeden Neustart.
func (s *Postgres) EnsureClassifierSchema(ctx context.Context, primary, fallback string) error {
	if err := sharedstore.EnsureClassifierSchema(ctx, s.db); err != nil {
		return err
	}
	return sharedstore.SeedClassifierModels(ctx, s.db, primary, fallback)
}

// ClassifierModels liefert die aktive Modellwahl (erfüllt
// classifier.ModelSource).
func (s *Postgres) ClassifierModels(ctx context.Context) (primary, fallback string, err error) {
	m, err := sharedstore.GetClassifierModels(ctx, s.db)
	if err != nil {
		return "", "", err
	}
	return m.Primary.Model, m.Fallback.Model, nil
}
