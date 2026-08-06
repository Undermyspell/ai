package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/michael/zumba-whatsapp-bot/internal/classifier"
)

type fakeRenderer struct {
	html   string
	called bool
	err    error
}

func (f *fakeRenderer) PNG(_ context.Context, html string, _ int) ([]byte, error) {
	f.called = true
	f.html = html
	return []byte("PNGDATA"), f.err
}

func statistikJSON() string {
	ev := groupMsg("statistik")
	buf, _ := json.Marshal(ev)
	return string(buf)
}

func TestTestEndpointFormatImage(t *testing.T) {
	s, _, snd := newTestServer(classifier.Invalid, friday)
	rnd := &fakeRenderer{}
	s.Renderer = rnd

	req := httptest.NewRequest("POST", "/test?format=image", strings.NewReader(statistikJSON()))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d: %s", rec.Code, rec.Body.String())
	}
	var out Outcome
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !rnd.called {
		t.Error("Renderer nicht aufgerufen")
	}
	if !strings.Contains(rnd.html, "ZUMBA") {
		t.Error("Karten-HTML sieht falsch aus")
	}
	if out.ImageBase64 == "" {
		t.Error("ImageBase64 fehlt")
	}
	if snd.called || snd.imageCalled {
		t.Error("Dry-Run darf nichts senden")
	}
}

func TestTestEndpointFormatImageOhneRenderer(t *testing.T) {
	s, _, _ := newTestServer(classifier.Invalid, friday)

	req := httptest.NewRequest("POST", "/test?format=image", strings.NewReader(statistikJSON()))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("code = %d, want 502", rec.Code)
	}
}

func TestWeeklyFormatImagePreviewSendetBild(t *testing.T) {
	s, st, snd := newTestServer(classifier.Invalid, time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	st.penaltyInput = penaltyFixture()
	s.Renderer = &fakeRenderer{}
	s.PreviewJID = "49123@s.whatsapp.net"

	req := httptest.NewRequest("POST", "/weekly-report?preview=true&format=image", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d: %s", rec.Code, rec.Body.String())
	}
	if snd.called {
		t.Error("bei format=image darf kein Text gesendet werden")
	}
	if !snd.imageCalled || snd.imageNumber != s.PreviewJID {
		t.Errorf("Bild-Vorschau nicht gesendet: %+v", snd)
	}
	if len(st.autoStrafen) != 0 {
		t.Errorf("Vorschau darf nicht persistieren: %v", st.autoStrafen)
	}
}
