package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/michael/zumba-admin-ui/internal/config"
)

func postEntscheider(t *testing.T, spy *spyStore, wer string) *httptest.ResponseRecorder {
	t.Helper()
	srv := New(spy, config.Config{}, true) // Mock-Modus: Beispiel-Infos statt classifier-service
	req := httptest.NewRequest(http.MethodPost, "/ki-modell/entscheider",
		strings.NewReader(url.Values{"wer": {wer}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	return rec
}

// Standard ist Gemini; die Seite zeigt den Schalter und das eigene Modell.
func TestKIModellZeigtEntscheiderUndModell(t *testing.T) {
	srv := New(newSpyStore(), config.Config{}, true)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ki-modell", nil))
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	for _, want := range []string{"Eigenes Modell", "NB-LogReg 0.2.0", "läuft mit", "Kopf", "logistische Regression"} {
		if !strings.Contains(body, want) {
			t.Errorf("%q fehlt", want)
		}
	}
	if !strings.Contains(body, `class="is-active" aria-pressed="true" hx-post="/ki-modell/entscheider" hx-vals="{&#34;wer&#34;:&#34;gemini&#34;}"`) {
		t.Error("Gemini ist nicht als aktiv markiert")
	}
}

func TestKIModellEntscheiderUmstellen(t *testing.T) {
	spy := newSpyStore()
	rec := postEntscheider(t, spy, "modell")
	if rec.Code != http.StatusOK || spy.setEntscheider != "modell" {
		t.Fatalf("code %d, gespeichert %q", rec.Code, spy.setEntscheider)
	}
	if !strings.Contains(rec.Body.String(), "entscheidet") {
		t.Error("Karte zeigt nicht, dass das eigene Modell entscheidet")
	}
}

func TestKIModellEntscheiderUnbekannt(t *testing.T) {
	spy := newSpyStore()
	if rec := postEntscheider(t, spy, "würfel"); rec.Code != http.StatusUnprocessableEntity || spy.setEntscheider != "" {
		t.Errorf("code %d, gespeichert %q – want 422, nichts", rec.Code, spy.setEntscheider)
	}
}

// Ohne classifier-service (echter Betrieb, keine URL) sagt die Karte das.
func TestKIModellOhneClassifier(t *testing.T) {
	srv := New(newSpyStore(), config.Config{}, false)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ki-modell", nil))
	if !strings.Contains(rec.Body.String(), "kein classifier-service konfiguriert") {
		t.Error("Hinweis auf fehlenden classifier-service fehlt")
	}
}
