// Package classifier portiert den n8n-Node "Absagen Classifier": ein Gemini-Agent,
// der eine WhatsApp-Nachricht als Zusage ("true"), Absage ("false") oder
// "invalid" klassifiziert. Primärmodell + Fallback wie im Workflow (needsFallback);
// welche Modelle das sind, stellt das Admin-UI zur Laufzeit um (ModelSource).
package classifier

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

//go:embed system-prompt.txt
var systemPrompt string

// Result ist exakt einer der drei vom Workflow erwarteten Werte.
type Result string

const (
	Zusage  Result = "true"
	Absage  Result = "false"
	Invalid Result = "invalid"
)

const geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/models"

// ModelSource liefert die aktuelle Modellwahl (Tabelle classifier_models,
// umgestellt im Admin-UI). Leere Werte heißen "nicht gesetzt".
type ModelSource interface {
	ClassifierModels(ctx context.Context) (primary, fallback string, err error)
}

type Gemini struct {
	apiKey        string
	model         string // aus der Konfiguration – gilt, solange Models nichts liefert
	fallbackModel string
	baseURL       string
	http          *http.Client

	// Models wird vor jeder Klassifizierung gefragt, damit eine Umstellung
	// ohne Neustart greift. nil = nur die Konfiguration.
	Models ModelSource
}

func NewGemini(apiKey, model, fallbackModel string) *Gemini {
	return &Gemini{
		apiKey:        apiKey,
		model:         model,
		fallbackModel: fallbackModel,
		baseURL:       geminiBaseURL,
		http:          &http.Client{Timeout: 30 * time.Second},
	}
}

// models liefert Haupt- und Fallback-Modell für den nächsten Aufruf. Ist die
// Wahl nicht lesbar, klassifiziert der Bot trotzdem – mit der Konfiguration.
func (g *Gemini) models(ctx context.Context) (primary, fallback string) {
	primary, fallback = g.model, g.fallbackModel
	if g.Models == nil {
		return primary, fallback
	}
	p, f, err := g.Models.ClassifierModels(ctx)
	if err != nil {
		log.Printf("⚠️  classifier: Modellwahl nicht lesbar: %v (→ %s / %s)", err, primary, fallback)
		return primary, fallback
	}
	if p != "" {
		primary = p
	}
	if f != "" {
		fallback = f
	}
	return primary, fallback
}

// Classification ist das Ergebnis eines Classifier-Laufs inkl. Gemini-Roh-Antwort
// und tatsächlich genutztem Modell (für Trace/Debugging in der Verlauf-Ansicht).
type Classification struct {
	Result Result
	Raw    string // ungetrimmte Antwort von Gemini
	Model  string // Modell, das die Antwort lieferte (Primär oder Fallback)
}

// Classify ruft erst das Primär-, bei Fehler das Fallback-Modell auf. Jede
// nicht eindeutige Antwort (alles außer "true"/"false") wird zu Invalid – so
// löst der nachgelagerte Switch (n8n) bei "invalid" keine DB-Aktion aus.
func (g *Gemini) Classify(ctx context.Context, message string) (Classification, error) {
	model, fallback := g.models(ctx)
	primary := model
	raw, err := g.generate(ctx, model, message)
	if err != nil && fallback != "" && fallback != primary {
		model = fallback
		raw, err = g.generate(ctx, model, message)
	}
	if err != nil {
		return Classification{Result: Invalid, Model: model}, err
	}
	return Classification{Result: normalize(raw), Raw: raw, Model: model}, nil
}

func normalize(raw string) Result {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true":
		return Zusage
	case "false":
		return Absage
	default:
		return Invalid
	}
}

type geminiRequest struct {
	SystemInstruction content   `json:"systemInstruction"`
	Contents          []content `json:"contents"`
	GenerationConfig  genConfig `json:"generationConfig"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text"`
}

type genConfig struct {
	Temperature float64 `json:"temperature"`
}

type geminiResponse struct {
	Candidates []struct {
		Content content `json:"content"`
	} `json:"candidates"`
}

func (g *Gemini) generate(ctx context.Context, model, message string) (string, error) {
	reqBody := geminiRequest{
		SystemInstruction: content{Parts: []part{{Text: systemPrompt}}},
		Contents:          []content{{Role: "user", Parts: []part{{Text: message}}}},
		GenerationConfig:  genConfig{Temperature: 0},
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	url := fmt.Sprintf("%s/%s:generateContent", g.baseURL, model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", g.apiKey)

	resp, err := g.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini %s: %w", model, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini %s: status %d: %s", model, resp.StatusCode, string(body))
	}

	var out geminiResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("gemini %s: decode: %w", model, err)
	}
	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini %s: empty response", model)
	}
	return out.Candidates[0].Content.Parts[0].Text, nil
}
