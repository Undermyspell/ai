package web

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/michael/zumba-admin-ui/internal/config"
	"github.com/michael/zumba-admin-ui/internal/store"
	"github.com/michael/zumba-admin-ui/web/templates/trace"
)

// Der Button schickt nur die ID an den Bot und zeigt dessen Ergebnis samt
// Link auf den neuen Verlauf.
func TestTraceReplayRuftBotUndZeigtErgebnis(t *testing.T) {
	var got string
	bot := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Method + " " + r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"path":"classify","classification":"false","action":"marked_absent",
			"date":"2026-11-19","decidedBy":"modell","traceId":418}`))
	}))
	defer bot.Close()
	h := New(newSpyStore(), config.Config{BotURL: bot.URL}, false).Routes()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/trace/417/replay", nil))

	if got != "POST /replay?trace=417" {
		t.Errorf("Bot-Aufruf %q", got)
	}
	body := rec.Body.String()
	for _, want := range []string{"Absage eingetragen", "eigenes Modell", "19.11.2026", `href="/trace/418"`} {
		if !strings.Contains(body, want) {
			t.Errorf("Antwort enthält %q nicht:\n%s", want, body)
		}
	}
	if !strings.Contains(rec.Header().Get("HX-Trigger"), "success") {
		t.Errorf("kein Erfolgs-Toast: %q", rec.Header().Get("HX-Trigger"))
	}
}

// Verweigert der Bot (z. B. "statistik"), steht sein Grund im Panel.
func TestTraceReplayZeigtGrundDesBots(t *testing.T) {
	bot := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "zu diesem Event ist keine Payload gespeichert", http.StatusUnprocessableEntity)
	}))
	defer bot.Close()
	h := New(newSpyStore(), config.Config{BotURL: bot.URL}, false).Routes()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/trace/5/replay", nil))
	if !strings.Contains(rec.Body.String(), "keine Payload gespeichert") || !strings.Contains(rec.Header().Get("HX-Trigger"), "error") {
		t.Errorf("Fehler nicht gezeigt: %s / %q", rec.Body.String(), rec.Header().Get("HX-Trigger"))
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/trace/abc/replay", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("ungültige ID: status %d", rec.Code)
	}
}

func renderDetail(t *testing.T, tr store.Trace) string {
	t.Helper()
	var buf bytes.Buffer
	if err := trace.Detail(tr).Render(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// Den Button gibt es nur mit gespeicherter Payload und nie für "statistik";
// eine Nachverarbeitung verlinkt ihr Original.
func TestTraceDetailReplayButton(t *testing.T) {
	at := time.Date(2026, 11, 19, 16, 56, 0, 0, time.Local)
	live := store.Trace{ID: 417, CreatedAt: at, Path: "classify", RawPayload: "{}"}
	if b := renderDetail(t, live); !strings.Contains(b, `hx-post="/trace/417/replay"`) || !strings.Contains(b, "Do, 19.11.") {
		t.Errorf("Button fehlt:\n%s", b)
	}
	for _, tr := range []store.Trace{
		{ID: 1, CreatedAt: at, Path: "statistik", RawPayload: "{}"},
		{ID: 2, CreatedAt: at, Path: "classify"},
	} {
		if strings.Contains(renderDetail(t, tr), "/replay") {
			t.Errorf("Button bei %+v", tr)
		}
	}
	replay := store.Trace{ID: 418, CreatedAt: at, Path: "classify", RawPayload: "{}", ReplayOf: 417}
	if b := renderDetail(t, replay); !strings.Contains(b, `href="/trace/417"`) {
		t.Errorf("Verweis aufs Original fehlt:\n%s", b)
	}
}
