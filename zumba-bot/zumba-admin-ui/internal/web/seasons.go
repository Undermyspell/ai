package web

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/michael/zumba-shared/domain"

	"github.com/michael/zumba-admin-ui/internal/store"
	"github.com/michael/zumba-admin-ui/internal/timeutil"
	"github.com/michael/zumba-admin-ui/web/templates/jahre"
)

// Stammtischjahre pflegen: den Beginn eines Jahres verschieben, solange er
// noch vor uns liegt, und das nächste Jahr anlegen. Vergangene Grenzen sind
// fest – sie würden abgeschlossene Auswertungen und Strafen-Serien
// nachträglich umschreiben.

// dateNum: "01.12.2026" – kompakt für Zeiträume und Meldungen.
func dateNum(t time.Time) string { return t.Format("02.01.2006") }

// seasonToday ist der heutige Tag auf derselben Basis wie die Jahresgrenzen
// aus der DB (Mitternacht UTC des lokalen Datums).
func seasonToday() time.Time { return domain.DateOnly(time.Now()) }

// checkSeasonStart prüft, ob der Beginn von label auf start wandern darf, und
// liefert sonst die Meldung für den Toast. seasons ist neuestes zuerst.
func checkSeasonStart(seasons []store.Season, label string, start, today time.Time) string {
	i := seasonIndex(seasons, label)
	if i < 0 {
		return "Stammtischjahr " + label + " gibt es nicht."
	}
	cur := seasons[i]
	if !cur.Start.After(today) {
		return "Stammtischjahr " + label + " hat schon begonnen – sein Beginn steht fest."
	}
	if !start.After(today) {
		return "Der neue Beginn muss in der Zukunft liegen."
	}
	if !start.Before(cur.End) {
		return "Der Beginn muss vor dem Jahresende (" + dateNum(cur.End) + ") liegen."
	}
	if i+1 < len(seasons) {
		prev := seasons[i+1]
		if !start.AddDate(0, 0, -1).After(prev.Start) {
			return "Der Beginn muss nach dem Beginn von " + prev.Label + " (" + dateNum(prev.Start) + ") liegen."
		}
	}
	return ""
}

func seasonIndex(seasons []store.Season, label string) int {
	for i, s := range seasons {
		if s.Label == label {
			return i
		}
	}
	return -1
}

// nextSeason schlägt das Jahr nach dem letzten gepflegten vor: es schließt
// lückenlos an und dauert ein Jahr. Angelegt werden darf es erst, wenn das
// letzte Jahr läuft – so steht immer höchstens ein Jahr im Voraus fest, und
// ein versehentlicher Klick legt nicht 2029 und 2030 an. ok=false nennt in
// why den Grund.
func nextSeason(seasons []store.Season, today time.Time) (next store.Season, why string, ok bool) {
	if len(seasons) == 0 {
		return store.Season{}, "Noch kein Stammtischjahr gepflegt.", false
	}
	last := seasons[0]
	n, err := strconv.Atoi(last.Label)
	if err != nil {
		return store.Season{}, "Das letzte Jahr heißt nicht wie eine Jahreszahl (" + last.Label + ") – das nächste bitte per SQL anlegen.", false
	}
	next = store.Season{Label: strconv.Itoa(n + 1)}
	next.Start = last.End.AddDate(0, 0, 1)
	next.End = next.Start.AddDate(1, 0, -1)
	if last.Start.After(today) {
		return next, last.Label + " ist schon angelegt. " + next.Label + " kannst du ab dem " + dateNum(last.Start) + " anlegen.", false
	}
	return next, "", true
}

func seasonsVM(seasons []store.Season, today time.Time) jahre.VM {
	vm := jahre.VM{}
	for i, s := range seasons {
		y := jahre.Year{
			Label: s.Label,
			Range: dateNum(s.Start) + " – " + dateNum(s.End),
		}
		ths := thursdaysIn(s.Start, s.End)
		y.Thursdays = len(ths)
		if len(ths) > 0 {
			y.First = timeutil.FormatDE(ths[0])
		}
		days := int(s.End.Sub(s.Start).Hours()/24) + 1
		switch {
		case s.End.Before(today):
			y.Status, y.Class = "abgeschlossen", ""
		case s.Start.After(today):
			y.Status, y.Class = "kommt", "is-amber"
			y.Note = fmt.Sprintf("beginnt in %d Tagen", int(s.Start.Sub(today).Hours()/24))
			y.Editable = true
			y.StartISO = timeutil.FormatISO(s.Start)
			y.MaxISO = timeutil.FormatISO(s.End.AddDate(0, 0, -1))
			minStart := today.AddDate(0, 0, 1)
			if i+1 < len(seasons) {
				prev := seasons[i+1]
				y.Prev = prev.Label
				if m := prev.Start.AddDate(0, 0, 2); m.After(minStart) {
					minStart = m
				}
			}
			y.MinISO = timeutil.FormatISO(minStart)
		default:
			y.Status, y.Class = "läuft", "is-blue"
			done := int(today.Sub(s.Start).Hours()/24) + 1
			y.Progress = done * 100 / days
			y.Note = fmt.Sprintf("noch %d Tage", days-done)
		}
		vm.Years = append(vm.Years, y)
	}

	next, why, ok := nextSeason(seasons, today)
	if ok {
		vm.Next = &jahre.Next{
			Label: next.Label,
			Range: dateNum(next.Start) + " – " + dateNum(next.End),
		}
		// Läuft das letzte gepflegte Jahr, endet danach jede Auswertung mit
		// einem Fehler – das soll auffallen, bevor es so weit ist.
		vm.Gap = "Nach dem " + dateNum(seasons[0].End) + " ist kein Stammtischjahr gepflegt – Bot und Admin-UI melden dann Fehler."
	} else {
		vm.NextBlocked = why
	}
	return vm
}

func (s *Server) handleSeasons(w http.ResponseWriter, r *http.Request) {
	seasons, err := s.store.ListSeasons(r.Context())
	if err != nil {
		s.fail(w, "list seasons", err)
		return
	}
	s.render(w, r, s.meta("Stammtischjahre", "jahre"), jahre.Page(seasonsVM(seasons, seasonToday())))
}

func (s *Server) handleSeasonStart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	label := r.PathValue("label")
	start, err := timeutil.ParseISO(r.FormValue("start"))
	if err != nil {
		s.triggerToast(w, "error", "Ungültiges Datum.")
		http.Error(w, "ungültiges Datum", http.StatusUnprocessableEntity)
		return
	}
	seasons, err := s.store.ListSeasons(ctx)
	if err != nil {
		s.fail(w, "list seasons", err)
		return
	}
	today := seasonToday()
	if i := seasonIndex(seasons, label); i >= 0 && seasons[i].Start.Equal(start) {
		s.triggerToast(w, "success", "Unverändert – "+label+" beginnt schon am "+dateNum(start)+".")
		s.renderSeasonsRegion(w, r, seasons)
		return
	}
	if msg := checkSeasonStart(seasons, label, start, today); msg != "" {
		// Kein 422: die Region kommt neu, damit das Feld wieder den
		// gespeicherten Beginn zeigt statt der abgelehnten Eingabe.
		s.triggerToast(w, "error", msg)
		s.renderSeasonsRegion(w, r, seasons)
		return
	}
	if err := s.store.MoveSeasonStart(ctx, label, start); err != nil {
		s.fail(w, "move season start", err)
		return
	}
	log.Printf("📅 Stammtischjahr %s beginnt jetzt am %s", label, timeutil.FormatISO(start))
	msg := label + " beginnt am " + dateNum(start) + "."
	if i := seasonIndex(seasons, label); i+1 < len(seasons) {
		msg += " " + seasons[i+1].Label + " endet am " + dateNum(start.AddDate(0, 0, -1)) + ". Der Bot übernimmt das in spätestens 10 Minuten."
	}
	s.triggerToast(w, "success", msg)
	s.reloadSeasonsRegion(w, r)
}

func (s *Server) handleSeasonAdd(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	seasons, err := s.store.ListSeasons(ctx)
	if err != nil {
		s.fail(w, "list seasons", err)
		return
	}
	next, why, ok := nextSeason(seasons, seasonToday())
	if !ok {
		s.triggerToast(w, "error", why)
		s.renderSeasonsRegion(w, r, seasons)
		return
	}
	if err := s.store.AddSeason(ctx, next.Label, next.Start, next.End); err != nil {
		s.fail(w, "add season", err)
		return
	}
	log.Printf("📅 Stammtischjahr %s angelegt: %s – %s", next.Label, timeutil.FormatISO(next.Start), timeutil.FormatISO(next.End))
	s.triggerToast(w, "success", "Stammtischjahr "+next.Label+" angelegt.")
	s.reloadSeasonsRegion(w, r)
}

// reloadSeasonsRegion liest die Jahre nach einer Änderung neu ein.
func (s *Server) reloadSeasonsRegion(w http.ResponseWriter, r *http.Request) {
	seasons, err := s.store.ListSeasons(r.Context())
	if err != nil {
		s.fail(w, "list seasons", err)
		return
	}
	s.renderSeasonsRegion(w, r, seasons)
}

func (s *Server) renderSeasonsRegion(w http.ResponseWriter, r *http.Request, seasons []store.Season) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := jahre.Region(seasonsVM(seasons, seasonToday())).Render(r.Context(), w); err != nil {
		log.Printf("render jahre region: %v", err)
	}
}
