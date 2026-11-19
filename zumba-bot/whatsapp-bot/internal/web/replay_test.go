package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sharedstore "github.com/michael/zumba-shared/store"

	"github.com/michael/zumba-whatsapp-bot/internal/classifier"
	"github.com/michael/zumba-whatsapp-bot/internal/shadow"
	"github.com/michael/zumba-whatsapp-bot/internal/tracestore"
)

// fakeTraces hält aufgezeichnete Events und merkt sich neu gespeicherte.
type fakeTraces struct {
	byID  map[int64]tracestore.Trace
	saved []tracestore.Trace
}

func (f *fakeTraces) Get(_ context.Context, id int64) (tracestore.Trace, error) {
	t, ok := f.byID[id]
	if !ok {
		return tracestore.Trace{}, tracestore.ErrNotFound
	}
	return t, nil
}

func (f *fakeTraces) Save(_ context.Context, t tracestore.Trace) (int64, error) {
	f.saved = append(f.saved, t)
	return 900 + int64(len(f.saved)), nil
}

// recordedTrace ist ein live aufgezeichnetes Event vom Donnerstag 01.01.2026,
// 17:56 Ortszeit.
func recordedTrace(t *testing.T, id int64, text string) tracestore.Trace {
	t.Helper()
	raw, err := json.Marshal(groupMsg(text))
	if err != nil {
		t.Fatal(err)
	}
	return tracestore.Trace{ID: id, CreatedAt: time.Date(2026, 1, 1, 16, 56, 48, 0, time.UTC), RawPayload: raw}
}

func replay(s *Server, query string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	s.Routes().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/replay"+query, nil))
	return w
}

// Am Freitag nachverarbeitet: Guards und Absage gelten für den Donnerstag der
// Originalnachricht; der neue Trace verweist aufs Original, das Protokoll
// prüft auf Doppelte.
func TestReplayVerarbeitetAmTagDerNachricht(t *testing.T) {
	s, st, snd := newTestServer(classifier.Absage, friday)
	tr := &fakeTraces{byID: map[int64]tracestore.Trace{417: recordedTrace(t, 417, "sag leider ab")}}
	s.Traces, s.Tracer = tr, tr
	ml := &fakeML{}
	s.ML = ml

	w := replay(s, "?trace=417")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var out Outcome
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Action != "marked_absent" || out.Date != "2026-01-01" {
		t.Errorf("Outcome %+v, want marked_absent am 2026-01-01", out)
	}
	if st.absentUserID != "user-123" || st.absentDate.Format("2006-01-02") != "2026-01-01" {
		t.Errorf("MarkAbsent %q am %s", st.absentUserID, st.absentDate)
	}
	if len(tr.saved) != 1 || tr.saved[0].ReplayOf != 417 || out.TraceID != 901 {
		t.Errorf("Trace gespeichert %+v, TraceID %d", tr.saved, out.TraceID)
	}
	if len(ml.records) != 1 || ml.records[0].DedupeDay.Format("2006-01-02") != "2026-01-01" {
		t.Errorf("Protokoll %+v – soll für den 01.01. auf Doppelte prüfen", ml.records)
	}
	if snd.called || snd.imageCalled {
		t.Error("Nachverarbeitung hat etwas gesendet")
	}
}

// Es entscheidet, was jetzt eingestellt ist – hier das eigene Modell.
func TestReplayNutztAktuellenEntscheider(t *testing.T) {
	s, st, _ := newTestServer(classifier.Absage, thursday) // Gemini würde absagen
	tr := &fakeTraces{byID: map[int64]tracestore.Trace{5: recordedTrace(t, 5, "schau ma moi")}}
	s.Traces, s.Tracer = tr, tr
	s.ML = &fakeML{pred: shadow.Prediction{Label: "invalid", Confidence: 0.57, Model: "NB-LogReg 0.2.0"}}
	s.Entscheider = fakeEntscheider(sharedstore.EntscheiderModell)

	w := replay(s, "?trace=5")
	var out Outcome
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.DecidedBy != sharedstore.EntscheiderModell || out.Action != "none" || st.absentUserID != "" {
		t.Errorf("Outcome %+v, Absage %q – das Modell entscheidet", out, st.absentUserID)
	}
}

func TestReplayVerweigert(t *testing.T) {
	s, _, snd := newTestServer(classifier.Invalid, thursday)
	tr := &fakeTraces{byID: map[int64]tracestore.Trace{
		1: recordedTrace(t, 1, " Statistik "),
		2: {ID: 2, CreatedAt: thursday}, // ohne Payload
	}}
	s.Traces, s.Tracer = tr, tr

	for _, c := range []struct {
		query string
		want  int
	}{
		{"?trace=1", http.StatusUnprocessableEntity}, // würde in die Gruppe senden
		{"?trace=2", http.StatusUnprocessableEntity},
		{"?trace=3", http.StatusNotFound},
		{"?trace=abc", http.StatusBadRequest},
		{"", http.StatusBadRequest},
	} {
		if w := replay(s, c.query); w.Code != c.want {
			t.Errorf("%q: status %d, want %d", c.query, w.Code, c.want)
		}
	}
	if snd.called || snd.imageCalled || len(tr.saved) != 0 {
		t.Error("verweigerte Nachverarbeitung hat gesendet oder aufgezeichnet")
	}

	s.Traces = nil
	if w := replay(s, "?trace=1"); w.Code != http.StatusServiceUnavailable {
		t.Errorf("ohne Trace-Quelle: status %d", w.Code)
	}
}

// Ein Event von einem anderen Tag als Donnerstag bleibt auch nachverarbeitet
// ignoriert (gleiche Guards wie live).
func TestReplayFremderTagBleibtIgnoriert(t *testing.T) {
	s, st, _ := newTestServer(classifier.Absage, thursday)
	tr := recordedTrace(t, 7, "sag ab")
	tr.CreatedAt = friday
	traces := &fakeTraces{byID: map[int64]tracestore.Trace{7: tr}}
	s.Traces, s.Tracer = traces, traces

	w := replay(s, "?trace=7")
	var out Outcome
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.Path != "ignored" || st.absentUserID != "" || len(traces.saved) != 0 {
		t.Errorf("Outcome %+v, Absage %q, Traces %d", out, st.absentUserID, len(traces.saved))
	}
}
