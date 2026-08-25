package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/michael/zumba-shared/domain"
)

// EnsureSeasonsSchema legt die Tabelle der Stammtischjahre idempotent an und
// seedet sie einmalig. whatsapp-bot und zumba-admin-ui rufen beide dieselbe
// Funktion beim Start (Deploy-Reihenfolge ist offen) – wie bei den Strafen.
//
// Der EXCLUDE-Constraint verhindert überlappende Jahre: nur so ist "das Jahr
// zu Zeitpunkt t" eindeutig. daterange bringt seine gist-Opklasse mit, es
// braucht keine Extension.
func EnsureSeasonsSchema(ctx context.Context, e Execer) error {
	// Ein einziges parameterloses Statement-Bündel: lib/pq nutzt dafür das
	// Simple-Query-Protokoll, das alles in eine implizite Transaktion packt –
	// der Advisory Lock gilt also für Anlegen UND Seed. Ohne ihn können Bot
	// und Admin-UI beim gleichzeitigen Start beide "Tabelle ist leer" sehen
	// und der zweite Seed scheitert am Overlap-Constraint.
	//
	// Geseedet wird nur die leere Tabelle – gepflegte Jahre werden nie
	// überschrieben.
	//
	// 2025 läuft wie die Folgejahre vom 1.12. bis 30.11., obwohl Absagen erst
	// ab 13.10.2025 erfasst sind: die ~45 Donnerstage davor zählen wegen
	// attendance-by-default als anwesend, das Jahr liest sich also
	// entsprechend gut. Bewusste Entscheidung zugunsten einheitlicher
	// Jahresgrenzen.
	const q = `
		SELECT pg_advisory_xact_lock(hashtext('zumba.seasons'));

		CREATE TABLE IF NOT EXISTS public.seasons (
		  id         BIGSERIAL PRIMARY KEY,
		  label      TEXT NOT NULL UNIQUE,
		  start_date DATE NOT NULL,
		  end_date   DATE NOT NULL,
		  CONSTRAINT seasons_range CHECK (end_date > start_date),
		  CONSTRAINT seasons_no_overlap
		    EXCLUDE USING gist (daterange(start_date, end_date, '[]') WITH &&)
		);

		INSERT INTO public.seasons (label, start_date, end_date)
		SELECT * FROM (VALUES
		  ('2025', DATE '2024-12-01', DATE '2025-11-30'),
		  ('2026', DATE '2025-12-01', DATE '2026-11-30'),
		  ('2027', DATE '2026-12-01', DATE '2027-11-30')
		) AS v(label, start_date, end_date)
		WHERE NOT EXISTS (SELECT 1 FROM public.seasons);`
	if _, err := e.ExecContext(ctx, q); err != nil {
		return fmt.Errorf("EnsureSeasonsSchema: %w", err)
	}
	return nil
}

const seasonCols = `id, label, start_date, end_date`

func scanSeason(row interface{ Scan(...any) error }) (domain.Season, error) {
	var s domain.Season
	if err := row.Scan(&s.ID, &s.Label, &s.Start, &s.End); err != nil {
		return domain.Season{}, err
	}
	return s, nil
}

// ListSeasons liefert alle Jahre, neuestes zuerst (Reihenfolge des
// Jahres-Umschalters im Admin-UI).
func ListSeasons(ctx context.Context, q Queryer) ([]domain.Season, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT `+seasonCols+` FROM public.seasons ORDER BY start_date DESC`)
	if err != nil {
		return nil, fmt.Errorf("ListSeasons: %w", err)
	}
	defer rows.Close()

	var out []domain.Season
	for rows.Next() {
		s, err := scanSeason(rows)
		if err != nil {
			return nil, fmt.Errorf("ListSeasons scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SeasonAt liefert das Jahr, in das t fällt, sonst domain.ErrNoSeason.
func SeasonAt(ctx context.Context, q RowQueryer, t time.Time) (domain.Season, error) {
	const query = `
		SELECT ` + seasonCols + ` FROM public.seasons
		WHERE $1::date BETWEEN start_date AND end_date`
	s, err := scanSeason(q.QueryRowContext(ctx, query, domain.DateOnly(t)))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Season{}, fmt.Errorf("%w: %s", domain.ErrNoSeason, t.Format("2006-01-02"))
	}
	if err != nil {
		return domain.Season{}, fmt.Errorf("SeasonAt: %w", err)
	}
	return s, nil
}

// SeasonByLabel liefert ein Jahr über seinen Slug ("2026").
func SeasonByLabel(ctx context.Context, q RowQueryer, label string) (domain.Season, error) {
	const query = `SELECT ` + seasonCols + ` FROM public.seasons WHERE label = $1`
	s, err := scanSeason(q.QueryRowContext(ctx, query, label))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Season{}, fmt.Errorf("%w: %q", domain.ErrNoSeason, label)
	}
	if err != nil {
		return domain.Season{}, fmt.Errorf("SeasonByLabel: %w", err)
	}
	return s, nil
}

// SeasonCache hält die Jahre im Speicher. Sie ändern sich höchstens jährlich,
// werden aber bei praktisch jedem Request gebraucht.
type SeasonCache struct {
	db  RowQueryer
	ttl time.Duration

	mu       sync.Mutex
	seasons  []domain.Season
	loadedAt time.Time
}

func NewSeasonCache(db RowQueryer, ttl time.Duration) *SeasonCache {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &SeasonCache{db: db, ttl: ttl}
}

// Invalidate erzwingt ein Neuladen (nach dem Pflegen eines Jahres).
func (c *SeasonCache) Invalidate() {
	c.mu.Lock()
	c.seasons, c.loadedAt = nil, time.Time{}
	c.mu.Unlock()
}

func (c *SeasonCache) all(ctx context.Context) ([]domain.Season, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.seasons != nil && time.Since(c.loadedAt) < c.ttl {
		return c.seasons, nil
	}
	seasons, err := ListSeasons(ctx, c.db)
	if err != nil {
		return nil, err
	}
	c.seasons, c.loadedAt = seasons, time.Now()
	return seasons, nil
}

// All liefert alle Jahre, neuestes zuerst.
func (c *SeasonCache) All(ctx context.Context) ([]domain.Season, error) {
	return c.all(ctx)
}

// At liefert das Jahr zum Zeitpunkt t, sonst domain.ErrNoSeason.
func (c *SeasonCache) At(ctx context.Context, t time.Time) (domain.Season, error) {
	seasons, err := c.all(ctx)
	if err != nil {
		return domain.Season{}, err
	}
	for _, s := range seasons {
		if s.Contains(t) {
			return s, nil
		}
	}
	return domain.Season{}, fmt.Errorf("%w: %s", domain.ErrNoSeason, t.Format("2006-01-02"))
}

// ByLabel liefert das Jahr zum Slug, sonst domain.ErrNoSeason.
func (c *SeasonCache) ByLabel(ctx context.Context, label string) (domain.Season, error) {
	seasons, err := c.all(ctx)
	if err != nil {
		return domain.Season{}, err
	}
	for _, s := range seasons {
		if s.Label == label {
			return s, nil
		}
	}
	return domain.Season{}, fmt.Errorf("%w: %q", domain.ErrNoSeason, label)
}
