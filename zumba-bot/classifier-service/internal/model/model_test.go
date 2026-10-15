package model

import (
	"encoding/json"
	"errors"
	"io/fs"
	"math"
	"os"
	"testing"

	modeldata "github.com/michael/zumba-classifier/model"
)

type goldenCase struct {
	Text  string             `json:"text"`
	Proba map[string]float64 `json:"proba"`
	Label string             `json:"label"` // nach Schwelle
}

// TestGoldenParity vergleicht die Go-Inferenz mit den von sklearn berechneten
// Wahrscheinlichkeiten und Labels (ml-classifier/scripts/export_weights_v2.py).
//
// golden.json enthält echte Nachrichten und liegt deshalb nicht im Repo —
// ohne die Datei wird der Test übersprungen.
func TestGoldenParity(t *testing.T) {
	raw, err := os.ReadFile("../../model/golden.json")
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("golden.json fehlt — lokal via ml-classifier/scripts/export_weights_v2.py erzeugen")
	}
	if err != nil {
		t.Fatalf("golden.json: %v", err)
	}
	m, err := Load(modeldata.ModelGZ)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var cases []goldenCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("golden.json: %v", err)
	}
	if len(cases) == 0 {
		t.Fatal("golden.json leer")
	}

	const tol = 1e-9
	for _, c := range cases {
		pred := m.Predict(c.Text)
		for class, want := range c.Proba {
			got := pred.Probs[class]
			if math.Abs(got-want) > tol {
				t.Errorf("Text %q Klasse %s: got %.12f want %.12f",
					c.Text, class, got, want)
			}
		}
		if c.Label != "" && pred.Label != c.Label {
			t.Errorf("Text %q: Label %q, want %q", c.Text, pred.Label, c.Label)
		}
	}
}

func TestPredictBasics(t *testing.T) {
	m, err := Load(modeldata.ModelGZ)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for text, want := range map[string]string{
		"Muss mi heut abmelden ❌":   "false",
		"Meld mich wieder an":       "true",
		"Wer bringt d'Kartn mit? 🃏": "invalid",
	} {
		pred := m.Predict(text)
		if pred.Label != want {
			t.Errorf("%q erkannt als %q, want %q", text, pred.Label, want)
		}
		var sum float64
		for _, p := range pred.Probs {
			sum += p
		}
		if math.Abs(sum-1) > 1e-9 {
			t.Errorf("%q: Wahrscheinlichkeiten summieren zu %f", text, sum)
		}
		if pred.Model == "" {
			t.Errorf("%q: Modellname fehlt", text)
		}
	}
}

// Unter der Schwelle wird aus jedem Label invalid – der Bot tut dann nichts.
func TestSchwelleFaelltAufInvalid(t *testing.T) {
	m, err := Load(modeldata.ModelGZ)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	m.Threshold = 1.01 // nichts erreicht 101 %
	if got := m.Predict("Muss mi heut abmelden ❌").Label; got != LabelInvalid {
		t.Errorf("Label %q, want invalid", got)
	}
}

func TestWortTokenWiePython(t *testing.T) {
	var got []string
	wordNgrams("i a ned ❌ ab 19⁰⁰uhr a_b", 1, 1, func(s string) { got = append(got, s) })
	want := []string{"i", "a", "ned", "❌", "ab", "19⁰⁰uhr", "a_b"}
	if len(got) != len(want) {
		t.Fatalf("Token %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Token %d: %q, want %q", i, got[i], want[i])
		}
	}
}
