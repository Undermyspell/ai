package evolution

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client sendet WhatsApp-Texte über die Evolution API
// (n8n-Node "Send to WhatsApp (Evolution API)").
type Client struct {
	baseURL  string
	apiKey   string
	instance string
	http     *http.Client
}

func NewClient(baseURL, apiKey, instance string) *Client {
	return &Client{
		baseURL:  baseURL,
		apiKey:   apiKey,
		instance: instance,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

type sendTextRequest struct {
	Number string `json:"number"`
	Text   string `json:"text"`
}

// SendText: POST {baseURL}/message/sendText/{instance} mit Header apikey,
// Body {number, text}.
func (c *Client) SendText(ctx context.Context, number, text string) error {
	buf, err := json.Marshal(sendTextRequest{Number: number, Text: text})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	url := fmt.Sprintf("%s/message/sendText/%s", c.baseURL, c.instance)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("sendText: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sendText: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

type sendMediaRequest struct {
	Number    string `json:"number"`
	MediaType string `json:"mediatype"`
	MimeType  string `json:"mimetype"`
	FileName  string `json:"fileName"`
	Media     string `json:"media"` // base64 (ohne data:-Präfix)
	Caption   string `json:"caption,omitempty"`
}

// SendImage: POST {baseURL}/message/sendMedia/{instance} — verschickt ein
// PNG als Bild-Nachricht mit optionaler Caption.
func (c *Client) SendImage(ctx context.Context, number, caption string, png []byte) error {
	buf, err := json.Marshal(sendMediaRequest{
		Number:    number,
		MediaType: "image",
		MimeType:  "image/png",
		FileName:  "zumba-stats.png",
		Media:     base64.StdEncoding.EncodeToString(png),
		Caption:   caption,
	})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	url := fmt.Sprintf("%s/message/sendMedia/%s", c.baseURL, c.instance)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("sendMedia: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sendMedia: status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
