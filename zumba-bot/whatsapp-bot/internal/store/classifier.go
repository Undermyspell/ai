package store

import (
	"context"
	"fmt"
	"time"

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

// ClassifierEntscheider liefert, wer die Zu-/Absage entscheidet (gemini |
// modell; Seite /ki-modell im Admin-UI). Ohne Eintrag gilt gemini.
func (s *Postgres) ClassifierEntscheider(ctx context.Context) (string, error) {
	e, err := sharedstore.GetEntscheider(ctx, s.db)
	return e.Wer, err
}

// IsExcludedDay sagt, ob day ein Sperrtag ist (excluded_days) – erfüllt
// web.SperrtagSource für die Tisch-Erinnerung an den Wirt.
func (s *Postgres) IsExcludedDay(ctx context.Context, day time.Time) (bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT 1 FROM excluded_days WHERE date = $1::date`, day.Format("2006-01-02"))
	if err != nil {
		return false, fmt.Errorf("excluded_days: %w", err)
	}
	defer rows.Close()
	return rows.Next(), rows.Err()
}
