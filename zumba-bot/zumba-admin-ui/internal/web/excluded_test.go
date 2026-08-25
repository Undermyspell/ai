package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/michael/zumba-admin-ui/internal/config"
	"github.com/michael/zumba-admin-ui/internal/store"
)

func TestPostExcludedThursday(t *testing.T) {
	spy := newSpyStore()
	srv := New(spy, testCfg(), false)
	form := url.Values{"date": {"2026-01-01"}} // Thursday
	req := httptest.NewRequest("POST", "/excluded", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if spy.insertedExcluded != "2026-01-01" {
		t.Errorf("InsertExcludedDay not called with Thursday, got %q", spy.insertedExcluded)
	}
}

func TestPostExcludedRejectsNonThursday(t *testing.T) {
	spy := newSpyStore()
	srv := New(spy, testCfg(), false)
	form := url.Values{"date": {"2026-01-02"}} // Friday
	req := httptest.NewRequest("POST", "/excluded", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code = %d, want 422", rec.Code)
	}
	if spy.insertedExcluded != "" {
		t.Error("InsertExcludedDay must not be called for non-Thursday")
	}
}

func TestDeleteExcluded(t *testing.T) {
	spy := newSpyStore()
	srv := New(spy, testCfg(), false)
	req := httptest.NewRequest("DELETE", "/excluded/2026-01-01", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if spy.deletedExcluded != "2026-01-01" {
		t.Errorf("DeleteExcludedDay not called, got %q", spy.deletedExcluded)
	}
}

func testCfg() config.Config { return config.Config{} }

// Schreibzugriffe in ein abgeschlossenes Stammtischjahr müssen abgelehnt
// werden – auch ohne ?jahr= am Request, denn der Guard hängt am Datum.
func TestPostExcludedRejectsArchivedSeason(t *testing.T) {
	spy := newSpyStore()
	spy.seasons = []store.Season{archivedSeason}
	srv := New(spy, testCfg(), false)
	form := url.Values{"date": {"2019-01-03"}} // Donnerstag im abgelaufenen Jahr
	req := httptest.NewRequest("POST", "/excluded", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("code = %d, want 409", rec.Code)
	}
	if spy.insertedExcluded != "" {
		t.Error("InsertExcludedDay darf im Archiv nicht laufen")
	}
}

// Ohne gepflegtes Jahr gibt es keinen stillen Fallback.
func TestToggleAbsenceRejectsMissingSeason(t *testing.T) {
	spy := newSpyStore()
	spy.seasons = nil
	srv := New(spy, testCfg(), false)
	form := url.Values{"userId": {"u01"}, "date": {"2026-01-01"}}
	req := httptest.NewRequest("POST", "/toggle-absence", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code = %d, want 422", rec.Code)
	}
}
