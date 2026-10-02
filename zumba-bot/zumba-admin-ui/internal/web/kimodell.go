package web

import (
	"log"
	"net/http"

	sharedstore "github.com/michael/zumba-shared/store"

	"github.com/michael/zumba-admin-ui/internal/store"
	"github.com/michael/zumba-admin-ui/internal/timeutil"
	"github.com/michael/zumba-admin-ui/web/templates/kimodell"
)

type modelOption struct {
	ID    string
	Label string
	// Limits: kostenloses Kontingent von AI Studio (Stand 10/2026). Sie
	// entscheiden, wie viele Nachrichten ein Donnerstag verträgt.
	Limits string
}

// modelOptions sind die Modelle, die die Seite anbietet – und die einzigen,
// die sie speichert.
var modelOptions = []modelOption{
	{ID: "gemma-4-31b-it", Label: "Gemma 4 31B", Limits: "30/min · 14.400/Tag"},
	{ID: "gemini-3.8-flash", Label: "Gemini 3.8 Flash", Limits: "5/min · 20/Tag"},
	{ID: "gemini-3.5-flash-lite", Label: "Gemini 3.5 Flash Lite", Limits: "15/min · 500/Tag"},
}

func findModel(id string) (modelOption, bool) {
	for _, o := range modelOptions {
		if o.ID == id {
			return o, true
		}
	}
	return modelOption{}, false
}

var roleTitles = map[string]string{
	sharedstore.ClassifierPrimary:  "Hauptmodell",
	sharedstore.ClassifierFallback: "Fallback",
}

func kiModellVM(m store.ClassifierModels) kimodell.VM {
	role := func(rolle string, own, other sharedstore.ClassifierModel, hint string) kimodell.RoleVM {
		r := kimodell.RoleVM{
			Rolle:   rolle,
			Title:   roleTitles[rolle],
			Hint:    hint,
			Current: own.Model,
		}
		_, r.CurrentKnown = findModel(own.Model)
		if !own.UpdatedAt.IsZero() {
			t := own.UpdatedAt.Local()
			r.SinceText = "seit " + timeutil.FormatDEShort(t) + ", " + t.Format("15:04")
		}
		for _, o := range modelOptions {
			opt := kimodell.OptionVM{ID: o.ID, Label: o.Label, Hint: o.Limits, Selected: o.ID == own.Model}
			if o.ID == other.Model {
				opt.Disabled = true
				opt.Hint = "ist " + roleTitles[otherRole(rolle)]
			}
			r.Options = append(r.Options, opt)
		}
		return r
	}
	return kimodell.VM{Roles: []kimodell.RoleVM{
		role(sharedstore.ClassifierPrimary, m.Primary, m.Fallback, "Klassifiziert jede Nachricht."),
		role(sharedstore.ClassifierFallback, m.Fallback, m.Primary, "Springt ein, wenn das Hauptmodell einen Fehler liefert oder nicht antwortet."),
	}}
}

func otherRole(rolle string) string {
	if rolle == sharedstore.ClassifierPrimary {
		return sharedstore.ClassifierFallback
	}
	return sharedstore.ClassifierPrimary
}

func (s *Server) handleKIModell(w http.ResponseWriter, r *http.Request) {
	m, err := s.store.ClassifierModels(r.Context())
	if err != nil {
		s.fail(w, "classifier models", err)
		return
	}
	s.render(w, r, s.meta("KI-Modell", "kimodell"), kimodell.Page(kiModellVM(m)))
}

func (s *Server) handleKIModellSet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rolle, model := r.FormValue("rolle"), r.FormValue("model")
	if _, ok := roleTitles[rolle]; !ok {
		s.triggerToast(w, "error", "Unbekannte Rolle.")
		http.Error(w, "unbekannte Rolle", http.StatusUnprocessableEntity)
		return
	}
	opt, ok := findModel(model)
	if !ok {
		s.triggerToast(w, "error", "Unbekanntes Modell.")
		http.Error(w, "unbekanntes Modell", http.StatusUnprocessableEntity)
		return
	}
	m, err := s.store.ClassifierModels(ctx)
	if err != nil {
		s.fail(w, "classifier models", err)
		return
	}
	other := m.Primary.Model
	if rolle == sharedstore.ClassifierPrimary {
		other = m.Fallback.Model
	}
	if model == other {
		// Kein 422: die Region kommt neu, damit die Auswahl wieder den
		// gespeicherten Stand zeigt statt des abgelehnten Klicks.
		s.triggerToast(w, "error", opt.Label+" ist schon "+roleTitles[otherRole(rolle)]+" – Haupt- und Fallback-Modell müssen sich unterscheiden.")
		s.renderKIModellRegion(w, r, m)
		return
	}
	if err := s.store.SetClassifierModel(ctx, rolle, model); err != nil {
		s.fail(w, "set classifier model", err)
		return
	}
	log.Printf("🧠 KI-Modell: %s → %s", rolle, model)
	if m, err = s.store.ClassifierModels(ctx); err != nil {
		s.fail(w, "classifier models", err)
		return
	}
	s.triggerToast(w, "success", roleTitles[rolle]+": "+opt.Label+" – gilt ab der nächsten Nachricht.")
	s.renderKIModellRegion(w, r, m)
}

func (s *Server) renderKIModellRegion(w http.ResponseWriter, r *http.Request, m store.ClassifierModels) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := kimodell.Region(kiModellVM(m)).Render(r.Context(), w); err != nil {
		log.Printf("render ki-modell region: %v", err)
	}
}
