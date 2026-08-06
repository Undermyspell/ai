package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/michael/zumba-whatsapp-bot/internal/penalty"
)

// EnsureStrafenSchema legt die Strafen-Tabelle idempotent an und migriert
// kleinere Schema-Erweiterungen. Die identische DDL steht auch im
// zumba-admin-ui (beide Services schreiben in die Tabellen, die
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

// PenaltyInputs sammelt die Eingangsdaten für penalty.Assess zum Stichtag
// asOf: User (mit auf 2025-12-01 geklemmtem Start), deren
// Abwesenheits-Donnerstage, Sperrtage und strafen-Zeilen. Alle Queries sind
// auf [2025-12-01, asOf] begrenzt – Zeilen außerhalb können das Ergebnis
// nicht beeinflussen (Assess betrachtet nur Donnerstage in diesem Fenster,
// und Resets zukünftiger strafen-Zeilen liegen immer nach deren datum).
func (s *Postgres) PenaltyInputs(ctx context.Context, asOf time.Time) (penalty.Input, error) {
	var in penalty.Input
	minStart := penalty.ClampStart(nil)

	const usersQ = `SELECT "userId", "userName", "startDate" FROM public.users`
	rows, err := s.db.QueryContext(ctx, usersQ)
	if err != nil {
		return in, fmt.Errorf("PenaltyInputs users: %w", err)
	}
	defer rows.Close()
	idx := map[string]int{}
	for rows.Next() {
		var (
			u     penalty.UserData
			start sql.NullTime
		)
		if err := rows.Scan(&u.UserID, &u.Name, &start); err != nil {
			return in, fmt.Errorf("PenaltyInputs users scan: %w", err)
		}
		var sd *time.Time
		if start.Valid {
			sd = &start.Time
		}
		u.EffectiveStart = penalty.ClampStart(sd)
		idx[u.UserID] = len(in.Users)
		in.Users = append(in.Users, u)
	}
	if err := rows.Err(); err != nil {
		return in, err
	}

	const absQ = `
		SELECT "userId", date FROM public.stammtisch_abwesenheit
		WHERE EXTRACT(ISODOW FROM date) = 4
		  AND date >= $1 AND date <= $2
		  AND date NOT IN (SELECT date FROM excluded_days)`
	arows, err := s.db.QueryContext(ctx, absQ, minStart, asOf)
	if err != nil {
		return in, fmt.Errorf("PenaltyInputs absences: %w", err)
	}
	defer arows.Close()
	for arows.Next() {
		var (
			uid string
			d   time.Time
		)
		if err := arows.Scan(&uid, &d); err != nil {
			return in, fmt.Errorf("PenaltyInputs absences scan: %w", err)
		}
		if i, ok := idx[uid]; ok {
			in.Users[i].Absences = append(in.Users[i].Absences, d)
		}
	}
	if err := arows.Err(); err != nil {
		return in, err
	}

	const exclQ = `SELECT date FROM excluded_days WHERE date >= $1 AND date <= $2`
	erows, err := s.db.QueryContext(ctx, exclQ, minStart, asOf)
	if err != nil {
		return in, fmt.Errorf("PenaltyInputs excluded: %w", err)
	}
	defer erows.Close()
	for erows.Next() {
		var d time.Time
		if err := erows.Scan(&d); err != nil {
			return in, fmt.Errorf("PenaltyInputs excluded scan: %w", err)
		}
		in.Excluded = append(in.Excluded, d)
	}
	if err := erows.Err(); err != nil {
		return in, err
	}

	const strafenQ = `
		SELECT id, "userId", art, datum, COALESCE(betrag, 0), status,
		       created_at, beglichen_am, geloescht_am
		FROM strafen
		WHERE datum <= $1`
	srows, err := s.db.QueryContext(ctx, strafenQ, asOf)
	if err != nil {
		return in, fmt.Errorf("PenaltyInputs strafen: %w", err)
	}
	defer srows.Close()
	for srows.Next() {
		var (
			r          penalty.Row
			art, state string
			beg, del   sql.NullTime
		)
		if err := srows.Scan(&r.ID, &r.UserID, &art, &r.Datum, &r.Betrag,
			&state, &r.CreatedAt, &beg, &del); err != nil {
			return in, fmt.Errorf("PenaltyInputs strafen scan: %w", err)
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
		in.Rows = append(in.Rows, r)
	}
	return in, srows.Err()
}

// InsertAutoStrafen persistiert alle Marker in einem Statement (eine
// Round-Trip, atomar – kein Teilzustand bei Fehlern).
func (s *Postgres) InsertAutoStrafen(ctx context.Context, marks []AutoStrafe) error {
	if len(marks) == 0 {
		return nil
	}
	userIDs := make([]string, len(marks))
	datums := make([]string, len(marks))
	for i, m := range marks {
		userIDs[i] = m.UserID
		datums[i] = m.Datum.Format("2006-01-02")
	}
	const q = `
		INSERT INTO strafen ("userId", art, datum)
		SELECT u, 'fehltage', d
		FROM unnest($1::text[], $2::date[]) AS t(u, d)
		ON CONFLICT ("userId", datum) WHERE art = 'fehltage' DO NOTHING`
	if _, err := s.db.ExecContext(ctx, q, pq.Array(userIDs), pq.Array(datums)); err != nil {
		return fmt.Errorf("InsertAutoStrafen: %w", err)
	}
	return nil
}
