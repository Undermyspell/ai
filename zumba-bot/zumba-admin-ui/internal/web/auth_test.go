package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/michael/zumba-admin-ui/internal/config"
)

func authCfg() config.Config {
	cfg := testCfg()
	cfg.Auth = config.AuthConfig{
		User:          "admin",
		Password:      "geheim",
		SessionSecret: "test-secret",
		CookieSecure:  true,
	}
	return cfg
}

func authSrv() http.Handler { return New(newSpyStore(), authCfg(), false).Routes() }

func get(srv http.Handler, path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func doLogin(t *testing.T, srv http.Handler, user, pass, next string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{"username": {user}, "password": {pass}, "next": {next}}
	return postForm(t, srv, "/login", form)
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range (&http.Response{Header: rec.Header()}).Cookies() {
		if c.Name == sessionCookieName {
			return c
		}
	}
	t.Fatal("kein Session-Cookie gesetzt")
	return nil
}

// Ohne Anmeldung führt jede geschützte Seite auf /login – mit ?next=, damit
// man danach dort landet, wo man hinwollte.
func TestGeschuetzteSeiteLeitetAufLoginUm(t *testing.T) {
	rec := get(authSrv(), "/strafen?jahr=2026")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("code = %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/login?next=") || !strings.Contains(loc, "strafen") {
		t.Errorf("Location = %q", loc)
	}
}

func TestOeffentlichePfadeOhneLogin(t *testing.T) {
	srv := authSrv()
	for _, p := range []string{"/healthz", "/login", "/static/css/styles.css"} {
		if rec := get(srv, p); rec.Code != http.StatusOK {
			t.Errorf("%s: code = %d, want 200", p, rec.Code)
		}
	}
}

func TestLoginSeiteRendert(t *testing.T) {
	rec := get(authSrv(), "/login")
	if !strings.Contains(rec.Body.String(), "Willkommen zurück") {
		t.Error("Login-Seite rendert nicht")
	}
}

func TestFalschesPasswortMeldetFehler(t *testing.T) {
	rec := doLogin(t, authSrv(), "admin", "falsch", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "stimmt nicht") {
		t.Error("keine Fehlermeldung auf der Seite")
	}
	for _, c := range (&http.Response{Header: rec.Header()}).Cookies() {
		if c.Name == sessionCookieName && c.Value != "" {
			t.Error("Session-Cookie trotz falschem Passwort")
		}
	}
}

func TestFalscherBenutzerMeldetFehler(t *testing.T) {
	if rec := doLogin(t, authSrv(), "root", "geheim", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", rec.Code)
	}
}

// Das Cookie muss HttpOnly, Secure und SameSite=Strict sein – es ist der
// einzige Zugangsnachweis.
func TestLoginSetztHardenedCookie(t *testing.T) {
	srv := authSrv()
	rec := doLogin(t, srv, "admin", "geheim", "/strafen")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("code = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/strafen" {
		t.Errorf("Location = %q, want /strafen", loc)
	}
	c := sessionCookie(t, rec)
	if !c.HttpOnly {
		t.Error("Cookie ist nicht HttpOnly")
	}
	if !c.Secure {
		t.Error("Cookie ist nicht Secure")
	}
	if c.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite = %v, want Strict", c.SameSite)
	}
	if c.Path != "/" {
		t.Errorf("Path = %q, want /", c.Path)
	}

	if rec := get(srv, "/strafen", c); rec.Code != http.StatusOK {
		t.Fatalf("mit Cookie: code = %d, want 200", rec.Code)
	}
}

// Ein Cookie aus einer anderen Instanz (anderer Schlüssel) darf nicht gelten.
func TestFremdesCookieGiltNicht(t *testing.T) {
	other := authCfg()
	other.Auth.SessionSecret = "anderes-secret"
	fremd := New(newSpyStore(), other, false)
	rec := httptest.NewRecorder()
	fremd.setSessionCookie(rec, "admin")
	c := sessionCookie(t, rec)

	if rec := get(authSrv(), "/strafen", c); rec.Code != http.StatusSeeOther {
		t.Fatalf("code = %d, want 303 (Umleitung auf /login)", rec.Code)
	}
}

func TestManipuliertesCookieGiltNicht(t *testing.T) {
	srv := authSrv()
	c := sessionCookie(t, doLogin(t, srv, "admin", "geheim", ""))
	c.Value = strings.Replace(c.Value, ".", ".9", 1) // Ablaufzeit verbiegen
	if rec := get(srv, "/strafen", c); rec.Code != http.StatusSeeOther {
		t.Fatalf("code = %d, want 303", rec.Code)
	}
}

func TestLogoutLoeschtCookie(t *testing.T) {
	srv := authSrv()
	c := sessionCookie(t, doLogin(t, srv, "admin", "geheim", ""))

	req := httptest.NewRequest("POST", "/logout", nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	cleared := sessionCookie(t, rec)
	if cleared.Value != "" || cleared.MaxAge >= 0 {
		t.Errorf("Cookie nicht gelöscht: %+v", cleared)
	}
	if rec.Header().Get("Location") != "/login" {
		t.Errorf("Location = %q, want /login", rec.Header().Get("Location"))
	}
}

// HTMX-Aufrufe dürfen keine Umleitung als Fragment einswappen – sie bekommen
// HX-Redirect.
func TestHtmxBekommtHxRedirect(t *testing.T) {
	req := httptest.NewRequest("POST", "/toggle-absence", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	authSrv().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", rec.Code)
	}
	if rec.Header().Get("HX-Redirect") != "/login" {
		t.Errorf("HX-Redirect = %q", rec.Header().Get("HX-Redirect"))
	}
}

// ?next= darf nur innerhalb der App umleiten.
func TestSafeNext(t *testing.T) {
	cases := map[string]string{
		"":                     "/dashboard",
		"/strafen?jahr=2026":   "/strafen?jahr=2026",
		"//evil.example.com":   "/dashboard",
		"https://evil.example": "/dashboard",
		"/login":               "/dashboard",
	}
	for in, want := range cases {
		if got := safeNext(in); got != want {
			t.Errorf("safeNext(%q) = %q, want %q", in, got, want)
		}
	}
}

// Ohne ADMIN_PASSWORD ist der Login aus – das ist der lokale Entwicklungsfall.
func TestOhnePasswortKeinLogin(t *testing.T) {
	srv := New(newSpyStore(), testCfg(), false).Routes()
	if rec := get(srv, "/strafen"); rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
}
