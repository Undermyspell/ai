package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/michael/zumba-admin-ui/internal/config"
)

// wirtBot steht für den whatsapp-bot und merkt sich Pfad und Query.
func wirtBot(t *testing.T, calls *[]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls = append(*calls, r.URL.Path+"?"+r.URL.RawQuery)
		out := `{"message":"Servus @498912345678 👋","recipient":"1234@g.us","mentioned":"498912345678","dryRun":true}`
		if r.URL.Query().Get("preview") == "true" {
			out = `{"message":"Servus @4917012345678 👋","recipient":"1234@g.us","mentioned":"4917012345678","previewTo":"4917012345678@s.whatsapp.net"}`
		}
		_, _ = w.Write([]byte(out))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func runBotTest(h http.Handler, form url.Values) string {
	req := httptest.NewRequest("POST", "/bot-test/run", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Body.String()
}

func TestBotTestSeiteHatDasWirtSzenario(t *testing.T) {
	rec := httptest.NewRecorder()
	New(newSpyStore(), testCfg(), false).Routes().ServeHTTP(rec, httptest.NewRequest("GET", "/bot-test", nil))
	for _, want := range []string{`id="sz-wirt"`, "Tisch-Erinnerung", `data-for="wirt"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("Seite enthält %q nicht", want)
		}
	}
}

// Das Szenario "wirt" ruft /wirt-reminder auf – Stichtag und Format der
// anderen Szenarien gehen dabei nicht mit.
func TestBotTestWirtDryRunUndVorschau(t *testing.T) {
	var calls []string
	bot := wirtBot(t, &calls)
	h := New(newSpyStore(), config.Config{BotURL: bot.URL}, false).Routes()

	body := runBotTest(h, url.Values{"szenario": {"wirt"}, "mode": {"dryrun"}, "date": {"2026-11-12"}, "format": {"image"}})
	for _, want := range []string{"Servus @498912345678", "nicht gesendet", "Markiert @498912345678", "Do 16:00"} {
		if !strings.Contains(body, want) {
			t.Errorf("Dry-Run-Antwort enthält %q nicht:\n%s", want, body)
		}
	}

	body = runBotTest(h, url.Values{"szenario": {"wirt"}, "mode": {"preview"}})
	for _, want := range []string{"nicht an die Gruppe", "@4917012345678 (du statt des Wirts)"} {
		if !strings.Contains(body, want) {
			t.Errorf("Vorschau-Antwort enthält %q nicht:\n%s", want, body)
		}
	}

	if len(calls) != 2 || calls[0] != "/wirt-reminder?dryRun=true" || calls[1] != "/wirt-reminder?preview=true" {
		t.Errorf("Bot-Aufrufe = %q", calls)
	}
}
