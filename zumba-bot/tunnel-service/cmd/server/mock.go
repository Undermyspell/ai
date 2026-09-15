package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/michael/zumba-tunnel/internal/ngrok"
)

// mockAgent ersetzt den ngrok-Agenten bei lokaler Entwicklung (MOCK=true):
// kein Authtoken, kein Netz, aber dieselben Zustände inklusive der zwei
// Sekunden Aufbauzeit — sonst ließe sich die Anzeige im Admin-UI ("baut auf")
// gar nicht sehen.
type mockAgent struct {
	mu      sync.Mutex
	tunnels map[string]ngrok.Tunnel
}

func newMockAgent() *mockAgent {
	return &mockAgent{tunnels: map[string]ngrok.Tunnel{}}
}

func (m *mockAgent) List(context.Context) (map[string]ngrok.Tunnel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]ngrok.Tunnel, len(m.tunnels))
	for name, t := range m.tunnels {
		t.Requests += rand.Int63n(3) // etwas Bewegung in der Anzeige
		m.tunnels[name] = t
		out[name] = t
	}
	return out, nil
}

func (m *mockAgent) Start(ctx context.Context, name, _ string) (string, error) {
	select {
	case <-time.After(2 * time.Second):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	url := fmt.Sprintf("https://mock-%s-%04d.ngrok-free.app", name, rand.Intn(10000))
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tunnels[name] = ngrok.Tunnel{Name: name, URL: url}
	return url, nil
}

func (m *mockAgent) Stop(ctx context.Context, name string) error {
	select {
	case <-time.After(time.Second):
	case <-ctx.Done():
		return ctx.Err()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tunnels, name)
	return nil
}
