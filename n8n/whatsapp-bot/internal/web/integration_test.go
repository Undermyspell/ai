package web

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/michael/zumba-whatsapp-bot/internal/classifier"
	"github.com/michael/zumba-whatsapp-bot/internal/sink"
	"github.com/michael/zumba-whatsapp-bot/internal/store"
)

// statsStore liefert deterministische Statistik für den End-to-End-Test.
type statsStore struct{ fakeStore }

func (*statsStore) UserStats(context.Context) ([]store.Stat, error) {
	return []store.Stat{
		{Name: "Anna", Attendance: 10, Away: 0, Percent: 100, Streak: 5},
		{Name: "Ben", Attendance: 8, Away: 2, Percent: 80, Streak: -2},
	}, nil
}

// TestWebhookStatistikEndToEnd schickt die echte Beispiel-JSON
// (reference/example-requests/statistik.json) durch den HTTP-Handler und prüft,
// dass das gerenderte Ergebnis im Sink landet – komplett ohne Evolution API.
func TestWebhookStatistikEndToEnd(t *testing.T) {
	payload, err := os.ReadFile(filepath.Join("..", "..", "reference", "example-requests", "statistik.json"))
	if err != nil {
		t.Fatalf("read example: %v", err)
	}

	var buf bytes.Buffer
	loc, _ := time.LoadLocation("Europe/Berlin")
	s := New(&statsStore{}, fakeClassifier{result: classifier.Invalid}, sink.NewWriter(&buf), testGroup, loc)

	srv := httptest.NewServer(s.Routes())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/webhook/whatsapp", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	out := buf.String()
	for _, want := range []string{
		"491717868843-1443438520@g.us", // Empfänger (remoteJid aus der Beispiel-JSON)
		"🍻 *ZUMBA STATS*",
		"🥇 *Anna*",
		"🥈 *Ben*",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Sink-Output enthält %q nicht.\n--- Output ---\n%s", want, out)
		}
	}
}
