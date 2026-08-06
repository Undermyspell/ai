// Package renderer ist der HTTP-Client zum renderer-service, der HTML per
// headless Chromium in ein PNG rendert (Statistik-Bild-Karte).
package renderer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		// Chromium startet pro Request neu; auf dem Pi dauert das ein paar
		// Sekunden — großzügiges Timeout.
		http: &http.Client{Timeout: 90 * time.Second},
	}
}

type renderRequest struct {
	HTML  string `json:"html"`
	Width int    `json:"width"`
}

// PNG rendert das HTML-Dokument mit der gegebenen Viewport-Breite.
func (c *Client) PNG(ctx context.Context, html string, width int) ([]byte, error) {
	buf, err := json.Marshal(renderRequest{HTML: html, Width: width})
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/render", bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("render: body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("render: status %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}
