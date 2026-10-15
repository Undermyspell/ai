package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	sharedstore "github.com/michael/zumba-shared/store"

	"github.com/michael/zumba-admin-ui/internal/db"
	"github.com/michael/zumba-admin-ui/internal/timeutil"
)

type Postgres struct {
	db      *db.Postgres
	seasons *sharedstore.SeasonCache
}

func NewPostgres(p *db.Postgres) *Postgres {
	return &Postgres{db: p, seasons: sharedstore.NewSeasonCache(p, seasonCacheTTL)}
}

// seasonCacheTTL: Stammtischjahre ändern sich höchstens jährlich, werden aber
// für jede Seite gebraucht.
const seasonCacheTTL = 10 * time.Minute

// EnsureSeasonsSchema legt die Jahres-Tabelle idempotent an und seedet sie
// einmalig (geteilte DDL; der Bot ruft dieselbe Funktion).
func (s *Postgres) EnsureSeasonsSchema(ctx context.Context) error {
	return sharedstore.EnsureSeasonsSchema(ctx, s.db)
}

func (s *Postgres) ListSeasons(ctx context.Context) ([]Season, error) {
	return s.seasons.All(ctx)
}

func (s *Postgres) SeasonAt(ctx context.Context, t time.Time) (Season, error) {
	return s.seasons.At(ctx, t)
}

func (s *Postgres) SeasonByLabel(ctx context.Context, label string) (Season, error) {
	return s.seasons.ByLabel(ctx, label)
}

// MoveSeasonStart und AddSeason leeren den Jahres-Cache, damit die eigene
// Oberfläche die Änderung sofort zeigt; der Bot holt sie nach spätestens
// seasonCacheTTL.
func (s *Postgres) MoveSeasonStart(ctx context.Context, label string, start time.Time) error {
	defer s.seasons.Invalidate()
	return sharedstore.MoveSeasonStart(ctx, s.db, label, start)
}

func (s *Postgres) AddSeason(ctx context.Context, label string, start, end time.Time) error {
	defer s.seasons.Invalidate()
	return sharedstore.AddSeason(ctx, s.db, label, start, end)
}

func (s *Postgres) ListUsers(ctx context.Context) ([]User, error) {
	const q = `
		SELECT "userId", "userName", "startDate"
		FROM users
		ORDER BY "userName"
	`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("ListUsers: %w", err)
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.StartDate); err != nil {
			return nil, fmt.Errorf("ListUsers scan: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Postgres) GetUser(ctx context.Context, userID string) (*User, error) {
	const q = `SELECT "userId", "userName", "startDate" FROM users WHERE "userId" = $1`
	var u User
	err := s.db.QueryRowContext(ctx, q, userID).Scan(&u.ID, &u.Name, &u.StartDate)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("GetUser: %w", err)
	}
	return &u, nil
}

func (s *Postgres) ListThursdays(ctx context.Context, p timeutil.Period) ([]time.Time, error) {
	const q = `
		WITH all_thursdays AS (
			SELECT d::date AS thursday
			FROM generate_series($1::date, $2::date, interval '1 day') AS d
			WHERE EXTRACT(DOW FROM d) = 4
		)
		SELECT thursday FROM all_thursdays
		WHERE thursday NOT IN (SELECT date FROM excluded_days WHERE date >= $1 AND date <= $2)
		ORDER BY thursday DESC
	`
	rows, err := s.db.QueryContext(ctx, q, p.Start, p.EffectiveEnd())
	if err != nil {
		return nil, fmt.Errorf("ListThursdays: %w", err)
	}
	defer rows.Close()

	var out []time.Time
	for rows.Next() {
		var t time.Time
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("ListThursdays scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Postgres) ListExcludedDays(ctx context.Context, p timeutil.Period) ([]time.Time, error) {
	const q = `
		SELECT date FROM excluded_days
		WHERE date >= $1 AND date <= $2
		ORDER BY date DESC
	`
	rows, err := s.db.QueryContext(ctx, q, p.Start, p.End)
	if err != nil {
		return nil, fmt.Errorf("ListExcludedDays: %w", err)
	}
	defer rows.Close()

	var out []time.Time
	for rows.Next() {
		var t time.Time
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("ListExcludedDays scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Postgres) ListAbsences(ctx context.Context, p timeutil.Period) ([]Absence, error) {
	const q = `
		SELECT "userId", date, message
		FROM stammtisch_abwesenheit
		WHERE date >= $1 AND date <= $2
		  AND EXTRACT(DOW FROM date) = 4
		  AND date NOT IN (SELECT date FROM excluded_days)
		ORDER BY date DESC, "userId"
	`
	rows, err := s.db.QueryContext(ctx, q, p.Start, p.EffectiveEnd())
	if err != nil {
		return nil, fmt.Errorf("ListAbsences: %w", err)
	}
	defer rows.Close()

	var out []Absence
	for rows.Next() {
		var a Absence
		if err := rows.Scan(&a.UserID, &a.Date, &a.Message); err != nil {
			return nil, fmt.Errorf("ListAbsences scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Leaderboard nutzt die geteilte Rangliste-Query aus dem shared-Modul
// (queries/leaderboard.sql – früher hier als leaderboardQ dupliziert).
func (s *Postgres) Leaderboard(ctx context.Context, p timeutil.Period) ([]LeaderboardRow, error) {
	return sharedstore.Leaderboard(ctx, s.db, p)
}

// ThursdayStrip aggregiert die Strip-Kacheln komplett in SQL: Donnerstage bis
// heute (inkl. Sperrtage), Abmelde-Zahl je Tag, jüngste N, aufsteigend.
func (s *Postgres) ThursdayStrip(ctx context.Context, p timeutil.Period, limit int) ([]StripDay, error) {
	const q = `
		WITH days AS (
			SELECT d::date AS day
			FROM generate_series($1::date, LEAST($2::date, current_date), interval '1 day') AS d
			WHERE EXTRACT(DOW FROM d) = 4
		),
		recent AS (
			SELECT day,
			       EXISTS (SELECT 1 FROM excluded_days ed WHERE ed.date = day) AS excluded,
			       (SELECT count(*) FROM stammtisch_abwesenheit a WHERE a.date = day)::int AS away
			FROM days
			ORDER BY day DESC
			LIMIT CASE WHEN $3 > 0 THEN $3 END
		)
		SELECT day, excluded, away FROM recent ORDER BY day ASC`
	rows, err := s.db.QueryContext(ctx, q, p.Start, p.End, limit)
	if err != nil {
		return nil, fmt.Errorf("ThursdayStrip: %w", err)
	}
	defer rows.Close()

	var out []StripDay
	for rows.Next() {
		var d StripDay
		if err := rows.Scan(&d.Date, &d.Excluded, &d.Away); err != nil {
			return nil, fmt.Errorf("ThursdayStrip scan: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// AbsencesOn liefert die Abmeldungen genau eines Datums.
func (s *Postgres) AbsencesOn(ctx context.Context, date time.Time) ([]Absence, error) {
	const q = `
		SELECT "userId", date, message
		FROM stammtisch_abwesenheit
		WHERE date = $1
		ORDER BY "userId"`
	rows, err := s.db.QueryContext(ctx, q, date)
	if err != nil {
		return nil, fmt.Errorf("AbsencesOn: %w", err)
	}
	defer rows.Close()

	var out []Absence
	for rows.Next() {
		var a Absence
		if err := rows.Scan(&a.UserID, &a.Date, &a.Message); err != nil {
			return nil, fmt.Errorf("AbsencesOn scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListUserAbsences liefert die Abmeldungen eines Users im Zeitraum.
func (s *Postgres) ListUserAbsences(ctx context.Context, p timeutil.Period, userID string) ([]Absence, error) {
	const q = `
		SELECT "userId", date, message
		FROM stammtisch_abwesenheit
		WHERE "userId" = $3
		  AND date >= $1 AND date <= $2
		  AND EXTRACT(DOW FROM date) = 4
		  AND date NOT IN (SELECT date FROM excluded_days)
		ORDER BY date DESC`
	rows, err := s.db.QueryContext(ctx, q, p.Start, p.EffectiveEnd(), userID)
	if err != nil {
		return nil, fmt.Errorf("ListUserAbsences: %w", err)
	}
	defer rows.Close()

	var out []Absence
	for rows.Next() {
		var a Absence
		if err := rows.Scan(&a.UserID, &a.Date, &a.Message); err != nil {
			return nil, fmt.Errorf("ListUserAbsences scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// IsExcludedDay prüft einen einzelnen Tag per EXISTS.
func (s *Postgres) IsExcludedDay(ctx context.Context, date time.Time) (bool, error) {
	const q = `SELECT EXISTS (SELECT 1 FROM excluded_days WHERE date = $1)`
	var excluded bool
	if err := s.db.QueryRowContext(ctx, q, date).Scan(&excluded); err != nil {
		return false, fmt.Errorf("IsExcludedDay: %w", err)
	}
	return excluded, nil
}

func (s *Postgres) InsertAbsence(ctx context.Context, userID string, date time.Time, message *string) error {
	const q = `
		INSERT INTO public.stammtisch_abwesenheit ("userId", date, message)
		VALUES ($1, $2, $3)
		ON CONFLICT ("userId", date) DO UPDATE SET message = EXCLUDED.message`
	if _, err := s.db.ExecContext(ctx, q, userID, date, message); err != nil {
		return fmt.Errorf("InsertAbsence: %w", err)
	}
	return nil
}

func (s *Postgres) DeleteAbsence(ctx context.Context, userID string, date time.Time) error {
	const q = `DELETE FROM public.stammtisch_abwesenheit WHERE "userId" = $1 AND date = $2`
	if _, err := s.db.ExecContext(ctx, q, userID, date); err != nil {
		return fmt.Errorf("DeleteAbsence: %w", err)
	}
	return nil
}

// ToggleAbsence kippt den Abmelde-Status ohne vorherigen Lese-Roundtrip:
// DELETE zuerst – hat es getroffen, war der User abgemeldet und ist jetzt
// anwesend; sonst wird die Abmeldung eingetragen.
func (s *Postgres) ToggleAbsence(ctx context.Context, userID string, date time.Time) (bool, error) {
	const del = `DELETE FROM public.stammtisch_abwesenheit WHERE "userId" = $1 AND date = $2`
	res, err := s.db.ExecContext(ctx, del, userID, date)
	if err != nil {
		return false, fmt.Errorf("ToggleAbsence delete: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return false, nil
	}
	if err := s.InsertAbsence(ctx, userID, date, nil); err != nil {
		return false, fmt.Errorf("ToggleAbsence: %w", err)
	}
	return true, nil
}

func (s *Postgres) InsertExcludedDay(ctx context.Context, date time.Time) error {
	// Idempotent ohne Abhängigkeit von einem UNIQUE-Constraint auf excluded_days(date).
	const q = `INSERT INTO public.excluded_days (date)
		SELECT $1::date
		WHERE NOT EXISTS (SELECT 1 FROM public.excluded_days WHERE date = $1)`
	if _, err := s.db.ExecContext(ctx, q, date); err != nil {
		return fmt.Errorf("InsertExcludedDay: %w", err)
	}
	return nil
}

func (s *Postgres) DeleteExcludedDay(ctx context.Context, date time.Time) error {
	const q = `DELETE FROM public.excluded_days WHERE date = $1`
	if _, err := s.db.ExecContext(ctx, q, date); err != nil {
		return fmt.Errorf("DeleteExcludedDay: %w", err)
	}
	return nil
}

func (s *Postgres) ListTraces(ctx context.Context, limit int) ([]Trace, error) {
	const q = `
		SELECT id, created_at, user_name, message, message_type, path,
		       classification, action, has_error
		FROM bot_trace
		WHERE created_at > now() - interval '21 days'
		ORDER BY created_at DESC
		LIMIT $1`
	rows, err := s.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("ListTraces: %w", err)
	}
	defer rows.Close()

	var out []Trace
	for rows.Next() {
		var t Trace
		if err := rows.Scan(&t.ID, &t.CreatedAt, &t.UserName, &t.Message,
			&t.MessageType, &t.Path, &t.Classification, &t.Action, &t.HasError); err != nil {
			return nil, fmt.Errorf("ListTraces scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Postgres) GetTrace(ctx context.Context, id int64) (*Trace, error) {
	const q = `
		SELECT id, created_at, remote_jid, user_id, user_name, message, message_type,
		       path, classification, action, has_error,
		       COALESCE(raw_payload::text, ''), trace::text
		FROM bot_trace WHERE id = $1`
	var t Trace
	var rawPayload, stepsJSON string
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&t.ID, &t.CreatedAt, &t.RemoteJid, &t.UserID, &t.UserName, &t.Message,
		&t.MessageType, &t.Path, &t.Classification, &t.Action, &t.HasError,
		&rawPayload, &stepsJSON)
	if err != nil {
		return nil, fmt.Errorf("GetTrace: %w", err)
	}
	if err := json.Unmarshal([]byte(stepsJSON), &t.Steps); err != nil {
		return nil, fmt.Errorf("GetTrace steps: %w", err)
	}
	if rawPayload != "" {
		var pretty bytes.Buffer
		if json.Indent(&pretty, []byte(rawPayload), "", "  ") == nil {
			t.RawPayload = pretty.String()
		} else {
			t.RawPayload = rawPayload
		}
	}
	return &t, nil
}

// --- Probebetrieb (ml_messages) ---

func (s *Postgres) MLShadowStats(ctx context.Context) (MLShadowStats, error) {
	var st MLShadowStats
	const totals = `
		SELECT count(*),
		       count(*) FILTER (WHERE model_label IS NOT NULL),
		       count(*) FILTER (WHERE agree),
		       count(*) FILTER (WHERE verified)
		FROM ml_messages`
	if err := s.db.QueryRowContext(ctx, totals).Scan(&st.Total, &st.WithModel, &st.Agree, &st.Verified); err != nil {
		return st, fmt.Errorf("MLShadowStats: %w", err)
	}
	const perLabel = `
		SELECT COALESCE(gemini_label,''), count(*), count(*) FILTER (WHERE agree)
		FROM ml_messages
		GROUP BY 1 ORDER BY 1`
	rows, err := s.db.QueryContext(ctx, perLabel)
	if err != nil {
		return st, fmt.Errorf("MLShadowStats per label: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ls MLLabelStat
		if err := rows.Scan(&ls.Label, &ls.Total, &ls.Agree); err != nil {
			return st, fmt.Errorf("MLShadowStats scan: %w", err)
		}
		st.PerLabel = append(st.PerLabel, ls)
	}
	return st, rows.Err()
}

// --- Manueller ML-Test (ml_test_messages) ---

// EnsureMLSchema legt die ML-Tabellen des Admin-UI idempotent an: die
// manuellen Testfälle und die synthetischen Trainingsnachrichten (Schreiber
// ist jeweils das Admin-UI bzw. das Import-Skript, daher liegt das Schema
// hier und nicht im Bot). ml_messages gehört dem Bot; hier kommt nur die
// Spalte labeled_at dazu – die legt auch der Bot an, falls er die Tabelle
// erst nach dem Admin-UI erzeugt.
func (s *Postgres) EnsureMLSchema(ctx context.Context) error {
	const q = `
		CREATE TABLE IF NOT EXISTS ml_test_messages (
		  id               BIGSERIAL PRIMARY KEY,
		  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
		  message          TEXT NOT NULL,
		  model_label      TEXT NOT NULL,
		  model_confidence DOUBLE PRECISION NOT NULL,
		  expected_label   TEXT
		);
		CREATE INDEX IF NOT EXISTS ml_test_messages_created_idx
		  ON ml_test_messages (created_at DESC);
		ALTER TABLE ml_test_messages ADD COLUMN IF NOT EXISTS labeled_at TIMESTAMPTZ;

		ALTER TABLE IF EXISTS ml_messages ADD COLUMN IF NOT EXISTS labeled_at TIMESTAMPTZ;
		ALTER TABLE IF EXISTS ml_messages ADD COLUMN IF NOT EXISTS entschieden_von TEXT;
		ALTER TABLE IF EXISTS ml_messages ADD COLUMN IF NOT EXISTS modell_version TEXT;

		-- text_hash: Hash des normalisierten Texts (Import-Skript), blockt
		-- exakte Duplikate schon beim Insert.
		CREATE TABLE IF NOT EXISTS ml_synthetic_messages (
		  id              BIGSERIAL PRIMARY KEY,
		  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
		  message         TEXT NOT NULL,
		  text_hash       TEXT NOT NULL UNIQUE,
		  generated_label TEXT CHECK (generated_label IN ('true', 'false', 'invalid')),
		  label           TEXT CHECK (label IN ('true', 'false', 'invalid')),
		  art             TEXT,
		  sprache         TEXT,
		  batch_id        TEXT,
		  verworfen       BOOLEAN NOT NULL DEFAULT false,
		  labeled_at      TIMESTAMPTZ
		);
		CREATE INDEX IF NOT EXISTS ml_synthetic_messages_created_idx
		  ON ml_synthetic_messages (created_at DESC);`
	_, err := s.db.ExecContext(ctx, q)
	return err
}

func (s *Postgres) InsertMLTest(ctx context.Context, message, modelLabel string, confidence float64) (int64, error) {
	const q = `
		INSERT INTO ml_test_messages (message, model_label, model_confidence)
		VALUES ($1, $2, $3) RETURNING id`
	var id int64
	if err := s.db.QueryRowContext(ctx, q, message, modelLabel, confidence).Scan(&id); err != nil {
		return 0, fmt.Errorf("InsertMLTest: %w", err)
	}
	return id, nil
}

func (s *Postgres) ListMLTests(ctx context.Context, limit int) ([]MLTestMessage, error) {
	const q = `
		SELECT id, created_at, message, model_label, model_confidence, expected_label
		FROM ml_test_messages
		ORDER BY created_at DESC
		LIMIT $1`
	rows, err := s.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("ListMLTests: %w", err)
	}
	defer rows.Close()

	var out []MLTestMessage
	for rows.Next() {
		var m MLTestMessage
		if err := rows.Scan(&m.ID, &m.CreatedAt, &m.Message, &m.ModelLabel,
			&m.ModelConfidence, &m.ExpectedLabel); err != nil {
			return nil, fmt.Errorf("ListMLTests scan: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Postgres) JudgeMLTest(ctx context.Context, id int64, expectedLabel string) (*MLTestMessage, error) {
	const q = `
		UPDATE ml_test_messages SET expected_label = $2, labeled_at = now()
		WHERE id = $1
		RETURNING id, created_at, message, model_label, model_confidence, expected_label`
	var m MLTestMessage
	err := s.db.QueryRowContext(ctx, q, id, expectedLabel).Scan(
		&m.ID, &m.CreatedAt, &m.Message, &m.ModelLabel, &m.ModelConfidence, &m.ExpectedLabel)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("JudgeMLTest: Eintrag %d nicht gefunden", id)
	}
	if err != nil {
		return nil, fmt.Errorf("JudgeMLTest: %w", err)
	}
	return &m, nil
}

func (s *Postgres) DeleteMLTest(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM ml_test_messages WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("DeleteMLTest: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("DeleteMLTest: Eintrag %d nicht gefunden", id)
	}
	return nil
}

// --- ML-Daten (/ml-daten): alle drei Quellen ---

// mlSource beschreibt, wie eine Quelle auf MLItem abgebildet wird. Alle drei
// liefern dieselbe Spaltenliste (fehlende Felder als NULL bzw. leer), damit ein
// einziger Scan reicht.
type mlSource struct {
	table string
	cols  string // SELECT-/RETURNING-Liste in scanMLItem-Reihenfolge
	label string // SQL-Ausdruck des aktuell gültigen Labels
}

var mlSources = map[MLQuelle]mlSource{
	// Whatsapp: ein Label gilt erst, wenn der Eintrag geprüft ist; ohne
	// corrected_label war Gemini richtig (Semantik der alten Prüfung).
	MLWhatsapp: {
		table: "ml_messages",
		cols: `id, created_at, message, COALESCE(user_name, ''), gemini_label,
		       model_label, model_confidence,
		       CASE WHEN verified THEN COALESCE(corrected_label, gemini_label) END,
		       labeled_at, NULL::text, '', '', '', false,
		       COALESCE(entschieden_von, ''), COALESCE(modell_version, '')`,
		label: "CASE WHEN verified THEN COALESCE(corrected_label, gemini_label) END",
	},
	MLManuell: {
		table: "ml_test_messages",
		cols: `id, created_at, message, '', NULL::text,
		       model_label, model_confidence, expected_label,
		       labeled_at, NULL::text, '', '', '', false, '', ''`,
		label: "expected_label",
	},
	MLSynthetics: {
		table: "ml_synthetic_messages",
		cols: `id, created_at, message, '', NULL::text,
		       NULL::text, NULL::float8, label,
		       labeled_at, generated_label, COALESCE(art, ''), COALESCE(sprache, ''),
		       COALESCE(batch_id, ''), verworfen, '', ''`,
		label: "label",
	},
}

func mlSourceFor(q MLQuelle) (mlSource, error) {
	src, ok := mlSources[q]
	if !ok {
		return mlSource{}, fmt.Errorf("unbekannte Quelle %q", q)
	}
	return src, nil
}

func scanMLItem(q MLQuelle, sc interface{ Scan(...any) error }) (MLItem, error) {
	it := MLItem{Quelle: q}
	err := sc.Scan(&it.ID, &it.CreatedAt, &it.Message, &it.UserName, &it.GeminiLabel,
		&it.ModelLabel, &it.ModelConfidence, &it.Label,
		&it.LabeledAt, &it.GeneratedLabel, &it.Art, &it.Sprache, &it.BatchID, &it.Verworfen,
		&it.DecidedBy, &it.ModelVersion)
	return it, err
}

func (s *Postgres) ListMLItems(ctx context.Context, q MLQuelle, f MLFilter) ([]MLItem, error) {
	src, err := mlSourceFor(q)
	if err != nil {
		return nil, err
	}
	var where []string
	var args []any
	switch f.Status {
	case MLOffen:
		where = append(where, "labeled_at IS NULL")
	case MLGeprueft:
		where = append(where, "labeled_at IS NOT NULL")
	case MLAbweichung:
		if q == MLWhatsapp {
			where = append(where, "gemini_label IS DISTINCT FROM model_label")
		}
	}
	if f.Label != "" {
		args = append(args, f.Label)
		where = append(where, fmt.Sprintf("%s = $%d", src.label, len(args)))
	}
	if f.Query != "" {
		args = append(args, f.Query)
		where = append(where, fmt.Sprintf("strpos(lower(message), lower($%d)) > 0", len(args)))
	}
	query := "SELECT " + src.cols + " FROM " + src.table
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 500
	}
	args = append(args, limit)
	query += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", len(args))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("ListMLItems %s: %w", q, err)
	}
	defer rows.Close()
	var out []MLItem
	for rows.Next() {
		it, err := scanMLItem(q, rows)
		if err != nil {
			return nil, fmt.Errorf("ListMLItems %s scan: %w", q, err)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Postgres) MLCounts(ctx context.Context, q MLQuelle) (MLCounts, error) {
	src, err := mlSourceFor(q)
	if err != nil {
		return MLCounts{}, err
	}
	verworfen := "false"
	if q == MLSynthetics {
		verworfen = "verworfen"
	}
	done := "labeled_at IS NOT NULL AND NOT " + verworfen
	query := fmt.Sprintf(`
		SELECT count(*),
		       count(*) FILTER (WHERE labeled_at IS NULL),
		       count(*) FILTER (WHERE %[1]s),
		       count(*) FILTER (WHERE %[2]s),
		       count(*) FILTER (WHERE %[1]s AND %[3]s = 'true'),
		       count(*) FILTER (WHERE %[1]s AND %[3]s = 'false'),
		       count(*) FILTER (WHERE %[1]s AND %[3]s = 'invalid')
		FROM %[4]s`, done, verworfen, src.label, src.table)
	var c MLCounts
	if err := s.db.QueryRowContext(ctx, query).Scan(&c.Total, &c.Offen, &c.Geprueft,
		&c.Verworfen, &c.Zusage, &c.Absage, &c.Invalid); err != nil {
		return c, fmt.Errorf("MLCounts %s: %w", q, err)
	}
	return c, nil
}

func (s *Postgres) LabelMLItem(ctx context.Context, q MLQuelle, id int64, label string) (*MLItem, error) {
	src, err := mlSourceFor(q)
	if err != nil {
		return nil, err
	}
	var set string
	switch q {
	case MLWhatsapp:
		// Immer explizit setzen – auch wenn es Geminis Label ist.
		set = "verified = true, corrected_label = $2"
	case MLManuell:
		set = "expected_label = $2"
	case MLSynthetics:
		set = "label = $2, verworfen = false"
	}
	query := "UPDATE " + src.table + " SET " + set + ", labeled_at = now() WHERE id = $1 RETURNING " + src.cols
	it, err := scanMLItem(q, s.db.QueryRowContext(ctx, query, id, label))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("LabelMLItem: %s-Eintrag %d nicht gefunden", q, id)
	}
	if err != nil {
		return nil, fmt.Errorf("LabelMLItem %s: %w", q, err)
	}
	return &it, nil
}

func (s *Postgres) DiscardMLSynthetic(ctx context.Context, id int64) (*MLItem, error) {
	query := `UPDATE ml_synthetic_messages SET label = NULL, verworfen = true, labeled_at = now()
		WHERE id = $1 RETURNING ` + mlSources[MLSynthetics].cols
	it, err := scanMLItem(MLSynthetics, s.db.QueryRowContext(ctx, query, id))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("DiscardMLSynthetic: Eintrag %d nicht gefunden", id)
	}
	if err != nil {
		return nil, fmt.Errorf("DiscardMLSynthetic: %w", err)
	}
	return &it, nil
}
