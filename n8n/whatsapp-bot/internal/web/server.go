// Package web bildet die Verzweigungslogik des n8n-"Zumba"-Workflows ab:
// ein Webhook empfängt Evolution-Events und löst entweder den Statistik- oder
// den Klassifizierungs-Pfad aus.
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/michael/zumba-whatsapp-bot/internal/classifier"
	"github.com/michael/zumba-whatsapp-bot/internal/evolution"
	"github.com/michael/zumba-whatsapp-bot/internal/report"
	"github.com/michael/zumba-whatsapp-bot/internal/store"
	"github.com/michael/zumba-whatsapp-bot/internal/tracestore"
)

// Classifier klassifiziert eine Nachricht (entkoppelt für Tests).
type Classifier interface {
	Classify(ctx context.Context, message string) (classifier.Classification, error)
}

// Sender verschickt WhatsApp-Texte (entkoppelt für Tests).
type Sender interface {
	SendText(ctx context.Context, number, text string) error
}

// Tracer persistiert einen aufgezeichneten Event-Trace (optional, nil = aus).
type Tracer interface {
	Save(ctx context.Context, t tracestore.Trace) error
}

type Server struct {
	store      store.Store
	classifier Classifier
	sender     Sender
	groupJID   string
	location   *time.Location

	// Now ist überschreibbar für Tests (Donnerstag-Prüfung / Tagesdatum).
	Now func() time.Time

	// Tracer zeichnet Gruppen-/Donnerstag-Events auf (von main gesetzt; nil = aus).
	Tracer Tracer

	// PreviewJID ist das Ziel des "Vorschau"-Modus der Bot-Test-Seite (von main
	// gesetzt; leer = Vorschau aus).
	PreviewJID string
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
	mux.HandleFunc("POST /weekly-report", s.handleWeekly)
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
	Action         string `json:"action"`         // marked_absent|marked_present|would_mark_absent|would_mark_present|none
	Message        string `json:"message"`        // Statistik-Text bzw. Eingabe-Text
	Recipient      string `json:"recipient"`
	Date           string `json:"date"`
	UserID         string `json:"userId"`
	Reason         string `json:"reason"`
	DryRun         bool   `json:"dryRun"`              // true: nichts gesendet/geschrieben, nur berechnet
	PreviewTo      string `json:"previewTo,omitempty"` // gesetzt: Nachricht wurde als Vorschau an diese Nummer geschickt
}

// run verarbeitet ein Event und protokolliert jeden Entscheidungspunkt im
// Recorder. bypassGuards überspringt die Donnerstag-/Gruppen-Prüfung (Test-Pfad).
// dryRun berechnet das Ergebnis, ohne zu senden oder in die DB zu schreiben.
func (s *Server) run(ctx context.Context, ev evolution.WebhookEvent, bypassGuards, dryRun bool, recs ...*tracestore.Recorder) Outcome {
	rec := tracestore.NewRecorder()
	if len(recs) > 0 && recs[0] != nil {
		rec = recs[0]
	}
	msg := ev.Message()
	rec.Step(tracestore.NodeReceived, tracestore.OutcomeInfo, "Webhook empfangen",
		fmt.Sprintf("%s (%s) · Typ %q", ev.UserName(), ev.UserID(), ev.MessageType()))

	// Verzweigung 1: "statistik"
	if strings.EqualFold(strings.TrimSpace(msg), "statistik") {
		rec.Step(tracestore.NodeCheckStatistik, tracestore.OutcomePass, `"statistik"?`, "ja")
		text := s.runStats(ctx, ev.RemoteJid(), dryRun, rec)
		return Outcome{Path: "statistik", Message: text, Recipient: ev.RemoteJid(), DryRun: dryRun}
	}
	rec.Step(tracestore.NodeCheckStatistik, tracestore.OutcomeInfo, `"statistik"?`, "nein")

	// Verzweigung 2: Guards (messageType / Gruppe / Donnerstag)
	if !bypassGuards {
		if ev.MessageType() != "conversation" {
			rec.Step(tracestore.NodeGuardType, tracestore.OutcomeFail, "messageType == conversation?", "nein: "+ev.MessageType())
			rec.Step(tracestore.NodeIgnored, tracestore.OutcomeInfo, "Ignoriert", "kein conversation-Event")
			return Outcome{Path: "ignored", Reason: "guard: messageType != conversation"}
		}
		rec.Step(tracestore.NodeGuardType, tracestore.OutcomePass, "messageType == conversation?", "ja")

		if ev.RemoteJid() != s.groupJID {
			rec.Step(tracestore.NodeGuardGroup, tracestore.OutcomeFail, "Zumba-Gruppe?", "nein")
			rec.Step(tracestore.NodeIgnored, tracestore.OutcomeInfo, "Ignoriert", "andere Gruppe/Chat")
			return Outcome{Path: "ignored", Reason: "guard: andere Gruppe"}
		}
		rec.Step(tracestore.NodeGuardGroup, tracestore.OutcomePass, "Zumba-Gruppe?", "ja")

		if !s.isThursday() {
			rec.Step(tracestore.NodeGuardThursday, tracestore.OutcomeFail, "Donnerstag?", "nein")
			rec.Step(tracestore.NodeIgnored, tracestore.OutcomeInfo, "Ignoriert", "nicht Donnerstag")
			return Outcome{Path: "ignored", Reason: "guard: nicht Donnerstag"}
		}
		rec.Step(tracestore.NodeGuardThursday, tracestore.OutcomePass, "Donnerstag?", s.today().Format("Mon, 2006-01-02"))
	} else {
		rec.Step(tracestore.NodeGuardType, tracestore.OutcomeInfo, "Guards", "übersprungen (Test-Pfad)")
	}

	// Classifier (Gemini)
	c, err := s.classifier.Classify(ctx, msg)
	if err != nil {
		rec.Step(tracestore.NodeClassify, tracestore.OutcomeError, "Classifier (Gemini)", err.Error())
		log.Printf("⚠️  classifier: %v (→ %s)", err, c.Result)
	} else {
		rec.Step(tracestore.NodeClassify, tracestore.OutcomeInfo, "Classifier (Gemini)",
			fmt.Sprintf("→ %s  (roh: %q · %s)", c.Result, c.Raw, c.Model))
	}

	userID := ev.UserID()
	today := s.today()
	out := Outcome{
		Path:           "classify",
		Classification: string(c.Result),
		Action:         "none",
		Message:        msg,
		Recipient:      ev.RemoteJid(),
		UserID:         userID,
		Date:           today.Format("2006-01-02"),
		DryRun:         dryRun,
	}
	switch c.Result {
	case classifier.Absage:
		if dryRun {
			out.Action = "would_mark_absent"
			rec.Step(tracestore.NodeMarkAbsent, tracestore.OutcomeInfo, "Absage: DB-Insert", "Dry-Run – nicht geschrieben")
		} else if err := s.store.MarkAbsent(ctx, userID, today, msg); err != nil {
			rec.Step(tracestore.NodeMarkAbsent, tracestore.OutcomeError, "Absage: DB-Insert", err.Error())
			log.Printf("⚠️  MarkAbsent(%s): %v", userID, err)
		} else {
			out.Action = "marked_absent"
			rec.Step(tracestore.NodeMarkAbsent, tracestore.OutcomePass, "Absage: DB-Insert", "eingetragen für "+out.Date)
			log.Printf("📝 Absage: %s (%s)", ev.UserName(), userID)
		}
	case classifier.Zusage:
		if dryRun {
			out.Action = "would_mark_present"
			rec.Step(tracestore.NodeMarkPresent, tracestore.OutcomeInfo, "Zusage: DB-Delete", "Dry-Run – nicht geschrieben")
		} else if err := s.store.MarkPresent(ctx, userID, today); err != nil {
			rec.Step(tracestore.NodeMarkPresent, tracestore.OutcomeError, "Zusage: DB-Delete", err.Error())
			log.Printf("⚠️  MarkPresent(%s): %v", userID, err)
		} else {
			out.Action = "marked_present"
			rec.Step(tracestore.NodeMarkPresent, tracestore.OutcomePass, "Zusage: DB-Delete", "entfernt für "+out.Date)
			log.Printf("📝 Zusage: %s (%s)", ev.UserName(), userID)
		}
	default:
		rec.Step(tracestore.NodeNoAction, tracestore.OutcomeInfo, "keine Aktion", "classification invalid")
	}
	return out
}

// runStats baut den Ranglisten-Text und protokolliert Berechnung + Versand.
func (s *Server) runStats(ctx context.Context, receiver string, dryRun bool, rec *tracestore.Recorder) string {
	stats, err := s.store.UserStats(ctx)
	if err != nil {
		rec.Step(tracestore.NodeBuildStats, tracestore.OutcomeError, "Statistik berechnen", err.Error())
		log.Printf("⚠️  UserStats: %v", err)
		return ""
	}
	text := report.Build(stats)
	rec.Step(tracestore.NodeBuildStats, tracestore.OutcomePass, "Statistik berechnen", fmt.Sprintf("%d Nutzer", len(stats)))
	if dryRun {
		rec.Step(tracestore.NodeSendStats, tracestore.OutcomeInfo, "An Gruppe senden", "Dry-Run – nicht gesendet")
		return text
	}
	if err := s.sender.SendText(ctx, receiver, text); err != nil {
		rec.Step(tracestore.NodeSendStats, tracestore.OutcomeError, "An Gruppe senden", err.Error())
		log.Printf("⚠️  SendText(%s): %v", receiver, err)
	} else {
		rec.Step(tracestore.NodeSendStats, tracestore.OutcomePass, "An Gruppe senden", "→ "+receiver)
		log.Printf("📊 Statistik gesendet an %s", receiver)
	}
	return text
}

// handleWeekly versendet den automatischen Wochenreport an die Zumba-Gruppe
// (per CronJob donnerstags 21:00 aufgerufen). ?dryRun=true berechnet den Text
// nur und sendet nicht – für den Test-Button im Admin-UI.
func (s *Server) handleWeekly(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	dryRun := q.Get("dryRun") == "true"
	preview := q.Get("preview") == "true" && s.PreviewJID != ""

	text := s.weeklyText(r.Context(), !(dryRun || preview))
	out := Outcome{Path: "statistik", Message: text, Recipient: s.groupJID, DryRun: dryRun || preview}
	if preview && text != "" {
		if err := s.sender.SendText(r.Context(), s.PreviewJID, text); err != nil {
			log.Printf("⚠️  Vorschau-Versand(%s): %v", s.PreviewJID, err)
		} else {
			out.PreviewTo = s.PreviewJID
			out.DryRun = false
			log.Printf("📱 Wochenreport-Vorschau gesendet an %s", s.PreviewJID)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// weeklyText baut den Wochenreport-Text (mit Hinweis-Header) und sendet ihn bei
// send=true an die konfigurierte Zumba-Gruppe.
func (s *Server) weeklyText(ctx context.Context, send bool) string {
	stats, err := s.store.UserStats(ctx)
	if err != nil {
		log.Printf("⚠️  UserStats: %v", err)
		return ""
	}
	text := report.BuildWeekly(stats)
	if send {
		if err := s.sender.SendText(ctx, s.groupJID, text); err != nil {
			log.Printf("⚠️  SendText(Wochenreport, %s): %v", s.groupJID, err)
		} else {
			log.Printf("📅 Wochenreport gesendet an %s", s.groupJID)
		}
	}
	return text
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var ev evolution.WebhookEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		log.Printf("⚠️  webhook: invalid JSON: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	rec := tracestore.NewRecorder()
	out := s.run(r.Context(), ev, false, false, rec)
	w.WriteHeader(http.StatusOK)
	s.recordTrace(r.Context(), ev, body, out, rec)
}

// recordTrace persistiert den Trace, aber nur für Events aus der Zumba-Gruppe an
// einem Donnerstag (best-effort: Fehler werden nur geloggt).
func (s *Server) recordTrace(ctx context.Context, ev evolution.WebhookEvent, body []byte, out Outcome, rec *tracestore.Recorder) {
	if s.Tracer == nil || ev.RemoteJid() != s.groupJID || !s.isThursday() {
		return
	}
	t := tracestore.Trace{
		RemoteJid:      ev.RemoteJid(),
		UserID:         ev.UserID(),
		UserName:       ev.UserName(),
		Message:        ev.Message(),
		MessageType:    ev.MessageType(),
		Path:           out.Path,
		Classification: out.Classification,
		Action:         out.Action,
		HasError:       rec.HasError(),
		RawPayload:     body,
		Steps:          rec.Steps(),
	}
	if err := s.Tracer.Save(ctx, t); err != nil {
		log.Printf("⚠️  trace save: %v", err)
	}
}

func (s *Server) handleTest(w http.ResponseWriter, r *http.Request) {
	var ev evolution.WebhookEvent
	if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	q := r.URL.Query()
	dryRun := q.Get("dryRun") == "true"
	preview := q.Get("preview") == "true" && s.PreviewJID != ""

	// Vorschau verhält sich wie Dry-Run (keine Gruppe/DB), schickt die erzeugte
	// Nachricht aber zusätzlich an die Vorschau-Nummer.
	out := s.run(r.Context(), ev, true, dryRun || preview, tracestore.NewRecorder())
	if preview && out.Path == "statistik" && out.Message != "" {
		if err := s.sender.SendText(r.Context(), s.PreviewJID, out.Message); err != nil {
			log.Printf("⚠️  Vorschau-Versand(%s): %v", s.PreviewJID, err)
		} else {
			out.PreviewTo = s.PreviewJID
			out.DryRun = false
			log.Printf("📱 Vorschau gesendet an %s", s.PreviewJID)
		}
	}
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
