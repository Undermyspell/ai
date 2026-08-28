package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/michael/zumba-whatsapp-bot/internal/classifier"
	"github.com/michael/zumba-whatsapp-bot/internal/report"
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

func TestStatsFormatImageSendetBildAnGruppe(t *testing.T) {
	s, _, snd := newTestServer(classifier.Invalid, friday)
	s.Renderer = &fakeRenderer{}
	s.StatsFormat = "image"

	out := s.run(context.Background(), groupMsg("statistik"), false, false, s.today())
	if out.Path != "statistik" {
		t.Fatalf("Path = %q", out.Path)
	}
	if !snd.imageCalled || snd.imageNumber != testGroup {
		t.Errorf("Bild nicht an Gruppe gesendet: %+v", snd)
	}
	if snd.called {
		t.Error("bei erfolgreichem Bild darf kein Text gesendet werden")
	}
}

func TestStatsFormatImageFallbackAufText(t *testing.T) {
	s, _, snd := newTestServer(classifier.Invalid, friday)
	s.Renderer = &fakeRenderer{err: errRender}
	s.StatsFormat = "image"

	s.run(context.Background(), groupMsg("statistik"), false, false, s.today())
	if !snd.called {
		t.Error("Render-Fehler muss auf Text zurückfallen")
	}
}

var errRender = errors.New("chromium kaputt")

func TestWeeklyImageRenderFehlerFallbackText(t *testing.T) {
	s, st, snd := newTestServer(classifier.Invalid, time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	st.penaltyInput = penaltyFixture()
	s.Renderer = &fakeRenderer{err: errRender}

	req := httptest.NewRequest("POST", "/weekly-report?format=image", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d: %s", rec.Code, rec.Body.String())
	}
	if snd.imageCalled {
		t.Error("Bild darf bei Render-Fehler nicht gesendet werden")
	}
	if !snd.called {
		t.Error("Render-Fehler muss auf Text-Versand zurückfallen")
	}
}

func TestWeeklyImageSendFehlerFallbackText(t *testing.T) {
	s, st, snd := newTestServer(classifier.Invalid, time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	st.penaltyInput = penaltyFixture()
	s.Renderer = &fakeRenderer{}
	snd.imageErr = errRender

	req := httptest.NewRequest("POST", "/weekly-report?format=image", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d: %s", rec.Code, rec.Body.String())
	}
	if !snd.called {
		t.Error("SendImage-Fehler muss auf Text-Versand zurückfallen")
	}
}

// marker ist ein Textschnipsel, der nur im HTML des jeweiligen Designs steht.
var marker = map[string]string{
	"wrapped":  "ZUMBA STATS",
	"zeitung":  "Der Zumba-Anzeiger",
	"arena":    "MATCHDAY",
	"formular": "ANWESENHEITSNACHWEIS",
}

// Ohne ?cardStyle bestimmt die Rotation das Design des Wochenreports.
func TestWochenreportNimmtDesignAusRotation(t *testing.T) {
	s, st, _ := newTestServer(classifier.Invalid, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
	st.penaltyInput = penaltyFixture()
	rnd := &fakeRenderer{}
	s.Renderer = rnd
	s.Cards = report.NewCardRotation([]string{"wrapped", "zeitung", "arena", "formular"})

	// ?date= erzwingt Dry-Run – der Stichtag bestimmt trotzdem das Design.
	req := httptest.NewRequest("POST", "/weekly-report?format=image&date=2026-08-06", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d: %s", rec.Code, rec.Body.String())
	}
	want := s.Cards.ForWeek(time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(rnd.html, marker[want]) {
		t.Errorf("Wochenreport nicht im Design %q gerendert", want)
	}
}

// Der Bot-Test wählt weiterhin selbst: ?cardStyle schlägt die Rotation.
func TestWochenreportCardStyleSchlaegtRotation(t *testing.T) {
	s, st, _ := newTestServer(classifier.Invalid, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC))
	st.penaltyInput = penaltyFixture()
	rnd := &fakeRenderer{}
	s.Renderer = rnd
	s.Cards = report.NewCardRotation([]string{"formular"})

	req := httptest.NewRequest("POST", "/weekly-report?format=image&cardStyle=zeitung&date=2026-08-06", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rnd.html, marker["zeitung"]) {
		t.Error("ausdrückliches cardStyle wurde übergangen")
	}
}

// Die "statistik" in der Gruppe zieht ihr Design aus derselben Auswahl.
func TestStatistikBildNimmtDesignAusRotation(t *testing.T) {
	s, _, snd := newTestServer(classifier.Invalid, thursday)
	rnd := &fakeRenderer{}
	s.Renderer = rnd
	s.StatsFormat = "image"
	s.Cards = report.NewCardRotation([]string{"arena"})

	req := httptest.NewRequest("POST", "/webhook/whatsapp", strings.NewReader(statistikJSON()))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)

	if !snd.imageCalled {
		t.Fatal("kein Bild gesendet")
	}
	if !strings.Contains(rnd.html, marker["arena"]) {
		t.Error("Statistik nicht im Design der Rotation gerendert")
	}
}
