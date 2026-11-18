package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Woher der KI-Text eines Tages kommt.
const (
	WirtQuelleAuto = "auto" // vom CronJob am Vormittag vorgeneriert
	WirtQuelleTest = "test" // im Bot-Test erzeugt und von Hand übernommen
)

// Was am Nachmittag tatsächlich rausging.
const (
	WirtAlsKI       = "ki"
	WirtAlsStandard = "standard"
)

// WirtSettings sind die Einstellungen der Tisch-Erinnerung an den Wirt
// (Admin-UI-Seite "Tisch-Erinnerung").
type WirtSettings struct {
	// Versand: der automatische Versand am Donnerstag ist eingeschaltet. Ohne
	// gespeicherte Einstellung gilt true – so bleibt nach einem frischen
	// Deploy alles beim Alten.
	Versand bool
	// Model schreibt den Text. "" = noch nicht gesetzt, der Bot nimmt dann
	// den Wert aus seiner Konfiguration.
	Model     string
	UpdatedAt time.Time
}

// WirtNachricht ist der Stand der Erinnerung eines Donnerstags: der
// vorgenerierte KI-Text, ob er abgelehnt wurde und was am Ende rausging.
type WirtNachricht struct {
	Tag time.Time
	// Text ist der KI-Text mit Platzhalter für die Markierung (wirt.Platzhalter),
	// ohne Kopf und Fußzeile. "" = es gibt keinen (Generierung fehlgeschlagen
	// oder nie gelaufen).
	Text   string
	Model  string
	Quelle string // WirtQuelleAuto | WirtQuelleTest
	// Fehler: warum die Vorgenerierung keinen Text geliefert hat.
	Fehler      string
	ErzeugtAm   time.Time
	AbgelehntAm *time.Time
	GesendetAm  *time.Time
	GesendetAls string // WirtAlsKI | WirtAlsStandard ("" = nicht gesendet)
}

// Verwendbar: es gibt einen KI-Text und er wurde nicht abgelehnt – er geht
// also raus. Sonst die Standardnachricht.
func (n *WirtNachricht) Verwendbar() bool {
	return n != nil && n.Text != "" && n.AbgelehntAm == nil
}

// EnsureWirtSchema legt die Tabellen der Tisch-Erinnerung idempotent an. Wie
// bei classifier_models reiht der Advisory-Lock den gleichzeitigen Start von
// Bot und Admin-UI hintereinander.
func EnsureWirtSchema(ctx context.Context, e Execer) error {
	const q = `
		SELECT pg_advisory_xact_lock(hashtext('wirt_settings'));
		CREATE TABLE IF NOT EXISTS wirt_settings (
		  id         SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
		  versand    BOOLEAN NOT NULL DEFAULT true,
		  model      TEXT,
		  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE IF NOT EXISTS wirt_nachrichten (
		  tag          DATE PRIMARY KEY,
		  text         TEXT NOT NULL DEFAULT '',
		  model        TEXT NOT NULL DEFAULT '',
		  quelle       TEXT NOT NULL DEFAULT 'auto' CHECK (quelle IN ('auto','test')),
		  fehler       TEXT,
		  erzeugt_am   TIMESTAMPTZ NOT NULL DEFAULT now(),
		  abgelehnt_am TIMESTAMPTZ,
		  gesendet_am  TIMESTAMPTZ,
		  gesendet_als TEXT CHECK (gesendet_als IN ('ki','standard'))
		)`
	_, err := e.ExecContext(ctx, q)
	return err
}

// SeedWirtModel trägt das Startmodell nur ein, solange keines gewählt ist –
// eine Umstellung im Admin-UI überlebt so jeden Neustart. Ein leerer Wert
// schreibt nichts.
func SeedWirtModel(ctx context.Context, e Execer, model string) error {
	if model == "" {
		return nil
	}
	const q = `
		INSERT INTO wirt_settings (id, model) VALUES (1, $1)
		ON CONFLICT (id) DO UPDATE SET model = EXCLUDED.model
		WHERE wirt_settings.model IS NULL`
	_, err := e.ExecContext(ctx, q, model)
	return err
}

// GetWirtSettings liefert die Einstellungen; ohne Zeile gilt: Versand an,
// kein Modell gewählt.
func GetWirtSettings(ctx context.Context, q RowQueryer) (WirtSettings, error) {
	s := WirtSettings{Versand: true}
	var model sql.NullString
	err := q.QueryRowContext(ctx,
		`SELECT versand, model, updated_at FROM wirt_settings WHERE id = 1`,
	).Scan(&s.Versand, &model, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return WirtSettings{Versand: true}, nil
	}
	if err != nil {
		return WirtSettings{Versand: true}, fmt.Errorf("wirt_settings: %w", err)
	}
	s.Model = model.String
	return s, nil
}

// SetWirtVersand schaltet den automatischen Versand ein oder aus (Upsert).
func SetWirtVersand(ctx context.Context, e Execer, versand bool) error {
	const q = `
		INSERT INTO wirt_settings (id, versand, updated_at) VALUES (1, $1, now())
		ON CONFLICT (id) DO UPDATE SET versand = EXCLUDED.versand, updated_at = now()`
	_, err := e.ExecContext(ctx, q, versand)
	return err
}

// SetWirtModel wählt das Modell, das den Text schreibt (Upsert). Die Prüfung,
// ob das Modell erlaubt ist, liegt beim Aufrufer.
func SetWirtModel(ctx context.Context, e Execer, model string) error {
	const q = `
		INSERT INTO wirt_settings (id, model, updated_at) VALUES (1, $1, now())
		ON CONFLICT (id) DO UPDATE SET model = EXCLUDED.model, updated_at = now()`
	_, err := e.ExecContext(ctx, q, model)
	return err
}

const wirtNachrichtCols = `tag, text, model, quelle, COALESCE(fehler, ''), erzeugt_am,
	abgelehnt_am, gesendet_am, COALESCE(gesendet_als, '')`

type rowScanner interface{ Scan(dest ...any) error }

func scanWirtNachricht(r rowScanner) (WirtNachricht, error) {
	var (
		n                   WirtNachricht
		abgelehnt, gesendet sql.NullTime
	)
	err := r.Scan(&n.Tag, &n.Text, &n.Model, &n.Quelle, &n.Fehler, &n.ErzeugtAm,
		&abgelehnt, &gesendet, &n.GesendetAls)
	if abgelehnt.Valid {
		n.AbgelehntAm = &abgelehnt.Time
	}
	if gesendet.Valid {
		n.GesendetAm = &gesendet.Time
	}
	return n, err
}

// isoDay: das Datum geht als Text raus – ein Zeitstempel würde je nach
// Zeitzone der DB beim Cast auf DATE den Tag verschieben.
func isoDay(t time.Time) string { return t.Format("2006-01-02") }

// GetWirtNachricht liefert den Stand des Tages tag; nil = es gibt (noch)
// nichts.
func GetWirtNachricht(ctx context.Context, q RowQueryer, tag time.Time) (*WirtNachricht, error) {
	n, err := scanWirtNachricht(q.QueryRowContext(ctx,
		`SELECT `+wirtNachrichtCols+` FROM wirt_nachrichten WHERE tag = $1::date`, isoDay(tag)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("wirt_nachrichten: %w", err)
	}
	return &n, nil
}

// ListWirtNachrichten liefert die letzten limit Tage, jüngster zuerst.
func ListWirtNachrichten(ctx context.Context, q Queryer, limit int) ([]WirtNachricht, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT `+wirtNachrichtCols+` FROM wirt_nachrichten ORDER BY tag DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("wirt_nachrichten: %w", err)
	}
	defer rows.Close()
	var out []WirtNachricht
	for rows.Next() {
		n, err := scanWirtNachricht(rows)
		if err != nil {
			return nil, fmt.Errorf("wirt_nachrichten scan: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// SaveWirtEntwurf speichert den KI-Text für den Tag tag. Mit ersetzen=false
// (Vorgenerierung) bleibt ein vorhandener Text unangetastet – auch ein
// abgelehnter, sonst höbe ein Wiederholungslauf die Ablehnung auf. Mit
// ersetzen=true (von Hand übernommen) gilt der neue Text und eine frühere
// Ablehnung ist vom Tisch. Was schon gesendet ist, ändert niemand mehr.
// gespeichert=false heißt: nichts geschrieben.
func SaveWirtEntwurf(ctx context.Context, e Execer, tag time.Time, text, model, quelle string, ersetzen bool) (gespeichert bool, err error) {
	const q = `
		INSERT INTO wirt_nachrichten (tag, text, model, quelle) VALUES ($1::date, $2, $3, $4)
		ON CONFLICT (tag) DO UPDATE SET text = EXCLUDED.text, model = EXCLUDED.model,
		  quelle = EXCLUDED.quelle, fehler = NULL, erzeugt_am = now(), abgelehnt_am = NULL
		WHERE wirt_nachrichten.gesendet_am IS NULL AND ($5 OR wirt_nachrichten.text = '')`
	res, err := e.ExecContext(ctx, q, isoDay(tag), text, model, quelle, ersetzen)
	if err != nil {
		return false, fmt.Errorf("wirt_nachrichten: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// SaveWirtFehler hält fest, warum die Vorgenerierung am Tag tag keinen Text
// geliefert hat. Ein vorhandener Text bleibt unberührt.
func SaveWirtFehler(ctx context.Context, e Execer, tag time.Time, fehler string) error {
	const q = `
		INSERT INTO wirt_nachrichten (tag, fehler) VALUES ($1::date, $2)
		ON CONFLICT (tag) DO UPDATE SET fehler = EXCLUDED.fehler, erzeugt_am = now()
		WHERE wirt_nachrichten.gesendet_am IS NULL AND wirt_nachrichten.text = ''`
	_, err := e.ExecContext(ctx, q, isoDay(tag), fehler)
	return err
}

// SetWirtAbgelehnt lehnt den KI-Text des Tages ab (dann geht die
// Standardnachricht raus) oder nimmt die Ablehnung zurück. geaendert=false:
// es gibt keinen Text oder die Erinnerung ist schon gesendet.
func SetWirtAbgelehnt(ctx context.Context, e Execer, tag time.Time, abgelehnt bool) (geaendert bool, err error) {
	const q = `
		UPDATE wirt_nachrichten SET abgelehnt_am = CASE WHEN $2 THEN now() END
		WHERE tag = $1::date AND text <> '' AND gesendet_am IS NULL`
	res, err := e.ExecContext(ctx, q, isoDay(tag), abgelehnt)
	if err != nil {
		return false, fmt.Errorf("wirt_nachrichten: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// MarkWirtGesendet hält fest, dass die Erinnerung am Tag tag rausging und ob
// es der KI-Text oder die Standardnachricht war.
func MarkWirtGesendet(ctx context.Context, e Execer, tag time.Time, als string) error {
	if als != WirtAlsKI && als != WirtAlsStandard {
		return fmt.Errorf("wirt_nachrichten: unbekannte Variante %q", als)
	}
	const q = `
		INSERT INTO wirt_nachrichten (tag, gesendet_am, gesendet_als) VALUES ($1::date, now(), $2)
		ON CONFLICT (tag) DO UPDATE SET gesendet_am = now(), gesendet_als = EXCLUDED.gesendet_als`
	_, err := e.ExecContext(ctx, q, isoDay(tag), als)
	return err
}
