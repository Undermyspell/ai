package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/michael/zumba-admin-ui/internal/penalty"
)

func postForm(t *testing.T, srv http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestStrafenPageRendert(t *testing.T) {
	spy := newSpyStore()
	srv := New(spy, testCfg(), false).Routes()
	req := httptest.NewRequest("GET", "/strafen", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Strafenkasse") {
		t.Error("Seite rendert nicht")
	}
}

func TestAddNoShowStrafe(t *testing.T) {
	spy := newSpyStore()
	srv := New(spy, testCfg(), false).Routes()
	rec := postForm(t, srv, "/strafen", url.Values{
		"userId": {"u01"}, "datum": {"2026-01-01"}, "betrag": {"50"}, // Donnerstag
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if len(spy.strafen) != 1 || spy.strafen[0].Art != penalty.ArtNoShow || spy.strafen[0].Betrag != 50 {
		t.Errorf("Strafe nicht angelegt: %+v", spy.strafen)
	}
}

func TestAddNoShowStrafeNurDonnerstag(t *testing.T) {
	spy := newSpyStore()
	srv := New(spy, testCfg(), false).Routes()
	rec := postForm(t, srv, "/strafen", url.Values{
		"userId": {"u01"}, "datum": {"2026-01-02"}, // Freitag
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code = %d, want 422", rec.Code)
	}
	if len(spy.strafen) != 0 {
		t.Errorf("Strafe darf nicht angelegt werden: %+v", spy.strafen)
	}
}

func TestBegleicheUndLoescheStrafe(t *testing.T) {
	spy := newSpyStore()
	_ = spy.InsertNoShowStrafe(context.TODO(), "u01", mustDate("2026-01-01"), 50)
	srv := New(spy, testCfg(), false).Routes()

	rec := postForm(t, srv, "/strafen/1/begleichen", url.Values{})
	if rec.Code != http.StatusOK || spy.beglichenStrafe != 1 {
		t.Fatalf("begleichen: code=%d, id=%d", rec.Code, spy.beglichenStrafe)
	}

	req := httptest.NewRequest("DELETE", "/strafen/1", nil)
	del := httptest.NewRecorder()
	srv.ServeHTTP(del, req)
	if del.Code != http.StatusOK || spy.geloeschteStrafe != 1 {
		t.Fatalf("löschen: code=%d, id=%d", del.Code, spy.geloeschteStrafe)
	}
}

// Erkennung + Persistenz: 5 Fehltage in Folge erzeugen beim Seitenaufruf einen
// Marker (InsertAutoStrafe), der danach in der Liste auftaucht.
func TestStrafenSeitePersistiertAutoStrafe(t *testing.T) {
	spy := newSpyStore()
	for _, d := range []string{"2026-01-01", "2026-01-08", "2026-01-15", "2026-01-22", "2026-01-29"} {
		_ = spy.InsertAbsence(context.TODO(), "u01", mustDate(d), nil)
	}
	srv := New(spy, testCfg(), false).Routes()
	req := httptest.NewRequest("GET", "/strafen", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	if len(spy.strafen) != 1 || spy.strafen[0].Art != penalty.ArtFehltage {
		t.Fatalf("Auto-Strafe nicht persistiert: %+v", spy.strafen)
	}
	if !strings.Contains(rec.Body.String(), "25€") {
		t.Errorf("25€ nicht in der Seite")
	}
}
