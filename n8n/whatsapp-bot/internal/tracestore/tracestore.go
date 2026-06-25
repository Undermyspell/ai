// Package tracestore persistiert einen Schritt-für-Schritt-Trace jedes
// aufgezeichneten Webhook-Events (Zumba-Gruppe, donnerstags) in der zumba-DB.
// Die Admin-UI liest die Traces und rendert daraus den Flow-Graphen.
package tracestore

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// Knoten-IDs des festen Bot-Flow-Graphen (die UI mappt sie auf Karten).
const (
	NodeReceived       = "received"
	NodeCheckStatistik = "check_statistik"
	NodeBuildStats     = "build_stats"
	NodeSendStats      = "send_stats"
	NodeGuardType      = "guard_type"
	NodeGuardGroup     = "guard_group"
	NodeGuardThursday  = "guard_thursday"
	NodeClassify       = "classify"
	NodeMarkAbsent     = "mark_absent"
	NodeMarkPresent    = "mark_present"
	NodeNoAction       = "no_action"
	NodeIgnored        = "ignored"
)

// Outcome-Werte eines Schritts (steuern die Farbcodierung in der UI).
const (
	OutcomePass  = "pass"  // Bedingung erfüllt / Aktion erfolgreich
	OutcomeFail  = "fail"  // Bedingung nicht erfüllt (führte zum Abbruch/Ignorieren)
	OutcomeInfo  = "info"  // neutraler Schritt
	OutcomeError = "error" // Fehler (DB/Classifier/Senden)
)

// Step ist ein einzelner Entscheidungspunkt im Trace.
type Step struct {
	Node    string `json:"node"`
	Outcome string `json:"outcome"`
	Label   string `json:"label"`
	Detail  string `json:"detail"`
}

// Recorder sammelt die Schritte während eines run()-Durchlaufs.
type Recorder struct{ steps []Step }

func NewRecorder() *Recorder { return &Recorder{} }

func (r *Recorder) Step(node, outcome, label, detail string) {
	r.steps = append(r.steps, Step{Node: node, Outcome: outcome, Label: label, Detail: detail})
}

func (r *Recorder) Steps() []Step { return r.steps }

func (r *Recorder) HasError() bool {
	for _, s := range r.steps {
		if s.Outcome == OutcomeError {
			return true
		}
	}
	return false
}

// Trace ist eine persistierte Aufzeichnung eines Events.
type Trace struct {
	ID             int64
	CreatedAt      time.Time
	RemoteJid      string
	UserID         string
	UserName       string
	Message        string
	MessageType    string
	Path           string
	Classification string
	Action         string
	HasError       bool
	RawPayload     json.RawMessage
	Steps          []Step
}

// Store schreibt Traces in die zumba-DB.
type Store struct{ db *sql.DB }

func New(db *sql.DB) *Store { return &Store{db: db} }

const schemaSQL = `
CREATE TABLE IF NOT EXISTS bot_trace (
  id             BIGSERIAL PRIMARY KEY,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  remote_jid     TEXT,
  user_id        TEXT,
  user_name      TEXT,
  message        TEXT,
  message_type   TEXT,
  path           TEXT,
  classification TEXT,
  action         TEXT,
  has_error      BOOLEAN NOT NULL DEFAULT false,
  raw_payload    JSONB,
  trace          JSONB NOT NULL
);
CREATE INDEX IF NOT EXISTS bot_trace_created_idx ON bot_trace (created_at DESC);`

// EnsureSchema legt die Tabelle idempotent an (beim Start aufgerufen).
func (s *Store) EnsureSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schemaSQL)
	return err
}

const retentionSQL = `DELETE FROM bot_trace WHERE created_at < now() - interval '21 days'`

const insertSQL = `
INSERT INTO bot_trace
  (remote_jid, user_id, user_name, message, message_type, path, classification, action, has_error, raw_payload, trace)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

// Save schreibt einen Trace und räumt anschließend Einträge älter als 21 Tage ab.
func (s *Store) Save(ctx context.Context, t Trace) error {
	steps, err := json.Marshal(t.Steps)
	if err != nil {
		return err
	}
	var rawPayload any
	if len(t.RawPayload) > 0 {
		rawPayload = []byte(t.RawPayload)
	}
	if _, err := s.db.ExecContext(ctx, insertSQL,
		t.RemoteJid, t.UserID, t.UserName, t.Message, t.MessageType,
		t.Path, t.Classification, t.Action, t.HasError, rawPayload, steps,
	); err != nil {
		return err
	}
	_, _ = s.db.ExecContext(ctx, retentionSQL) // best-effort
	return nil
}
