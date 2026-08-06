package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/michael/zumba-whatsapp-bot/internal/classifier"
	"github.com/michael/zumba-shared/penalty"
)

// penaltyFixture: ein User mit 5 Fehltagen in Folge bis zum Test-Donnerstag.
func penaltyFixture() penalty.Input {
	start := time.Date(2025, 12, 4, 0, 0, 0, 0, time.UTC) // Donnerstag
	var absences []time.Time
	for i := 0; i < 5; i++ {
		absences = append(absences, start.AddDate(0, 0, 7*i))
	}
	return penalty.Input{Users: []penalty.UserData{{
		UserID: "user-123", Name: "Tester",
		EffectiveStart: start, Absences: absences,
	}}}
}

func TestStatistikEnthaeltStrafenblock(t *testing.T) {
	s, st, snd := newTestServer(classifier.Invalid, time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	st.penaltyInput = penaltyFixture()
	s.run(context.Background(), groupMsg("statistik"), false, false, s.today())
	if !strings.Contains(snd.text, "── 💸 *STRAFEN* ──") {
		t.Fatalf("Strafenblock fehlt:\n%s", snd.text)
	}
	if !strings.Contains(snd.text, "⚠️ *Tester* – 25€ (5x in Folge gefehlt)") {
		t.Errorf("Strafenzeile fehlt:\n%s", snd.text)
	}
	// Echter Lauf → Marker persistiert.
	if len(st.autoStrafen) != 1 || st.autoStrafen[0] != "user-123|2025-12-04" {
		t.Errorf("InsertAutoStrafe erwartet, got %v", st.autoStrafen)
	}
}

func TestWeeklyDryRunPersistiertNicht(t *testing.T) {
	s, st, snd := newTestServer(classifier.Invalid, time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	st.penaltyInput = penaltyFixture()

	req := httptest.NewRequest("POST", "/weekly-report?dryRun=true", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	if len(st.autoStrafen) != 0 {
		t.Errorf("Dry-Run darf nicht persistieren: %v", st.autoStrafen)
	}
	if snd.called {
		t.Error("Dry-Run darf nicht senden")
	}
	if !strings.Contains(rec.Body.String(), "STRAFEN") {
		t.Error("Strafenblock fehlt im Dry-Run-Text")
	}
}

func TestWeeklyEchtPersistiert(t *testing.T) {
	s, st, snd := newTestServer(classifier.Invalid, time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	st.penaltyInput = penaltyFixture()

	req := httptest.NewRequest("POST", "/weekly-report", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	if len(st.autoStrafen) != 1 {
		t.Errorf("echter Lauf muss Marker persistieren: %v", st.autoStrafen)
	}
	if !snd.called || snd.number != testGroup {
		t.Errorf("Versand an Gruppe erwartet: %+v", snd)
	}
}

// Simulierter Stichtag: date-Param erzwingt Dry-Run und verschiebt das
// Sichtbarkeits-Fenster des Strafenblocks.
func TestWeeklyMitDatumErzwingtDryRun(t *testing.T) {
	s, st, snd := newTestServer(classifier.Invalid, time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	st.penaltyInput = penaltyFixture()

	req := httptest.NewRequest("POST", "/weekly-report?date=2026-01-01", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	if snd.called {
		t.Error("date-Param ohne preview darf nie senden")
	}
	if len(st.autoStrafen) != 0 {
		t.Errorf("simulierter Lauf darf nicht persistieren: %v", st.autoStrafen)
	}

	bad := httptest.NewRequest("POST", "/weekly-report?date=quatsch", nil)
	badRec := httptest.NewRecorder()
	s.Routes().ServeHTTP(badRec, bad)
	if badRec.Code != http.StatusBadRequest {
		t.Errorf("ungültiges Datum: code = %d, want 400", badRec.Code)
	}
}
