package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/michael/zumba-admin-ui/internal/config"
	"github.com/michael/zumba-admin-ui/internal/tunnel"
)

// fakeTunnel steht für den tunnel-service: merkt sich die Aufrufe und liefert
// einen einstellbaren Status zurück.
type fakeTunnel struct {
	status    tunnel.Status
	err       error
	openCalls []openCall
	closed    []string
	closedAll int
}

type openCall struct {
	target string
	ttl    time.Duration
}

func (f *fakeTunnel) Status(context.Context) (tunnel.Status, error) {
	return f.status, f.err
}

func (f *fakeTunnel) Open(_ context.Context, target string, ttl time.Duration) (tunnel.Status, error) {
	f.openCalls = append(f.openCalls, openCall{target, ttl})
	if f.err != nil {
		return tunnel.Status{}, f.err
	}
	return f.status, nil
}

func (f *fakeTunnel) Close(_ context.Context, target string) (tunnel.Status, error) {
	f.closed = append(f.closed, target)
	return f.status, f.err
}

func (f *fakeTunnel) CloseAll(context.Context) (tunnel.Status, error) {
	f.closedAll++
	return f.status, f.err
}

func statusWith(state tunnel.State) tunnel.Status {
	since := time.Now().Add(-10 * time.Minute)
	expires := time.Now().Add(90 * time.Minute)
	return tunnel.Status{
		Targets: []tunnel.Target{{
			Name:      "wrapped",
			Label:     "Wrapped",
			State:     state,
			URL:       "https://abc.ngrok-free.app",
			Since:     &since,
			ExpiresAt: &expires,
			Requests:  12,
		}},
		DefaultTTLSeconds: 7200,
		MaxTTLSeconds:     86400,
	}
}

func publicSrv(f *fakeTunnel) (*Server, http.Handler) {
	srv := New(newSpyStore(), config.Config{TunnelURL: "http://tunnel:8080"}, false)
	srv.tunnel = f
	return srv, srv.Routes()
}

func TestOeffentlichSeiteZeigtZielUndZustand(t *testing.T) {
	_, h := publicSrv(&fakeTunnel{status: statusWith(tunnel.StateActive)})
	rec := get(h, "/public")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Wrapped", "öffentlich erreichbar", "https://abc.ngrok-free.app", "schließt"} {
		if !strings.Contains(body, want) {
			t.Errorf("Seite enthält %q nicht", want)
		}
	}
}

func TestOhneTunnelURLNurHinweisUndKeineSchalter(t *testing.T) {
	srv := New(newSpyStore(), config.Config{}, false)
	h := srv.Routes()

	rec := get(h, "/public")
	if !strings.Contains(rec.Body.String(), "Kein tunnel-service konfiguriert") {
		t.Error("Hinweis auf fehlende TUNNEL_URL fehlt")
	}
	// Ohne Dienst darf es die schaltenden Routen gar nicht geben.
	post := postForm(t, h, "/public/open", url.Values{"target": {"wrapped"}})
	if post.Code != http.StatusNotFound {
		t.Errorf("POST /public/open = %d, erwartet 404", post.Code)
	}
}

func TestOpenReichtZielUndLaufzeitDurch(t *testing.T) {
	f := &fakeTunnel{status: statusWith(tunnel.StateOpening)}
	_, h := publicSrv(f)

	rec := postForm(t, h, "/public/open", url.Values{"target": {"wrapped"}, "hours": {"8"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	if len(f.openCalls) != 1 || f.openCalls[0].target != "wrapped" || f.openCalls[0].ttl != 8*time.Hour {
		t.Fatalf("Aufrufe = %+v", f.openCalls)
	}
	if !strings.Contains(rec.Body.String(), "baut auf") {
		t.Error("Antwort zeigt den Aufbau nicht an")
	}
}

func TestOpenOhneStundenNimmtDieVorgabeDesDienstes(t *testing.T) {
	f := &fakeTunnel{status: statusWith(tunnel.StateOpening)}
	_, h := publicSrv(f)

	postForm(t, h, "/public/open", url.Values{"target": {"wrapped"}})
	// ttl = 0 heißt: der tunnel-service entscheidet (DEFAULT_TTL).
	if len(f.openCalls) != 1 || f.openCalls[0].ttl != 0 {
		t.Fatalf("Aufrufe = %+v", f.openCalls)
	}
}

func TestCloseUndCloseAll(t *testing.T) {
	f := &fakeTunnel{status: statusWith(tunnel.StateClosing)}
	_, h := publicSrv(f)

	postForm(t, h, "/public/close", url.Values{"target": {"wrapped"}})
	if len(f.closed) != 1 || f.closed[0] != "wrapped" {
		t.Fatalf("geschlossen = %+v", f.closed)
	}
	postForm(t, h, "/public/close-all", url.Values{})
	if f.closedAll != 1 {
		t.Fatalf("close-all Aufrufe = %d", f.closedAll)
	}
}

func TestTunnelServiceNichtErreichbarZeigtGrundStattFehlerseite(t *testing.T) {
	_, h := publicSrv(&fakeTunnel{err: errNotReachable{}})
	rec := get(h, "/public")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, die Seite soll stehenbleiben", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "tunnel-service nicht erreichbar") {
		t.Error("Grund wird nicht angezeigt")
	}
}

type errNotReachable struct{}

func (errNotReachable) Error() string { return "tunnel-service nicht erreichbar: connection refused" }

// Solange etwas öffentlich hängt, sind Bot-Test und ML-Test dicht: der
// Preview-Modus des Bot-Tests verschickt echte WhatsApp-Nachrichten.
func TestBotTestUndMLTestSindBeiOffenemTunnelGesperrt(t *testing.T) {
	_, h := publicSrv(&fakeTunnel{status: statusWith(tunnel.StateActive)})

	for _, path := range []string{"/bot-test", "/ml-test"} {
		rec := get(h, path)
		if rec.Code != http.StatusForbidden {
			t.Errorf("GET %s = %d, erwartet 403", path, rec.Code)
		}
	}
	// Der Wochenreport-Testlauf ebenso — er geht auf denselben Bot-Endpoint.
	rec := postForm(t, h, "/bot-test/run", url.Values{"szenario": {"wochenreport"}})
	if rec.Code != http.StatusForbidden {
		t.Errorf("POST /bot-test/run = %d, erwartet 403", rec.Code)
	}
}

func TestBotTestBleibtOffenWennKeinTunnelHaengt(t *testing.T) {
	_, h := publicSrv(&fakeTunnel{status: statusWith(tunnel.StateInactive)})
	if rec := get(h, "/bot-test"); rec.Code != http.StatusOK {
		t.Errorf("GET /bot-test = %d, erwartet 200", rec.Code)
	}
}

// Menüpunkte, die gesperrt sind, verschwinden auch aus der Navigation.
func TestNavigationBlendetGesperrteSeitenAus(t *testing.T) {
	_, h := publicSrv(&fakeTunnel{status: statusWith(tunnel.StateActive)})
	get(h, "/public") // füllt den zwischengespeicherten Stand
	body := get(h, "/dashboard").Body.String()
	if strings.Contains(body, `href="/bot-test"`) {
		t.Error("Bot-Test steht trotz offenem Tunnel im Menü")
	}
	if !strings.Contains(body, `href="/public"`) {
		t.Error("Öffentlich fehlt im Menü")
	}
}

func TestLoginBremseWaechstMitJedemFehlversuch(t *testing.T) {
	var th loginThrottle
	now := time.Now()

	first := th.fail(now)
	second := th.fail(now)
	third := th.fail(now)
	if first != failedLoginDelay || second != 2*failedLoginDelay || third != 4*failedLoginDelay {
		t.Fatalf("Wartezeiten = %s / %s / %s", first, second, third)
	}

	for i := 0; i < 20; i++ {
		th.fail(now)
	}
	if got := th.fail(now); got != maxLoginDelay {
		t.Fatalf("Deckel = %s, erwartet %s", got, maxLoginDelay)
	}

	// Erfolgreiche Anmeldung setzt zurück.
	th.reset()
	if got := th.fail(now); got != failedLoginDelay {
		t.Fatalf("nach reset = %s, erwartet %s", got, failedLoginDelay)
	}

	// Und ohne Fehlversuch im Zeitfenster ebenfalls.
	th.fail(now)
	if got := th.fail(now.Add(loginFailWindow + time.Minute)); got != failedLoginDelay {
		t.Fatalf("nach Zeitfenster = %s, erwartet %s", got, failedLoginDelay)
	}
}

// fakeBot steht für den whatsapp-bot: nimmt POST /notify entgegen und merkt
// sich den Text.
type fakeBot struct {
	srv    *httptest.Server
	texts  []string
	status int
}

func newFakeBot(t *testing.T) *fakeBot {
	t.Helper()
	b := &fakeBot{status: http.StatusOK}
	b.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/notify" {
			t.Errorf("unerwarteter Pfad %s", r.URL.Path)
		}
		var req struct {
			Text string `json:"text"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		b.texts = append(b.texts, req.Text)
		w.WriteHeader(b.status)
	}))
	t.Cleanup(b.srv.Close)
	return b
}

func publicSrvWithBot(f *fakeTunnel, bot *fakeBot) (*Server, http.Handler) {
	srv := New(newSpyStore(), config.Config{TunnelURL: "http://tunnel:8080", BotURL: bot.srv.URL}, false)
	srv.tunnel = f
	return srv, srv.Routes()
}

func TestNotifySchicktAdresseUndAblaufAnDenBot(t *testing.T) {
	bot := newFakeBot(t)
	_, h := publicSrvWithBot(&fakeTunnel{status: statusWith(tunnel.StateActive)}, bot)

	rec := postForm(t, h, "/public/notify", url.Values{"target": {"wrapped"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	if len(bot.texts) != 1 {
		t.Fatalf("Aufrufe am Bot = %d", len(bot.texts))
	}
	text := bot.texts[0]
	for _, want := range []string{"Wrapped", "https://abc.ngrok-free.app", "Offen bis"} {
		if !strings.Contains(text, want) {
			t.Errorf("Nachricht enthält %q nicht: %q", want, text)
		}
	}
	if !strings.Contains(rec.Header().Get("HX-Trigger"), "unterwegs") {
		t.Errorf("kein Erfolgs-Toast: %s", rec.Header().Get("HX-Trigger"))
	}
}

func TestNotifyNurBeiOffenemTunnel(t *testing.T) {
	bot := newFakeBot(t)
	_, h := publicSrvWithBot(&fakeTunnel{status: statusWith(tunnel.StateInactive)}, bot)

	rec := postForm(t, h, "/public/notify", url.Values{"target": {"wrapped"}})
	if len(bot.texts) != 0 {
		t.Fatalf("es wurde trotz geschlossenem Tunnel gesendet: %v", bot.texts)
	}
	if !strings.Contains(rec.Header().Get("HX-Trigger"), "Nur offene Tunnel") {
		t.Errorf("Hinweis fehlt: %s", rec.Header().Get("HX-Trigger"))
	}
}

func TestNotifyBremstZweitenKlick(t *testing.T) {
	bot := newFakeBot(t)
	_, h := publicSrvWithBot(&fakeTunnel{status: statusWith(tunnel.StateActive)}, bot)

	postForm(t, h, "/public/notify", url.Values{"target": {"wrapped"}})
	rec := postForm(t, h, "/public/notify", url.Values{"target": {"wrapped"}})
	if len(bot.texts) != 1 {
		t.Fatalf("Aufrufe am Bot = %d, erwartet 1", len(bot.texts))
	}
	if !strings.Contains(rec.Header().Get("HX-Trigger"), "kurz warten") {
		t.Errorf("Bremse meldet sich nicht: %s", rec.Header().Get("HX-Trigger"))
	}
}

func TestNotifyMeldetFehlerDesBots(t *testing.T) {
	bot := newFakeBot(t)
	bot.status = http.StatusServiceUnavailable
	_, h := publicSrvWithBot(&fakeTunnel{status: statusWith(tunnel.StateActive)}, bot)

	rec := postForm(t, h, "/public/notify", url.Values{"target": {"wrapped"}})
	if !strings.Contains(rec.Header().Get("HX-Trigger"), "Versand fehlgeschlagen") {
		t.Errorf("Fehler wird nicht gemeldet: %s", rec.Header().Get("HX-Trigger"))
	}
}

func TestNotifyBremseLaesstNachAblaufWiederZu(t *testing.T) {
	var th notifyThrottle
	now := time.Now()
	if !th.allow(now) {
		t.Fatal("erster Versand muss durchgehen")
	}
	if th.allow(now.Add(notifyMinInterval - time.Second)) {
		t.Fatal("zu früh, hätte gebremst werden müssen")
	}
	if !th.allow(now.Add(notifyMinInterval + time.Second)) {
		t.Fatal("nach der Wartezeit muss es wieder gehen")
	}
}
