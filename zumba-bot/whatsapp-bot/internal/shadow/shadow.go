// Package shadow spricht das eigene ML-Modell (classifier-service) an und
// protokolliert jede Klassifizierung dauerhaft in ml_messages (keine
// Retention — die Tabelle ist zugleich der wachsende Trainings-/Eval-Pool).
//
// Wer entscheidet, stellt das Admin-UI ein (/ki-modell): Entscheidet Gemini,
// fragt RecordAsync das eigene Modell im Hintergrund dazu; entscheidet das
// eigene Modell, holt RecordAsync Geminis Label nach. So stehen immer beide
// Label nebeneinander. Das Protokoll ist best-effort und darf den
// Webhook-Flow nie ausbremsen oder brechen.
package shadow

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// execer ist der DB-Zugriff des Protokolls (*sql.DB; entkoppelt für Tests).
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type Shadow struct {
	db         execer
	serviceURL string
	client     *http.Client

	// Gemini holt Geminis Label nach, wenn das eigene Modell entschieden hat
	// (von main gesetzt; nil = dann fehlt Geminis Label im Protokoll).
	Gemini func(ctx context.Context, message string) (string, error)

	// LabelTimeout begrenzt das Nachholen der Label (eigenes Modell, Gemini
	// samt Fallback). InsertTimeout gilt nur für den INSERT und läuft erst
	// danach los: Hängt Gemini, wird der Eintrag trotzdem geschrieben – nur
	// ohne Geminis Label.
	LabelTimeout  time.Duration
	InsertTimeout time.Duration
}

func New(db *sql.DB, serviceURL string) *Shadow {
	return &Shadow{
		db:         db,
		serviceURL: serviceURL,
		client:     &http.Client{Timeout: 3 * time.Second},
		// Gemini: Haupt- und Fallback-Modell mit je bis zu 30 s HTTP-Timeout.
		LabelTimeout:  45 * time.Second,
		InsertTimeout: 5 * time.Second,
	}
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS ml_messages (
  id               BIGSERIAL PRIMARY KEY,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  user_id          TEXT,
  user_name        TEXT,
  message          TEXT NOT NULL,
  gemini_label     TEXT,
  model_label      TEXT,
  model_confidence DOUBLE PRECISION,
  agree            BOOLEAN,
  verified         BOOLEAN NOT NULL DEFAULT false,
  corrected_label  TEXT
);
CREATE INDEX IF NOT EXISTS ml_messages_created_idx ON ml_messages (created_at DESC);
-- labeled_at: wann die Nachricht im Admin-UI (/ml-daten) von Hand gelabelt
-- wurde. Pflegt nur das Admin-UI; es legt die Spalte selbst auch an.
ALTER TABLE ml_messages ADD COLUMN IF NOT EXISTS labeled_at TIMESTAMPTZ;
-- entschieden_von: gemini | modell (NULL = vor dem Schalter, also Gemini);
-- modell_version: welches eigene Modell das model_label lieferte.
ALTER TABLE ml_messages ADD COLUMN IF NOT EXISTS entschieden_von TEXT;
ALTER TABLE ml_messages ADD COLUMN IF NOT EXISTS modell_version TEXT;`

// EnsureSchema legt ml_messages idempotent an (beim Start aufgerufen).
func (s *Shadow) EnsureSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schemaSQL)
	return err
}

// Prediction ist die Antwort des classifier-service (Label schon nach dessen
// Schwelle).
type Prediction struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
	Model      string  `json:"model"` // „NB-LogReg 0.2.0"
}

// Record ist ein Eintrag fürs Protokoll. Fehlende Label holt RecordAsync nach.
type Record struct {
	UserID, UserName, Message string
	DecidedBy                 string      // gemini | modell
	GeminiLabel               string      // "" = noch nicht da
	Model                     *Prediction // nil = noch nicht da

	// DedupeDay (Mitternacht, Ortszeit) gesetzt: nur schreiben, wenn es für
	// Nutzer und Text an diesem Tag noch keine Zeile gibt. Für die
	// Nachverarbeitung aus dem Verlauf – der Trainings-Pool soll keine
	// Doppelten bekommen, eine verlorene Zeile aber nachgetragen werden.
	DedupeDay time.Time
}

// Classify fragt das eigene Modell (synchron, 3 s Timeout).
func (s *Shadow) Classify(ctx context.Context, message string) (Prediction, error) {
	body, err := json.Marshal(map[string]string{"text": message})
	if err != nil {
		return Prediction{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.serviceURL+"/classify", bytes.NewReader(body))
	if err != nil {
		return Prediction{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return Prediction{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Prediction{}, fmt.Errorf("classifier-service: HTTP %d", resp.StatusCode)
	}
	var pred Prediction
	if err := json.NewDecoder(resp.Body).Decode(&pred); err != nil {
		return Prediction{}, err
	}
	return pred, nil
}

// RecordAsync holt fehlende Label nach und schreibt den Eintrag. Läuft
// komplett asynchron (eigene Kontexte), damit der Webhook-Handler nicht auf
// Service, Gemini oder DB wartet.
func (s *Shadow) RecordAsync(r Record) {
	go func() {
		if err := s.record(r); err != nil {
			log.Printf("⚠️  shadow: %v", err)
		}
	}()
}

func (s *Shadow) record(r Record) error {
	s.fillLabels(&r)

	// Eigener Kontext: das Nachholen darf die Zeit für den INSERT nicht
	// aufbrauchen (sonst geht die Nachricht dem Trainings-Pool verloren).
	ctx, cancel := context.WithTimeout(context.Background(), s.InsertTimeout)
	defer cancel()
	return s.insert(ctx, r)
}

// fillLabels holt fehlende Label nach, zusammen höchstens LabelTimeout lang.
// Fehler werden nur geloggt: die Nachricht und das andere Label sind als
// Daten auch so wertvoll.
func (s *Shadow) fillLabels(r *Record) {
	ctx, cancel := context.WithTimeout(context.Background(), s.LabelTimeout)
	defer cancel()
	if r.Model == nil {
		if pred, err := s.Classify(ctx, r.Message); err != nil {
			log.Printf("⚠️  shadow classify: %v", err)
		} else {
			r.Model = &pred
		}
	}
	if r.GeminiLabel == "" && s.Gemini != nil {
		if label, err := s.Gemini(ctx, r.Message); err != nil {
			log.Printf("⚠️  shadow gemini: %v (Eintrag ohne Geminis Label)", err)
		} else {
			r.GeminiLabel = label
		}
	}
}

const insertSQL = `
		INSERT INTO ml_messages
		  (user_id, user_name, message, gemini_label, model_label, model_confidence, agree,
		   entschieden_von, modell_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

// insertDedupeSQL schreibt nur, wenn es für Nutzer + Text im Zeitraum
// [$10, $11) noch keine Zeile gibt (Nachverarbeitung).
const insertDedupeSQL = `
		INSERT INTO ml_messages
		  (user_id, user_name, message, gemini_label, model_label, model_confidence, agree,
		   entschieden_von, modell_version)
		SELECT $1::text, $2::text, $3::text, $4::text, $5::text, $6::double precision, $7::boolean,
		       $8::text, $9::text
		WHERE NOT EXISTS (
		  SELECT 1 FROM ml_messages
		  WHERE user_id IS NOT DISTINCT FROM $1::text AND message = $3::text
		    AND created_at >= $10::timestamptz AND created_at < $11::timestamptz)`

func (s *Shadow) insert(ctx context.Context, r Record) error {
	var gemini, modelLabel, version sql.NullString
	var confidence sql.NullFloat64
	var agree sql.NullBool
	if r.GeminiLabel != "" {
		gemini = sql.NullString{String: r.GeminiLabel, Valid: true}
	}
	if r.Model != nil {
		modelLabel = sql.NullString{String: r.Model.Label, Valid: true}
		confidence = sql.NullFloat64{Float64: r.Model.Confidence, Valid: true}
		version = sql.NullString{String: r.Model.Model, Valid: r.Model.Model != ""}
		if gemini.Valid {
			agree = sql.NullBool{Bool: r.Model.Label == r.GeminiLabel, Valid: true}
		}
	}

	args := []any{r.UserID, r.UserName, r.Message,
		gemini, modelLabel, confidence, agree, r.DecidedBy, version}
	if r.DedupeDay.IsZero() {
		if _, err := s.db.ExecContext(ctx, insertSQL, args...); err != nil {
			return fmt.Errorf("ml_messages insert: %w", err)
		}
		return nil
	}
	args = append(args, r.DedupeDay, r.DedupeDay.AddDate(0, 0, 1))
	res, err := s.db.ExecContext(ctx, insertDedupeSQL, args...)
	if err != nil {
		return fmt.Errorf("ml_messages insert: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		log.Printf("ℹ️  shadow: Nachricht von %s ist für den %s schon protokolliert",
			r.UserName, r.DedupeDay.Format("02.01."))
	}
	return nil
}
