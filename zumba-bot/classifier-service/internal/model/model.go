// Package model implementiert die Inferenz des in ml-classifier trainierten
// NB-LogReg-Modells in pure Go (Modelldatei Format 2, export_weights_v2.py).
//
// Merkmale wie in sklearn: binäre char_wb-Zeichen-n-Gramme (Wörter mit
// Leerzeichen gepolstert, Unicode-Runen statt Bytes – Emojis!) und
// Wort-n-Gramme (Token-Muster wie WORD_PATTERN in models_v2.py). Je Klasse
// liegt pro Merkmal ein fertiges Gewicht vor (Naive-Bayes-Verhältnis schon
// eingerechnet); Score = Intercept + Summe der Gewichte der vorkommenden
// Merkmale, Wahrscheinlichkeit = Sigmoid, über die Klassen auf 1 normiert.
// Liegt die höchste Wahrscheinlichkeit unter der Schwelle, lautet das Label
// invalid (der Bot tut dann nichts).
//
// Die Implementierung wird über golden.json gegen die Python-Referenz getestet.
package model

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"strings"
)

// LabelInvalid ist das Label, auf das die Schwelle zurückfällt.
const LabelInvalid = "invalid"

// wordToken entspricht WORD_PATTERN (?u)\b\w+\b|[❌✅] aus models_v2.py:
// Pythons \w sind Unicode-Buchstaben, -Ziffern und der Unterstrich – RE2s \w
// wäre nur ASCII.
var wordToken = regexp.MustCompile(`[\p{L}\p{N}_]+|[❌✅]`)

type ngrams struct {
	Min     int                  `json:"ngram_min"`
	Max     int                  `json:"ngram_max"`
	Weights map[string][]float64 `json:"weights"`
}

// Meta beschreibt das Modell für Menschen (GET /info, Admin-UI /ki-modell).
type Meta map[string]any

type Model struct {
	Format    int       `json:"format"`
	Kind      string    `json:"kind"`
	Classes   []string  `json:"classes"`
	Lowercase bool      `json:"lowercase"`
	Char      ngrams    `json:"char"`
	Word      ngrams    `json:"word"`
	Intercept []float64 `json:"intercept"`
	Threshold float64   `json:"threshold"`
	Meta      Meta      `json:"meta"`
}

// Load liest das gzip-komprimierte model.json (Format 2).
func Load(gzData []byte) (*Model, error) {
	zr, err := gzip.NewReader(bytes.NewReader(gzData))
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	defer zr.Close()
	raw, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("gunzip: %w", err)
	}
	var m Model
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("json: %w", err)
	}
	if m.Format != 2 || m.Kind != "nb_logreg" {
		return nil, fmt.Errorf("modelldatei: Format %d / %q wird nicht unterstützt", m.Format, m.Kind)
	}
	if len(m.Classes) == 0 || len(m.Intercept) != len(m.Classes) ||
		len(m.Char.Weights) == 0 || len(m.Word.Weights) == 0 {
		return nil, fmt.Errorf("modelldatei inkonsistent")
	}
	hasInvalid := false
	for _, c := range m.Classes {
		hasInvalid = hasInvalid || c == LabelInvalid
	}
	if !hasInvalid {
		return nil, fmt.Errorf("modelldatei: Klasse %q fehlt", LabelInvalid)
	}
	return &m, nil
}

// Name ist Modellname und Version für Antworten und Logs („NB-LogReg 0.2.0").
func (m *Model) Name() string {
	name, _ := m.Meta["name"].(string)
	version, _ := m.Meta["version"].(string)
	return strings.TrimSpace(name + " " + version)
}

// NumFeatures zählt die Merkmale (Zeichen + Wörter).
func (m *Model) NumFeatures() int { return len(m.Char.Weights) + len(m.Word.Weights) }

// charNgrams repliziert sklearns _char_wb_ngrams: Wörter mit Leerzeichen
// gepolstert, n-Gramme pro Fenstergröße; kürzere Wörter zählen genau einmal.
func charNgrams(text string, minN, maxN int, emit func(string)) {
	for _, word := range strings.Fields(text) {
		r := []rune(" " + word + " ")
		wlen := len(r)
		for n := minN; n <= maxN; n++ {
			end := n
			if end > wlen {
				end = wlen
			}
			emit(string(r[:end]))
			offset := 0
			for offset+n < wlen {
				offset++
				emit(string(r[offset : offset+n]))
			}
			if offset == 0 { // Wort kürzer/gleich Fenster: nur einmal zählen
				break
			}
		}
	}
}

// wordNgrams repliziert sklearns _word_ngrams über die Token des Musters.
func wordNgrams(text string, minN, maxN int, emit func(string)) {
	tokens := wordToken.FindAllString(text, -1)
	for n := minN; n <= maxN; n++ {
		for i := 0; i+n <= len(tokens); i++ {
			emit(strings.Join(tokens[i:i+n], " "))
		}
	}
}

// Prediction ist das Ergebnis einer Klassifikation.
type Prediction struct {
	Label      string             `json:"label"`
	Confidence float64            `json:"confidence"`
	Probs      map[string]float64 `json:"probs"`
	Model      string             `json:"model"`
}

// pyLower entspricht Pythons str.lower(): wie strings.ToLower, nur dass „İ"
// (U+0130) wie in Python zu „i" + kombinierendem Punkt (U+0307) wird – die
// einzige unbedingte Mehrzeichen-Abbildung beim Kleinschreiben. (Das
// griechische End-Sigma behandelt Python kontextabhängig; das bilden wir
// nicht nach.)
func pyLower(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, "\u0130", "i\u0307"))
}

// Predict klassifiziert eine Nachricht.
func (m *Model) Predict(text string) Prediction {
	if m.Lowercase {
		text = pyLower(text)
	}
	scores := make([]float64, len(m.Classes))
	copy(scores, m.Intercept)

	// binär: jedes Merkmal zählt höchstens einmal, je Analysator getrennt
	add := func(weights map[string][]float64, seen map[string]bool) func(string) {
		return func(gram string) {
			if seen[gram] {
				return
			}
			if w, ok := weights[gram]; ok {
				seen[gram] = true
				for c := range scores {
					scores[c] += w[c]
				}
			}
		}
	}
	charNgrams(text, m.Char.Min, m.Char.Max, add(m.Char.Weights, map[string]bool{}))
	wordNgrams(text, m.Word.Min, m.Word.Max, add(m.Word.Weights, map[string]bool{}))

	probs := make(map[string]float64, len(m.Classes))
	var sum float64
	sig := make([]float64, len(scores))
	for c, s := range scores {
		sig[c] = 1 / (1 + math.Exp(-s))
		sum += sig[c]
	}
	best := 0
	for c, name := range m.Classes {
		probs[name] = sig[c] / sum
		if sig[c] > sig[best] {
			best = c
		}
	}
	pred := Prediction{
		Label:      m.Classes[best],
		Confidence: probs[m.Classes[best]],
		Probs:      probs,
		Model:      m.Name(),
	}
	if pred.Confidence < m.Threshold {
		pred.Label = LabelInvalid
	}
	return pred
}
