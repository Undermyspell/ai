package web

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/michael/zumba-shared/penalty"

	"github.com/michael/zumba-admin-ui/internal/store"
	"github.com/michael/zumba-admin-ui/internal/timeutil"
	"github.com/michael/zumba-admin-ui/web/emoji"
	"github.com/michael/zumba-admin-ui/web/templates/strafen"
)

// strafenLage ist die bewertete Strafenlage eines Stammtischjahres zum
// heutigen Tag (Stichtag-Simulation gibt es nur auf der Bot-Test-Seite über den
// Wochenreport-Endpoint). Im Archiv ist der Stichtag das Jahresende, damit ein
// abgeschlossenes Jahr denselben Stand zeigt wie an seinem letzten Tag.
type strafenLage struct {
	stichtag  time.Time
	users     []store.User
	thursdays []time.Time // gültige Donnerstage bis zum Stichtag, neueste zuerst
	entries   []penalty.Entry
}

// strafenLage bewertet die Strafen. Mit persist werden neu erkannte
// Fehltage-Strafen idempotent persistiert (Marker), damit sie sofort
// begleich-/löschbar sind – dieselbe Erkennung läuft auch im Bot beim Report.
// In abgeschlossenen Jahren wird NIE geschrieben: das bloße Öffnen einer
// Archivseite darf keine Zeilen anlegen. Das Dashboard liest nur (persist
// = false) und zählt Kandidaten trotzdem als offen.
func (s *Server) strafenLage(ctx context.Context, season store.Season, persist bool) (strafenLage, error) {
	persist = persist && !archived(season)
	stichtag := season.ClampAsOf(timeutil.StartOfDay(time.Now()))
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		return strafenLage{}, err
	}
	period := timeutil.Period{Start: season.Start, End: stichtag}
	absences, err := s.store.ListAbsences(ctx, period)
	if err != nil {
		return strafenLage{}, err
	}
	excludedDays, err := s.store.ListExcludedDays(ctx, period)
	if err != nil {
		return strafenLage{}, err
	}
	rows, err := s.store.ListSeasonStrafen(ctx, season)
	if err != nil {
		return strafenLage{}, err
	}
	// Für das No-Show-Formular: nur echte Stammtisch-Donnerstage anbieten.
	thursdays, err := s.store.ListThursdays(ctx, period)
	if err != nil {
		return strafenLage{}, err
	}

	input := func(rows []penalty.Row) penalty.Input {
		byUser := make(map[string][]time.Time)
		for _, a := range absences {
			byUser[a.UserID] = append(byUser[a.UserID], a.Date)
		}
		in := penalty.Input{Excluded: excludedDays, Rows: rows}
		for _, u := range users {
			in.Users = append(in.Users, penalty.UserData{
				UserID: u.ID, Name: u.Name,
				EffectiveStart: season.ClampStart(u.StartDate),
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
		if e.ID != 0 || !persist {
			continue
		}
		if err := s.store.InsertAutoStrafe(ctx, e.UserID, e.Datum); err != nil {
			log.Printf("insert auto strafe: %v", err)
			continue
		}
		persisted = true
	}
	if persisted {
		if rows, err = s.store.ListSeasonStrafen(ctx, season); err != nil {
			return strafenLage{}, err
		}
		entries = penalty.Assess(input(rows), stichtag)
	}
	return strafenLage{stichtag: stichtag, users: users, thursdays: thursdays, entries: entries}, nil
}

// strafenVM baut die Strafen-Seite aus der (persistierten) Strafenlage.
func (s *Server) strafenVM(ctx context.Context, season store.Season) (strafen.PageVM, error) {
	lage, err := s.strafenLage(ctx, season, true)
	if err != nil {
		return strafen.PageVM{}, err
	}
	vm := strafen.PageVM{
		Users:         lage.users,
		NoShowDefault: penalty.NoShowDefault,
		ReadOnly:      archived(season),
	}
	for _, t := range lage.thursdays {
		vm.Thursdays = append(vm.Thursdays, strafen.Option{Value: timeutil.FormatISO(t), Label: timeutil.FormatDE(t)})
	}
	for _, e := range lage.entries {
		if e.Status == penalty.StatusGeloescht {
			continue
		}
		row := strafen.Row{
			ID: e.ID, UserName: e.Name, Emoji: emoji.For(e.Name), Art: e.Art, Datum: e.Datum,
			Tage: e.Tage, Betrag: e.Betrag, Status: e.Status,
			BeglichenAm: e.BeglichenAm,
			Sichtbar:    penalty.VisibleAt(e, lage.stichtag),
		}
		if e.Status == penalty.StatusBeglichen && e.BeglichenAm != nil {
			bis := penalty.NextThursday(*e.BeglichenAm)
			row.SichtbarBis = &bis
		}
		switch e.Status {
		case penalty.StatusOffen:
			vm.OffenSum += e.Betrag
			vm.OffenCount++
		case penalty.StatusBeglichen:
			vm.BeglichenSum += e.Betrag
		}
		if row.Sichtbar {
			vm.ReportCount++
		}
		vm.Rows = append(vm.Rows, row)
	}
	return vm, nil
}

func (s *Server) handleStrafen(w http.ResponseWriter, r *http.Request) {
	season, ok := s.pageSeason(w, r)
	if !ok {
		return
	}
	vm, err := s.strafenVM(r.Context(), season)
	if err != nil {
		s.fail(w, "strafen", err)
		return
	}
	meta := s.seasonMeta(r, "Strafen", "strafen", season)
	meta.RefreshURL = r.URL.RequestURI()
	s.render(w, r, meta, strafen.Page(vm))
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
	if !s.requireWritable(w, r, datum) {
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
	if !s.strafeWritable(w, r, id) {
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
	if !s.strafeWritable(w, r, id) {
		return
	}
	if err := s.store.LoescheStrafe(r.Context(), id); err != nil {
		s.fail(w, "loesche strafe", err)
		return
	}
	s.triggerToast(w, "success", "Strafe gelöscht – der Fehltage-Zähler beginnt von vorn.")
	s.renderStrafenRegion(w, r)
}

// strafeWritable erlaubt Begleichen/Löschen nur für Strafen des laufenden
// Jahres. Der Check hängt an der Strafe selbst, nicht am ?jahr= des Requests –
// ein HTMX-Aufruf ohne Jahres-Parameter darf kein abgeschlossenes Jahr ändern.
func (s *Server) strafeWritable(w http.ResponseWriter, r *http.Request, id int64) bool {
	ctx := r.Context()
	season, err := s.store.SeasonAt(ctx, timeutil.StartOfDay(time.Now()))
	if err != nil {
		s.triggerToast(w, "error", "Kein laufendes Stammtischjahr – keine Änderungen möglich.")
		http.Error(w, "kein laufendes Stammtischjahr", http.StatusConflict)
		return false
	}
	rows, err := s.store.ListSeasonStrafen(ctx, season)
	if err != nil {
		s.fail(w, "strafen", err)
		return false
	}
	for _, row := range rows {
		if row.ID == id {
			return true
		}
	}
	s.triggerToast(w, "error", "Strafe gehört nicht zum laufenden Stammtischjahr.")
	http.Error(w, "Jahr abgeschlossen", http.StatusConflict)
	return false
}

// renderStrafenRegion rendert nur die Liste (HTMX-Swap-Ziel).
func (s *Server) renderStrafenRegion(w http.ResponseWriter, r *http.Request) {
	season, ok := s.pageSeason(w, r)
	if !ok {
		return
	}
	vm, err := s.strafenVM(r.Context(), season)
	if err != nil {
		s.fail(w, "strafen", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := strafen.ListRegion(vm).Render(r.Context(), w); err != nil {
		log.Printf("render strafen region: %v", err)
	}
}
