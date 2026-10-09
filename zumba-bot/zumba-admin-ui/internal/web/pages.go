package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/michael/zumba-shared/domain"
	"github.com/michael/zumba-shared/penalty"

	"github.com/michael/zumba-admin-ui/internal/store"
	"github.com/michael/zumba-admin-ui/internal/timeutil"
	"github.com/michael/zumba-admin-ui/web/emoji"
	"github.com/michael/zumba-admin-ui/web/templates/dashboard"
	"github.com/michael/zumba-admin-ui/web/templates/days"
	"github.com/michael/zumba-admin-ui/web/templates/members"
	"github.com/michael/zumba-admin-ui/web/templates/partials"
)

// seasonData bündelt, was Dashboard, Donnerstage und Mitglieder gemeinsam
// brauchen: die Rangliste und alle Donnerstage des Jahres bis heute (inkl.
// Sperrtage) mit Abmelde-Zahl – zusammen zwei Abfragen.
type seasonData struct {
	Board []store.LeaderboardRow
	Days  []store.StripDay // aufsteigend
}

func (s *Server) loadSeasonData(ctx context.Context, season store.Season) (seasonData, error) {
	board, err := s.store.Leaderboard(ctx, season.Period)
	if err != nil {
		return seasonData{}, fmt.Errorf("leaderboard: %w", err)
	}
	strip, err := s.store.ThursdayStrip(ctx, season.Period, 0)
	if err != nil {
		return seasonData{}, fmt.Errorf("strip: %w", err)
	}
	return seasonData{Board: board, Days: strip}, nil
}

// members zählt, wer an day schon dabei war – wer später eingestiegen ist,
// zählt an früheren Donnerstagen weder als anwesend noch als abgemeldet.
func (d seasonData) members(day time.Time) int {
	n := 0
	for _, r := range d.Board {
		if !r.EffectiveStart.After(day) {
			n++
		}
	}
	return n
}

func (d seasonData) present(sd store.StripDay) int {
	return max(d.members(sd.Date)-sd.Away, 0)
}

func (d seasonData) pct(sd store.StripDay) int {
	return partials.Pct(d.present(sd), d.members(sd.Date))
}

// valid sind die gezählten Donnerstage (ohne Sperrtage), aufsteigend.
func (d seasonData) valid() []store.StripDay {
	out := make([]store.StripDay, 0, len(d.Days))
	for _, sd := range d.Days {
		if !sd.Excluded {
			out = append(out, sd)
		}
	}
	return out
}

func (d seasonData) excludedCount() int {
	return len(d.Days) - len(d.valid())
}

// avgRate ist der Mittelwert der Quoten aller Mitglieder (wie bisher).
func avgRate(board []store.LeaderboardRow) int {
	if len(board) == 0 {
		return 0
	}
	sum := 0.0
	for _, r := range board {
		sum += r.AttendPercent
	}
	return partials.Percent(sum / float64(len(board)))
}

// seasonQuery hält ?jahr= für Links fest, wenn ein Archivjahr angezeigt wird.
func seasonQuery(r *http.Request) string {
	if label := r.URL.Query().Get("jahr"); label != "" {
		return "?" + url.Values{"jahr": {label}}.Encode()
	}
	return ""
}

func dayTitle(d time.Time, present, total int, excluded bool) string {
	if excluded {
		return timeutil.FormatDE(d) + " – Sperrtag"
	}
	return fmt.Sprintf("%s – %d/%d da", timeutil.FormatDE(d), present, total)
}

// longDate: "2. Oktober 2026".
func longDate(t time.Time) string {
	return fmt.Sprintf("%d. %s %d", t.Day(), partials.MonthsLong[t.Month()-1], t.Year())
}

/* ── Dashboard ──────────────────────────────────────────────────────── */

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	season, ok := s.pageSeason(w, r)
	if !ok {
		return
	}
	data, err := s.loadSeasonData(ctx, season)
	if err != nil {
		s.fail(w, "dashboard", err)
		return
	}
	readOnly := archived(season)
	q := seasonQuery(r)

	vm := dashboard.ViewModel{
		U:             len(data.Board),
		AvgRate:       avgRate(data.Board),
		DayCount:      len(data.valid()),
		ExcludedCount: data.excludedCount(),
		Next:          s.nextStammtisch(),
		Query:         q,
		ReadOnly:      readOnly,
	}
	if label, delta, ok := s.prevSeasonDelta(ctx, season, vm.AvgRate); ok {
		vm.AvgDelta = fmt.Sprintf("%+d %% ggü. %s", delta, label)
		vm.AvgDeltaClass = "is-green"
		if delta < 0 {
			vm.AvgDeltaClass = "is-red"
		}
	}

	// Kasse: dieselbe Bewertung wie auf der Strafen-Seite, aber ohne Marker
	// zu schreiben – das Dashboard liest nur.
	if lage, err := s.strafenLage(ctx, season, false); err != nil {
		s.fail(w, "strafen", err)
		return
	} else {
		for _, e := range lage.entries {
			if e.Status == penalty.StatusOffen {
				vm.OffenSum += e.Betrag
				vm.OffenCount++
			}
		}
	}

	if err := s.fillDashboardDay(ctx, &vm, data, r.URL.Query().Get("tag"), r.URL.Query().Get("jahr")); err != nil {
		s.fail(w, "dashboard day", err)
		return
	}
	vm.Attention = attention(data.Board, q)
	vm.Months, vm.Range = seasonBars(data, vm.Day.ISO)
	plaetze := boardRanks(data.Board)
	for i, row := range data.Board {
		if i == 7 {
			break
		}
		vm.Top = append(vm.Top, dashboard.RankRow{
			Rank: plaetze[i], UserID: row.UserID, Name: row.UserName, Emoji: emoji.For(row.UserName),
			Pct: partials.Percent(row.AttendPercent), Streak: row.Streak,
		})
	}

	meta := s.seasonMeta(r, "Dashboard", "dashboard", season)
	meta.RefreshURL = r.URL.RequestURI()
	s.render(w, r, meta, dashboard.Page(vm))
}

// fillDashboardDay füllt die Karte „Zuletzt am Stammtisch": den gewählten
// Donnerstag (?tag=), sonst den jüngsten gültigen.
func (s *Server) fillDashboardDay(ctx context.Context, vm *dashboard.ViewModel, data seasonData, tag, jahr string) error {
	valid := data.valid()
	if len(valid) == 0 {
		vm.Day = dashboard.DayVM{Long: "Noch kein Donnerstag", DM: "–"}
		return nil
	}
	idx := len(valid) - 1
	for i, sd := range valid {
		if timeutil.FormatISO(sd.Date) == tag {
			idx = i
		}
	}
	sd := valid[idx]
	present := data.present(sd)
	day := dashboard.DayVM{
		ISO:     timeutil.FormatISO(sd.Date),
		DM:      partials.DM(sd.Date),
		Long:    longDate(sd.Date),
		Present: present,
		Pct:     data.pct(sd),
	}
	link := func(d time.Time) string {
		v := url.Values{"tag": {timeutil.FormatISO(d)}}
		if jahr != "" {
			v.Set("jahr", jahr)
		}
		return "/dashboard?" + v.Encode()
	}
	if idx > 0 {
		prev := valid[idx-1]
		diff := present - data.present(prev)
		day.Delta = fmt.Sprintf("%+d zur Vorwoche", diff)
		day.DeltaClass = "is-green"
		if diff < 0 {
			day.DeltaClass = "is-red"
		}
		day.PrevHref = link(prev.Date)
	}
	if idx < len(valid)-1 {
		day.NextHref = link(valid[idx+1].Date)
	}

	absences, err := s.store.AbsencesOn(ctx, sd.Date)
	if err != nil {
		return err
	}
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		return err
	}
	names := make(map[string]string, len(users))
	for _, u := range users {
		names[u.ID] = u.Name
	}
	for _, a := range absences {
		name := names[a.UserID]
		if name == "" {
			name = a.UserID
		}
		p := dashboard.Person{UserID: a.UserID, Name: name, Emoji: emoji.For(name)}
		if a.Message != nil {
			p.Message = *a.Message
		}
		day.Absent = append(day.Absent, p)
	}
	sort.Slice(day.Absent, func(i, j int) bool { return day.Absent[i].Name < day.Absent[j].Name })
	vm.Day = day
	return nil
}

// attention sind die laufenden Absage-Serien ab drei Wochen – ab fünf wird
// es teuer.
func attention(board []store.LeaderboardRow, q string) []dashboard.AttnRow {
	var out []dashboard.AttnRow
	for _, r := range board {
		weeks := -r.Streak
		if weeks < 3 {
			continue
		}
		row := dashboard.AttnRow{
			UserID: r.UserID, Name: r.UserName, Emoji: emoji.For(r.UserName), Weeks: weeks,
			Meter: min(100, weeks*100/penalty.MinFehltage),
		}
		if rest := penalty.MinFehltage - weeks; rest > 0 {
			row.Class = "is-amber"
			row.Badge = fmt.Sprintf("%d/%d · noch %d Wo.", weeks, penalty.MinFehltage, rest)
		} else {
			row.Class = "is-red"
			row.Badge = fmt.Sprintf("%d € läuft", penalty.Betrag(weeks))
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Weeks > out[j].Weeks })
	return out
}

// seasonBars gruppiert die Donnerstage nach Monat für den Saisonverlauf.
func seasonBars(data seasonData, selISO string) ([]dashboard.Month, string) {
	var months []dashboard.Month
	lastKey := ""
	for _, sd := range data.Days {
		key := sd.Date.Format("2006-01")
		if key != lastKey {
			months = append(months, dashboard.Month{Label: partials.MonthsAbbr[sd.Date.Month()-1]})
			lastKey = key
		}
		iso := timeutil.FormatISO(sd.Date)
		bar := dashboard.Bar{ISO: iso, Excluded: sd.Excluded}
		if sd.Excluded {
			bar.Class = "is-excluded"
			bar.Title = dayTitle(sd.Date, 0, 0, true)
		} else {
			pct := data.pct(sd)
			bar.H = max(6, pct)
			bar.Class = partials.Tier(pct)
			if iso == selISO {
				bar.Class = "is-sel"
			}
			bar.Title = dayTitle(sd.Date, data.present(sd), data.members(sd.Date), false)
		}
		months[len(months)-1].Bars = append(months[len(months)-1].Bars, bar)
	}
	rng := ""
	if len(data.Days) > 0 {
		first, last := data.Days[0].Date, data.Days[len(data.Days)-1].Date
		rng = partials.MonthsAbbr[first.Month()-1] + " – " + partials.MonthsAbbr[last.Month()-1]
	}
	return months, rng
}

// prevSeasonDelta vergleicht die Quote mit dem Vorjahr (dessen Endstand).
func (s *Server) prevSeasonDelta(ctx context.Context, season store.Season, avg int) (string, int, bool) {
	seasons, err := s.store.ListSeasons(ctx)
	if err != nil {
		return "", 0, false
	}
	var prev *store.Season
	for i := range seasons {
		sn := seasons[i]
		if sn.End.Before(season.Start) && (prev == nil || sn.End.After(prev.End)) {
			prev = &seasons[i]
		}
	}
	if prev == nil {
		return "", 0, false
	}
	board, err := s.store.Leaderboard(ctx, prev.Period)
	if err != nil || len(board) == 0 {
		return "", 0, false
	}
	return prev.Label, avg - avgRate(board), true
}

// nextStammtisch: der nächste Donnerstag ab heute und der jüngste bis heute.
func (s *Server) nextStammtisch() partials.NextStammtisch {
	today := timeutil.StartOfDay(time.Now())
	inDays := (int(time.Thursday) - int(today.Weekday()) + 7) % 7
	next := today.AddDate(0, 0, inDays)
	last := today.AddDate(0, 0, -((int(today.Weekday()) - int(time.Thursday) + 7) % 7))
	return partials.NextStammtisch{
		Label:   "Do, " + partials.DM(next),
		InDays:  inDays,
		LastISO: timeutil.FormatISO(last),
	}
}

/* ── Donnerstage ────────────────────────────────────────────────────── */

func (s *Server) handleDays(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	season, ok := s.pageSeason(w, r)
	if !ok {
		return
	}
	data, err := s.loadSeasonData(ctx, season)
	if err != nil {
		s.fail(w, "days", err)
		return
	}
	absences, err := s.store.ListAbsences(ctx, season.Period)
	if err != nil {
		s.fail(w, "absences", err)
		return
	}
	absent := make(map[string]bool, len(absences))
	for _, a := range absences {
		absent[a.UserID+"|"+timeutil.FormatISO(a.Date)] = true
	}

	vm := days.ListVM{ReadOnly: archived(season), Query: seasonQuery(r)}
	valid := data.valid()
	if len(valid) > 0 {
		vm.SelISO = timeutil.FormatISO(valid[len(valid)-1].Date)
	}

	for i, sd := range data.Days {
		iso := timeutil.FormatISO(sd.Date)
		present, total := data.present(sd), data.members(sd.Date)
		pct := data.pct(sd)
		month := partials.MonthsAbbr[sd.Date.Month()-1]
		vm.Tiles = append(vm.Tiles, days.Tile{
			ISO: iso, Day: sd.Date.Day(), Month: month,
			Title:    dayTitle(sd.Date, present, total, sd.Excluded),
			Class:    partials.Tier(pct),
			Excluded: sd.Excluded,
			Current:  iso == vm.SelISO,
		})
		col := days.Col{
			ISO: iso, Day: sd.Date.Format("02"), Month: month,
			Excluded:   sd.Excluded,
			MonthStart: i == 0 || data.Days[i-1].Date.Month() != sd.Date.Month(),
			Present:    present,
		}
		if sd.Excluded {
			col.Info = timeutil.FormatDE(sd.Date) + " · Sperrtag"
			col.CountClass = "is-excluded"
		} else {
			col.Info = fmt.Sprintf("%s · %d/%d anwesend", timeutil.FormatDE(sd.Date), present, total)
			switch {
			case pct >= 85:
				col.CountClass = "is-top"
			case pct < 50:
				col.CountClass = "is-low"
			}
		}
		if iso == vm.SelISO {
			vm.SelInfo = col.Info
		}
		vm.Cols = append(vm.Cols, col)
	}

	for _, row := range data.Board {
		mr := days.Row{
			UserID: row.UserID, Name: row.UserName, Emoji: emoji.For(row.UserName),
			Pct: partials.Percent(row.AttendPercent),
		}
		for i, sd := range data.Days {
			iso := timeutil.FormatISO(sd.Date)
			c := days.Cell{ISO: iso, Excluded: sd.Excluded, Border: vm.Cols[i].MonthStart}
			c.NotYet = !sd.Excluded && row.EffectiveStart.After(sd.Date)
			c.Absent = absent[row.UserID+"|"+iso]
			label := partials.DM(sd.Date)
			switch {
			case sd.Excluded:
				c.Title = row.UserName + " · " + label + " · Sperrtag"
			case c.NotYet:
				c.Title = row.UserName + " · " + label + " · noch nicht dabei"
			case c.Absent:
				c.Title = row.UserName + " · " + label + " · abgemeldet"
			default:
				c.Title = row.UserName + " · " + label + " · anwesend"
			}
			mr.Cells = append(mr.Cells, c)
		}
		vm.Rows = append(vm.Rows, mr)
	}

	meta := s.seasonMeta(r, "Donnerstage", "days", season)
	meta.RefreshURL = r.URL.RequestURI()
	s.render(w, r, meta, days.List(vm))
}

// handleDayDetail zeigt einen Donnerstag: mit ?sheet=1 als Bottom-Sheet
// (HTMX-Fragment), sonst als eigene Seite.
func (s *Server) handleDayDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	date, err := timeutil.ParseISO(r.PathValue("date"))
	if err != nil {
		http.Error(w, "ungültiges Datum", http.StatusBadRequest)
		return
	}
	vm, err := s.sheetVM(ctx, date)
	if err != nil {
		s.fail(w, "day", err)
		return
	}
	if r.URL.Query().Get("sheet") != "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := days.Sheet(vm).Render(ctx, w); err != nil {
			s.fail(w, "render sheet", err)
		}
		return
	}
	meta := s.meta(timeutil.FormatDEShort(date), "days")
	meta.Heading = vm.Long
	meta.RefreshURL = r.URL.RequestURI()
	s.render(w, r, meta, days.Detail(vm))
}

func (s *Server) sheetVM(ctx context.Context, date time.Time) (days.SheetVM, error) {
	vm := days.SheetVM{
		ISO:  timeutil.FormatISO(date),
		Long: weekdayLong(date) + ", " + longDate(date),
		KW:   partials.KW(date),
	}

	season, err := s.store.SeasonAt(ctx, date)
	if err != nil && !errors.Is(err, domain.ErrNoSeason) {
		return vm, err
	}
	// Ohne gepflegtes Jahr ist der Tag nicht auswertbar – anzeigen ja,
	// ändern nein.
	hasSeason := err == nil
	vm.ReadOnly = !hasSeason || archived(season)

	excluded, err := s.store.IsExcludedDay(ctx, date)
	if err != nil {
		return vm, err
	}
	vm.Excluded = excluded

	// Nachbarn und Chip-Leiste: die gültigen Donnerstage des Jahres.
	if hasSeason {
		thursdays, err := s.store.ListThursdays(ctx, season.Period) // neueste zuerst
		if err != nil {
			return vm, err
		}
		asc := make([]time.Time, len(thursdays))
		for i, t := range thursdays {
			asc[len(thursdays)-1-i] = t
		}
		idx := -1
		for i, t := range asc {
			if t.Equal(date) {
				idx = i
			}
		}
		if idx > 0 {
			vm.PrevISO = timeutil.FormatISO(asc[idx-1])
		}
		if idx >= 0 && idx < len(asc)-1 {
			vm.NextISO = timeutil.FormatISO(asc[idx+1])
		}
		// 14 Chips, der gewählte Tag möglichst weit rechts.
		end := len(asc)
		if idx >= 0 {
			end = min(len(asc), max(idx+4, 14))
		}
		for _, t := range asc[max(0, end-14):end] {
			iso := timeutil.FormatISO(t)
			vm.Strip = append(vm.Strip, days.Chip{ISO: iso, Label: partials.DM(t), Active: iso == vm.ISO})
		}
	}

	if excluded {
		return vm, nil
	}
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		return vm, err
	}
	absences, err := s.store.AbsencesOn(ctx, date)
	if err != nil {
		return vm, err
	}
	msgs := make(map[string]*string, len(absences))
	for _, a := range absences {
		msgs[a.UserID] = a.Message
	}
	for _, u := range users {
		msg, absent := msgs[u.ID]
		p := days.Person{UserID: u.ID, Name: u.Name, Emoji: emoji.For(u.Name), Absent: absent}
		if msg != nil {
			p.Message = *msg
		}
		if absent {
			vm.Absent++
		} else {
			vm.Present++
		}
		vm.Cells = append(vm.Cells, p)
	}
	// Alphabetisch, damit beim Umschalten niemand unter dem Finger wegrutscht.
	sort.SliceStable(vm.Cells, func(i, j int) bool { return vm.Cells[i].Name < vm.Cells[j].Name })
	return vm, nil
}

func weekdayLong(t time.Time) string {
	return []string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"}[t.Weekday()]
}

/* ── Mitglieder ─────────────────────────────────────────────────────── */

func (s *Server) handleMembers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	season, ok := s.pageSeason(w, r)
	if !ok {
		return
	}
	board, err := s.store.Leaderboard(ctx, season.Period)
	if err != nil {
		s.fail(w, "leaderboard", err)
		return
	}
	q := seasonQuery(r)
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	vm := members.ListVM{Query: q, Search: search}
	needle := strings.ToLower(search)
	for _, row := range board {
		if needle != "" && !strings.Contains(strings.ToLower(row.UserName), needle) {
			continue
		}
		vm.Cards = append(vm.Cards, members.Card{
			UserID: row.UserID, Name: row.UserName, Emoji: emoji.For(row.UserName),
			Pct: partials.Percent(row.AttendPercent), Attend: row.AttendanceCount, Away: row.AwayCount,
			Streak: row.Streak,
		})
	}
	// Ein eindeutiger Treffer aus der Suche führt direkt zum Mitglied.
	if search != "" && len(vm.Cards) == 1 {
		http.Redirect(w, r, "/members/"+vm.Cards[0].UserID+q, http.StatusSeeOther)
		return
	}
	meta := s.seasonMeta(r, "Mitglieder", "members", season)
	meta.RefreshURL = r.URL.RequestURI()
	s.render(w, r, meta, members.List(vm))
}

func (s *Server) handleMemberDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	season, ok := s.pageSeason(w, r)
	if !ok {
		return
	}
	userID := r.PathValue("userId")
	user, err := s.store.GetUser(ctx, userID)
	if err != nil {
		s.fail(w, "user", err)
		return
	}
	if user == nil {
		http.NotFound(w, r)
		return
	}
	data, err := s.loadSeasonData(ctx, season)
	if err != nil {
		s.fail(w, "member", err)
		return
	}
	absences, err := s.store.ListUserAbsences(ctx, season.Period, userID)
	if err != nil {
		s.fail(w, "absences", err)
		return
	}
	msgs := make(map[string]*string, len(absences))
	for _, a := range absences {
		msgs[timeutil.FormatISO(a.Date)] = a.Message
	}

	vm := members.DetailVM{
		UserID: user.ID, Name: user.Name, Emoji: emoji.For(user.Name),
		U: len(data.Board), ReadOnly: archived(season), Query: seasonQuery(r),
	}
	start := time.Time{}
	plaetze := boardRanks(data.Board)
	for i, row := range data.Board {
		if row.UserID == userID {
			vm.Rank = plaetze[i]
			vm.Pct = partials.Percent(row.AttendPercent)
			vm.Attend, vm.Away, vm.Streak = row.AttendanceCount, row.AwayCount, row.Streak
			start = row.EffectiveStart
		}
	}
	for _, sd := range data.Days {
		if sd.Date.Before(start) {
			continue // vor dem Einstieg
		}
		iso := timeutil.FormatISO(sd.Date)
		msg, absent := msgs[iso]
		state := "anwesend"
		switch {
		case sd.Excluded:
			state = "Sperrtag"
		case absent:
			state = "abgemeldet"
		}
		vm.Strip = append(vm.Strip, members.Dot{
			ISO: iso, Absent: absent, Excluded: sd.Excluded,
			Title: partials.DM(sd.Date) + " · " + state,
		})
		if sd.Excluded {
			continue
		}
		e := members.Entry{ISO: iso, Label: timeutil.FormatDE(sd.Date), Absent: absent}
		if msg != nil {
			e.Message = *msg
		}
		vm.Entries = append([]members.Entry{e}, vm.Entries...)
	}

	meta := s.seasonMeta(r, user.Name, "members", season)
	meta.Heading = user.Name
	meta.RefreshURL = r.URL.RequestURI()
	s.render(w, r, meta, members.Detail(vm))
}

// boardRanks vergibt die Plätze der Rangliste wie im Sport ("1-2-2-4") –
// gleichauf ist, wer gleich oft da war und dieselbe Quote hat. Dieselbe Regel
// wie in der Bot-Statistik.
func boardRanks(board []store.LeaderboardRow) []int {
	return domain.CompetitionRanks(len(board), func(i int) bool {
		return board[i].AttendanceCount == board[i-1].AttendanceCount && board[i].AttendPercent == board[i-1].AttendPercent
	})
}
