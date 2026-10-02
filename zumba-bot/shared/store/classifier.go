package store

import (
	"context"
	"fmt"
	"time"
)

// Rollen der Classifier-Modelle: der Bot fragt erst das Haupt-, bei Fehler
// das Fallback-Modell.
const (
	ClassifierPrimary  = "primary"
	ClassifierFallback = "fallback"
)

// ClassifierModel ist die Wahl für eine Rolle. Model == "" heißt: noch nicht
// gesetzt – der Bot nimmt dann den Wert aus seiner Konfiguration.
type ClassifierModel struct {
	Model     string
	UpdatedAt time.Time
}

// ClassifierModels ist die aktive Modellwahl des Bots.
type ClassifierModels struct {
	Primary  ClassifierModel
	Fallback ClassifierModel
}

// EnsureClassifierSchema legt die Tabelle der Modellwahl idempotent an.
// whatsapp-bot und zumba-admin-ui rufen sie beide beim Start – beim Deploy
// oft in derselben Sekunde, und zwei parallele CREATE TABLE IF NOT EXISTS
// kollidieren an pg_type. Der Advisory-Lock reiht sie hintereinander: ohne
// Parameter läuft der Block als eine Simple Query in einer impliziten
// Transaktion, der Lock hält bis zu deren Ende.
func EnsureClassifierSchema(ctx context.Context, e Execer) error {
	const q = `
		SELECT pg_advisory_xact_lock(hashtext('classifier_models'));
		CREATE TABLE IF NOT EXISTS classifier_models (
		  rolle      TEXT PRIMARY KEY CHECK (rolle IN ('primary','fallback')),
		  model      TEXT NOT NULL,
		  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`
	_, err := e.ExecContext(ctx, q)
	return err
}

// SeedClassifierModels trägt Startwerte nur für Rollen ein, die noch keine
// Wahl haben – eine Umstellung im Admin-UI überlebt so jeden Neustart. Leere
// Werte werden übersprungen.
func SeedClassifierModels(ctx context.Context, e Execer, primary, fallback string) error {
	const q = `
		INSERT INTO classifier_models (rolle, model)
		SELECT r, m FROM (VALUES ($1, $2), ($3, $4)) AS v(r, m)
		WHERE m <> ''
		ON CONFLICT (rolle) DO NOTHING`
	_, err := e.ExecContext(ctx, q, ClassifierPrimary, primary, ClassifierFallback, fallback)
	return err
}

// GetClassifierModels liefert die aktive Wahl; fehlende Rollen bleiben leer.
func GetClassifierModels(ctx context.Context, q Queryer) (ClassifierModels, error) {
	rows, err := q.QueryContext(ctx, `SELECT rolle, model, updated_at FROM classifier_models`)
	if err != nil {
		return ClassifierModels{}, fmt.Errorf("classifier_models: %w", err)
	}
	defer rows.Close()
	var out ClassifierModels
	for rows.Next() {
		var (
			rolle string
			m     ClassifierModel
		)
		if err := rows.Scan(&rolle, &m.Model, &m.UpdatedAt); err != nil {
			return ClassifierModels{}, fmt.Errorf("classifier_models scan: %w", err)
		}
		switch rolle {
		case ClassifierPrimary:
			out.Primary = m
		case ClassifierFallback:
			out.Fallback = m
		}
	}
	return out, rows.Err()
}

// SetClassifierModel stellt eine Rolle um (Upsert). Die Prüfung, ob das
// Modell erlaubt ist, liegt beim Aufrufer.
func SetClassifierModel(ctx context.Context, e Execer, rolle, model string) error {
	if rolle != ClassifierPrimary && rolle != ClassifierFallback {
		return fmt.Errorf("classifier_models: unbekannte Rolle %q", rolle)
	}
	const q = `
		INSERT INTO classifier_models (rolle, model, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (rolle) DO UPDATE SET model = EXCLUDED.model, updated_at = now()`
	_, err := e.ExecContext(ctx, q, rolle, model)
	return err
}
