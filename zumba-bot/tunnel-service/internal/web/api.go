// Package web ist die schmale HTTP-Schnittstelle des tunnel-service: Status
// lesen, ein bekanntes Ziel öffnen, schließen. Mehr kann sie nicht — und
// genau das ist der Zweck. Die volle Agent-API des ngrok-Agenten bliebe
// sonst für jeden Pod im Namespace erreichbar, inklusive "mach mir einen
// tcp-Tunnel auf Postgres".
package web

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/michael/zumba-tunnel/internal/config"
	"github.com/michael/zumba-tunnel/internal/controller"
)

type Server struct {
	ctrl *controller.Controller
	cfg  config.Config
}

func New(ctrl *controller.Controller, cfg config.Config) *Server {
	return &Server{ctrl: ctrl, cfg: cfg}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /status", s.handleStatus)
	mux.HandleFunc("POST /open", s.handleOpen)
	mux.HandleFunc("POST /close", s.handleClose)
	mux.HandleFunc("POST /close-all", s.handleCloseAll)
	return mux
}

type statusResponse struct {
	Targets           []controller.Status `json:"targets"`
	DefaultTTLSeconds int                 `json:"defaultTtlSeconds"`
	MaxTTLSeconds     int                 `json:"maxTtlSeconds"`
}

type targetRequest struct {
	Target     string `json:"target"`
	TTLSeconds int    `json:"ttlSeconds"`
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	s.writeStatus(w)
}

func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request) {
	var req targetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "ungültiges JSON", http.StatusBadRequest)
		return
	}
	err := s.ctrl.Open(req.Target, time.Duration(req.TTLSeconds)*time.Second)
	switch {
	case errors.Is(err, controller.ErrUnknownTarget):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, controller.ErrBusy):
		http.Error(w, err.Error(), http.StatusConflict)
	case err != nil:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	default:
		s.writeStatus(w)
	}
}

func (s *Server) handleClose(w http.ResponseWriter, r *http.Request) {
	var req targetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "ungültiges JSON", http.StatusBadRequest)
		return
	}
	err := s.ctrl.Close(req.Target)
	switch {
	case errors.Is(err, controller.ErrUnknownTarget):
		http.Error(w, err.Error(), http.StatusNotFound)
	case err != nil:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	default:
		s.writeStatus(w)
	}
}

func (s *Server) handleCloseAll(w http.ResponseWriter, _ *http.Request) {
	s.ctrl.CloseAll()
	s.writeStatus(w)
}

// writeStatus ist auch die Antwort auf open/close: das Admin-UI bekommt den
// neuen Zustand in einem Zug und muss nicht sofort nachfragen.
func (s *Server) writeStatus(w http.ResponseWriter) {
	resp := statusResponse{
		Targets:           s.ctrl.Status(),
		DefaultTTLSeconds: int(s.cfg.DefaultTTL.Seconds()),
		MaxTTLSeconds:     int(s.cfg.MaxTTL.Seconds()),
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Antwort schreiben: %v", err)
	}
}
