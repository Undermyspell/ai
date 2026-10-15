package web

import (
	"context"
	"errors"
	"testing"

	sharedstore "github.com/michael/zumba-shared/store"

	"github.com/michael/zumba-whatsapp-bot/internal/classifier"
	"github.com/michael/zumba-whatsapp-bot/internal/shadow"
)

type fakeML struct {
	pred    shadow.Prediction
	err     error
	calls   int
	records []shadow.Record
}

func (f *fakeML) Classify(context.Context, string) (shadow.Prediction, error) {
	f.calls++
	return f.pred, f.err
}

func (f *fakeML) RecordAsync(r shadow.Record) { f.records = append(f.records, r) }

type fakeEntscheider string

func (f fakeEntscheider) ClassifierEntscheider(context.Context) (string, error) {
	return string(f), nil
}

// Standard: Gemini entscheidet, das eigene Modell wird nur fürs Protokoll
// (im Hintergrund) gefragt.
func TestGeminiEntscheidetStandard(t *testing.T) {
	s, st, _ := newTestServer(classifier.Absage, thursday)
	ml := &fakeML{pred: shadow.Prediction{Label: "true", Confidence: 0.9, Model: "NB-LogReg 0.2.0"}}
	s.ML = ml
	out := s.run(context.Background(), groupMsg("muss mi obmeldn"), false, false, s.today())

	if out.DecidedBy != sharedstore.EntscheiderGemini || out.Classification != "false" {
		t.Fatalf("entschieden von %q als %q, want gemini/false", out.DecidedBy, out.Classification)
	}
	if st.absentUserID == "" {
		t.Error("Geminis Absage nicht eingetragen")
	}
	if ml.calls != 0 {
		t.Errorf("eigenes Modell synchron gefragt (%d×) – soll nur im Protokoll laufen", ml.calls)
	}
	if len(ml.records) != 1 || ml.records[0].GeminiLabel != "false" || ml.records[0].Model != nil ||
		ml.records[0].DecidedBy != sharedstore.EntscheiderGemini {
		t.Errorf("Protokoll %+v", ml.records)
	}
}

func TestModellEntscheidet(t *testing.T) {
	s, st, _ := newTestServer(classifier.Zusage, thursday) // Gemini würde zusagen
	ml := &fakeML{pred: shadow.Prediction{Label: "false", Confidence: 0.97, Model: "NB-LogReg 0.2.0"}}
	s.ML = ml
	s.Entscheider = fakeEntscheider(sharedstore.EntscheiderModell)
	out := s.run(context.Background(), groupMsg("muss mi obmeldn"), false, false, s.today())

	if out.DecidedBy != sharedstore.EntscheiderModell || out.Classification != "false" {
		t.Fatalf("entschieden von %q als %q, want modell/false", out.DecidedBy, out.Classification)
	}
	if st.absentUserID == "" || st.presentUserID != "" {
		t.Errorf("Wirkung falsch: %+v", st)
	}
	r := ml.records
	if len(r) != 1 || r[0].DecidedBy != sharedstore.EntscheiderModell || r[0].Model == nil || r[0].GeminiLabel != "" {
		t.Errorf("Protokoll %+v – Geminis Label soll nachgeholt werden", r)
	}
}

// Antwortet das eigene Modell nicht, springt Gemini ein – kein Stillstand.
func TestModellAusfallGeminiSpringtEin(t *testing.T) {
	s, st, _ := newTestServer(classifier.Absage, thursday)
	s.ML = &fakeML{err: errors.New("connection refused")}
	s.Entscheider = fakeEntscheider(sharedstore.EntscheiderModell)
	out := s.run(context.Background(), groupMsg("muss mi obmeldn"), false, false, s.today())

	if out.DecidedBy != sharedstore.EntscheiderGemini || out.Classification != "false" || st.absentUserID == "" {
		t.Fatalf("entschieden von %q als %q (abwesend %q), want gemini/false", out.DecidedBy, out.Classification, st.absentUserID)
	}
}

// Ohne konfigurierten classifier-service entscheidet Gemini, auch wenn das
// Admin-UI auf das eigene Modell steht.
func TestModellOhneServiceGemini(t *testing.T) {
	s, _, _ := newTestServer(classifier.Invalid, thursday)
	s.Entscheider = fakeEntscheider(sharedstore.EntscheiderModell)
	if out := s.run(context.Background(), groupMsg("servus"), false, false, s.today()); out.DecidedBy != sharedstore.EntscheiderGemini {
		t.Errorf("entschieden von %q, want gemini", out.DecidedBy)
	}
}

// Test-/Dry-Run-Pfad protokolliert nie.
func TestDryRunProtokolliertNicht(t *testing.T) {
	s, _, _ := newTestServer(classifier.Absage, thursday)
	ml := &fakeML{pred: shadow.Prediction{Label: "false", Model: "x"}}
	s.ML = ml
	s.Entscheider = fakeEntscheider(sharedstore.EntscheiderModell)
	s.run(context.Background(), groupMsg("muss mi obmeldn"), false, true, s.today())
	if len(ml.records) != 0 {
		t.Errorf("Dry-Run protokolliert: %+v", ml.records)
	}
}
