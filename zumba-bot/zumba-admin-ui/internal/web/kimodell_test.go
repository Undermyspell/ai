package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	sharedstore "github.com/michael/zumba-shared/store"
)

func postKIModell(t *testing.T, spy *spyStore, rolle, model string) *httptest.ResponseRecorder {
	t.Helper()
	srv := New(spy, testCfg(), false)
	form := url.Values{"rolle": {rolle}, "model": {model}}
	req := httptest.NewRequest("POST", "/ki-modell", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	return rec
}

func spyWithModels(primary, fallback string) *spyStore {
	spy := newSpyStore()
	spy.classifier.Primary = sharedstore.ClassifierModel{Model: primary}
	spy.classifier.Fallback = sharedstore.ClassifierModel{Model: fallback}
	return spy
}

func TestKIModellPageShowsSelection(t *testing.T) {
	spy := spyWithModels("gemma-4-31b-it", "gemini-3.8-flash")
	srv := New(spy, testCfg(), false)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, httptest.NewRequest("GET", "/ki-modell", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Hauptmodell", "Fallback", "Gemini 3.5 Flash Lite", `data-model="gemma-4-31b-it" data-role="primary"`, `data-model="gemini-3.8-flash" data-role="fallback"`} {
		if !strings.Contains(body, want) {
			t.Errorf("Seite enthält %q nicht", want)
		}
	}
}

func TestKIModellSwitch(t *testing.T) {
	spy := spyWithModels("gemma-4-31b-it", "gemini-3.8-flash")
	rec := postKIModell(t, spy, "primary", "gemini-3.5-flash-lite")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if spy.setModel != "primary=gemini-3.5-flash-lite" {
		t.Errorf("SetClassifierModel = %q", spy.setModel)
	}
	if !strings.Contains(rec.Header().Get("HX-Trigger"), "success") {
		t.Errorf("HX-Trigger = %q, want success-Toast", rec.Header().Get("HX-Trigger"))
	}
}

// Haupt und Fallback gleich hieße: kein Fallback. Die Region kommt trotzdem
// neu (200), damit die Auswahl wieder den gespeicherten Stand zeigt.
func TestKIModellRejectsSameAsOtherRole(t *testing.T) {
	spy := spyWithModels("gemma-4-31b-it", "gemini-3.8-flash")
	rec := postKIModell(t, spy, "fallback", "gemma-4-31b-it")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if spy.setModel != "" {
		t.Errorf("SetClassifierModel darf nicht laufen, got %q", spy.setModel)
	}
	if !strings.Contains(rec.Header().Get("HX-Trigger"), "error") {
		t.Errorf("HX-Trigger = %q, want error-Toast", rec.Header().Get("HX-Trigger"))
	}
}

func TestKIModellRejectsUnknown(t *testing.T) {
	cases := []struct{ rolle, model string }{
		{"primary", "gemini-2.5-flash"},
		{"tertiary", "gemma-4-31b-it"},
	}
	for _, tc := range cases {
		spy := spyWithModels("gemma-4-31b-it", "gemini-3.8-flash")
		rec := postKIModell(t, spy, tc.rolle, tc.model)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s=%s: code = %d, want 422", tc.rolle, tc.model, rec.Code)
		}
		if spy.setModel != "" {
			t.Errorf("%s=%s: SetClassifierModel darf nicht laufen", tc.rolle, tc.model)
		}
	}
}
