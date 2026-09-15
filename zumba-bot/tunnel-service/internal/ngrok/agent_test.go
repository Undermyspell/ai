package ngrok

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListFasstHttpUndHttpsZusammen(t *testing.T) {
	// Der Agent meldet einen http-Tunnel als zwei Einträge. Für uns ist das
	// ein Ziel mit einer https-URL.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"tunnels":[
			{"name":"wrapped (http)","public_url":"http://abc.ngrok-free.app","proto":"http","metrics":{"http":{"count":2}}},
			{"name":"wrapped","public_url":"https://abc.ngrok-free.app","proto":"https","metrics":{"http":{"count":5}}}
		]}`)
	}))
	defer srv.Close()

	got, err := New(srv.URL).List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Tunnel = %d, erwartet 1: %+v", len(got), got)
	}
	tun := got["wrapped"]
	if tun.URL != "https://abc.ngrok-free.app" {
		t.Fatalf("URL = %q, erwartet die https-Variante", tun.URL)
	}
	if tun.Requests != 7 {
		t.Fatalf("Requests = %d, erwartet 7 (beide Einträge)", tun.Requests)
	}
}

func TestStartSchicktDieErwarteteDefinition(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"name":"wrapped","public_url":"https://abc.ngrok-free.app","proto":"https"}`)
	}))
	defer srv.Close()

	url, err := New(srv.URL).Start(context.Background(), "wrapped", "zumba-wrapped:8080")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if url != "https://abc.ngrok-free.app" {
		t.Fatalf("URL = %q", url)
	}
	if body["addr"] != "zumba-wrapped:8080" || body["proto"] != "http" || body["name"] != "wrapped" {
		t.Fatalf("Request-Body = %+v", body)
	}
	// schemes=https: der Endpoint soll http gar nicht erst annehmen.
	schemes, _ := body["schemes"].([]any)
	if len(schemes) != 1 || schemes[0] != "https" {
		t.Fatalf("schemes = %+v, erwartet [https]", body["schemes"])
	}
	if body["inspect"] != false {
		t.Fatalf("inspect = %v, erwartet false", body["inspect"])
	}
}

func TestStartReichtDieAgentMeldungDurch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error_code":"ERR_NGROK_334","status_code":400,"msg":"failed to start tunnel","details":{"err":"endpoint already online\nmore detail"}}`)
	}))
	defer srv.Close()

	_, err := New(srv.URL).Start(context.Background(), "wrapped", "zumba-wrapped:8080")
	if err == nil {
		t.Fatal("Fehler erwartet")
	}
	if err.Error() != "endpoint already online" {
		t.Fatalf("Fehlertext = %q, erwartet die erste Zeile aus details.err", err.Error())
	}
}

func TestStopAkzeptiertBereitsGeschlossen(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/tunnels/wrapped") || r.Method != http.MethodDelete {
			t.Errorf("unerwarteter Aufruf: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if err := New(srv.URL).Stop(context.Background(), "wrapped"); err != nil {
		t.Fatalf("404 darf kein Fehler sein: %v", err)
	}
}

func TestStopMeldetEchteFehler(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"msg":"agent kaputt"}`)
	}))
	defer srv.Close()

	if err := New(srv.URL).Stop(context.Background(), "wrapped"); err == nil || err.Error() != "agent kaputt" {
		t.Fatalf("Fehler = %v, erwartet \"agent kaputt\"", err)
	}
}

func TestAgentNichtErreichbar(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // Port ist jetzt tot

	_, err := New(url).List(context.Background())
	if err == nil || !strings.Contains(err.Error(), "nicht erreichbar") {
		t.Fatalf("Fehler = %v, erwartet \"nicht erreichbar\"", err)
	}
}
