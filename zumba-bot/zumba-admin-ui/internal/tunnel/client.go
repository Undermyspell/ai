// Package tunnel spricht den tunnel-service an, der die öffentlichen
// ngrok-Tunnel schaltet. Das Admin-UI kennt bewusst nur diesen schmalen
// Dienst und nicht die ngrok-Agent-API — die könnte beliebige Ziele
// veröffentlichen, auch Postgres.
package tunnel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type State string

const (
	StateInactive State = "inactive"
	StateOpening  State = "opening"
	StateActive   State = "active"
	StateClosing  State = "closing"
	StateError    State = "error"
)

// ErrBusy: der tunnel-service baut gerade auf oder ab. Kein Fehler im
// eigentlichen Sinn — die Seite pollt ohnehin weiter.
var ErrBusy = errors.New("Tunnel ist gerade in Arbeit")

type Target struct {
	Name      string     `json:"name"`
	Label     string     `json:"label"`
	State     State      `json:"state"`
	URL       string     `json:"url"`
	Since     *time.Time `json:"since"`
	ExpiresAt *time.Time `json:"expiresAt"`
	Requests  int64      `json:"requests"`
	Error     string     `json:"error"`
}

// Open: Zustand, in dem das Ziel öffentlich erreichbar ist oder gerade
// erreichbar wird. Danach richtet sich die Sperre der Bot-Test-Seiten.
func (t Target) Open() bool {
	return t.State == StateActive || t.State == StateOpening || t.State == StateClosing
}

type Status struct {
	Targets           []Target `json:"targets"`
	DefaultTTLSeconds int      `json:"defaultTtlSeconds"`
	MaxTTLSeconds     int      `json:"maxTtlSeconds"`
}

// AnyOpen meldet, ob irgendein Ziel gerade öffentlich hängt.
func (s Status) AnyOpen() bool {
	for _, t := range s.Targets {
		if t.Open() {
			return true
		}
	}
	return false
}

type Client struct {
	base string
	http *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		base: strings.TrimRight(baseURL, "/"),
		http: &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *Client) Status(ctx context.Context) (Status, error) {
	return c.call(ctx, http.MethodGet, "/status", nil)
}

func (c *Client) Open(ctx context.Context, target string, ttl time.Duration) (Status, error) {
	return c.call(ctx, http.MethodPost, "/open", map[string]any{
		"target":     target,
		"ttlSeconds": int(ttl.Seconds()),
	})
}

func (c *Client) Close(ctx context.Context, target string) (Status, error) {
	return c.call(ctx, http.MethodPost, "/close", map[string]any{"target": target})
}

func (c *Client) CloseAll(ctx context.Context) (Status, error) {
	return c.call(ctx, http.MethodPost, "/close-all", nil)
}

func (c *Client) call(ctx context.Context, method, path string, in any) (Status, error) {
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return Status{}, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return Status{}, err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return Status{}, fmt.Errorf("tunnel-service nicht erreichbar: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusConflict {
		return Status{}, ErrBusy
	}
	if resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if msg == "" {
			msg = resp.Status
		}
		return Status{}, fmt.Errorf("tunnel-service: %s", msg)
	}
	var status Status
	if err := json.Unmarshal(raw, &status); err != nil {
		return Status{}, fmt.Errorf("Antwort des tunnel-service unlesbar: %w", err)
	}
	return status, nil
}
