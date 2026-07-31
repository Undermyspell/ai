package web

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/michael/zumba-admin-ui/internal/penalty"
	"github.com/michael/zumba-admin-ui/internal/timeutil"
	"github.com/michael/zumba-admin-ui/web/templates/strafen"
)

// stichtagFrom liest das simulierte Abfragedatum (?stichtag=YYYY-MM-DD),
// Default heute. Damit lässt sich prüfen, welche Strafen ein Report zu einem
// beliebigen Zeitpunkt ausweisen würde (z. B. Beglichen-Fenster bis zum
// Folgedonnerstag).
func stichtagFrom(r *http.Request) time.Time {
	if s := r.URL.Query().Get("stichtag"); s != "" {
		if d, err := timeutil.ParseISO(s); err == nil {
			return d
		}
	}
	return timeutil.StartOfDay(time.Now())
}

// strafenVM bewertet alle Strafen zum Stichtag. Neu erkannte Fehltage-Strafen
// werden dabei idempotent persistiert (Marker), damit sie sofort begleich-/
// löschbar sind – dieselbe Erkennung läuft auch im Bot beim echten Report.
func (s *Server) strafenVM(ctx context.Context, stichtag time.Time) (strafen.PageVM, error) {
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		return strafen.PageVM{}, err
	}
	period := timeutil.Period{Start: s.cfg.EvalPeriodStart, End: stichtag}
	absences, err := s.store.ListAbsences(ctx, period)
	if err != nil {
		return strafen.PageVM{}, err
	}
	excluded, err := s.store.ListExcludedDays(ctx, period)
	if err != nil {
		return strafen.PageVM{}, err
	}
	rows, err := s.store.ListStrafen(ctx)
	if err != nil {
		return strafen.PageVM{}, err
	}
	// Für das No-Show-Formular: nur echte Stammtisch-Donnerstage anbieten.
	thursdays, err := s.store.ListThursdays(ctx, period)
	if err != nil {
		return strafen.PageVM{}, err
	}

	input := func(rows []penalty.Row) penalty.Input {
		byUser := make(map[string][]time.Time)
		for _, a := range absences {
			byUser[a.UserID] = append(byUser[a.UserID], a.Date)
		}
		in := penalty.Input{Excluded: excluded, Rows: rows}
		for _, u := range users {
			in.Users = append(in.Users, penalty.UserData{
				UserID: u.ID, Name: u.Name,
				EffectiveStart: penalty.ClampStart(u.StartDate),
				Absences:       byUser[u.ID],
			})
		}
		return in
	}

	entries := penalty.Assess(input(rows), stichtag)

	// Kandidaten (ID == 0) persistieren und einmal neu bewerten, damit die
	// Aktions-Buttons echte IDs haben.
	persisted := false
	for _, e := range entries {
		if e.ID != 0 {
			continue
		}
		if err := s.store.InsertAutoStrafe(ctx, e.UserID, e.Datum); err != nil {
			log.Printf("insert auto strafe: %v", err)
			continue
		}
		persisted = true
	}
	if persisted {
		if rows, err = s.store.ListStrafen(ctx); err != nil {
			return strafen.PageVM{}, err
		}
		entries = penalty.Assess(input(rows), stichtag)
	}

	vm := strafen.PageVM{
		Stichtag:      timeutil.FormatISO(stichtag),
		StichtagHeute: timeutil.FormatISO(stichtag) == timeutil.FormatISO(time.Now()),
		Users:         users,
		Thursdays:     thursdays,
		NoShowDefault: penalty.NoShowDefault,
	}
	for _, e := range entries {
		if e.Status == penalty.StatusGeloescht {
			continue
		}
		row := strafen.Row{
			ID: e.ID, UserName: e.Name, Art: e.Art, Datum: e.Datum,
			Tage: e.Tage, Betrag: e.Betrag, Status: e.Status,
			BeglichenAm: e.BeglichenAm,
			Sichtbar:    penalty.VisibleAt(e, stichtag),
		}
		if e.Status == penalty.StatusBeglichen && e.BeglichenAm != nil {
			bis := penalty.NextThursday(*e.BeglichenAm)
			row.SichtbarBis = &bis
		}
		vm.Rows = append(vm.Rows, row)
	}
	return vm, nil
}

func (s *Server) handleStrafen(w http.ResponseWriter, r *http.Request) {
	vm, err := s.strafenVM(r.Context(), stichtagFrom(r))
	if err != nil {
		s.fail(w, "strafen", err)
		return
	}
	s.render(w, r, s.meta("Strafen", "strafen"), strafen.Page(vm))
}

func (s *Server) handleAddStrafe(w http.ResponseWriter, r *http.Request) {
	userID := r.FormValue("userId")
	if userID == "" {
		s.triggerToast(w, "error", "Mitglied fehlt.")
		http.Error(w, "userId fehlt", http.StatusUnprocessableEntity)
		return
	}
	datum, err := timeutil.ParseISO(r.FormValue("datum"))
	if err != nil {
		s.triggerToast(w, "error", "Ungültiges Datum.")
		http.Error(w, "ungültiges Datum", http.StatusUnprocessableEntity)
		return
	}
	if !timeutil.IsThursday(datum) {
		s.triggerToast(w, "error", "No-Shows gibt es nur an Donnerstagen.")
		http.Error(w, "kein Donnerstag", http.StatusUnprocessableEntity)
		return
	}
	betrag := penalty.NoShowDefault
	if b := r.FormValue("betrag"); b != "" {
		v, err := strconv.Atoi(b)
		if err != nil || v <= 0 {
			s.triggerToast(w, "error", "Ungültiger Betrag.")
			http.Error(w, "ungültiger Betrag", http.StatusUnprocessableEntity)
			return
		}
		betrag = v
	}
	if err := s.store.InsertNoShowStrafe(r.Context(), userID, datum, betrag); err != nil {
		s.fail(w, "insert strafe", err)
		return
	}
	s.triggerToast(w, "success", "Strafe angelegt.")
	s.renderStrafenRegion(w, r)
}

func (s *Server) handleBegleicheStrafe(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "ungültige ID", http.StatusUnprocessableEntity)
		return
	}
	if err := s.store.BegleicheStrafe(r.Context(), id); err != nil {
		s.fail(w, "begleiche strafe", err)
		return
	}
	s.triggerToast(w, "success", "Strafe beglichen – erscheint noch bis zum Folgedonnerstag im Report.")
	s.renderStrafenRegion(w, r)
}

func (s *Server) handleDeleteStrafe(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "ungültige ID", http.StatusUnprocessableEntity)
		return
	}
	if err := s.store.LoescheStrafe(r.Context(), id); err != nil {
		s.fail(w, "loesche strafe", err)
		return
	}
	s.triggerToast(w, "success", "Strafe gelöscht – der Fehltage-Zähler beginnt von vorn.")
	s.renderStrafenRegion(w, r)
}

// renderStrafenRegion rendert nur die Liste (HTMX-Swap-Ziel).
func (s *Server) renderStrafenRegion(w http.ResponseWriter, r *http.Request) {
	vm, err := s.strafenVM(r.Context(), stichtagFrom(r))
	if err != nil {
		s.fail(w, "strafen", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := strafen.ListRegion(vm).Render(r.Context(), w); err != nil {
		log.Printf("render strafen region: %v", err)
	}
}
