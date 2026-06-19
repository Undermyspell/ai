package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/michael/zumba-admin-ui/internal/config"
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

func testCfg() config.Config {
	return config.Config{
		EvalPeriodStart: mustDate("2025-12-01"),
		EvalPeriodEnd:   mustDate("2026-11-30"),
	}
}
