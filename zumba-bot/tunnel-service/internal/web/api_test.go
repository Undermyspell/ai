package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/michael/zumba-tunnel/internal/config"
	"github.com/michael/zumba-tunnel/internal/controller"
	"github.com/michael/zumba-tunnel/internal/ngrok"
)

type stubAgent struct {
	started chan string
	release chan struct{}
}

func (s *stubAgent) List(ctx context.Context) (map[string]ngrok.Tunnel, error) {
	return map[string]ngrok.Tunnel{}, nil
}

func (s *stubAgent) Start(_ context.Context, name, _ string) (string, error) {
	s.started <- name
	<-s.release // hält den Aufbau an, damit "opening" beobachtbar ist
	return "https://" + name + ".ngrok-free.app", nil
}

func (s *stubAgent) Stop(context.Context, string) error { return nil }

func newServer(t *testing.T) (*httptest.Server, *stubAgent) {
	t.Helper()
	cfg := config.Config{
		Targets:    []config.Target{{Name: "wrapped", Label: "Wrapped", Addr: "zumba-wrapped:8080"}},
		DefaultTTL: 2 * time.Hour,
		MaxTTL:     24 * time.Hour,
	}
	agent := &stubAgent{started: make(chan string, 1), release: make(chan struct{})}
	srv := httptest.NewServer(New(controller.New(agent, cfg), cfg).Routes())
	t.Cleanup(srv.Close)
	return srv, agent
}

func post(t *testing.T, srv *httptest.Server, path, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(srv.URL+path, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func decode(t *testing.T, resp *http.Response) statusResponse {
	t.Helper()
	defer resp.Body.Close()
	var out statusResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("Antwort lesen: %v", err)
	}
	return out
}

func TestStatusListetZieleUndFristen(t *testing.T) {
	srv, _ := newServer(t)
	resp, err := http.Get(srv.URL + "/status")
	if err != nil {
		t.Fatalf("GET /status: %v", err)
	}
	out := decode(t, resp)
	if len(out.Targets) != 1 || out.Targets[0].Name != "wrapped" {
		t.Fatalf("Ziele = %+v", out.Targets)
	}
	if out.Targets[0].State != controller.StateInactive {
		t.Fatalf("Startzustand = %s, erwartet inactive", out.Targets[0].State)
	}
	if out.DefaultTTLSeconds != 7200 || out.MaxTTLSeconds != 86400 {
		t.Fatalf("Fristen = %d/%d", out.DefaultTTLSeconds, out.MaxTTLSeconds)
	}
}

func TestOpenAntwortetSofortMitOpening(t *testing.T) {
	srv, agent := newServer(t)
	resp := post(t, srv, "/open", `{"target":"wrapped","ttlSeconds":3600}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Status = %d", resp.StatusCode)
	}
	out := decode(t, resp)
	if out.Targets[0].State != controller.StateOpening {
		t.Fatalf("Zustand = %s, erwartet opening", out.Targets[0].State)
	}

	<-agent.started
	close(agent.release)
}

func TestOpenAufUnbekanntesZielIst404(t *testing.T) {
	srv, _ := newServer(t)
	resp := post(t, srv, "/open", `{"target":"postgres"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Status = %d, erwartet 404", resp.StatusCode)
	}
}

func TestOpenWaehrendAufbauIst409(t *testing.T) {
	srv, agent := newServer(t)
	first := post(t, srv, "/open", `{"target":"wrapped"}`)
	first.Body.Close()
	<-agent.started

	second := post(t, srv, "/open", `{"target":"wrapped"}`)
	defer second.Body.Close()
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("Status = %d, erwartet 409", second.StatusCode)
	}
	close(agent.release)
}

func TestKaputtesJsonIst400(t *testing.T) {
	srv, _ := newServer(t)
	resp := post(t, srv, "/open", `{`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Status = %d, erwartet 400", resp.StatusCode)
	}
}

func TestCloseAllAntwortetMitStatus(t *testing.T) {
	srv, _ := newServer(t)
	resp := post(t, srv, "/close-all", ``)
	out := decode(t, resp)
	if len(out.Targets) != 1 {
		t.Fatalf("Ziele = %+v", out.Targets)
	}
}
