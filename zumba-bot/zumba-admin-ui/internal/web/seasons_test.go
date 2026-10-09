package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/michael/zumba-shared/domain"

	"github.com/michael/zumba-admin-ui/internal/store"
	"github.com/michael/zumba-admin-ui/internal/timeutil"
)

func mkSeason(label string, start, end time.Time) store.Season {
	s := store.Season{Label: label}
	s.Start, s.End = start, end
	return s
}

// seasonsAround: ein laufendes Jahr und das kommende, relativ zu heute –
// die Regeln hängen am heutigen Datum.
func seasonsAround(today time.Time) []store.Season {
	curStart := today.AddDate(0, 0, -100)
	nextStart := today.AddDate(0, 0, 50)
	return []store.Season{
		mkSeason("2027", nextStart, nextStart.AddDate(1, 0, -1)),
		mkSeason("2026", curStart, nextStart.AddDate(0, 0, -1)),
	}
}

func TestCheckSeasonStart(t *testing.T) {
	today := domain.DateOnly(time.Now())
	seasons := seasonsAround(today)
	cur, next := seasons[1], seasons[0]

	cases := []struct {
		name  string
		label string
		start time.Time
		want  string // Teil der Meldung, "" = erlaubt
	}{
		{"später", "2027", next.Start.AddDate(0, 0, 9), ""},
		{"früher", "2027", next.Start.AddDate(0, 0, -20), ""},
		{"laufendes Jahr", "2026", cur.Start.AddDate(0, 0, 5), "hat schon begonnen"},
		{"heute", "2027", today, "in der Zukunft"},
		{"nach dem Ende", "2027", next.End, "vor dem Jahresende"},
		{"vor dem Vorjahr", "2027", cur.Start.AddDate(0, 0, 1), "in der Zukunft"},
		{"unbekannt", "1999", next.Start, "gibt es nicht"},
	}
	for _, c := range cases {
		got := checkSeasonStart(seasons, c.label, c.start, today)
		if c.want == "" && got != "" || c.want != "" && !strings.Contains(got, c.want) {
			t.Errorf("%s: Meldung %q, erwartet %q", c.name, got, c.want)
		}
	}

	// Das Vorjahr muss mindestens einen Tag behalten – auch wenn sein
	// Beginn in der Zukunft liegt.
	future := []store.Season{
		mkSeason("2028", today.AddDate(0, 0, 40), today.AddDate(1, 0, 40)),
		mkSeason("2027", today.AddDate(0, 0, 10), today.AddDate(0, 0, 39)),
	}
	if got := checkSeasonStart(future, "2028", today.AddDate(0, 0, 11), today); !strings.Contains(got, "nach dem Beginn von 2027") {
		t.Errorf("Vorjahr ohne Tag: Meldung %q", got)
	}
}

func TestNextSeason(t *testing.T) {
	today := domain.DateOnly(time.Now())

	// Das kommende Jahr steht schon – kein weiteres im Voraus.
	if _, why, ok := nextSeason(seasonsAround(today), today); ok || !strings.Contains(why, "2027 ist schon angelegt") {
		t.Errorf("mit künftigem Jahr: ok=%v, why=%q", ok, why)
	}

	// Läuft das letzte Jahr, schließt das nächste lückenlos an.
	running := []store.Season{mkSeason("2027", today.AddDate(0, 0, -5), mustDate("2099-11-30"))}
	next, _, ok := nextSeason(running, today)
	if !ok || next.Label != "2028" || !next.Start.Equal(mustDate("2099-12-01")) || !next.End.Equal(mustDate("2100-11-30")) {
		t.Errorf("nächstes Jahr = %s %s–%s (ok=%v)", next.Label, timeutil.FormatISO(next.Start), timeutil.FormatISO(next.End), ok)
	}
}

func postSeasons(t *testing.T, spy *spyStore, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	srv := New(spy, testCfg(), false)
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	return rec
}

func TestSeasonStartHandler(t *testing.T) {
	today := domain.DateOnly(time.Now())
	spy := newSpyStore()
	spy.seasons = seasonsAround(today)
	start := spy.seasons[0].Start.AddDate(0, 0, 9)

	rec := postSeasons(t, spy, "/stammtischjahre/2027/beginn", url.Values{"start": {timeutil.FormatISO(start)}})
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if want := "2027=" + timeutil.FormatISO(start); spy.movedSeason != want {
		t.Errorf("MoveSeasonStart = %q, want %q", spy.movedSeason, want)
	}
	if !strings.Contains(rec.Header().Get("HX-Trigger"), "success") {
		t.Errorf("HX-Trigger = %q, want success-Toast", rec.Header().Get("HX-Trigger"))
	}

	// Das laufende Jahr bleibt, wie es ist.
	spy.movedSeason = ""
	rec = postSeasons(t, spy, "/stammtischjahre/2026/beginn", url.Values{"start": {timeutil.FormatISO(today.AddDate(0, 0, 3))}})
	if spy.movedSeason != "" {
		t.Errorf("laufendes Jahr verschoben: %q", spy.movedSeason)
	}
	if !strings.Contains(rec.Header().Get("HX-Trigger"), "error") {
		t.Errorf("HX-Trigger = %q, want error-Toast", rec.Header().Get("HX-Trigger"))
	}
}

func TestSeasonAddHandler(t *testing.T) {
	today := domain.DateOnly(time.Now())

	spy := newSpyStore()
	spy.seasons = seasonsAround(today)
	postSeasons(t, spy, "/stammtischjahre", nil)
	if spy.addedSeason != "" {
		t.Errorf("trotz künftigem Jahr angelegt: %q", spy.addedSeason)
	}

	spy.seasons = []store.Season{mkSeason("2027", today.AddDate(0, 0, -5), mustDate("2099-11-30"))}
	rec := postSeasons(t, spy, "/stammtischjahre", nil)
	if want := "2028=2099-12-01..2100-11-30"; spy.addedSeason != want {
		t.Errorf("AddSeason = %q, want %q", spy.addedSeason, want)
	}
	if !strings.Contains(rec.Header().Get("HX-Trigger"), "success") {
		t.Errorf("HX-Trigger = %q, want success-Toast", rec.Header().Get("HX-Trigger"))
	}
}

func TestSeasonsPage(t *testing.T) {
	today := domain.DateOnly(time.Now())
	spy := newSpyStore()
	spy.seasons = seasonsAround(today)
	srv := New(spy, testCfg(), false)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, httptest.NewRequest("GET", "/stammtischjahre", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"läuft", "kommt", `hx-post="/stammtischjahre/2027/beginn"`, "2027 ist schon angelegt"} {
		if !strings.Contains(body, want) {
			t.Errorf("Seite enthält %q nicht", want)
		}
	}
	if strings.Contains(body, `hx-post="/stammtischjahre/2026/beginn"`) {
		t.Errorf("laufendes Jahr hat ein Datumsfeld")
	}
}
