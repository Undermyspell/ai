// Package ngrok spricht die lokale Agent-API des ngrok-Agenten an
// (https://ngrok.com/docs/agent/api/). Die API kennt keine Authentifizierung —
// deshalb lauscht der Agent nur auf 127.0.0.1 und nur dieser Service redet mit
// ihm.
package ngrok

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Tunnel ist ein laufender Agent-Endpoint, so wie der Agent ihn meldet.
type Tunnel struct {
	Name     string
	URL      string
	Requests int64
}

type Client struct {
	base string
	http *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		base: strings.TrimRight(baseURL, "/"),
		// Der Aufbau eines Tunnels geht über die ngrok-Cloud und dauert
		// gelegentlich ein paar Sekunden; 20s sind großzügig, aber endlich.
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

// agentTunnel ist die Antwortform der Agent-API (nur die Felder, die wir
// brauchen).
type agentTunnel struct {
	Name      string `json:"name"`
	PublicURL string `json:"public_url"`
	Proto     string `json:"proto"`
	Metrics   struct {
		HTTP struct {
			Count int64 `json:"count"`
		} `json:"http"`
	} `json:"metrics"`
}

// List meldet die aktuell offenen Tunnel. Ein http-Tunnel kann als zwei
// Einträge auftauchen (http und https); wir behalten https und schneiden den
// Namenszusatz " (http)" ab, den der Agent dem zweiten Eintrag anhängt.
func (c *Client) List(ctx context.Context) (map[string]Tunnel, error) {
	var body struct {
		Tunnels []agentTunnel `json:"tunnels"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/tunnels", nil, &body); err != nil {
		return nil, err
	}
	out := make(map[string]Tunnel, len(body.Tunnels))
	for _, t := range body.Tunnels {
		name := strings.TrimSuffix(t.Name, " (http)")
		existing, seen := out[name]
		if seen && !strings.HasPrefix(t.PublicURL, "https://") {
			continue // https gewinnt gegen http
		}
		out[name] = Tunnel{
			Name:     name,
			URL:      t.PublicURL,
			Requests: existing.Requests + t.Metrics.HTTP.Count,
		}
	}
	return out, nil
}

// Start öffnet einen Tunnel auf addr und liefert die öffentliche URL.
// schemes=https erzwingt, dass der Endpoint nur über https erreichbar ist —
// ein http-Aufruf soll gar nicht erst durchgehen.
func (c *Client) Start(ctx context.Context, name, addr string) (string, error) {
	req := map[string]any{
		"name":    name,
		"proto":   "http",
		"addr":    addr,
		"schemes": []string{"https"},
		// Der Request-Inspektor im Agenten hält Antworten im Speicher. Wir
		// schauen nie hinein, also aus — spart RAM auf dem Pi.
		"inspect": false,
	}
	var resp agentTunnel
	if err := c.do(ctx, http.MethodPost, "/api/tunnels", req, &resp); err != nil {
		return "", err
	}
	if resp.PublicURL == "" {
		return "", fmt.Errorf("Agent hat keine öffentliche URL geliefert")
	}
	return resp.PublicURL, nil
}

// Stop schließt den Tunnel. Ein bereits geschlossener Tunnel ist kein Fehler —
// das Ergebnis ist dasselbe.
func (c *Client) Stop(ctx context.Context, name string) error {
	err := c.do(ctx, http.MethodDelete, "/api/tunnels/"+name, nil, nil)
	var status statusError
	if ok := asStatusError(err, &status); ok && status.code == http.StatusNotFound {
		return nil
	}
	return err
}

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("ngrok-Agent nicht erreichbar: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return statusError{code: resp.StatusCode, msg: agentError(raw, resp.Status)}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("Antwort des Agenten unlesbar: %w", err)
	}
	return nil
}

// agentError zieht die lesbare Meldung aus der Fehlerantwort des Agenten.
// Dort steckt der eigentliche Grund drin ("endpoint already online",
// "authentication failed"), den der Nutzer im Admin-UI sehen soll.
func agentError(raw []byte, fallback string) string {
	var e struct {
		Msg     string `json:"msg"`
		Details struct {
			Err string `json:"err"`
		} `json:"details"`
		ErrorCode string `json:"error_code"`
	}
	if err := json.Unmarshal(raw, &e); err == nil {
		switch {
		case e.Details.Err != "":
			return firstLine(e.Details.Err)
		case e.Msg != "":
			return firstLine(e.Msg)
		case e.ErrorCode != "":
			return e.ErrorCode
		}
	}
	if len(raw) > 0 && len(raw) < 200 {
		return strings.TrimSpace(string(raw))
	}
	return fallback
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

type statusError struct {
	code int
	msg  string
}

func (e statusError) Error() string { return e.msg }

func asStatusError(err error, target *statusError) bool {
	se, ok := err.(statusError)
	if ok {
		*target = se
	}
	return ok
}
