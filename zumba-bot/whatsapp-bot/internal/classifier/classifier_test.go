package classifier

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type fakeSource struct {
	primary, fallback string
	err               error
}

func (f fakeSource) ClassifierModels(context.Context) (string, string, error) {
	return f.primary, f.fallback, f.err
}

// fakeGemini antwortet je Modell mit einem festen Text; Modelle in fail
// bekommen 503. calls hält die angefragten Modelle in Reihenfolge fest.
func fakeGemini(t *testing.T, fail ...string) (*Gemini, *[]string) {
	t.Helper()
	var (
		mu    sync.Mutex
		calls []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		model := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/"), ":generateContent")
		mu.Lock()
		calls = append(calls, model)
		mu.Unlock()
		for _, f := range fail {
			if f == model {
				http.Error(w, `{"error":{"code":503}}`, http.StatusServiceUnavailable)
				return
			}
		}
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"false\n"}]}}]}`))
	}))
	t.Cleanup(srv.Close)
	g := NewGemini("key", "cfg-primary", "cfg-fallback")
	g.baseURL = srv.URL
	return g, &calls
}

func TestClassifyUsesModelSource(t *testing.T) {
	g, calls := fakeGemini(t)
	g.Models = fakeSource{primary: "db-primary", fallback: "db-fallback"}

	c, err := g.Classify(context.Background(), "Muss mi abmelden")
	if err != nil {
		t.Fatal(err)
	}
	if c.Result != Absage || c.Model != "db-primary" {
		t.Errorf("got %+v, want Absage von db-primary", c)
	}
	if got := strings.Join(*calls, ","); got != "db-primary" {
		t.Errorf("calls = %s", got)
	}
}

func TestClassifyFallbackFromModelSource(t *testing.T) {
	g, calls := fakeGemini(t, "db-primary")
	g.Models = fakeSource{primary: "db-primary", fallback: "db-fallback"}

	c, err := g.Classify(context.Background(), "Muss mi abmelden")
	if err != nil {
		t.Fatal(err)
	}
	if c.Model != "db-fallback" {
		t.Errorf("model = %s, want db-fallback", c.Model)
	}
	if got := strings.Join(*calls, ","); got != "db-primary,db-fallback" {
		t.Errorf("calls = %s", got)
	}
}

// Ist die Wahl nicht lesbar oder nur halb gesetzt, gilt für die Lücken die
// Konfiguration – klassifiziert wird in jedem Fall.
func TestClassifyModelSourceGaps(t *testing.T) {
	cases := []struct {
		name string
		src  fakeSource
		want string
	}{
		{"Fehler", fakeSource{err: errors.New("db weg")}, "cfg-primary,cfg-fallback"},
		{"nur Fallback gesetzt", fakeSource{fallback: "db-fallback"}, "cfg-primary,db-fallback"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, calls := fakeGemini(t, "cfg-primary")
			g.Models = tc.src
			if _, err := g.Classify(context.Background(), "x"); err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(*calls, ","); got != tc.want {
				t.Errorf("calls = %s, want %s", got, tc.want)
			}
		})
	}
}
