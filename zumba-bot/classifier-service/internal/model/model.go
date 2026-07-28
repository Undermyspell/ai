// Package model implementiert die Inferenz des in ml-classifier trainierten
// tfidf_logreg-Modells in pure Go: char_wb-n-Gramm-Tokenizer, TF-IDF-Transform
// und multinomiale logistische Regression (Softmax).
//
// Die Implementierung repliziert sklearn Zeichen für Zeichen (Unicode-Runen,
// nicht Bytes!) und wird über golden.json gegen die Python-Referenz getestet.
package model

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
)

type Model struct {
	Classes    []string       `json:"classes"`
	NgramMin   int            `json:"ngram_min"`
	NgramMax   int            `json:"ngram_max"`
	Sublinear  bool           `json:"sublinear_tf"`
	Lowercase  bool           `json:"lowercase"`
	Vocabulary map[string]int `json:"vocabulary"`
	IDF        []float64      `json:"idf"`
	Coef       [][]float64    `json:"coef"`
	Intercept  []float64      `json:"intercept"`
}

// Load liest das gzip-komprimierte model.json.
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
	if len(m.Classes) == 0 || len(m.Coef) != len(m.Classes) ||
		len(m.Intercept) != len(m.Classes) || len(m.Vocabulary) == 0 {
		return nil, fmt.Errorf("modelldatei inkonsistent")
	}
	return &m, nil
}

// ngramCounts repliziert sklearns _char_wb_ngrams: Wörter mit Leerzeichen
// gepolstert, n-Gramme pro Fenstergröße; kürzere Wörter zählen genau einmal.
func (m *Model) ngramCounts(text string) map[int]int {
	if m.Lowercase {
		text = strings.ToLower(text)
	}
	counts := make(map[int]int)
	emit := func(gram string) {
		if idx, ok := m.Vocabulary[gram]; ok {
			counts[idx]++
		}
	}
	for _, word := range strings.Fields(text) {
		r := []rune(" " + word + " ")
		wlen := len(r)
		for n := m.NgramMin; n <= m.NgramMax; n++ {
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
	return counts
}

// Prediction ist das Ergebnis einer Klassifikation.
type Prediction struct {
	Label      string             `json:"label"`
	Confidence float64            `json:"confidence"`
	Probs      map[string]float64 `json:"probs"`
}

// Predict führt TF-IDF (sublinear, L2-Norm) + Softmax-LogReg aus.
func (m *Model) Predict(text string) Prediction {
	counts := m.ngramCounts(text)

	x := make(map[int]float64, len(counts))
	var sq float64
	for idx, cnt := range counts {
		tf := float64(cnt)
		if m.Sublinear {
			tf = 1 + math.Log(tf)
		}
		v := tf * m.IDF[idx]
		x[idx] = v
		sq += v * v
	}
	if sq > 0 {
		norm := math.Sqrt(sq)
		for idx := range x {
			x[idx] /= norm
		}
	}

	scores := make([]float64, len(m.Classes))
	for c := range m.Classes {
		s := m.Intercept[c]
		row := m.Coef[c]
		for idx, v := range x {
			s += row[idx] * v
		}
		scores[c] = s
	}

	// Softmax, numerisch stabil
	maxS := scores[0]
	for _, s := range scores[1:] {
		if s > maxS {
			maxS = s
		}
	}
	var sum float64
	exps := make([]float64, len(scores))
	for i, s := range scores {
		exps[i] = math.Exp(s - maxS)
		sum += exps[i]
	}

	probs := make(map[string]float64, len(m.Classes))
	best := 0
	for i, c := range m.Classes {
		probs[c] = exps[i] / sum
		if exps[i] > exps[best] {
			best = i
		}
	}
	return Prediction{
		Label:      m.Classes[best],
		Confidence: probs[m.Classes[best]],
		Probs:      probs,
	}
}
