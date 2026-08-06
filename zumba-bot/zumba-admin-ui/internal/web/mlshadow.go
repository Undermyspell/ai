package web

import (
	"net/http"
	"strconv"

	"github.com/michael/zumba-admin-ui/web/templates/mldocs"
	"github.com/michael/zumba-admin-ui/web/templates/mlshadow"
)

const mlListLimit = 300

var mlValidLabels = map[string]bool{"true": true, "false": true, "invalid": true}

func (s *Server) handleMLShadow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	onlyDisagree := r.URL.Query().Get("filter") == "disagree"

	stats, err := s.store.MLShadowStats(ctx)
	if err != nil {
		s.fail(w, "ml stats", err)
		return
	}
	msgs, err := s.store.ListMLMessages(ctx, onlyDisagree, mlListLimit)
	if err != nil {
		s.fail(w, "ml messages", err)
		return
	}
	vm := mlshadow.VM{Stats: stats, Messages: msgs, OnlyDisagree: onlyDisagree}
	s.render(w, r, s.meta("ML-Shadow", "mlshadow"), mlshadow.Page(vm))
}

// handleMLVerify setzt das handgeprüfte Label eines ml_messages-Eintrags und
// liefert die aktualisierte Zeile als HTMX-Partial zurück.
func (s *Server) handleMLVerify(w http.ResponseWriter, r *http.Request) {
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

	// UPDATE ... RETURNING liefert die Zeile fürs Swap direkt mit.
	m, err := s.store.VerifyMLMessage(ctx, id, &label)
	if err != nil {
		s.triggerToast(w, "error", "Speichern fehlgeschlagen.")
		s.fail(w, "ml verify", err)
		return
	}

	s.triggerToast(w, "success", "Label gespeichert.")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = mlshadow.Row(*m).Render(ctx, w)
}

func (s *Server) handleMLDocs(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, s.meta("ML-Doku", "mldocs"), mldocs.Page())
}
