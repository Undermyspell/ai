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
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	var ev evolution.WebhookEvent
	if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
		log.Printf("⚠️  webhook: invalid JSON: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	s.process(r.Context(), ev)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) process(ctx context.Context, ev evolution.WebhookEvent) {
	msg := ev.Message()

	// Pfad 1: Statistik-Anfrage (n8n: IF "Statistik request").
	if strings.EqualFold(strings.TrimSpace(msg), "statistik") {
		s.sendStats(ctx, ev.RemoteJid())
		return
	}

	// Pfad 2: gültige Nachricht aus der Zumba-Gruppe an einem Donnerstag
	// (n8n: IF "Check for valid message from Zumba group", 3 Bedingungen AND).
	if ev.MessageType() != "conversation" || ev.RemoteJid() != s.groupJID || !s.isThursday() {
		return
	}

	res, err := s.classifier.Classify(ctx, msg)
	if err != nil {
		log.Printf("⚠️  classifier: %v (→ %s)", err, res)
	}

	userID := ev.UserID()
	today := s.today()
	switch res {
	case classifier.Absage: // "false" → Absage eintragen (UPSERT)
		if err := s.store.MarkAbsent(ctx, userID, today, msg); err != nil {
			log.Printf("⚠️  MarkAbsent(%s): %v", userID, err)
		} else {
			log.Printf("📝 Absage: %s (%s)", ev.UserName(), userID)
		}
	case classifier.Zusage: // "true" → Absage entfernen
		if err := s.store.MarkPresent(ctx, userID, today); err != nil {
			log.Printf("⚠️  MarkPresent(%s): %v", userID, err)
		} else {
			log.Printf("📝 Zusage: %s (%s)", ev.UserName(), userID)
		}
	default: // "invalid" → keine Aktion (Switch matcht nicht)
	}
}

func (s *Server) sendStats(ctx context.Context, receiver string) {
	stats, err := s.store.UserStats(ctx)
	if err != nil {
		log.Printf("⚠️  UserStats: %v", err)
		return
	}
	text := report.Build(stats)
	if err := s.sender.SendText(ctx, receiver, text); err != nil {
		log.Printf("⚠️  SendText(%s): %v", receiver, err)
		return
	}
	log.Printf("📊 Statistik gesendet an %s", receiver)
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
