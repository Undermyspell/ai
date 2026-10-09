package web

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/a-h/templ"

	"github.com/michael/zumba-shared/domain"

	"github.com/michael/zumba-admin-ui/assets"
	"github.com/michael/zumba-admin-ui/internal/config"
	"github.com/michael/zumba-admin-ui/internal/store"
	"github.com/michael/zumba-admin-ui/internal/timeutil"
	"github.com/michael/zumba-admin-ui/internal/tunnel"
	"github.com/michael/zumba-admin-ui/web/templates"
)

type Server struct {
	store    store.Store
	cfg      config.Config
	mockMode bool

	// tunnel steuert die öffentlichen ngrok-Tunnel. nil = TUNNEL_URL nicht
	// gesetzt, dann gibt es die Öffentlich-Seite nur als Hinweis.
	tunnel tunnelClient
	gate   publicGate

	// loginThrottle bremst Fehlversuche am Login — wichtig, sobald die Seite
	// über einen Tunnel öffentlich erreichbar ist.
	loginThrottle loginThrottle

	// notify bremst den WhatsApp-Versand der Tunnel-Adresse.
	notify notifyThrottle

	// ephemeralKey signiert Session-Cookies, wenn kein SESSION_SECRET gesetzt
	// ist. Er lebt nur so lange wie der Prozess.
	ephemeralKey []byte
}

func New(s store.Store, cfg config.Config, mockMode bool) *Server {
	srv := &Server{store: s, cfg: cfg, mockMode: mockMode, ephemeralKey: randomKey()}
	if cfg.TunnelURL != "" {
		srv.tunnel = tunnel.New(cfg.TunnelURL)
	}
	return srv
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

	mux.HandleFunc("GET /login", s.handleLoginPage)
	mux.HandleFunc("POST /login", s.handleLoginSubmit)
	mux.HandleFunc("POST /logout", s.handleLogout)

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
	// Bot- und ML-Test bleiben zu, solange etwas öffentlich hängt: die
	// Bot-Test-Seite kann im Preview-Modus eine echte WhatsApp-Nachricht
	// auslösen (siehe blockWhilePublic).
	sealed := func(h http.HandlerFunc) http.HandlerFunc {
		return s.blockWhilePublic(h).ServeHTTP
	}
	mux.HandleFunc("GET /bot-test", sealed(s.handleBotTest))
	mux.HandleFunc("GET /bot-test/example/{kind}", sealed(s.handleBotTestExample))
	mux.HandleFunc("POST /bot-test/run", sealed(s.handleBotTestRun))
	mux.HandleFunc("GET /trace", s.handleTraceList)
	mux.HandleFunc("GET /trace/{id}", s.handleTraceDetail)
	mux.HandleFunc("GET /ml-shadow", s.handleMLShadow)
	mux.HandleFunc("POST /ml-shadow/verify/{id}", s.handleMLVerify)
	mux.HandleFunc("GET /ml-test", sealed(s.handleMLTest))
	mux.HandleFunc("POST /ml-test/run", sealed(s.handleMLTestRun))
	mux.HandleFunc("POST /ml-test/judge/{id}", sealed(s.handleMLTestJudge))
	mux.HandleFunc("DELETE /ml-test/{id}", sealed(s.handleMLTestDelete))
	mux.HandleFunc("GET /ml-doku", s.handleMLDocs)
	// KI-Modell des Bots. Nicht gesperrt bei offenem Tunnel: Umschalten
	// verschickt nichts, es ändert nur die nächste Klassifizierung.
	mux.HandleFunc("GET /ki-modell", s.handleKIModell)
	mux.HandleFunc("POST /ki-modell", s.handleKIModellSet)
	// Bild-Designs des Bots: Rotation und Karte des nächsten Wochenreports.
	// Wie das KI-Modell nicht gesperrt – Speichern verschickt nichts.
	mux.HandleFunc("GET /bild-designs", s.handleBildDesigns)
	mux.HandleFunc("POST /bild-designs/rotation", s.handleBildDesignsRotation)
	mux.HandleFunc("POST /bild-designs/naechster", s.handleBildDesignsNaechster)

	// Öffentlich-Seite. Schalten geht nur, wenn es einen tunnel-service gibt —
	// ohne ihn zeigt die Seite bloß den Hinweis.
	mux.HandleFunc("GET /public", s.handlePublic)
	if s.tunnel != nil {
		mux.HandleFunc("GET /public/status", s.handlePublicStatus)
		mux.HandleFunc("POST /public/open", s.handlePublicOpen)
		mux.HandleFunc("POST /public/close", s.handlePublicClose)
		mux.HandleFunc("POST /public/close-all", s.handlePublicCloseAll)
		mux.HandleFunc("POST /public/notify", s.handlePublicNotify)
	}

	return logRequests(s.requireLogin(mux))
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
	// PublicOpen kommt aus dem zwischengespeicherten Stand — für das Ausblenden
	// der gesperrten Menüpunkte reicht das, und es spart pro Seitenaufruf eine
	// Anfrage an den tunnel-service.
	open, _ := s.gate.get()
	return templates.PageMeta{
		Title:       title,
		ActiveNav:   active,
		MockMode:    s.mockMode,
		AuthEnabled: s.cfg.Auth.Enabled(),
		PublicOpen:  open,
		Next:        s.nextStammtisch(),
	}
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
	// Keine HTML-Antwort: „absenceChanged" lässt jede Ansicht, die an
	// Anwesenheiten hängt (#page, das Bottom-Sheet), sich selbst neu laden.
	if nowAbsent {
		s.triggerToast(w, "success", "Als abgemeldet markiert.", "absenceChanged")
	} else {
		s.triggerToast(w, "success", "Als anwesend markiert.", "absenceChanged")
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) fail(w http.ResponseWriter, what string, err error) {
	log.Printf("%s: %v", what, err)
	http.Error(w, "interner Fehler", http.StatusInternalServerError)
}

// triggerToast zeigt im Browser einen Toast; events sind weitere
// HTMX-Ereignisse, die mit derselben Antwort ausgelöst werden.
func (s *Server) triggerToast(w http.ResponseWriter, level, msg string, events ...string) {
	// JSON object form of HX-Trigger so the client receives event detail.
	// Header-Werte liest der Browser als Latin-1 – Umlaute gehen deshalb als
	// \uXXXX-Escapes raus, sonst steht im Toast "kÃ¶nnen".
	payload := fmt.Sprintf(`{"showToast":{"level":%s,"msg":%s}`, jsonASCII(level), jsonASCII(msg))
	for _, ev := range events {
		payload += fmt.Sprintf(`,%s:true`, jsonASCII(ev))
	}
	w.Header().Set("HX-Trigger", payload+"}")
}

// jsonASCII kodiert s als JSON-String, der nur aus ASCII besteht.
func jsonASCII(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 0x20 || r >= 0x7f:
			for _, u := range utf16.Encode([]rune{r}) {
				fmt.Fprintf(&b, `\u%04x`, u)
			}
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
