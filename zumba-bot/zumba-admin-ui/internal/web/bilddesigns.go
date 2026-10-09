package web

import (
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/michael/zumba-shared/cards"

	"github.com/michael/zumba-admin-ui/internal/store"
	"github.com/michael/zumba-admin-ui/internal/timeutil"
	"github.com/michael/zumba-admin-ui/web/templates/bilddesigns"
)

// bilddesigns.go: Seite /bild-designs – welche Bild-Designs der Bot im Umlauf
// hat und welche Karte der nächste Wochenreport bekommt. Der Bot liest die
// Einstellungen vor jeder Karte, eine Änderung greift ohne Neustart.

// kartenNow ist die Uhr der Seite (Tests stellen sie).
var kartenNow = time.Now

// vorschauWochen: so viele Donnerstage zeigt die Vorschau.
const vorschauWochen = 8

func bildDesignsVM(cs store.CardSettings, now time.Time) bilddesigns.VM {
	queue := cards.Clean(cs.Rotation)
	var next *cards.Next
	if cs.Naechster != nil {
		next = &cards.Next{Tag: cs.Naechster.Tag, Style: cs.Naechster.Style}
	}

	termin := cards.NextReport(now)
	wochen := cards.Upcoming(queue, next, termin, vorschauWochen)
	vm := bilddesigns.VM{
		Termin:       timeutil.FormatDE(termin) + " · 21:00",
		TerminKurz:   termin.Format("02.01."),
		Kommt:        wochen[0].Style,
		KommtLabel:   cards.Label(wochen[0].Style),
		Fest:         wochen[0].Fest,
		RotationLeer: cs.RotationGesetzt && len(queue) == 0,
		OhneDB:       !cs.RotationGesetzt,
	}
	if vm.Fest {
		vm.FestID = vm.Kommt
	}
	vm.RotationKommt = cards.Label(cards.Default)
	if len(queue) > 0 {
		vm.RotationKommt = cards.Label(queue[0])
	}
	if !cs.UpdatedAt.IsZero() {
		vm.Since = cs.UpdatedAt.Local().Format("02.01.2006, 15:04")
	}
	if z := cs.Zuletzt; z != nil {
		vm.Zuletzt = z.Tag.Format("02.01.") + " · " + cards.Label(z.Style)
	}
	for _, id := range cs.Rotation {
		if !cards.Known(id) {
			vm.Unbekannt = append(vm.Unbekannt, id)
		}
	}

	// Erst die Warteschlange in ihrer Reihenfolge – jedes Design mit seinem
	// Donnerstag (eine Einmal-Auswahl schiebt alle um eine Woche) –, dann der
	// Rest im Katalog.
	versatz := 0
	if vm.Fest {
		versatz = 1
	}
	for i, id := range queue {
		vm.Schlange = append(vm.Schlange, bilddesigns.Design{
			ID: id, Label: cards.Label(id), Platz: i + 1,
			Datum: termin.AddDate(0, 0, 7*(i+versatz)).Format("02.01."),
		})
	}
	for _, s := range cards.Catalog() {
		if !slices.Contains(queue, s.ID) {
			vm.Frei = append(vm.Frei, bilddesigns.Design{ID: s.ID, Label: s.Label})
		}
	}
	for _, s := range cards.Catalog() {
		vm.Alle = append(vm.Alle, bilddesigns.Design{ID: s.ID, Label: s.Label})
	}

	for _, w := range wochen {
		vm.Wochen = append(vm.Wochen, bilddesigns.Woche{Datum: w.Tag.Format("02.01."), Label: cards.Label(w.Style), Fest: w.Fest})
	}
	return vm
}

func (s *Server) handleBildDesigns(w http.ResponseWriter, r *http.Request) {
	cs, err := s.store.CardSettings(r.Context())
	if err != nil {
		s.fail(w, "card settings", err)
		return
	}
	s.render(w, r, s.meta("Bild-Designs", "bilddesigns"), bilddesigns.Page(bildDesignsVM(cs, kartenNow())))
}

func (s *Server) handleBildDesignsRotation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	angehakt := r.Form["style"]
	for _, id := range angehakt {
		if !cards.Known(id) {
			s.triggerToast(w, "error", "Unbekanntes Design.")
			http.Error(w, "unbekanntes Design", http.StatusUnprocessableEntity)
			return
		}
	}
	// Reihenfolge = Reihenfolge im Formular (per Drag & Drop sortiert).
	rotation := cards.Clean(angehakt)
	if err := s.store.SetCardRotation(ctx, rotation); err != nil {
		s.fail(w, "set card rotation", err)
		return
	}
	log.Printf("🎨 Bild-Designs in Rotation: %v", rotation)
	msg := "Gespeichert – oben steht das Design für den nächsten Donnerstag."
	if len(rotation) == 0 {
		msg = "Rotation leer – es kommt immer das Live-Design (Wrapped)."
	}
	s.triggerToast(w, "success", msg)
	s.renderBildDesignsRegion(w, r)
}

func (s *Server) handleBildDesignsNaechster(w http.ResponseWriter, r *http.Request) {
	style := r.FormValue("style")
	if style != "" && !cards.Known(style) {
		s.triggerToast(w, "error", "Unbekanntes Design.")
		http.Error(w, "unbekanntes Design", http.StatusUnprocessableEntity)
		return
	}
	// Der Tag kommt nie aus dem Formular: gewählt wird immer für den
	// nächsten Wochenreport ab jetzt.
	termin := cards.NextReport(kartenNow())
	if err := s.store.SetNextCard(r.Context(), termin, style); err != nil {
		s.fail(w, "set next card", err)
		return
	}
	if style == "" {
		log.Printf("🎨 Wochenreport %s: Rotation", termin.Format("02.01."))
		s.triggerToast(w, "success", "Am "+termin.Format("02.01.")+" entscheidet wieder die Rotation.")
	} else {
		log.Printf("🎨 Wochenreport %s: %s (fest gewählt)", termin.Format("02.01."), style)
		s.triggerToast(w, "success", "Am "+termin.Format("02.01.")+" kommt "+cards.Label(style)+".")
	}
	s.renderBildDesignsRegion(w, r)
}

func (s *Server) renderBildDesignsRegion(w http.ResponseWriter, r *http.Request) {
	cs, err := s.store.CardSettings(r.Context())
	if err != nil {
		s.fail(w, "card settings", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := bilddesigns.Region(bildDesignsVM(cs, kartenNow())).Render(r.Context(), w); err != nil {
		log.Printf("render bild-designs region: %v", err)
	}
}
