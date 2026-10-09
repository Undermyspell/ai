package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	sharedstore "github.com/michael/zumba-shared/store"
)

// Montag, 12.10.2026 – der nächste Wochenreport ist Do 15.10.
func fixKartenNow(t *testing.T) {
	t.Helper()
	alt := kartenNow
	kartenNow = func() time.Time { return time.Date(2026, 10, 12, 9, 0, 0, 0, time.Local) }
	t.Cleanup(func() { kartenNow = alt })
}

func postKarten(t *testing.T, spy *spyStore, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	srv := New(spy, testCfg(), false)
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	return rec
}

func TestBildDesignsSeiteZeigtRotationUndNaechsteKarte(t *testing.T) {
	fixKartenNow(t)
	spy := newSpyStore()
	spy.cards = sharedstore.CardSettings{RotationGesetzt: true, Rotation: []string{"arena", "gipfelbuch"}}
	srv := New(spy, testCfg(), false)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, httptest.NewRequest("GET", "/bild-designs", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Nächster Wochenreport", "Warteschlange", `data-kommt="arena"`, `data-id="arena"`, `class="queue-date is-next">15.10.`, `>22.10.</span>`, `value="wetterbericht"`, "Die nächsten Donnerstage"} {
		if !strings.Contains(body, want) {
			t.Errorf("Seite enthält %q nicht", want)
		}
	}
}

// Die Reihenfolge im Formular (per Drag & Drop sortiert) ist die
// Warteschlange – oben kommt am nächsten Donnerstag.
func TestBildDesignsWarteschlangeNachFormularReihenfolge(t *testing.T) {
	fixKartenNow(t)
	spy := newSpyStore()
	spy.cards = sharedstore.CardSettings{RotationGesetzt: true, Rotation: []string{"zeitung", "arena", "wrapped"}}
	rec := postKarten(t, spy, "/bild-designs/rotation", url.Values{"style": {"wetterbericht", "zeitung", "wrapped", "kassenbon"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d: %s", rec.Code, rec.Body.String())
	}
	want := []string{"wetterbericht", "zeitung", "wrapped", "kassenbon"}
	if !slices.Equal(spy.cards.Rotation, want) {
		t.Errorf("Warteschlange = %v, will %v", spy.cards.Rotation, want)
	}
	if !strings.Contains(rec.Body.String(), `data-kommt="wetterbericht"`) {
		t.Error("oberstes Design kommt nicht als nächstes")
	}
	if !strings.Contains(rec.Header().Get("HX-Trigger"), "success") {
		t.Errorf("HX-Trigger = %q", rec.Header().Get("HX-Trigger"))
	}
}

// Eine Einmal-Auswahl schiebt die Schlange eine Woche nach hinten.
func TestBildDesignsEinmalAuswahlSchiebtSchlange(t *testing.T) {
	fixKartenNow(t)
	spy := newSpyStore()
	spy.cards = sharedstore.CardSettings{
		RotationGesetzt: true, Rotation: []string{"arena", "zeitung"},
		Naechster: &sharedstore.NextCard{Tag: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), Style: "kassenbon"},
	}
	srv := New(spy, testCfg(), false)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, httptest.NewRequest("GET", "/bild-designs", nil))
	body := rec.Body.String()
	for _, want := range []string{`data-kommt="kassenbon"`, `class="queue-date is-next">22.10.`, `>29.10.</span>`} {
		if !strings.Contains(body, want) {
			t.Errorf("Seite enthält %q nicht", want)
		}
	}
}

func TestBildDesignsRotationLehntUnbekanntesAb(t *testing.T) {
	spy := newSpyStore()
	rec := postKarten(t, spy, "/bild-designs/rotation", url.Values{"style": {"wrapped", "gibtsnicht"}})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code = %d, will 422", rec.Code)
	}
	if spy.cards.RotationGesetzt {
		t.Error("trotz Fehler gespeichert")
	}
}

// Die Einmal-Auswahl gilt immer für den nächsten Wochenreport – der Tag kommt
// nie aus dem Formular.
func TestBildDesignsNaechsteKarte(t *testing.T) {
	fixKartenNow(t)
	spy := newSpyStore()
	rec := postKarten(t, spy, "/bild-designs/naechster", url.Values{"style": {"wetterbericht"}, "tag": {"2030-01-01"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d: %s", rec.Code, rec.Body.String())
	}
	n := spy.cards.Naechster
	if n == nil || n.Style != "wetterbericht" || n.Tag.Format("2006-01-02") != "2026-10-15" {
		t.Fatalf("Einmal-Auswahl = %+v", n)
	}
	if !strings.Contains(rec.Body.String(), `data-kommt="wetterbericht"`) {
		t.Error("Region zeigt die gewählte Karte nicht")
	}

	// Leer = zurück zur Rotation.
	postKarten(t, spy, "/bild-designs/naechster", url.Values{"style": {""}})
	if spy.cards.Naechster != nil {
		t.Errorf("nicht aufgehoben: %+v", spy.cards.Naechster)
	}
	if rec := postKarten(t, spy, "/bild-designs/naechster", url.Values{"style": {"gibtsnicht"}}); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("unbekanntes Design: code = %d, will 422", rec.Code)
	}
}
