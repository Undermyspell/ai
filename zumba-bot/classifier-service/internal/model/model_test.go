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
}

// TestGoldenParity vergleicht die Go-Inferenz mit den von sklearn berechneten
// Wahrscheinlichkeiten (ml-classifier/scripts/export_weights.py).
//
// golden.json enthält echte Nachrichten und liegt deshalb nicht im Repo —
// ohne die Datei wird der Test übersprungen.
func TestGoldenParity(t *testing.T) {
	raw, err := os.ReadFile("../../model/golden.json")
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("golden.json fehlt — lokal via ml-classifier/scripts/export_weights.py erzeugen")
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
	}
}

func TestPredictBasics(t *testing.T) {
	m, err := Load(modeldata.ModelGZ)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	pred := m.Predict("Muss mi heut abmelden ❌")
	if pred.Label != "false" {
		t.Errorf("Absage erkannt als %q", pred.Label)
	}
	var sum float64
	for _, p := range pred.Probs {
		sum += p
	}
	if math.Abs(sum-1) > 1e-9 {
		t.Errorf("Wahrscheinlichkeiten summieren zu %f", sum)
	}
}
