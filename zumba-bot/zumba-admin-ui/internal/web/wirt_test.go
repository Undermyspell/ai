package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/michael/zumba-admin-ui/internal/config"
	"github.com/michael/zumba-admin-ui/internal/tunnel"
)

func wirtPost(h http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestWirtSeiteZeigtDenTest(t *testing.T) {
	rec := httptest.NewRecorder()
	New(newSpyStore(), testCfg(), false).Routes().ServeHTTP(rec, httptest.NewRequest("GET", "/wirt", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	for _, want := range []string{"Erinnerung testen", "WIRT_JID", `hx-post="/wirt/test"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("Seite enthält %q nicht", want)
		}
	}
}

// wirtBot steht für den whatsapp-bot und merkt sich die Query.
func wirtBot(t *testing.T, queries *[]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wirt-reminder" {
			t.Errorf("unerwarteter Pfad %s", r.URL.Path)
		}
		*queries = append(*queries, r.URL.RawQuery)
		out := `{"message":"Servus @498912345678 👋","recipient":"1234@g.us","mentioned":"498912345678","dryRun":true}`
		if r.URL.Query().Get("preview") == "true" {
			out = `{"message":"Servus @4917012345678 👋","recipient":"1234@g.us","mentioned":"4917012345678","previewTo":"4917012345678@s.whatsapp.net"}`
		}
		_, _ = w.Write([]byte(out))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestWirtTestDryRunUndVorschau(t *testing.T) {
	var queries []string
	bot := wirtBot(t, &queries)
	h := New(newSpyStore(), config.Config{BotURL: bot.URL}, false).Routes()

	rec := wirtPost(h, "/wirt/test", url.Values{"mode": {"dryrun"}})
	body := rec.Body.String()
	if !strings.Contains(body, "Servus @498912345678") || !strings.Contains(body, "Stammtisch-Gruppe – nicht gesendet") {
		t.Errorf("Dry-Run-Antwort:\n%s", body)
	}

	rec = wirtPost(h, "/wirt/test", url.Values{"mode": {"preview"}})
	if body := rec.Body.String(); !strings.Contains(body, "nicht in die Gruppe") || !strings.Contains(body, "@4917012345678") {
		t.Errorf("Vorschau-Antwort:\n%s", body)
	}
	if len(queries) != 2 || queries[0] != "dryRun=true" || queries[1] != "preview=true" {
		t.Errorf("Bot-Aufrufe = %q", queries)
	}

	// Zweiter Klick direkt danach: gebremst, kein weiterer Bot-Aufruf.
	rec = wirtPost(h, "/wirt/test", url.Values{"mode": {"preview"}})
	if len(queries) != 2 || !strings.Contains(rec.Body.String(), "kurz warten") {
		t.Errorf("zweite Vorschau nicht gebremst: %q\n%s", queries, rec.Body.String())
	}
}

// Die Vorschau verschickt eine echte WhatsApp – bei offenem Tunnel ist die
// Seite deshalb gesperrt wie der Bot-Test.
func TestWirtIstBeiOffenemTunnelGesperrt(t *testing.T) {
	_, h := publicSrv(&fakeTunnel{status: statusWith(tunnel.StateActive)})
	if rec := wirtPost(h, "/wirt/test", url.Values{"mode": {"preview"}}); rec.Code != http.StatusForbidden {
		t.Errorf("POST /wirt/test = %d, erwartet 403", rec.Code)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/wirt", nil))
	if rec.Code != http.StatusForbidden {
		t.Errorf("GET /wirt = %d, erwartet 403", rec.Code)
	}
}
