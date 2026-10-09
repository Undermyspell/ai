package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sharedstore "github.com/michael/zumba-shared/store"
	"github.com/michael/zumba-whatsapp-bot/internal/classifier"
	"github.com/michael/zumba-whatsapp-bot/internal/report"
)

type fakeCardSettings struct {
	cs  sharedstore.CardSettings
	err error

	advanced []string // "2026-08-06=zeitung(fest)" je AdvanceCardQueue
}

func (f *fakeCardSettings) CardSettings(context.Context) (sharedstore.CardSettings, error) {
	return f.cs, f.err
}

func (f *fakeCardSettings) AdvanceCardQueue(_ context.Context, tag time.Time, style string, fest bool) error {
	e := tag.Format("2006-01-02") + "=" + style
	if fest {
		e += "(fest)"
	}
	f.advanced = append(f.advanced, e)
	return nil
}

func runWeekly(t *testing.T, s *Server, query string) {
	t.Helper()
	req := httptest.NewRequest("POST", "/weekly-report?format=image"+query, nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d: %s", rec.Code, rec.Body.String())
	}
}

func cardTestServer(t *testing.T, now time.Time) (*Server, *fakeRenderer) {
	t.Helper()
	s, st, _ := newTestServer(classifier.Invalid, now)
	st.penaltyInput = penaltyFixture()
	rnd := &fakeRenderer{}
	s.Renderer = rnd
	return s, rnd
}

// Mit gespeicherter Warteschlange kommt deren oberstes Design – nicht die
// Datumsrechnung über CARD_STYLES.
func TestWochenreportNimmtOberstesDerSchlange(t *testing.T) {
	s, rnd := cardTestServer(t, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
	s.Cards = report.NewCardRotation([]string{"formular"})
	s.CardSettings = &fakeCardSettings{cs: sharedstore.CardSettings{RotationGesetzt: true, Rotation: []string{"zeitung", "arena"}}}

	runWeekly(t, s, "&date=2026-08-06")
	if !strings.Contains(rnd.html, marker["zeitung"]) {
		t.Error("nicht das oberste Design der Schlange")
	}
}

// Im Admin-UI für genau diesen Donnerstag gewählt → dieses Design.
func TestWochenreportNimmtEinmalAuswahl(t *testing.T) {
	s, rnd := cardTestServer(t, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
	s.CardSettings = &fakeCardSettings{cs: sharedstore.CardSettings{
		RotationGesetzt: true,
		Rotation:        []string{"arena"},
		Naechster:       &sharedstore.NextCard{Tag: time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC), Style: "zeitung"},
	}}

	runWeekly(t, s, "&date=2026-08-06")
	if !strings.Contains(rnd.html, marker["zeitung"]) {
		t.Error("Einmal-Auswahl für den Donnerstag übergangen")
	}
	runWeekly(t, s, "&date=2026-08-13")
	if !strings.Contains(rnd.html, marker["arena"]) {
		t.Error("eine Woche später nicht zurück zur Schlange")
	}
}

// Nur der echte Versand rückt die Schlange weiter – Dry-Run nicht.
func TestWochenreportRuecktSchlangeNachVersand(t *testing.T) {
	s, _ := cardTestServer(t, time.Date(2026, 8, 6, 19, 0, 0, 0, time.UTC))
	src := &fakeCardSettings{cs: sharedstore.CardSettings{RotationGesetzt: true, Rotation: []string{"zeitung", "arena"}}}
	s.CardSettings = src

	runWeekly(t, s, "&dryRun=true")
	if len(src.advanced) != 0 {
		t.Fatalf("Dry-Run hat weitergerückt: %v", src.advanced)
	}
	runWeekly(t, s, "")
	if len(src.advanced) != 1 || src.advanced[0] != "2026-08-06=zeitung" {
		t.Errorf("nach echtem Versand: %v", src.advanced)
	}

	// Einmal-Auswahl: festgehalten, aber als fest (Schlange bleibt stehen).
	src.advanced = nil
	src.cs.Naechster = &sharedstore.NextCard{Tag: time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC), Style: "arena"}
	runWeekly(t, s, "")
	if len(src.advanced) != 1 || src.advanced[0] != "2026-08-06=arena(fest)" {
		t.Errorf("Einmal-Auswahl: %v", src.advanced)
	}
}

// Ein Wiederholungslauf am selben Tag schickt das schon gesendete Design.
func TestWochenreportWiederholungNimmtGesendetesDesign(t *testing.T) {
	s, rnd := cardTestServer(t, time.Date(2026, 8, 6, 19, 0, 0, 0, time.UTC))
	s.CardSettings = &fakeCardSettings{cs: sharedstore.CardSettings{
		RotationGesetzt: true,
		Rotation:        []string{"arena", "zeitung"}, // schon weitergerückt
		Zuletzt:         &sharedstore.NextCard{Tag: time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC), Style: "zeitung"},
	}}
	runWeekly(t, s, "&dryRun=true")
	if !strings.Contains(rnd.html, marker["zeitung"]) {
		t.Error("Wiederholung am selben Tag mit anderem Design")
	}
}

// Ohne gespeicherte Schlange oder bei DB-Fehlern gilt CARD_STYLES.
func TestWochenreportOhneSchlangeNimmtCardStyles(t *testing.T) {
	s, rnd := cardTestServer(t, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
	s.Cards = report.NewCardRotation([]string{"formular"})

	for name, src := range map[string]*fakeCardSettings{
		"ohne Zeile":             {},
		"Rotation nicht gesetzt": {cs: sharedstore.CardSettings{Naechster: &sharedstore.NextCard{Tag: time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC), Style: "zeitung"}}},
		"DB-Fehler":              {err: errors.New("weg")},
	} {
		s.CardSettings = src
		runWeekly(t, s, "&date=2026-08-06")
		if !strings.Contains(rnd.html, marker["formular"]) {
			t.Errorf("%s: CARD_STYLES nicht als Fallback genommen", name)
		}
	}
}
