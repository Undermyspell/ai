package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/a-h/templ"

	"github.com/michael/zumba-shared/domain"

	"github.com/michael/zumba-admin-ui/assets"
	"github.com/michael/zumba-admin-ui/internal/config"
	"github.com/michael/zumba-admin-ui/internal/store"
	"github.com/michael/zumba-admin-ui/internal/timeutil"
	"github.com/michael/zumba-admin-ui/web/templates"
	"github.com/michael/zumba-admin-ui/web/templates/bottest"
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
	mux.HandleFunc("POST /excluded", s.handleAddExcluded)
	mux.HandleFunc("DELETE /excluded/{date}", s.handleDeleteExcluded)
	mux.HandleFunc("POST /toggle-absence", s.handleToggleAbsence)
	mux.HandleFunc("GET /strafen", s.handleStrafen)
	mux.HandleFunc("POST /strafen", s.handleAddStrafe)
	mux.HandleFunc("POST /strafen/{id}/begleichen", s.handleBegleicheStrafe)
	mux.HandleFunc("DELETE /strafen/{id}", s.handleDeleteStrafe)
	mux.HandleFunc("GET /bot-test", s.handleBotTest)
	mux.HandleFunc("GET /bot-test/example/{kind}", s.handleBotTestExample)
	mux.HandleFunc("POST /bot-test/run", s.handleBotTestRun)
	mux.HandleFunc("GET /trace", s.handleTraceList)
	mux.HandleFunc("GET /trace/{id}", s.handleTraceDetail)
	mux.HandleFunc("GET /ml-shadow", s.handleMLShadow)
	mux.HandleFunc("POST /ml-shadow/verify/{id}", s.handleMLVerify)
	mux.HandleFunc("GET /ml-test", s.handleMLTest)
	mux.HandleFunc("POST /ml-test/run", s.handleMLTestRun)
	mux.HandleFunc("POST /ml-test/judge/{id}", s.handleMLTestJudge)
	mux.HandleFunc("DELETE /ml-test/{id}", s.handleMLTestDelete)
	mux.HandleFunc("GET /ml-doku", s.handleMLDocs)

	return logRequests(mux)
}

// season löst das Stammtischjahr des Requests auf: ?jahr=<label> wählt ein
// bestimmtes (Archiv), ohne Parameter gilt das heute laufende. Ist keines
// gepflegt, kommt ein Fehler – lieber eine sichtbare Meldung als eine stille
// Auswertung des falschen Zeitraums.
func (s *Server) season(r *http.Request) (store.Season, error) {
	ctx := r.Context()
	if label := r.URL.Query().Get("jahr"); label != "" {
		return s.store.SeasonByLabel(ctx, label)
	}
	return s.store.SeasonAt(ctx, timeutil.StartOfDay(time.Now()))
}

// pageSeason ist season() für Seiten-Handler: bei einem unbekannten Jahr
// antwortet es selbst und liefert ok == false.
func (s *Server) pageSeason(w http.ResponseWriter, r *http.Request) (store.Season, bool) {
	season, err := s.season(r)
	if errors.Is(err, domain.ErrNoSeason) {
		log.Printf("season: %v", err)
		http.Error(w, "Für diesen Zeitraum ist kein Stammtischjahr gepflegt.", http.StatusNotFound)
		return store.Season{}, false
	}
	if err != nil {
		s.fail(w, "season", err)
		return store.Season{}, false
	}
	return season, true
}

// archived: das Jahr ist vorbei. Archiv-Ansichten sind read-only – sonst
// ändert man aus der Rückschau versehentlich abgeschlossene Jahre.
func archived(season store.Season) bool {
	return season.End.Before(timeutil.StartOfDay(time.Now()))
}

// requireWritable stellt sicher, dass das Datum in ein noch laufendes Jahr
// fällt. Der Guard hängt am Datum, nicht am ?jahr= des Requests: so greift er
// auch, wenn ein HTMX-Aufruf ohne Jahres-Parameter hereinkommt.
func (s *Server) requireWritable(w http.ResponseWriter, r *http.Request, date time.Time) bool {
	season, err := s.store.SeasonAt(r.Context(), date)
	if errors.Is(err, domain.ErrNoSeason) {
		s.triggerToast(w, "error", "Für dieses Datum ist kein Stammtischjahr gepflegt.")
		http.Error(w, "kein Stammtischjahr", http.StatusUnprocessableEntity)
		return false
	}
	if err != nil {
		s.fail(w, "season", err)
		return false
	}
	if archived(season) {
		s.triggerToast(w, "error", "Stammtischjahr "+season.Label+" ist abgeschlossen – keine Änderungen möglich.")
		http.Error(w, "Jahr abgeschlossen", http.StatusConflict)
		return false
	}
	return true
}

func (s *Server) meta(title, active string) templates.PageMeta {
	return templates.PageMeta{Title: title, ActiveNav: active, MockMode: s.mockMode}
}

// seasonMeta ergänzt meta um den Jahres-Umschalter. Nur Seiten mit
// Jahresbezug bekommen ihn – Bot-Test und die ML-Seiten haben keinen.
func (s *Server) seasonMeta(r *http.Request, title, active string, season store.Season) templates.PageMeta {
	m := s.meta(title, active)
	m.Season = season.Label
	m.SeasonRange = timeutil.FormatDEShort(season.Start) + " – " + timeutil.FormatDEShort(season.End)
	m.SeasonArchived = archived(season)

	seasons, err := s.store.ListSeasons(r.Context())
	if err != nil {
		log.Printf("list seasons: %v", err) // Umschalter entfällt, Seite bleibt
		return m
	}
	for _, sn := range seasons {
		q := url.Values{"jahr": {sn.Label}}
		m.Seasons = append(m.Seasons, templates.SeasonLink{
			Label:  sn.Label,
			Href:   r.URL.Path + "?" + q.Encode(),
			Active: sn.Label == season.Label,
		})
	}
	return m
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
	season, ok := s.pageSeason(w, r)
	if !ok {
		return
	}
	period := season.Period

	board, err := s.store.Leaderboard(ctx, period)
	if err != nil {
		s.fail(w, "leaderboard", err)
		return
	}

	strip, err := s.buildStrip(ctx, period, 0, len(board)) // alle Donnerstage – auf dem Dashboard auswählbar
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

	s.render(w, r, s.seasonMeta(r, "Dashboard", "dashboard", season), dashboard.Page(vm))
}

// Mitglieder sind jetzt direkt im Dashboard integriert; alte /members-Links umleiten.
func (s *Server) handleMembers(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/dashboard", http.StatusMovedPermanently)
}

func (s *Server) handleMemberDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	season, ok := s.pageSeason(w, r)
	if !ok {
		return
	}
	period := season.Period
	userId := r.PathValue("userId")

	user, err := s.store.GetUser(ctx, userId)
	if err != nil {
		s.fail(w, "user", err)
		return
	}
	if user == nil {
		http.NotFound(w, r)
		return
	}

	stats, err := s.store.UserLeaderboardRow(ctx, period, userId)
	if err != nil {
		s.fail(w, "leaderboard", err)
		return
	}

	thursdays, err := s.store.ListThursdays(ctx, period)
	if err != nil {
		s.fail(w, "thursdays", err)
		return
	}
	absences, err := s.store.ListUserAbsences(ctx, period, userId)
	if err != nil {
		s.fail(w, "absences", err)
		return
	}
	absenceMap := make(map[string]*string, len(absences))
	for _, a := range absences {
		absenceMap[timeutil.FormatISO(a.Date)] = a.Message
	}

	entries := make([]members.DetailEntry, 0, len(thursdays))
	for _, t := range thursdays {
		key := timeutil.FormatISO(t)
		msg, absent := absenceMap[key]
		entries = append(entries, members.DetailEntry{Date: t, Absent: absent, Message: msg})
	}

	s.render(w, r, s.seasonMeta(r, user.Name, "dashboard", season),
		members.Detail(members.DetailVM{
			User: *user, Stats: stats, Entries: entries, ReadOnly: archived(season),
		}))
}

func (s *Server) handleDays(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	season, ok := s.pageSeason(w, r)
	if !ok {
		return
	}
	period := season.Period

	users, err := s.store.ListUsers(ctx)
	if err != nil {
		s.fail(w, "users", err)
		return
	}
	dayAbsences, err := s.store.ListDayAbsences(ctx, period)
	if err != nil {
		s.fail(w, "day absences", err)
		return
	}
	strip, err := s.buildStrip(ctx, period, 12, len(users))
	if err != nil {
		s.fail(w, "strip", err)
		return
	}

	cards := make([]days.DayCard, 0, len(dayAbsences))
	for _, d := range dayAbsences {
		cards = append(cards, days.DayCard{
			Date:          d.Date,
			Attendance:    len(users) - len(d.AbsentUserIDs),
			AwayCount:     len(d.AbsentUserIDs),
			AbsentUserIDs: d.AbsentUserIDs,
		})
	}

	s.render(w, r, s.seasonMeta(r, "Donnerstage", "days", season),
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
	isExcluded, err := s.store.IsExcludedDay(ctx, date)
	if err != nil {
		s.fail(w, "excluded", err)
		return
	}

	var cells []days.Cell
	if !isExcluded {
		dayAbsences, err := s.store.AbsencesOn(ctx, date)
		if err != nil {
			s.fail(w, "absences", err)
			return
		}
		absMap := make(map[string]*string, len(dayAbsences))
		for _, a := range dayAbsences {
			absMap[a.UserID] = a.Message
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

	season, err := s.store.SeasonAt(ctx, date)
	if err != nil && !errors.Is(err, domain.ErrNoSeason) {
		s.fail(w, "season", err)
		return
	}
	// Ohne gepflegtes Jahr ist der Tag nicht auswertbar – anzeigen ja,
	// ändern nein.
	readOnly := err != nil || archived(season)

	s.render(w, r, s.meta(timeutil.FormatDEShort(date), "days"),
		days.Detail(days.DetailVM{Date: date, Excluded: isExcluded, Cells: cells, ReadOnly: readOnly}))
}

func (s *Server) handleExcluded(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	season, ok := s.pageSeason(w, r)
	if !ok {
		return
	}
	all, err := s.store.ListExcludedDays(ctx, season.Period)
	if err != nil {
		s.fail(w, "excluded", err)
		return
	}
	s.render(w, r, s.seasonMeta(r, "Sperrtage", "excluded", season),
		excluded.List(excluded.ListVM{Days: all, ReadOnly: archived(season)}))
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
	s.triggerToast(w, "success", "Sperrtag angelegt.")
	s.renderExcludedList(w, r)
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
	s.triggerToast(w, "success", "Sperrtag entfernt.")
	s.renderExcludedList(w, r)
}

// renderExcludedList renders just the list region (HTMX swap target).
func (s *Server) renderExcludedList(w http.ResponseWriter, r *http.Request) {
	season, ok := s.pageSeason(w, r)
	if !ok {
		return
	}
	all, err := s.store.ListExcludedDays(r.Context(), season.Period)
	if err != nil {
		s.fail(w, "excluded", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	vm := excluded.ListVM{Days: all, ReadOnly: archived(season)}
	if err := excluded.ListRegion(vm).Render(r.Context(), w); err != nil {
		log.Printf("render excluded region: %v", err)
	}
}

// buildStrip baut die Donnerstags-Kacheln aus einer einzigen SQL-Abfrage
// (Union, Abmelde-Zahl, Limit und Sortierung passieren in der DB).
// limit == 0 => alle (Dashboard); limit > 0 => nur die jüngsten N.
func (s *Server) buildStrip(ctx context.Context, period timeutil.Period, limit, totalUsers int) ([]partials.ThursdayStripItem, error) {
	days, err := s.store.ThursdayStrip(ctx, period, limit)
	if err != nil {
		return nil, err
	}

	out := make([]partials.ThursdayStripItem, 0, len(days))
	for _, d := range days {
		if d.Excluded {
			out = append(out, partials.ThursdayStripItem{Date: d.Date, Excluded: true})
			continue
		}
		out = append(out, partials.ThursdayStripItem{
			Date: d.Date,
			Rate: partials.RateLabel(totalUsers-d.Away, totalUsers),
		})
	}
	return out, nil
}

func (s *Server) handleToggleAbsence(w http.ResponseWriter, r *http.Request) {
	userID := r.FormValue("userId")
	date, err := timeutil.ParseISO(r.FormValue("date"))
	if err != nil {
		http.Error(w, "ungültiges Datum", http.StatusUnprocessableEntity)
		return
	}
	if userID == "" {
		http.Error(w, "userId fehlt", http.StatusUnprocessableEntity)
		return
	}
	if !s.requireWritable(w, r, date) {
		return
	}

	nowAbsent, err := s.store.ToggleAbsence(r.Context(), userID, date)
	if err != nil {
		s.fail(w, "toggle absence", err)
		return
	}
	if nowAbsent {
		s.triggerToast(w, "success", "Als abgemeldet markiert.")
	} else {
		s.triggerToast(w, "success", "Als anwesend markiert.")
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Der Toggle kommt nur aus einem schreibbaren Jahr zurück (requireWritable
	// oben), also nie read-only.
	if err := partials.AbsenceToggle(userID, date, nowAbsent, false).Render(r.Context(), w); err != nil {
		log.Printf("render toggle: %v", err)
	}
}

var botExampleKinds = map[string]bool{"statistik": true, "absage": true, "zusage": true}

func (s *Server) loadExample(kind string) (string, bool) {
	if !botExampleKinds[kind] {
		return "", false
	}
	raw, err := bottest.Examples.ReadFile("examples/" + kind + ".json")
	if err != nil {
		return "", false
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err != nil {
		return string(raw), true
	}
	return pretty.String(), true
}

func (s *Server) handleBotTest(w http.ResponseWriter, r *http.Request) {
	def, _ := s.loadExample("statistik")
	s.render(w, r, s.meta("Bot-Test", "bottest"),
		bottest.Page(bottest.PageVM{DefaultKind: "statistik", DefaultJSON: def}))
}

func (s *Server) handleBotTestExample(w http.ResponseWriter, r *http.Request) {
	body, ok := s.loadExample(r.PathValue("kind"))
	if !ok {
		http.Error(w, "unbekannt", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(body))
}

// botOutcome mirrors the whatsapp-bot Outcome JSON.
type botOutcome struct {
	Path           string `json:"path"`
	Classification string `json:"classification"`
	Action         string `json:"action"`
	Message        string `json:"message"`
	Recipient      string `json:"recipient"`
	Date           string `json:"date"`
	UserID         string `json:"userId"`
	Reason         string `json:"reason"`
	DryRun         bool   `json:"dryRun"`
	PreviewTo      string `json:"previewTo"`
	ImageBase64    string `json:"imageBase64"`
}

// modeQuery übersetzt den Modus der Bot-Test-Seite in den Query-Parameter des Bots.
// Die Testseite kennt nur dryrun und preview – sie löst NIE einen echten
// Gruppen-Versand aus (der passiert nur über echte Statistik-Webhooks + CronJob).
func modeQuery(mode string) string {
	if mode == "preview" {
		return "?preview=true"
	}
	return "?dryRun=true"
}

// handleBotTestRun führt den gewählten Testlauf aus. Das Szenario bestimmt den
// Bot-Endpoint: "wochenreport" ruft /weekly-report ohne Body auf, alles andere
// schickt das Beispiel-Event an /test. Modus, Stichtag und Ausgabeformat gelten
// für beide Wege – deshalb liegt alles in einem Formular.
func (s *Server) handleBotTestRun(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	weekly := r.FormValue("szenario") == "wochenreport"
	endpoint := "/test"
	if weekly {
		endpoint = "/weekly-report"
	}
	url := strings.TrimRight(s.cfg.BotURL, "/") + endpoint + modeQuery(r.FormValue("mode"))

	// Stichtag gilt für den Statistik-Pfad wie für den Wochenreport.
	if date := r.FormValue("date"); date != "" {
		url += "&date=" + date
	}
	if r.FormValue("format") == "image" {
		url += "&format=image"
		if cs := r.FormValue("cardStyle"); cs != "" {
			url += "&cardStyle=" + cs
		}
	} else if style := r.FormValue("style"); style != "" && style != "klassik" {
		// Alternative Textdesigns kennt nur der /test-Endpoint.
		url += "&style=" + style
	}

	if weekly {
		s.proxyBot(w, r, url, nil)
		return
	}
	s.proxyBot(w, r, url, strings.NewReader(r.FormValue("payload")))
}

// proxyBot schickt eine POST-Anfrage an den Bot und rendert dessen Outcome
// (bzw. ein Fehler-Panel) als HTMX-Fragment.
func (s *Server) proxyBot(w http.ResponseWriter, r *http.Request, url string, body io.Reader) {
	client := &http.Client{Timeout: 35 * time.Second}
	req, err := http.NewRequestWithContext(r.Context(), "POST", url, body)
	if err != nil {
		_ = bottest.ErrorPanel(err.Error()).Render(r.Context(), w)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		_ = bottest.ErrorPanel("Bot nicht erreichbar: "+err.Error()).Render(r.Context(), w)
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		_ = bottest.ErrorPanel("Bot-Status "+resp.Status+": "+string(respBody)).Render(r.Context(), w)
		return
	}
	var out botOutcome
	if err := json.Unmarshal(respBody, &out); err != nil {
		_ = bottest.ErrorPanel("Antwort nicht lesbar: "+err.Error()).Render(r.Context(), w)
		return
	}
	_ = bottest.Response(bottest.ResponseVM{
		Path: out.Path, Classification: out.Classification, Action: out.Action,
		Message: out.Message, Recipient: out.Recipient, Date: out.Date, UserID: out.UserID,
		DryRun: out.DryRun, PreviewTo: out.PreviewTo, ImageBase64: out.ImageBase64,
	}).Render(r.Context(), w)
}

func (s *Server) fail(w http.ResponseWriter, what string, err error) {
	log.Printf("%s: %v", what, err)
	http.Error(w, "interner Fehler", http.StatusInternalServerError)
}

func (s *Server) triggerToast(w http.ResponseWriter, level, msg string) {
	// JSON object form of HX-Trigger so the client receives event detail.
	payload := fmt.Sprintf(`{"showToast":{"level":%q,"msg":%q}}`, level, msg)
	w.Header().Set("HX-Trigger", payload)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
