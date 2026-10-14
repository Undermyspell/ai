package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/michael/zumba-admin-ui/internal/config"
)

func serveML(t *testing.T, spy *spyStore, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	srv := New(spy, config.Config{}, false)
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	return rec
}

// Die Seite zeigt alle drei Quellen als Reiter; ohne Parameter Whatsapp,
// gefiltert auf „offen".
func TestMLDatenZeigtDreiQuellen(t *testing.T) {
	spy := newSpyStore()
	rec := serveML(t, spy, http.MethodGet, "/ml-daten")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, tab := range []string{"Synthetics", "Whatsapp", "Manuell"} {
		if !strings.Contains(body, tab) {
			t.Errorf("Reiter %q fehlt", tab)
		}
	}
	if spy.mlFilter != "whatsapp:offen:" {
		t.Errorf("Filter = %q, want whatsapp:offen:", spy.mlFilter)
	}
}

// Jede Quelle hat ihren eigenen Menüpunkt; der der offenen Seite ist aktiv.
func TestMLDatenMenuepunkte(t *testing.T) {
	rec := serveML(t, newSpyStore(), http.MethodGet, "/ml-daten?quelle=manuell")
	body := rec.Body.String()
	for _, q := range []string{"synthetics", "whatsapp", "manuell"} {
		if !strings.Contains(body, `href="/ml-daten?quelle=`+q+`"`) {
			t.Errorf("Menüpunkt für %s fehlt", q)
		}
	}
	active := `class="nav-link is-active" aria-current="page"><span class="nav-ico" aria-hidden="true">✎</span>`
	if !strings.Contains(body, active) {
		t.Error("Menüpunkt Manuell ist nicht aktiv")
	}
	if !strings.Contains(body, "Prüffälle labeln") {
		t.Error("Überschrift des Menüpunkts fehlt")
	}
}

func TestMLDatenFilterAusQuery(t *testing.T) {
	cases := map[string]string{
		"/ml-daten?quelle=synthetics&status=alle&label=true": "synthetics:alle:true",
		"/ml-daten?quelle=whatsapp&status=abweichung":        "whatsapp:abweichung:",
		// Gemini ≠ Modell gibt es nur bei Whatsapp, Unbekanntes fällt auf offen.
		"/ml-daten?quelle=manuell&status=abweichung": "manuell:offen:",
		"/ml-daten?quelle=quatsch&label=vielleicht":  "whatsapp:offen:",
	}
	for target, want := range cases {
		spy := newSpyStore()
		if rec := serveML(t, spy, http.MethodGet, target); rec.Code != http.StatusOK {
			t.Fatalf("%s: code = %d", target, rec.Code)
		}
		if spy.mlFilter != want {
			t.Errorf("%s: Filter = %q, want %q", target, spy.mlFilter, want)
		}
	}
}

// Ein Label lässt sich jederzeit setzen und ändern; die Antwort ist die neue
// Zeile und stößt das Nachladen des Kopfs an.
func TestMLLabelSetzt(t *testing.T) {
	for _, q := range []string{"synthetics", "whatsapp", "manuell"} {
		spy := newSpyStore()
		rec := serveML(t, spy, http.MethodPost, "/ml-daten/"+q+"/7/label?label=invalid")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: code = %d, want 200", q, rec.Code)
		}
		if want := q + "/7=invalid"; spy.mlLabeled != want {
			t.Errorf("%s: gelabelt %q, want %q", q, spy.mlLabeled, want)
		}
		if got := rec.Header().Get("HX-Trigger"); got != "mlLabeled" {
			t.Errorf("%s: HX-Trigger = %q, want mlLabeled", q, got)
		}
		if !strings.Contains(rec.Body.String(), `id="ml-`+q+`-7"`) {
			t.Errorf("%s: Zeile fehlt in der Antwort", q)
		}
	}
}

func TestMLLabelVerwerfenNurSynthetics(t *testing.T) {
	spy := newSpyStore()
	if rec := serveML(t, spy, http.MethodPost, "/ml-daten/synthetics/3/label?label=verworfen"); rec.Code != http.StatusOK {
		t.Fatalf("synthetics: code = %d, want 200", rec.Code)
	}
	if spy.mlLabeled != "synthetics/3=verworfen" {
		t.Errorf("gelabelt %q", spy.mlLabeled)
	}

	// Echte Nachrichten und Prüffälle lassen sich nicht verwerfen.
	for _, q := range []string{"whatsapp", "manuell"} {
		spy := newSpyStore()
		rec := serveML(t, spy, http.MethodPost, "/ml-daten/"+q+"/3/label?label=verworfen")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: code = %d, want 400", q, rec.Code)
		}
		if spy.mlLabeled != "" {
			t.Errorf("%s: trotzdem gespeichert: %q", q, spy.mlLabeled)
		}
	}
}

func TestMLLabelLehntUngueltigesAb(t *testing.T) {
	for _, target := range []string{
		"/ml-daten/whatsapp/1/label?label=vielleicht",
		"/ml-daten/whatsapp/abc/label?label=true",
		"/ml-daten/quatsch/1/label?label=true",
	} {
		spy := newSpyStore()
		if rec := serveML(t, spy, http.MethodPost, target); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: code = %d, want 400", target, rec.Code)
		}
		if spy.mlLabeled != "" {
			t.Errorf("%s: trotzdem gespeichert: %q", target, spy.mlLabeled)
		}
	}
}

func TestMLShadowLeitetUm(t *testing.T) {
	rec := serveML(t, newSpyStore(), http.MethodGet, "/ml-shadow")
	if rec.Code != http.StatusFound {
		t.Fatalf("code = %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/ml-daten?quelle=whatsapp" {
		t.Errorf("Location = %q", loc)
	}
}
