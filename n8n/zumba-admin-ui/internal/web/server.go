package web

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/a-h/templ"

	"github.com/michael/zumba-admin-ui/assets"
	"github.com/michael/zumba-admin-ui/internal/config"
	"github.com/michael/zumba-admin-ui/internal/store"
	"github.com/michael/zumba-admin-ui/internal/timeutil"
	"github.com/michael/zumba-admin-ui/web/templates"
	"github.com/michael/zumba-admin-ui/web/templates/dashboard"
	"github.com/michael/zumba-admin-ui/web/templates/days"
	"github.com/michael/zumba-admin-ui/web/templates/excluded"
	"github.com/michael/zumba-admin-ui/web/templates/members"
	"github.com/michael/zumba-admin-ui/web/templates/partials"
)

type Server struct {
	store    store.Store
	cfg      config.Config
	mockMode bool
}

func New(s store.Store, cfg config.Config, mockMode bool) *Server {
	return &Server{store: s, cfg: cfg, mockMode: mockMode}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	staticFS, err := fs.Sub(assets.Static, "static")
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("GET /{$}", s.handleRoot)
	mux.HandleFunc("GET /dashboard", s.handleDashboard)
	mux.HandleFunc("GET /members", s.handleMembers)
	mux.HandleFunc("GET /members/{userId}", s.handleMemberDetail)
	mux.HandleFunc("GET /days", s.handleDays)
	mux.HandleFunc("GET /days/{date}", s.handleDayDetail)
	mux.HandleFunc("GET /excluded", s.handleExcluded)

	return logRequests(mux)
}

func (s *Server) period() timeutil.Period {
	return timeutil.Period{Start: s.cfg.EvalPeriodStart, End: s.cfg.EvalPeriodEnd}
}

func (s *Server) meta(title, active string) templates.PageMeta {
	return templates.PageMeta{Title: title, ActiveNav: active, MockMode: s.mockMode}
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, meta templates.PageMeta, body templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.Layout(meta).Render(templ.WithChildren(r.Context(), body), w); err != nil {
		log.Printf("render: %v", err)
	}
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	period := s.period()

	board, err := s.store.Leaderboard(ctx, period)
	if err != nil {
		s.fail(w, "leaderboard", err)
		return
	}

	strip, err := s.buildStrip(ctx, period)
	if err != nil {
		s.fail(w, "strip", err)
		return
	}

	totalThursdays := 0
	totalAtt := 0
	totalAbs := 0
	pctSum := 0.0
	for _, r := range board {
		if r.ThursdayCount > totalThursdays {
			totalThursdays = r.ThursdayCount
		}
		totalAtt += r.AttendanceCount
		totalAbs += r.AwayCount
		pctSum += r.AttendPercent
	}
	avgRate := 0
	if len(board) > 0 {
		avgRate = int(pctSum/float64(len(board)) + 0.5)
	}

	vm := dashboard.ViewModel{
		PeriodStart:      timeutil.FormatDEShort(period.Start),
		PeriodEnd:        timeutil.FormatDEShort(period.End),
		TotalThursdays:   totalThursdays,
		TotalUsers:       len(board),
		TotalAttendances: totalAtt,
		TotalAbsences:    totalAbs,
		AverageRate:      avgRate,
		StripItems:       strip,
		Leaderboard:      board,
	}

	s.render(w, r, s.meta("Dashboard", "dashboard"), dashboard.Page(vm))
}

func (s *Server) handleMembers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	period := s.period()

	board, err := s.store.Leaderboard(ctx, period)
	if err != nil {
		s.fail(w, "leaderboard", err)
		return
	}
	strip, err := s.buildStrip(ctx, period)
	if err != nil {
		s.fail(w, "strip", err)
		return
	}

	s.render(w, r, s.meta("Mitglieder", "members"),
		members.List(members.ListVM{StripItems: strip, Rows: board}))
}

func (s *Server) handleMemberDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	period := s.period()
	userId := r.PathValue("userId")

	users, err := s.store.ListUsers(ctx)
	if err != nil {
		s.fail(w, "users", err)
		return
	}
	var user *store.User
	for i := range users {
		if users[i].ID == userId {
			user = &users[i]
			break
		}
	}
	if user == nil {
		http.NotFound(w, r)
		return
	}

	board, err := s.store.Leaderboard(ctx, period)
	if err != nil {
		s.fail(w, "leaderboard", err)
		return
	}
	var stats store.LeaderboardRow
	for _, row := range board {
		if row.UserID == userId {
			stats = row
			break
		}
	}

	thursdays, err := s.store.ListThursdays(ctx, period)
	if err != nil {
		s.fail(w, "thursdays", err)
		return
	}
	absences, err := s.store.ListAbsences(ctx, period)
	if err != nil {
		s.fail(w, "absences", err)
		return
	}
	absenceMap := make(map[string]*string, len(absences))
	for _, a := range absences {
		if a.UserID == userId {
			absenceMap[timeutil.FormatISO(a.Date)] = a.Message
		}
	}

	entries := make([]members.DetailEntry, 0, len(thursdays))
	for _, t := range thursdays {
		key := timeutil.FormatISO(t)
		msg, absent := absenceMap[key]
		entries = append(entries, members.DetailEntry{Date: t, Absent: absent, Message: msg})
	}

	s.render(w, r, s.meta(user.Name, "members"),
		members.Detail(members.DetailVM{User: *user, Stats: stats, Entries: entries}))
}

func (s *Server) handleDays(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	period := s.period()

	users, err := s.store.ListUsers(ctx)
	if err != nil {
		s.fail(w, "users", err)
		return
	}
	thursdays, err := s.store.ListThursdays(ctx, period)
	if err != nil {
		s.fail(w, "thursdays", err)
		return
	}
	absences, err := s.store.ListAbsences(ctx, period)
	if err != nil {
		s.fail(w, "absences", err)
		return
	}
	strip, err := s.buildStrip(ctx, period)
	if err != nil {
		s.fail(w, "strip", err)
		return
	}

	awayByDate := make(map[string][]string, len(thursdays))
	for _, a := range absences {
		k := timeutil.FormatISO(a.Date)
		awayByDate[k] = append(awayByDate[k], a.UserID)
	}

	cards := make([]days.DayCard, 0, len(thursdays))
	for _, t := range thursdays {
		k := timeutil.FormatISO(t)
		away := awayByDate[k]
		cards = append(cards, days.DayCard{
			Date:          t,
			Attendance:    len(users) - len(away),
			AwayCount:     len(away),
			AbsentUserIDs: away,
		})
	}

	s.render(w, r, s.meta("Donnerstage", "days"),
		days.List(days.ListVM{StripItems: strip, Days: cards, TotalUsers: len(users)}))
}

func (s *Server) handleDayDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	dateStr := r.PathValue("date")
	date, err := timeutil.ParseISO(dateStr)
	if err != nil {
		http.Error(w, "ungültiges Datum", http.StatusBadRequest)
		return
	}

	users, err := s.store.ListUsers(ctx)
	if err != nil {
		s.fail(w, "users", err)
		return
	}
	period := s.period()
	excludedDays, err := s.store.ListExcludedDays(ctx, period)
	if err != nil {
		s.fail(w, "excluded", err)
		return
	}
	isExcluded := false
	for _, d := range excludedDays {
		if timeutil.FormatISO(d) == dateStr {
			isExcluded = true
			break
		}
	}

	var cells []days.Cell
	if !isExcluded {
		// Fetch absences for this specific date by filtering all in period.
		all, err := s.store.ListAbsences(ctx, period)
		if err != nil {
			s.fail(w, "absences", err)
			return
		}
		absMap := make(map[string]*string)
		for _, a := range all {
			if timeutil.FormatISO(a.Date) == dateStr {
				absMap[a.UserID] = a.Message
			}
		}
		cells = make([]days.Cell, 0, len(users))
		for _, u := range users {
			msg, absent := absMap[u.ID]
			cells = append(cells, days.Cell{
				UserID:  u.ID,
				Name:    u.Name,
				Absent:  absent,
				Message: msg,
			})
		}
		sort.SliceStable(cells, func(i, j int) bool {
			if cells[i].Absent != cells[j].Absent {
				return !cells[i].Absent // present first
			}
			return cells[i].Name < cells[j].Name
		})
	}

	s.render(w, r, s.meta(timeutil.FormatDEShort(date), "days"),
		days.Detail(days.DetailVM{Date: date, Excluded: isExcluded, Cells: cells}))
}

func (s *Server) handleExcluded(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	period := s.period()
	all, err := s.store.ListExcludedDays(ctx, period)
	if err != nil {
		s.fail(w, "excluded", err)
		return
	}
	s.render(w, r, s.meta("Sperrtage", "excluded"),
		excluded.List(excluded.ListVM{Days: all}))
}

func (s *Server) buildStrip(ctx context.Context, period timeutil.Period) ([]partials.ThursdayStripItem, error) {
	thursdays, err := s.store.ListThursdays(ctx, period)
	if err != nil {
		return nil, err
	}
	excludedDays, err := s.store.ListExcludedDays(ctx, period)
	if err != nil {
		return nil, err
	}
	absences, err := s.store.ListAbsences(ctx, period)
	if err != nil {
		return nil, err
	}
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		return nil, err
	}

	awayCounts := make(map[string]int)
	for _, a := range absences {
		awayCounts[timeutil.FormatISO(a.Date)]++
	}
	excludedSet := make(map[string]bool, len(excludedDays))
	for _, d := range excludedDays {
		excludedSet[timeutil.FormatISO(d)] = true
	}

	// Combine: take the most recent 12 days from union(thursdays, excluded), newest first, then reverse to oldest-first display.
	all := make(map[string]time.Time, len(thursdays)+len(excludedDays))
	for _, t := range thursdays {
		all[timeutil.FormatISO(t)] = t
	}
	for _, t := range excludedDays {
		all[timeutil.FormatISO(t)] = t
	}
	// Only keep Thursdays that are <= today.
	today := timeutil.StartOfDay(time.Now())
	dates := make([]time.Time, 0, len(all))
	for _, t := range all {
		if !t.After(today) && t.Weekday() == time.Thursday {
			dates = append(dates, t)
		}
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].After(dates[j]) })
	if len(dates) > 12 {
		dates = dates[:12]
	}
	// reverse: oldest-left, newest-right
	for i, j := 0, len(dates)-1; i < j; i, j = i+1, j-1 {
		dates[i], dates[j] = dates[j], dates[i]
	}

	totalUsers := len(users)
	out := make([]partials.ThursdayStripItem, 0, len(dates))
	for _, t := range dates {
		k := timeutil.FormatISO(t)
		if excludedSet[k] {
			out = append(out, partials.ThursdayStripItem{Date: t, Excluded: true})
			continue
		}
		away := awayCounts[k]
		attended := totalUsers - away
		out = append(out, partials.ThursdayStripItem{
			Date: t,
			Rate: partials.RateLabel(attended, totalUsers),
		})
	}
	return out, nil
}

func (s *Server) fail(w http.ResponseWriter, what string, err error) {
	log.Printf("%s: %v", what, err)
	http.Error(w, "interner Fehler", http.StatusInternalServerError)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
