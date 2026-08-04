package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/michael/zumba-admin-ui/internal/penalty"
)

// EnsureStrafenSchema legt die Strafen-Tabelle idempotent an und migriert
// kleinere Schema-Erweiterungen. Die identische DDL steht auch im
// whatsapp-bot (beide Services schreiben in die Tabellen, die
// Deploy-Reihenfolge ist offen) – Änderungen in beiden nachziehen.
func (s *Postgres) EnsureStrafenSchema(ctx context.Context) error {
	const q = `
		CREATE TABLE IF NOT EXISTS strafen (
		  id           BIGSERIAL PRIMARY KEY,
		  "userId"     TEXT NOT NULL,
		  art          TEXT NOT NULL CHECK (art IN ('fehltage','noshow')),
		  datum        DATE NOT NULL,
		  betrag       INT,
		  status       TEXT NOT NULL DEFAULT 'offen'
		               CHECK (status IN ('offen','beglichen','geloescht')),
		  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
		  beglichen_am TIMESTAMPTZ,
		  geloescht_am TIMESTAMPTZ
		);
		CREATE UNIQUE INDEX IF NOT EXISTS strafen_fehltage_unique
		  ON strafen ("userId", datum) WHERE art = 'fehltage';
		-- Absage-Zeitpunkt für Wrapped 2027 ("kurzfristigste Absage"):
		-- Altbestand bleibt bewusst NULL (Zeitpunkt unbekannt), Neueinträge
		-- bekommen den Default – gilt für Bot, Admin-UI und n8n gleichermaßen.
		ALTER TABLE public.stammtisch_abwesenheit
		  ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ;
		ALTER TABLE public.stammtisch_abwesenheit
		  ALTER COLUMN created_at SET DEFAULT now();`
	_, err := s.db.ExecContext(ctx, q)
	return err
}

func (s *Postgres) ListStrafen(ctx context.Context) ([]penalty.Row, error) {
	const q = `
		SELECT id, "userId", art, datum, COALESCE(betrag, 0), status,
		       created_at, beglichen_am, geloescht_am
		FROM strafen
		ORDER BY created_at DESC, id DESC`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("ListStrafen: %w", err)
	}
	defer rows.Close()

	var out []penalty.Row
	for rows.Next() {
		var (
			r          penalty.Row
			art, state string
			beg, del   sql.NullTime
		)
		if err := rows.Scan(&r.ID, &r.UserID, &art, &r.Datum, &r.Betrag,
			&state, &r.CreatedAt, &beg, &del); err != nil {
			return nil, fmt.Errorf("ListStrafen scan: %w", err)
		}
		r.Art, r.Status = penalty.Art(art), penalty.Status(state)
		if beg.Valid {
			t := beg.Time
			r.BeglichenAm = &t
		}
		if del.Valid {
			t := del.Time
			r.GeloeschtAm = &t
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Postgres) InsertAutoStrafe(ctx context.Context, userID string, datum time.Time) error {
	const q = `
		INSERT INTO strafen ("userId", art, datum)
		VALUES ($1, 'fehltage', $2)
		ON CONFLICT ("userId", datum) WHERE art = 'fehltage' DO NOTHING`
	if _, err := s.db.ExecContext(ctx, q, userID, datum); err != nil {
		return fmt.Errorf("InsertAutoStrafe: %w", err)
	}
	return nil
}

func (s *Postgres) InsertNoShowStrafe(ctx context.Context, userID string, datum time.Time, betrag int) error {
	const q = `INSERT INTO strafen ("userId", art, datum, betrag) VALUES ($1, 'noshow', $2, $3)`
	if _, err := s.db.ExecContext(ctx, q, userID, datum, betrag); err != nil {
		return fmt.Errorf("InsertNoShowStrafe: %w", err)
	}
	return nil
}

func (s *Postgres) BegleicheStrafe(ctx context.Context, id int64) error {
	const q = `
		UPDATE strafen SET status = 'beglichen', beglichen_am = now()
		WHERE id = $1 AND status = 'offen'`
	res, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("BegleicheStrafe: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("BegleicheStrafe: keine offene Strafe %d", id)
	}
	return nil
}

func (s *Postgres) LoescheStrafe(ctx context.Context, id int64) error {
	const q = `
		UPDATE strafen SET status = 'geloescht', geloescht_am = now()
		WHERE id = $1 AND status <> 'geloescht'`
	res, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("LoescheStrafe: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("LoescheStrafe: Strafe %d nicht gefunden", id)
	}
	return nil
}
