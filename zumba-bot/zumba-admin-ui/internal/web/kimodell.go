package web

import (
	"context"
	"fmt"
	"log"
	"net/http"

	sharedstore "github.com/michael/zumba-shared/store"

	"github.com/michael/zumba-admin-ui/internal/store"
	"github.com/michael/zumba-admin-ui/web/templates/kimodell"
)

type modelOption struct {
	ID    string
	Label string
	// PerMin/PerDay: kostenloses Kontingent von AI Studio (Stand 10/2026). Sie
	// entscheiden, wie viele Nachrichten ein Donnerstag verträgt.
	PerMin int
	PerDay int
}

// callsPerThursday: so viele Klassifizierungen braucht ein Donnerstag etwa,
// fehlgeschlagene Aufrufe zählen mit.
const callsPerThursday = 10

// modelOptions sind die Modelle, die die Seite anbietet – und die einzigen,
// die sie speichert.
var modelOptions = []modelOption{
	{ID: "gemma-4-31b-it", Label: "Gemma 4 31B", PerMin: 30, PerDay: 14400},
	{ID: "gemini-3.8-flash", Label: "Gemini 3.8 Flash", PerMin: 5, PerDay: 20},
	{ID: "gemini-3.5-flash-lite", Label: "Gemini 3.5 Flash Lite", PerMin: 15, PerDay: 500},
}

// limits: "30/min · 14.400/Tag".
func (o modelOption) limits() string {
	return fmt.Sprintf("%d/min · %s/Tag", o.PerMin, thousands(o.PerDay))
}

// thousands setzt deutsche Tausenderpunkte: 14400 → "14.400".
func thousands(n int) string {
	s := fmt.Sprintf("%d", n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	return s
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

// quotaClass: wie knapp ein Donnerstag das Tageslimit macht.
func quotaClass(perDay int) string {
	switch {
	case perDay < 50:
		return "is-red"
	case perDay < 1000:
		return "is-amber"
	default:
		return "is-green"
	}
}

func kiModellVM(m store.ClassifierModels, shadow store.MLShadowStats) kimodell.VM {
	vm := kimodell.VM{}
	ref := func(rolle string, own sharedstore.ClassifierModel) kimodell.Ref {
		if own.Model == "" {
			vm.Unknown = append(vm.Unknown, roleTitles[rolle]+": noch nicht gesetzt – der Bot nimmt den Wert aus dem Deployment.")
			return kimodell.Ref{}
		}
		o, ok := findModel(own.Model)
		if !ok {
			vm.Unknown = append(vm.Unknown, roleTitles[rolle]+": aktiv ist "+own.Model+" (nicht in der Auswahl).")
			return kimodell.Ref{Label: own.Model}
		}
		return kimodell.Ref{Label: o.Label, Limits: o.limits()}
	}
	vm.Primary = ref(sharedstore.ClassifierPrimary, m.Primary)
	vm.Fallback = ref(sharedstore.ClassifierFallback, m.Fallback)

	since := m.Primary.UpdatedAt
	if m.Fallback.UpdatedAt.After(since) {
		since = m.Fallback.UpdatedAt
	}
	if !since.IsZero() {
		vm.Since = since.Local().Format("02.01.2006, 15:04")
	}

	for _, o := range modelOptions {
		card := kimodell.Model{
			ID: o.ID, Label: o.Label,
			PerMin:     fmt.Sprintf("%d", o.PerMin),
			PerDay:     thousands(o.PerDay),
			Meter:      max(3, min(100, callsPerThursday*100/o.PerDay)),
			MeterClass: quotaClass(o.PerDay),
			IsPrimary:  o.ID == m.Primary.Model,
			IsFallback: o.ID == m.Fallback.Model,
		}
		thursdays := o.PerDay / callsPerThursday
		switch {
		case o.PerDay < 50:
			card.Note = fmt.Sprintf("⚠ höchstens %d Donnerstage pro Tag – als Hauptmodell knapp", thursdays)
			card.NoteRed = true
		case o.PerDay < 1000:
			card.Note = fmt.Sprintf("~%d Donnerstage pro Tag", thursdays)
		default:
			card.Note = "praktisch unbegrenzt für einen Donnerstag"
		}
		vm.Models = append(vm.Models, card)
	}

	for _, r := range []struct {
		role string
		own  sharedstore.ClassifierModel
	}{{"Haupt", m.Primary}, {"Fallback", m.Fallback}} {
		o, ok := findModel(r.own.Model)
		if !ok {
			continue
		}
		p := float64(callsPerThursday) / float64(o.PerDay) * 100
		q := kimodell.Quota{
			Role: r.role, Label: o.Label,
			Text: fmt.Sprintf("%d von %s Aufrufen am Donnerstag", callsPerThursday, thousands(o.PerDay)),
			Pct:  fmt.Sprintf("%.0f %%", p),
		}
		if p < 1 {
			q.Pct = "<1 %"
		}
		switch {
		case p > 40:
			q.Class = "is-red"
		case p > 5:
			q.Class = "is-amber"
		default:
			q.Class = "is-green"
		}
		vm.Quota = append(vm.Quota, q)
	}

	vm.Shadow = kimodell.Shadow{AgreePct: "—", Verified: shadow.Verified, Total: shadow.Total}
	if shadow.WithModel > 0 {
		vm.Shadow.AgreePct = fmt.Sprintf("%.0f %%", float64(shadow.Agree)/float64(shadow.WithModel)*100)
	}
	return vm
}

func otherRole(rolle string) string {
	if rolle == sharedstore.ClassifierPrimary {
		return sharedstore.ClassifierFallback
	}
	return sharedstore.ClassifierPrimary
}

// shadowStats für die Seitenspalte – fehlt die Tabelle, bleibt die Karte leer.
func (s *Server) shadowStats(ctx context.Context) store.MLShadowStats {
	st, err := s.store.MLShadowStats(ctx)
	if err != nil {
		log.Printf("ml stats: %v", err)
	}
	return st
}

func (s *Server) handleKIModell(w http.ResponseWriter, r *http.Request) {
	m, err := s.store.ClassifierModels(r.Context())
	if err != nil {
		s.fail(w, "classifier models", err)
		return
	}
	s.render(w, r, s.meta("KI-Modell", "kimodell"), kimodell.Page(kiModellVM(m, s.shadowStats(r.Context()))))
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
	if err := kimodell.Region(kiModellVM(m, s.shadowStats(r.Context()))).Render(r.Context(), w); err != nil {
		log.Printf("render ki-modell region: %v", err)
	}
}
