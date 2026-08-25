package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/michael/zumba-admin-ui/internal/config"
)

const noClassifierHint = "Kein classifier-service konfiguriert"

// Mit echter DB (mockMode = false) und ohne CLASSIFIER_URL meldet /ml-test,
// dass kein Klassifikator konfiguriert ist. Genau dieser Fall tritt bei einem
// alleinstehenden `make dev` auf – ./dev-local.sh setzt die Variable selbst.
func TestMLTestOhneClassifierURLZeigtHinweis(t *testing.T) {
	rec := getMLTest(t, config.Config{})
	if !strings.Contains(rec.Body.String(), noClassifierHint) {
		t.Errorf("Hinweis %q fehlt", noClassifierHint)
	}
}

func TestMLTestMitClassifierURLZeigtFormular(t *testing.T) {
	rec := getMLTest(t, config.Config{ClassifierURL: "http://localhost:8085"})
	if strings.Contains(rec.Body.String(), noClassifierHint) {
		t.Error("Hinweis darf mit gesetzter CLASSIFIER_URL nicht erscheinen")
	}
}

func getMLTest(t *testing.T, cfg config.Config) *httptest.ResponseRecorder {
	t.Helper()
	srv := New(newSpyStore(), cfg, false) // mockMode = false: wie im Betrieb
	req := httptest.NewRequest(http.MethodGet, "/ml-test", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	return rec
}
