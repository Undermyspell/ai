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

type Shadow struct {
	db         *sql.DB
	serviceURL string
	client     *http.Client

	// Gemini holt Geminis Label nach, wenn das eigene Modell entschieden hat
	// (von main gesetzt; nil = dann fehlt Geminis Label im Protokoll).
	Gemini func(ctx context.Context, message string) (string, error)
}

func New(db *sql.DB, serviceURL string) *Shadow {
	return &Shadow{
		db:         db,
		serviceURL: serviceURL,
		client:     &http.Client{Timeout: 3 * time.Second},
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
// komplett asynchron (eigener Kontext), damit der Webhook-Handler nicht auf
// Service, Gemini oder DB wartet.
func (s *Shadow) RecordAsync(r Record) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.record(ctx, r); err != nil {
			log.Printf("⚠️  shadow: %v", err)
		}
	}()
}

func (s *Shadow) record(ctx context.Context, r Record) error {
	// Fehlende Label nachholen; bei Fehler trotzdem loggen (die Nachricht und
	// das andere Label sind als Daten auch so wertvoll).
	if r.Model == nil {
		if pred, err := s.Classify(ctx, r.Message); err != nil {
			log.Printf("⚠️  shadow classify: %v", err)
		} else {
			r.Model = &pred
		}
	}
	if r.GeminiLabel == "" && s.Gemini != nil {
		if label, err := s.Gemini(ctx, r.Message); err != nil {
			log.Printf("⚠️  shadow gemini: %v", err)
		} else {
			r.GeminiLabel = label
		}
	}

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

	const q = `
		INSERT INTO ml_messages
		  (user_id, user_name, message, gemini_label, model_label, model_confidence, agree,
		   entschieden_von, modell_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	if _, err := s.db.ExecContext(ctx, q, r.UserID, r.UserName, r.Message,
		gemini, modelLabel, confidence, agree, r.DecidedBy, version); err != nil {
		return fmt.Errorf("ml_messages insert: %w", err)
	}
	return nil
}
