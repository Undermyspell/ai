// Package shadow protokolliert den Shadow-Modus des eigenen ML-Klassifikators:
// jede von Gemini klassifizierte Nachricht wird zusätzlich an den
// classifier-service geschickt und beide Ergebnisse dauerhaft in ml_messages
// gespeichert (keine Retention — die Tabelle ist zugleich der wachsende
// Trainings-/Eval-Datenpool). Gemini entscheidet weiterhin allein; alles hier
// ist best-effort und darf den Webhook-Flow nie ausbremsen oder brechen.
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
CREATE INDEX IF NOT EXISTS ml_messages_created_idx ON ml_messages (created_at DESC);`

// EnsureSchema legt ml_messages idempotent an (beim Start aufgerufen).
func (s *Shadow) EnsureSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schemaSQL)
	return err
}

// RecordAsync klassifiziert die Nachricht per classifier-service und schreibt
// beide Label in ml_messages. Läuft komplett asynchron (eigener Kontext), damit
// der Webhook-Handler nicht auf Service oder DB wartet.
func (s *Shadow) RecordAsync(userID, userName, message, geminiLabel string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.record(ctx, userID, userName, message, geminiLabel); err != nil {
			log.Printf("⚠️  shadow: %v", err)
		}
	}()
}

type prediction struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
}

func (s *Shadow) record(ctx context.Context, userID, userName, message, geminiLabel string) error {
	// Modell-Ergebnis holen; bei Fehler trotzdem loggen (Nachricht + Gemini-Label
	// sind als Trainingsdaten auch ohne Modell-Label wertvoll).
	var modelLabel sql.NullString
	var confidence sql.NullFloat64
	var agree sql.NullBool

	pred, err := s.classify(ctx, message)
	if err != nil {
		log.Printf("⚠️  shadow classify: %v", err)
	} else {
		modelLabel = sql.NullString{String: pred.Label, Valid: true}
		confidence = sql.NullFloat64{Float64: pred.Confidence, Valid: true}
		agree = sql.NullBool{Bool: pred.Label == geminiLabel, Valid: true}
	}

	const q = `
		INSERT INTO ml_messages
		  (user_id, user_name, message, gemini_label, model_label, model_confidence, agree)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	if _, err := s.db.ExecContext(ctx, q,
		userID, userName, message, geminiLabel, modelLabel, confidence, agree); err != nil {
		return fmt.Errorf("ml_messages insert: %w", err)
	}
	return nil
}

func (s *Shadow) classify(ctx context.Context, message string) (prediction, error) {
	body, err := json.Marshal(map[string]string{"text": message})
	if err != nil {
		return prediction{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.serviceURL+"/classify", bytes.NewReader(body))
	if err != nil {
		return prediction{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return prediction{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return prediction{}, fmt.Errorf("classifier-service: HTTP %d", resp.StatusCode)
	}
	var pred prediction
	if err := json.NewDecoder(resp.Body).Decode(&pred); err != nil {
		return prediction{}, err
	}
	return pred, nil
}
