package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/michael/zumba-tunnel/internal/config"
	"github.com/michael/zumba-tunnel/internal/ngrok"
)

// fakeAgent zählt Aufrufe und lässt sich auf Fehler stellen.
type fakeAgent struct {
	live     map[string]ngrok.Tunnel
	starts   int
	stops    int
	startErr error
	stopErr  error
}

func newFakeAgent() *fakeAgent { return &fakeAgent{live: map[string]ngrok.Tunnel{}} }

func (f *fakeAgent) List(context.Context) (map[string]ngrok.Tunnel, error) {
	out := make(map[string]ngrok.Tunnel, len(f.live))
	for k, v := range f.live {
		out[k] = v
	}
	return out, nil
}

func (f *fakeAgent) Start(_ context.Context, name, _ string) (string, error) {
	f.starts++
	if f.startErr != nil {
		return "", f.startErr
	}
	url := "https://" + name + ".ngrok-free.app"
	f.live[name] = ngrok.Tunnel{Name: name, URL: url}
	return url, nil
}

func (f *fakeAgent) Stop(_ context.Context, name string) error {
	f.stops++
	if f.stopErr != nil {
		return f.stopErr
	}
	delete(f.live, name)
	return nil
}

// harness hält Controller, Agent und die aufgeschobenen Hintergrundaufrufe.
// Die laufen in Tests nicht in Goroutinen, sondern erst auf flush() — sonst
// wäre die Reihenfolge Zufall.
type harness struct {
	t       *testing.T
	ctrl    *Controller
	agent   *fakeAgent
	pending []func()
	now     time.Time
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	agent := newFakeAgent()
	cfg := config.Config{
		Targets: []config.Target{
			{Name: "wrapped", Label: "Wrapped", Addr: "zumba-wrapped:8080"},
			{Name: "admin", Label: "Admin-UI", Addr: "zumba-admin-ui:8080"},
		},
		DefaultTTL: 2 * time.Hour,
		MaxTTL:     24 * time.Hour,
	}
	h := &harness{t: t, agent: agent, now: time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)}
	h.ctrl = New(agent, cfg)
	h.ctrl.now = func() time.Time { return h.now }
	h.ctrl.run = func(f func()) { h.pending = append(h.pending, f) }
	return h
}

func (h *harness) flush() {
	for len(h.pending) > 0 {
		f := h.pending[0]
		h.pending = h.pending[1:]
		f()
	}
}

func (h *harness) status(name string) Status {
	h.t.Helper()
	for _, s := range h.ctrl.Status() {
		if s.Name == name {
			return s
		}
	}
	h.t.Fatalf("Ziel %s nicht im Status", name)
	return Status{}
}

func TestOpenGehtUeberOpeningNachActive(t *testing.T) {
	h := newHarness(t)

	if err := h.ctrl.Open("wrapped", time.Hour); err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Vor dem Hintergrundaufruf muss "baut auf" sichtbar sein — das ist der
	// Zustand, den das Admin-UI anzeigt.
	if got := h.status("wrapped").State; got != StateOpening {
		t.Fatalf("Zustand vor flush = %s, erwartet %s", got, StateOpening)
	}

	h.flush()
	s := h.status("wrapped")
	if s.State != StateActive {
		t.Fatalf("Zustand = %s, erwartet %s", s.State, StateActive)
	}
	if s.URL != "https://wrapped.ngrok-free.app" {
		t.Fatalf("URL = %q", s.URL)
	}
	if s.ExpiresAt == nil || !s.ExpiresAt.Equal(h.now.Add(time.Hour)) {
		t.Fatalf("ExpiresAt = %v, erwartet %v", s.ExpiresAt, h.now.Add(time.Hour))
	}
}

func TestOpenBegrenztDieLaufzeitAufMaxTTL(t *testing.T) {
	h := newHarness(t)
	if err := h.ctrl.Open("wrapped", 100*time.Hour); err != nil {
		t.Fatalf("Open: %v", err)
	}
	h.flush()
	s := h.status("wrapped")
	if !s.ExpiresAt.Equal(h.now.Add(24 * time.Hour)) {
		t.Fatalf("ExpiresAt = %v, erwartet Deckelung auf 24h", s.ExpiresAt)
	}
}

func TestOpenOhneLaufzeitNimmtDefault(t *testing.T) {
	h := newHarness(t)
	_ = h.ctrl.Open("wrapped", 0)
	h.flush()
	if !h.status("wrapped").ExpiresAt.Equal(h.now.Add(2 * time.Hour)) {
		t.Fatalf("ohne TTL muss DefaultTTL gelten")
	}
}

func TestZweitesOpenVerlaengertNurDieFrist(t *testing.T) {
	h := newHarness(t)
	_ = h.ctrl.Open("wrapped", time.Hour)
	h.flush()

	h.now = h.now.Add(30 * time.Minute)
	if err := h.ctrl.Open("wrapped", 3*time.Hour); err != nil {
		t.Fatalf("zweites Open: %v", err)
	}
	h.flush()

	if h.agent.starts != 1 {
		t.Fatalf("Agent-Starts = %d, erwartet 1 (kein zweiter Tunnel)", h.agent.starts)
	}
	if !h.status("wrapped").ExpiresAt.Equal(h.now.Add(3 * time.Hour)) {
		t.Fatalf("Frist wurde nicht verlängert")
	}
}

func TestOpenWaehrendAufbauIstBusy(t *testing.T) {
	h := newHarness(t)
	_ = h.ctrl.Open("wrapped", time.Hour) // bleibt in StateOpening, kein flush
	if err := h.ctrl.Open("wrapped", time.Hour); !errors.Is(err, ErrBusy) {
		t.Fatalf("Fehler = %v, erwartet ErrBusy", err)
	}
}

func TestOpenUnbekanntesZiel(t *testing.T) {
	h := newHarness(t)
	if err := h.ctrl.Open("postgres", time.Hour); !errors.Is(err, ErrUnknownTarget) {
		t.Fatalf("Fehler = %v, erwartet ErrUnknownTarget", err)
	}
	if h.agent.starts != 0 {
		t.Fatalf("Agent wurde für ein unbekanntes Ziel aufgerufen")
	}
}

func TestStartFehlerLandetImZustand(t *testing.T) {
	h := newHarness(t)
	h.agent.startErr = errors.New("endpoint already online")
	_ = h.ctrl.Open("wrapped", time.Hour)
	h.flush()

	s := h.status("wrapped")
	if s.State != StateError {
		t.Fatalf("Zustand = %s, erwartet %s", s.State, StateError)
	}
	if s.Error != "endpoint already online" {
		t.Fatalf("Fehlertext = %q", s.Error)
	}
}

func TestCloseSchliesstUndRaeumtAuf(t *testing.T) {
	h := newHarness(t)
	_ = h.ctrl.Open("wrapped", time.Hour)
	h.flush()

	if err := h.ctrl.Close("wrapped"); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := h.status("wrapped").State; got != StateClosing {
		t.Fatalf("Zustand vor flush = %s, erwartet %s", got, StateClosing)
	}
	h.flush()

	s := h.status("wrapped")
	if s.State != StateInactive || s.URL != "" || s.ExpiresAt != nil {
		t.Fatalf("nach Close: %+v", s)
	}
	if _, ok := h.agent.live["wrapped"]; ok {
		t.Fatalf("Tunnel beim Agenten noch offen")
	}
}

func TestCloseAufGeschlossenemZielIstKeinFehler(t *testing.T) {
	h := newHarness(t)
	if err := h.ctrl.Close("wrapped"); err != nil {
		t.Fatalf("Close: %v", err)
	}
	h.flush()
	if h.agent.stops != 0 {
		t.Fatalf("Agent-Stops = %d, erwartet 0", h.agent.stops)
	}
}

func TestAbgelaufenerTunnelWirdGeschlossen(t *testing.T) {
	h := newHarness(t)
	_ = h.ctrl.Open("wrapped", time.Hour)
	h.flush()

	h.now = h.now.Add(59 * time.Minute)
	h.ctrl.Tick(context.Background())
	h.flush()
	if h.status("wrapped").State != StateActive {
		t.Fatalf("vor Ablauf darf nicht geschlossen werden")
	}

	h.now = h.now.Add(2 * time.Minute)
	h.ctrl.Tick(context.Background())
	h.flush()
	if got := h.status("wrapped").State; got != StateInactive {
		t.Fatalf("Zustand = %s, erwartet %s nach Ablauf", got, StateInactive)
	}
}

func TestCloseAllSchliesstAlles(t *testing.T) {
	h := newHarness(t)
	_ = h.ctrl.Open("wrapped", time.Hour)
	_ = h.ctrl.Open("admin", time.Hour)
	h.flush()

	h.ctrl.CloseAll()
	h.flush()
	for _, name := range []string{"wrapped", "admin"} {
		if got := h.status(name).State; got != StateInactive {
			t.Fatalf("%s = %s, erwartet %s", name, got, StateInactive)
		}
	}
}

func TestAbgleichErkenntVerschwundenenTunnel(t *testing.T) {
	h := newHarness(t)
	_ = h.ctrl.Open("wrapped", time.Hour)
	h.flush()

	// Agent neu gestartet: Tunnel weg, ohne dass wir etwas getan hätten.
	delete(h.agent.live, "wrapped")
	h.ctrl.Tick(context.Background())
	h.flush()

	if got := h.status("wrapped").State; got != StateInactive {
		t.Fatalf("Zustand = %s, erwartet %s", got, StateInactive)
	}
}

func TestErsterAbgleichUebernimmtLaufendeTunnel(t *testing.T) {
	h := newHarness(t)
	// Agent lief weiter, dieser Container ist neu gestartet.
	h.agent.live["admin"] = ngrok.Tunnel{Name: "admin", URL: "https://admin.ngrok-free.app", Requests: 7}

	h.ctrl.Tick(context.Background())
	h.flush()

	s := h.status("admin")
	if s.State != StateActive || s.URL != "https://admin.ngrok-free.app" || s.Requests != 7 {
		t.Fatalf("übernommener Tunnel: %+v", s)
	}
	// Übernommene Tunnel bekommen eine frische Frist, damit auch sie zugehen.
	if s.ExpiresAt == nil || !s.ExpiresAt.Equal(h.now.Add(2*time.Hour)) {
		t.Fatalf("ExpiresAt = %v, erwartet DefaultTTL", s.ExpiresAt)
	}
}

func TestSpaeterAufgetauchterTunnelWirdGeschlossen(t *testing.T) {
	h := newHarness(t)
	h.ctrl.Tick(context.Background()) // erster Abgleich: nichts offen
	h.flush()

	h.agent.live["admin"] = ngrok.Tunnel{Name: "admin", URL: "https://admin.ngrok-free.app"}
	h.ctrl.Tick(context.Background())
	h.flush()

	if got := h.status("admin").State; got != StateInactive {
		t.Fatalf("Zustand = %s, erwartet %s", got, StateInactive)
	}
	if _, ok := h.agent.live["admin"]; ok {
		t.Fatalf("unerwarteter Tunnel wurde nicht geschlossen")
	}
}

func TestFremderTunnelWirdGeschlossen(t *testing.T) {
	h := newHarness(t)
	// Etwas, das kein Ziel von uns ist — z.B. ein tcp-Tunnel auf Postgres.
	h.agent.live["postgres"] = ngrok.Tunnel{Name: "postgres", URL: "tcp://0.tcp.ngrok.io:12345"}

	h.ctrl.Tick(context.Background())
	h.flush()

	if _, ok := h.agent.live["postgres"]; ok {
		t.Fatalf("fremder Tunnel blieb offen")
	}
}

func TestAbgleichAktualisiertZaehler(t *testing.T) {
	h := newHarness(t)
	_ = h.ctrl.Open("wrapped", time.Hour)
	h.flush()

	h.agent.live["wrapped"] = ngrok.Tunnel{Name: "wrapped", URL: "https://wrapped.ngrok-free.app", Requests: 42}
	h.ctrl.Tick(context.Background())
	h.flush()

	if got := h.status("wrapped").Requests; got != 42 {
		t.Fatalf("Requests = %d, erwartet 42", got)
	}
}
