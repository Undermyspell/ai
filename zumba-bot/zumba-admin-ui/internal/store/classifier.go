package store

import (
	"context"

	sharedstore "github.com/michael/zumba-shared/store"
)

// ClassifierModels ist die aktive Modellwahl des Bots (shared-Typ).
type ClassifierModels = sharedstore.ClassifierModels

// EnsureClassifierSchema legt die Tabelle der Modellwahl idempotent an
// (geteilte DDL; der Bot ruft dieselbe Funktion und seedet die Startwerte).
func (s *Postgres) EnsureClassifierSchema(ctx context.Context) error {
	return sharedstore.EnsureClassifierSchema(ctx, s.db)
}

func (s *Postgres) ClassifierModels(ctx context.Context) (ClassifierModels, error) {
	return sharedstore.GetClassifierModels(ctx, s.db)
}

func (s *Postgres) SetClassifierModel(ctx context.Context, rolle, model string) error {
	return sharedstore.SetClassifierModel(ctx, s.db, rolle, model)
}

// Entscheider: wer die Zu-/Absage entscheidet (shared-Typ).
type Entscheider = sharedstore.Entscheider

func (s *Postgres) ClassifierEntscheider(ctx context.Context) (Entscheider, error) {
	return sharedstore.GetEntscheider(ctx, s.db)
}

func (s *Postgres) SetClassifierEntscheider(ctx context.Context, wer string) error {
	return sharedstore.SetEntscheider(ctx, s.db, wer)
}
