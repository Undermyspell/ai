package web

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/michael/zumba-admin-ui/internal/store"
	"github.com/michael/zumba-admin-ui/web/templates/mldaten"
	"github.com/michael/zumba-admin-ui/web/templates/mldocs"
)

// mlListLimit begrenzt die Liste je Quelle; mehr ist zum Labeln nicht nötig
// (Filter „Offen" bzw. die Suche grenzen ein).
const mlListLimit = 500

var mlValidLabels = map[string]bool{"true": true, "false": true, "invalid": true}

// mlLabelVerworfen ist das vierte „Label" der Synthetics: Nachricht fliegt
// aus dem Training.
const mlLabelVerworfen = "verworfen"

// handleMLDaten zeigt eine Quelle (?quelle=synthetics|whatsapp|manuell) mit
// Zählern, Filtern und den Nachrichten zum Labeln.
func (s *Server) handleMLDaten(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := mlQuelle(r)
	f := mlFilter(r, q)

	items, err := s.store.ListMLItems(ctx, q, f)
	if err != nil {
		s.fail(w, "ml items", err)
		return
	}
	vm := mldaten.VM{Head: s.mlHead(r, q), Filter: f, Items: items, Limit: mlListLimit}
	s.render(w, r, s.meta("ML-Daten", "ml-"+string(q)), mldaten.Page(vm))
}

// handleMLDatenKopf liefert nur Tabs und Zähler – nach jedem Label lädt sich
// der Kopf darüber neu, ohne die Liste anzufassen.
func (s *Server) handleMLDatenKopf(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = mldaten.Head(s.mlHead(r, mlQuelle(r))).Render(r.Context(), w)
}

// handleMLLabel setzt oder ändert das Label einer Nachricht (?label=) und
// liefert die Zeile neu. Erfolg ohne Toast – beim Durchlabeln wären es
// hunderte; die Zeile selbst zeigt das neue Label.
func (s *Server) handleMLLabel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := store.MLQuelle(r.PathValue("quelle"))
	if !q.Valid() {
		s.triggerToast(w, "error", "Unbekannte Quelle.")
		http.Error(w, "bad quelle", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.triggerToast(w, "error", "Ungültige ID.")
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	label := r.URL.Query().Get("label")

	var it *store.MLItem
	switch {
	case label == mlLabelVerworfen && q == store.MLSynthetics:
		it, err = s.store.DiscardMLSynthetic(ctx, id)
	case mlValidLabels[label]:
		it, err = s.store.LabelMLItem(ctx, q, id, label)
	default:
		s.triggerToast(w, "error", "Ungültiges Label.")
		http.Error(w, "bad label", http.StatusBadRequest)
		return
	}
	if err != nil {
		s.triggerToast(w, "error", "Speichern fehlgeschlagen.")
		s.fail(w, "ml label", err)
		return
	}

	w.Header().Set("HX-Trigger", "mlLabeled")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = mldaten.Row(*it).Render(ctx, w)
}

// handleMLShadowRedirect: /ml-shadow ist im Whatsapp-Tab von /ml-daten
// aufgegangen; alte Lesezeichen landen dort.
func (s *Server) handleMLShadowRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/ml-daten?quelle=whatsapp", http.StatusFound)
}

func (s *Server) handleMLDocs(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, s.meta("ML-Doku", "mldocs"), mldocs.Page())
}

// mlHead zählt alle drei Quellen für die Tabs. Fehlt eine Tabelle (der Bot
// legt ml_messages an), bleibt ihr Tab bei null statt die Seite zu kippen.
func (s *Server) mlHead(r *http.Request, active store.MLQuelle) mldaten.HeadVM {
	head := mldaten.HeadVM{Active: active}
	for _, q := range store.MLQuellen {
		c, err := s.store.MLCounts(r.Context(), q)
		if err != nil {
			log.Printf("ml counts %s: %v", q, err)
		}
		head.Tabs = append(head.Tabs, mldaten.Tab{Quelle: q, Counts: c})
		if q == active {
			head.Counts = c
		}
	}
	return head
}

func mlQuelle(r *http.Request) store.MLQuelle {
	q := store.MLQuelle(r.URL.Query().Get("quelle"))
	if !q.Valid() {
		return store.MLWhatsapp
	}
	return q
}

func mlStatus(r *http.Request, q store.MLQuelle) string {
	switch st := r.URL.Query().Get("status"); st {
	case store.MLGeprueft, store.MLAlle:
		return st
	case store.MLAbweichung:
		if q == store.MLWhatsapp {
			return st
		}
	}
	return store.MLOffen
}

func mlFilter(r *http.Request, q store.MLQuelle) store.MLFilter {
	f := store.MLFilter{
		Status: mlStatus(r, q),
		Query:  strings.TrimSpace(r.URL.Query().Get("q")),
		Limit:  mlListLimit,
	}
	if l := r.URL.Query().Get("label"); mlValidLabels[l] {
		f.Label = l
	}
	return f
}
