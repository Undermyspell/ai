// Package web bildet die Verzweigungslogik des n8n-"Zumba"-Workflows ab:
// ein Webhook empfängt Evolution-Events und löst entweder den Statistik- oder
// den Klassifizierungs-Pfad aus.
package web

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/michael/zumba-whatsapp-bot/internal/classifier"
	"github.com/michael/zumba-whatsapp-bot/internal/evolution"
	"github.com/michael/zumba-whatsapp-bot/internal/report"
	"github.com/michael/zumba-whatsapp-bot/internal/store"
)

// Classifier klassifiziert eine Nachricht (entkoppelt für Tests).
type Classifier interface {
	Classify(ctx context.Context, message string) (classifier.Result, error)
}

// Sender verschickt WhatsApp-Texte (entkoppelt für Tests).
type Sender interface {
	SendText(ctx context.Context, number, text string) error
}

type Server struct {
	store      store.Store
	classifier Classifier
	sender     Sender
	groupJID   string
	location   *time.Location

	// Now ist überschreibbar für Tests (Donnerstag-Prüfung / Tagesdatum).
	Now func() time.Time
}

func New(st store.Store, cl Classifier, snd Sender, groupJID string, loc *time.Location) *Server {
	return &Server{
		store:      st,
		classifier: cl,
		sender:     snd,
		groupJID:   groupJID,
		location:   loc,
		Now:        time.Now,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /webhook/whatsapp", s.handleWebhook)
	mux.HandleFunc("POST /test", s.handleTest)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

// Outcome beschreibt das Ergebnis eines Webhook-/Test-Durchlaufs.
type Outcome struct {
	Path           string `json:"path"`           // "statistik" | "classify" | "ignored"
	Classification string `json:"classification"` // "true"|"false"|"invalid"
	Action         string `json:"action"`         // "marked_absent"|"marked_present"|"none"
	Message        string `json:"message"`        // Statistik-Text bzw. Eingabe-Text
	Recipient      string `json:"recipient"`
	Date           string `json:"date"`
	UserID         string `json:"userId"`
	Reason         string `json:"reason"`
}

func (s *Server) run(ctx context.Context, ev evolution.WebhookEvent, bypassGuards bool) Outcome {
	msg := ev.Message()

	if strings.EqualFold(strings.TrimSpace(msg), "statistik") {
		text := s.statsText(ctx, ev.RemoteJid())
		return Outcome{Path: "statistik", Message: text, Recipient: ev.RemoteJid()}
	}

	if !bypassGuards {
		if ev.MessageType() != "conversation" || ev.RemoteJid() != s.groupJID || !s.isThursday() {
			return Outcome{Path: "ignored", Reason: "guard: messageType/group/Donnerstag nicht erfüllt"}
		}
	}

	res, err := s.classifier.Classify(ctx, msg)
	if err != nil {
		log.Printf("⚠️  classifier: %v (→ %s)", err, res)
	}
	userID := ev.UserID()
	today := s.today()
	out := Outcome{
		Path:           "classify",
		Classification: string(res),
		Action:         "none",
		Message:        msg,
		Recipient:      ev.RemoteJid(),
		UserID:         userID,
		Date:           today.Format("2006-01-02"),
	}
	switch res {
	case classifier.Absage:
		if err := s.store.MarkAbsent(ctx, userID, today, msg); err != nil {
			log.Printf("⚠️  MarkAbsent(%s): %v", userID, err)
		} else {
			out.Action = "marked_absent"
			log.Printf("📝 Absage: %s (%s)", ev.UserName(), userID)
		}
	case classifier.Zusage:
		if err := s.store.MarkPresent(ctx, userID, today); err != nil {
			log.Printf("⚠️  MarkPresent(%s): %v", userID, err)
		} else {
			out.Action = "marked_present"
			log.Printf("📝 Zusage: %s (%s)", ev.UserName(), userID)
		}
	}
	return out
}

// statsText baut den Ranglisten-Text, sendet ihn über den Sender und gibt ihn zurück.
func (s *Server) statsText(ctx context.Context, receiver string) string {
	stats, err := s.store.UserStats(ctx)
	if err != nil {
		log.Printf("⚠️  UserStats: %v", err)
		return ""
	}
	text := report.Build(stats)
	if err := s.sender.SendText(ctx, receiver, text); err != nil {
		log.Printf("⚠️  SendText(%s): %v", receiver, err)
	} else {
		log.Printf("📊 Statistik gesendet an %s", receiver)
	}
	return text
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	var ev evolution.WebhookEvent
	if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
		log.Printf("⚠️  webhook: invalid JSON: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	s.run(r.Context(), ev, false)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleTest(w http.ResponseWriter, r *http.Request) {
	var ev evolution.WebhookEvent
	if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	out := s.run(r.Context(), ev, true)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (s *Server) isThursday() bool {
	return s.Now().In(s.location).Weekday() == time.Thursday
}

// today liefert das heutige Datum (Mitternacht) in der konfigurierten Zeitzone –
// entspricht dem n8n-Ausdruck $today.
func (s *Server) today() time.Time {
	now := s.Now().In(s.location)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.location)
}
