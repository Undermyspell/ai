package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/michael/zumba-admin-ui/web/templates/login"
)

const (
	sessionCookieName = "zumba_admin_session"
	sessionTTL        = 7 * 24 * time.Hour
	// Kleine Bremse gegen stumpfes Durchprobieren. Ein Nutzer, ein Passwort –
	// mehr Schutz braucht das Logbuch nicht.
	failedLoginDelay = 500 * time.Millisecond
)

// sessionKey liefert den HMAC-Schlüssel für das Session-Cookie. Ohne
// konfiguriertes SESSION_SECRET wird beim Start einer gewürfelt – dann sind
// die Sessions nach einem Neustart ungültig, was für ein Admin-UI mit einer
// Replica in Ordnung ist.
func (s *Server) sessionKey() []byte {
	if s.cfg.Auth.SessionSecret != "" {
		return []byte(s.cfg.Auth.SessionSecret)
	}
	return s.ephemeralKey
}

func randomKey() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		log.Fatalf("session key: %v", err) // ohne Zufall kein sicheres Cookie
	}
	return key
}

// signSession baut den Cookie-Wert "<user>.<ablauf>.<signatur>". Der Server
// hält keinen Session-Speicher – die Signatur macht das Cookie fälschungssicher.
func (s *Server) signSession(user string, exp time.Time) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(user)) + "." + strconv.FormatInt(exp.Unix(), 10)
	return payload + "." + s.sign(payload)
}

func (s *Server) sign(payload string) string {
	mac := hmac.New(sha256.New, s.sessionKey())
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// validSession prüft Signatur, Ablauf und Nutzernamen des Cookies.
func (s *Server) validSession(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return false
	}
	payload := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(s.sign(payload)), []byte(parts[2])) {
		return false
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().After(time.Unix(exp, 0)) {
		return false
	}
	user, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	// Ein umbenannter ADMIN_USER entwertet alte Cookies.
	return string(user) == s.cfg.Auth.User
}

func (s *Server) loggedIn(r *http.Request) bool {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	return s.validSession(c.Value)
}

func (s *Server) setSessionCookie(w http.ResponseWriter, user string) {
	exp := time.Now().Add(sessionTTL)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    s.signSession(user, exp),
		Path:     "/",
		Expires:  exp,
		HttpOnly: true,
		Secure:   s.cfg.Auth.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cfg.Auth.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

// publicPath: alles, was ohne Login erreichbar sein muss. Der Rest der App
// liegt hinter requireLogin.
func publicPath(p string) bool {
	return p == "/healthz" || p == "/login" || p == "/logout" || strings.HasPrefix(p, "/static/")
}

// requireLogin schützt die App. Normale Aufrufe landen auf /login (mit ?next=,
// damit man nach dem Anmelden dort weitermacht, wo man hinwollte); HTMX-Aufrufe
// bekommen HX-Redirect, sonst würde die abgelaufene Sitzung als Fragment in die
// Seite geswappt.
func (s *Server) requireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.cfg.Auth.Enabled() || publicPath(r.URL.Path) || s.loggedIn(r) {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", "/login")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		target := "/login"
		if r.Method == http.MethodGet {
			target += "?next=" + url.QueryEscape(r.URL.RequestURI())
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
	})
}

// safeNext verhindert, dass ?next= auf eine fremde Seite umleitet: nur
// relative Pfade innerhalb der App sind erlaubt.
func safeNext(next string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return "/dashboard"
	}
	if publicPath(strings.SplitN(next, "?", 2)[0]) {
		return "/dashboard"
	}
	return next
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Auth.Enabled() || s.loggedIn(r) {
		http.Redirect(w, r, safeNext(r.URL.Query().Get("next")), http.StatusSeeOther)
		return
	}
	s.renderLogin(w, r, r.URL.Query().Get("next"), "")
}

func (s *Server) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Auth.Enabled() {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	user := strings.TrimSpace(r.FormValue("username"))
	pass := r.FormValue("password")
	next := r.FormValue("next")

	userOK := subtle.ConstantTimeCompare([]byte(user), []byte(s.cfg.Auth.User)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(s.cfg.Auth.Password)) == 1
	if !userOK || !passOK {
		time.Sleep(failedLoginDelay)
		log.Printf("login fehlgeschlagen für %q", user)
		w.WriteHeader(http.StatusUnauthorized)
		s.renderLogin(w, r, next, "Benutzername oder Passwort stimmt nicht.")
		return
	}

	s.setSessionCookie(w, s.cfg.Auth.User)
	http.Redirect(w, r, safeNext(next), http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// renderLogin zeichnet die Anmeldeseite. Sie hängt nicht am App-Layout, damit
// ohne Session weder Navigation noch Daten gerendert werden.
func (s *Server) renderLogin(w http.ResponseWriter, r *http.Request, next, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	vm := login.VM{User: s.cfg.Auth.User, Next: next, Error: errMsg}
	if err := login.Page(vm).Render(r.Context(), w); err != nil {
		log.Printf("render login: %v", err)
	}
}
