package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/michael/zumba-admin-ui/web/templates/mltest"
)

const mlTestLimit = 200

var mlTestClient = &http.Client{Timeout: 5 * time.Second}

func (s *Server) handleMLTest(w http.ResponseWriter, r *http.Request) {
	tests, err := s.store.ListMLTests(r.Context(), mlTestLimit)
	if err != nil {
		s.fail(w, "ml tests", err)
		return
	}
	configured := s.cfg.ClassifierURL != "" || s.mockMode
	s.render(w, r, s.meta("ML-Test", "mltest"), mltest.Page(tests, configured))
}

// handleMLTestRun klassifiziert die eingetippte Nachricht per classifier-service,
// speichert den Testfall und liefert die aktualisierte Liste als HTMX-Partial.
func (s *Server) handleMLTestRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	msg := strings.TrimSpace(r.FormValue("message"))
	if msg == "" {
		s.triggerToast(w, "error", "Nachricht ist leer.")
		http.Error(w, "empty message", http.StatusBadRequest)
		return
	}

	label, confidence, err := s.classifyMessage(msg)
	if err != nil {
		s.triggerToast(w, "error", "Classifier nicht erreichbar: "+err.Error())
		http.Error(w, "classifier unreachable", http.StatusBadGateway)
		return
	}

	if _, err := s.store.InsertMLTest(ctx, msg, label, confidence); err != nil {
		s.triggerToast(w, "error", "Speichern fehlgeschlagen.")
		s.fail(w, "ml test insert", err)
		return
	}

	tests, err := s.store.ListMLTests(ctx, mlTestLimit)
	if err != nil {
		s.fail(w, "ml tests reload", err)
		return
	}
	s.triggerToast(w, "success", "Klassifiziert — bitte bewerten.")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = mltest.List(tests).Render(ctx, w)
}

// handleMLTestJudge hinterlegt das erwartete Label und rendert die Zeile neu.
func (s *Server) handleMLTestJudge(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.triggerToast(w, "error", "Ungültige ID.")
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	label := r.URL.Query().Get("label")
	if !mlValidLabels[label] {
		s.triggerToast(w, "error", "Ungültiges Label.")
		http.Error(w, "bad label", http.StatusBadRequest)
		return
	}

	if err := s.store.JudgeMLTest(ctx, id, label); err != nil {
		s.triggerToast(w, "error", "Speichern fehlgeschlagen.")
		s.fail(w, "ml test judge", err)
		return
	}

	tests, err := s.store.ListMLTests(ctx, mlTestLimit)
	if err != nil {
		s.fail(w, "ml tests reload", err)
		return
	}
	for _, t := range tests {
		if t.ID == id {
			s.triggerToast(w, "success", "Bewertung gespeichert.")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_ = mltest.Row(t).Render(ctx, w)
			return
		}
	}
	s.triggerToast(w, "error", "Eintrag nicht gefunden.")
	http.Error(w, "not found", http.StatusNotFound)
}

// handleMLTestDelete löscht einen Testfall; die leere Antwort entfernt die
// Zeile per HTMX-outerHTML-Swap.
func (s *Server) handleMLTestDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.triggerToast(w, "error", "Ungültige ID.")
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteMLTest(r.Context(), id); err != nil {
		s.triggerToast(w, "error", "Löschen fehlgeschlagen.")
		s.fail(w, "ml test delete", err)
		return
	}
	s.triggerToast(w, "success", "Testfall gelöscht.")
	w.WriteHeader(http.StatusOK)
}

// classifyMessage ruft den classifier-service. Im Mock-Modus (keine DB, lokale
// Entwicklung) wird ohne konfigurierten Service eine feste Antwort geliefert.
func (s *Server) classifyMessage(msg string) (string, float64, error) {
	if s.cfg.ClassifierURL == "" {
		if s.mockMode {
			return "invalid", 0.5, nil
		}
		return "", 0, fmt.Errorf("CLASSIFIER_URL nicht konfiguriert")
	}
	body, _ := json.Marshal(map[string]string{"text": msg})
	url := strings.TrimRight(s.cfg.ClassifierURL, "/") + "/classify"
	resp, err := mlTestClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var pred struct {
		Label      string  `json:"label"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pred); err != nil {
		return "", 0, err
	}
	return pred.Label, pred.Confidence, nil
}
