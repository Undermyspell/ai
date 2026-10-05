package web

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/michael/zumba-admin-ui/internal/store"
	"github.com/michael/zumba-admin-ui/internal/timeutil"
	"github.com/michael/zumba-admin-ui/web/templates/excluded"
	"github.com/michael/zumba-admin-ui/web/templates/partials"
)

func (s *Server) handleExcluded(w http.ResponseWriter, r *http.Request) {
	season, ok := s.pageSeason(w, r)
	if !ok {
		return
	}
	vm, err := s.excludedVM(r.Context(), season)
	if err != nil {
		s.fail(w, "excluded", err)
		return
	}
	meta := s.seasonMeta(r, "Sperrtage", "excluded", season)
	meta.RefreshURL = r.URL.RequestURI()
	s.render(w, r, meta, excluded.List(vm))
}

func (s *Server) handleAddExcluded(w http.ResponseWriter, r *http.Request) {
	date, err := timeutil.ParseISO(r.FormValue("date"))
	if err != nil {
		s.triggerToast(w, "error", "Ungültiges Datum.")
		http.Error(w, "ungültiges Datum", http.StatusUnprocessableEntity)
		return
	}
	if !timeutil.IsThursday(date) {
		s.triggerToast(w, "error", "Nur Donnerstage können gesperrt werden.")
		http.Error(w, "kein Donnerstag", http.StatusUnprocessableEntity)
		return
	}
	if !s.requireWritable(w, r, date) {
		return
	}
	if err := s.store.InsertExcludedDay(r.Context(), date); err != nil {
		s.fail(w, "insert excluded", err)
		return
	}
	s.triggerToast(w, "success", "Sperrtag angelegt: "+timeutil.FormatDE(date)+".")
	s.renderExcludedRegion(w, r)
}

func (s *Server) handleDeleteExcluded(w http.ResponseWriter, r *http.Request) {
	date, err := timeutil.ParseISO(r.PathValue("date"))
	if err != nil {
		http.Error(w, "ungültiges Datum", http.StatusUnprocessableEntity)
		return
	}
	if !s.requireWritable(w, r, date) {
		return
	}
	if err := s.store.DeleteExcludedDay(r.Context(), date); err != nil {
		s.fail(w, "delete excluded", err)
		return
	}
	s.triggerToast(w, "success", "Sperrtag freigegeben: "+timeutil.FormatDE(date)+".")
	s.renderExcludedRegion(w, r)
}

// renderExcludedRegion rendert nur die Region (HTMX-Swap-Ziel).
func (s *Server) renderExcludedRegion(w http.ResponseWriter, r *http.Request) {
	season, ok := s.pageSeason(w, r)
	if !ok {
		return
	}
	vm, err := s.excludedVM(r.Context(), season)
	if err != nil {
		s.fail(w, "excluded", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := excluded.ListRegion(vm).Render(r.Context(), w); err != nil {
		log.Printf("render excluded region: %v", err)
	}
}

// excludedVM baut den Jahreskalender: alle Donnerstage des Stammtischjahres,
// vergangene mit ihrer Quote, kommende mit Kalenderwoche, gesperrte rot.
func (s *Server) excludedVM(ctx context.Context, season store.Season) (excluded.ListVM, error) {
	locked, err := s.store.ListExcludedDays(ctx, season.Period) // neueste zuerst
	if err != nil {
		return excluded.ListVM{}, err
	}
	data, err := s.loadSeasonData(ctx, season)
	if err != nil {
		return excluded.ListVM{}, err
	}
	isLocked := make(map[string]bool, len(locked))
	for _, d := range locked {
		isLocked[timeutil.FormatISO(d)] = true
	}
	past := make(map[string]store.StripDay, len(data.Days))
	for _, sd := range data.Days {
		past[timeutil.FormatISO(sd.Date)] = sd
	}
	current := ""
	if valid := data.valid(); len(valid) > 0 {
		current = timeutil.FormatISO(valid[len(valid)-1].Date)
	}

	vm := excluded.ListVM{Label: season.Label, ReadOnly: archived(season)}
	for _, t := range thursdaysIn(season.Start, season.End) {
		iso := timeutil.FormatISO(t)
		if len(vm.Months) == 0 || vm.Months[len(vm.Months)-1].Label != partials.MonthsLong[t.Month()-1] {
			vm.Months = append(vm.Months, excluded.Month{Label: partials.MonthsLong[t.Month()-1], Year: t.Year()})
		}
		d := excluded.Day{ISO: iso, Day: t.Day(), Locked: isLocked[iso], Current: iso == current}
		sd, wasPast := past[iso]
		switch {
		case d.Locked:
			d.Class, d.Sub = "is-locked", "✕"
			d.Title = timeutil.FormatDE(t) + " – gesperrt (klicken = freigeben)"
		case wasPast:
			pct := data.pct(sd)
			d.Class, d.Sub = partials.Tier(pct), fmt.Sprintf("%d%%", pct)
			d.Title = fmt.Sprintf("%s – %d/%d da (klicken = sperren)", timeutil.FormatDE(t), data.present(sd), data.members(sd.Date))
		default:
			d.Class, d.Sub = "is-upcoming", fmt.Sprintf("KW %d", partials.KW(t))
			d.Title = timeutil.FormatDE(t) + " – kommend (klicken = sperren)"
		}
		m := &vm.Months[len(vm.Months)-1]
		m.Days = append(m.Days, d)
	}
	for _, d := range locked {
		vm.Locked = append(vm.Locked, excluded.Locked{
			ISO:   timeutil.FormatISO(d),
			Label: timeutil.FormatDE(d),
			Sub:   fmt.Sprintf("KW %d · %s", partials.KW(d), timeutil.FormatISO(d)),
		})
	}
	for _, h := range thursdayHolidays(season.Start, season.End) {
		iso := timeutil.FormatISO(h.Date)
		vm.Suggestions = append(vm.Suggestions, excluded.Suggestion{
			Label: h.Name, ISO: iso, Date: timeutil.FormatDE(h.Date), Done: isLocked[iso],
		})
	}
	return vm, nil
}

// thursdaysIn liefert alle Donnerstage in [from, to], aufsteigend.
func thursdaysIn(from, to time.Time) []time.Time {
	d := timeutil.StartOfDay(from)
	for d.Weekday() != time.Thursday {
		d = d.AddDate(0, 0, 1)
	}
	var out []time.Time
	for ; !d.After(to); d = d.AddDate(0, 0, 7) {
		out = append(out, d)
	}
	return out
}

type holiday struct {
	Name string
	Date time.Time
}

// thursdayHolidays sind die (bayerischen) Feiertage im Zeitraum, die auf
// einen Donnerstag fallen – dazu Heiligabend und Silvester, an denen auch
// keiner kommt. Christi Himmelfahrt und Fronleichnam sind immer Donnerstage.
func thursdayHolidays(from, to time.Time) []holiday {
	var out []holiday
	loc := from.Location()
	for y := from.Year(); y <= to.Year(); y++ {
		day := func(m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, loc) }
		easter := easterSunday(y, loc)
		for _, h := range []holiday{
			{"Neujahr", day(time.January, 1)},
			{"Heilige Drei Könige", day(time.January, 6)},
			{"Tag der Arbeit", day(time.May, 1)},
			{"Christi Himmelfahrt", easter.AddDate(0, 0, 39)},
			{"Fronleichnam", easter.AddDate(0, 0, 60)},
			{"Mariä Himmelfahrt", day(time.August, 15)},
			{"Tag der Deutschen Einheit", day(time.October, 3)},
			{"Allerheiligen", day(time.November, 1)},
			{"Heiligabend", day(time.December, 24)},
			{"1. Weihnachtstag", day(time.December, 25)},
			{"2. Weihnachtstag", day(time.December, 26)},
			{"Silvester", day(time.December, 31)},
		} {
			if h.Date.Weekday() == time.Thursday && !h.Date.Before(timeutil.StartOfDay(from)) && !h.Date.After(to) {
				out = append(out, h)
			}
		}
	}
	return out
}

// easterSunday berechnet Ostersonntag (gregorianisch, Meeus/Jones/Butcher).
func easterSunday(y int, loc *time.Location) time.Time {
	a := y % 19
	b, c := y/100, y%100
	d, e := b/4, b%4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i, k := c/4, c%4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := (h+l-7*m+114)%31 + 1
	return time.Date(y, time.Month(month), day, 0, 0, 0, 0, loc)
}
