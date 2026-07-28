package web

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/michael/zumba-admin-ui/web/templates/trace"
)

// handleTraceList zeigt die letzten aufgezeichneten Bot-Events (max. 3 Wochen).
func (s *Server) handleTraceList(w http.ResponseWriter, r *http.Request) {
	traces, err := s.store.ListTraces(r.Context(), 200)
	if err != nil {
		s.fail(w, "ListTraces", err)
		return
	}
	s.render(w, r, s.meta("Verlauf", "trace"), trace.List(traces))
}

// handleTraceDetail zeigt den Flow-Graphen einer einzelnen Aufzeichnung.
func (s *Server) handleTraceDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "ungültige ID", http.StatusBadRequest)
		return
	}
	t, err := s.store.GetTrace(r.Context(), id)
	if err != nil {
		http.Error(w, "Aufzeichnung nicht gefunden", http.StatusNotFound)
		return
	}
	s.render(w, r, s.meta(fmt.Sprintf("Verlauf · #%d", t.ID), "trace"), trace.Detail(*t))
}
