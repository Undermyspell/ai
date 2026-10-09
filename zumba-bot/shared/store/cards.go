package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// CardSettings sind die Bild-Design-Einstellungen des Bots, gepflegt im
// Admin-UI (Seite "Bild-Designs"): welche Designs im Umlauf sind und ob für
// den nächsten Wochenreport ein bestimmtes Design fest gewählt ist.
type CardSettings struct {
	// Rotation: Design-IDs im Umlauf, in Reihenfolge (leer = Live-Design).
	// Gilt nur, wenn RotationGesetzt – sonst nimmt der Bot CARD_STYLES.
	Rotation        []string
	RotationGesetzt bool
	// Naechster: Einmal-Auswahl für den Wochenreport an Tag (nil = Rotation).
	Naechster *NextCard
	// Zuletzt: der zuletzt an die Gruppe gesendete Wochenreport (nil = noch
	// keiner). Ein Wiederholungslauf am selben Tag nimmt dasselbe Design.
	Zuletzt   *NextCard
	UpdatedAt time.Time
}

// NextCard ist die Einmal-Auswahl für einen Wochenreport.
type NextCard struct {
	Tag   time.Time // Donnerstag, für den sie gilt
	Style string
}

// EnsureCardSchema legt die Einstellungs-Tabelle idempotent an (eine Zeile,
// id = 1). rotation NULL heißt "noch nicht gesetzt" – dann gilt CARD_STYLES;
// '{}' heißt bewusst leer, also nur das Live-Design. Das ALTER zieht eine
// Tabelle aus dem ersten Entwurf (rotation NOT NULL DEFAULT '{}') nach. Wie bei
// classifier_models reiht der Advisory-Lock den gleichzeitigen Start von Bot
// und Admin-UI hintereinander.
func EnsureCardSchema(ctx context.Context, e Execer) error {
	const q = `
		SELECT pg_advisory_xact_lock(hashtext('card_settings'));
		CREATE TABLE IF NOT EXISTS card_settings (
		  id              INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
		  rotation        TEXT[],
		  naechster_tag   DATE,
		  naechster_style TEXT,
		  zuletzt_tag     DATE,
		  zuletzt_style   TEXT,
		  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		ALTER TABLE card_settings ALTER COLUMN rotation DROP NOT NULL, ALTER COLUMN rotation DROP DEFAULT,
		  ADD COLUMN IF NOT EXISTS zuletzt_tag DATE, ADD COLUMN IF NOT EXISTS zuletzt_style TEXT`
	_, err := e.ExecContext(ctx, q)
	return err
}

// SeedCardRotation trägt die Startrotation (CARD_STYLES) nur ein, solange
// keine Rotation gesetzt ist – eine Änderung im Admin-UI überlebt so jeden
// Neustart und jedes Deployment. Eine leere Liste schreibt nichts: ein Bot
// ohne CARD_STYLES (lokal) soll die Rotation nicht festnageln.
func SeedCardRotation(ctx context.Context, e Execer, rotation []string) error {
	if len(rotation) == 0 {
		return nil
	}
	const q = `
		INSERT INTO card_settings (id, rotation) VALUES (1, $1)
		ON CONFLICT (id) DO UPDATE SET rotation = EXCLUDED.rotation
		WHERE card_settings.rotation IS NULL`
	_, err := e.ExecContext(ctx, q, pq.Array(rotation))
	return err
}

// GetCardSettings liefert die Einstellungen; ohne Zeile bleibt alles leer
// (RotationGesetzt false, keine Einmal-Auswahl).
func GetCardSettings(ctx context.Context, q RowQueryer) (CardSettings, error) {
	var (
		s             CardSettings
		gesetzt       bool
		tag, zTag     sql.NullTime
		style, zStyle sql.NullString
	)
	err := q.QueryRowContext(ctx,
		`SELECT rotation IS NOT NULL, COALESCE(rotation, '{}'), naechster_tag, naechster_style,
		        zuletzt_tag, zuletzt_style, updated_at
		 FROM card_settings WHERE id = 1`,
	).Scan(&gesetzt, pq.Array(&s.Rotation), &tag, &style, &zTag, &zStyle, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return CardSettings{}, nil
	}
	if err != nil {
		return CardSettings{}, fmt.Errorf("card_settings: %w", err)
	}
	s.RotationGesetzt = gesetzt
	if tag.Valid && style.Valid && style.String != "" {
		// DATE kommt als Mitternacht UTC; das Datum zählt, nicht die Zone.
		s.Naechster = &NextCard{Tag: tag.Time, Style: style.String}
	}
	if zTag.Valid && zStyle.Valid && zStyle.String != "" {
		s.Zuletzt = &NextCard{Tag: zTag.Time, Style: zStyle.String}
	}
	return s, nil
}

// AdvanceCardQueue hält nach dem Versand eines Wochenreports fest, welches
// Design am Tag tag rausging, und rückt die Warteschlange weiter: stand das
// Design oben, wandert es ans Ende – "oberste kommt als nächstes" gilt so
// auch nächste Woche. Eine Einmal-Auswahl (fest) bewegt die Schlange nicht.
// Ein zweiter Aufruf für denselben Tag ändert nichts (Wiederholungslauf).
func AdvanceCardQueue(ctx context.Context, e Execer, tag time.Time, style string, fest bool) error {
	const q = `
		UPDATE card_settings SET
		  rotation = CASE
		    WHEN NOT $3 AND cardinality(rotation) > 1 AND rotation[1] = $2
		    THEN rotation[2:] || rotation[1:1]
		    ELSE rotation END,
		  zuletzt_tag = $1::date, zuletzt_style = $2
		WHERE id = 1 AND zuletzt_tag IS DISTINCT FROM $1::date`
	_, err := e.ExecContext(ctx, q, tag.Format("2006-01-02"), style, fest)
	return err
}

// SetCardRotation speichert die Designs im Umlauf (Upsert). Die Prüfung der
// IDs liegt beim Aufrufer.
func SetCardRotation(ctx context.Context, e Execer, rotation []string) error {
	const q = `
		INSERT INTO card_settings (id, rotation, updated_at) VALUES (1, $1, now())
		ON CONFLICT (id) DO UPDATE SET rotation = EXCLUDED.rotation, updated_at = now()`
	_, err := e.ExecContext(ctx, q, pq.Array(nonNil(rotation)))
	return err
}

// SetNextCard wählt das Design für den Wochenreport am Tag tag; style == ""
// hebt die Einmal-Auswahl auf (zurück zur Rotation). Die Rotation bleibt
// unberührt – auch "noch nicht gesetzt" (NULL), damit CARD_STYLES weiter gilt.
func SetNextCard(ctx context.Context, e Execer, tag time.Time, style string) error {
	// Das Datum geht als Text raus: ein Zeitstempel würde je nach Zeitzone
	// der DB beim Cast auf DATE den Tag verschieben.
	var t, s sql.NullString
	if style != "" {
		t = sql.NullString{String: tag.Format("2006-01-02"), Valid: true}
		s = sql.NullString{String: style, Valid: true}
	}
	const q = `
		INSERT INTO card_settings (id, naechster_tag, naechster_style, updated_at) VALUES (1, $1::date, $2, now())
		ON CONFLICT (id) DO UPDATE SET naechster_tag = EXCLUDED.naechster_tag,
		  naechster_style = EXCLUDED.naechster_style, updated_at = now()`
	_, err := e.ExecContext(ctx, q, t, s)
	return err
}

// nonNil: pq.Array(nil) wird NULL – die Spalte ist NOT NULL.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
