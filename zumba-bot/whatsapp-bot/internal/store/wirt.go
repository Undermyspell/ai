package store

import (
	"context"
	"time"

	sharedstore "github.com/michael/zumba-shared/store"
)

// wirt.go: Tisch-Erinnerung an den Wirt – Einstellungen (Versand an/aus,
// Modell) und der Stand je Donnerstag (KI-Text, Ablehnung, Versand). Das
// Admin-UI schreibt Einstellungen und Ablehnung, der Bot Text und Versand.

// EnsureWirtSchema legt die Tabellen an (geteilte DDL; Admin-UI ruft dieselbe
// Funktion) und trägt model als Startwert ein – nur solange keines gewählt
// ist.
func (s *Postgres) EnsureWirtSchema(ctx context.Context, model string) error {
	if err := sharedstore.EnsureWirtSchema(ctx, s.db); err != nil {
		return err
	}
	return sharedstore.SeedWirtModel(ctx, s.db, model)
}

func (s *Postgres) WirtSettings(ctx context.Context) (sharedstore.WirtSettings, error) {
	return sharedstore.GetWirtSettings(ctx, s.db)
}

func (s *Postgres) WirtNachricht(ctx context.Context, tag time.Time) (*sharedstore.WirtNachricht, error) {
	return sharedstore.GetWirtNachricht(ctx, s.db, tag)
}

// WirtVorherige liefert die letzten KI-Texte (jüngster zuerst), damit sich
// das Modell nicht wiederholt.
func (s *Postgres) WirtVorherige(ctx context.Context, limit int) ([]string, error) {
	// Tage ohne Text (nur Standardnachricht) zählen nicht – deshalb etwas
	// mehr lesen als gebraucht.
	rows, err := sharedstore.ListWirtNachrichten(ctx, s.db, limit*3)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, n := range rows {
		if n.Text != "" && len(out) < limit {
			out = append(out, n.Text)
		}
	}
	return out, nil
}

func (s *Postgres) SaveWirtEntwurf(ctx context.Context, tag time.Time, text, model string) (bool, error) {
	return sharedstore.SaveWirtEntwurf(ctx, s.db, tag, text, model, sharedstore.WirtQuelleAuto, false)
}

func (s *Postgres) SaveWirtFehler(ctx context.Context, tag time.Time, fehler string) error {
	return sharedstore.SaveWirtFehler(ctx, s.db, tag, fehler)
}

func (s *Postgres) MarkWirtGesendet(ctx context.Context, tag time.Time, als string) error {
	return sharedstore.MarkWirtGesendet(ctx, s.db, tag, als)
}
