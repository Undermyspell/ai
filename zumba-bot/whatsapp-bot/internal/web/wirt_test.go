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

const (
	testPreview = "4917012345678@s.whatsapp.net"
	testWirtNr  = "498912345678"
)

// mentionSender ist ein fakeSender, der zusätzlich markieren kann.
type mentionSender struct {
	fakeSender
	mentioned []string
}

func (m *mentionSender) SendTextMention(_ context.Context, number, text string, mentioned []string) error {
	m.called, m.number, m.text, m.mentioned = true, number, text, mentioned
	return nil
}

func wirtServer(previewJID string) (*Server, *mentionSender) {
	monday := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	s, _, _ := newTestServer(classifier.Invalid, monday)
	snd := &mentionSender{}
	s.sender = snd
	s.PreviewJID = previewJID
	s.WirtJID = testWirtNr + "@s.whatsapp.net"
	return s, snd
}

func postWirt(t *testing.T, s *Server, query string) (*httptest.ResponseRecorder, WirtOutcome) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/wirt-reminder"+query, nil))
	var out WirtOutcome
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("Antwort nicht lesbar: %v", err)
		}
	}
	return rec, out
}

func TestWirtDryRunSendetNichtUndMarkiertDenWirt(t *testing.T) {
	s, snd := wirtServer(testPreview)
	rec, out := postWirt(t, s, "?dryRun=true")
	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d: %s", rec.Code, rec.Body.String())
	}
	if snd.called {
		t.Fatal("Dry-Run darf nichts senden")
	}
	if !out.DryRun || out.Mentioned != testWirtNr || out.Recipient != testGroup {
		t.Errorf("Outcome = %+v", out)
	}
	if !strings.Contains(out.Message, "@"+testWirtNr) {
		t.Errorf("Wirt ist nicht markiert:\n%s", out.Message)
	}
}

// Die Vorschau geht an die eigene Nummer und markiert sie – weder die Gruppe
// noch der Wirt bekommen davon etwas mit.
func TestWirtVorschauGehtAnDieEigeneNummerUndMarkiertSie(t *testing.T) {
	s, snd := wirtServer(testPreview)
	rec, out := postWirt(t, s, "?preview=true")
	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d: %s", rec.Code, rec.Body.String())
	}
	if !snd.called || snd.number != testPreview {
		t.Fatalf("Empfänger = %q, erwartet die Vorschau-Nummer", snd.number)
	}
	if len(snd.mentioned) != 1 || snd.mentioned[0] != "4917012345678" {
		t.Errorf("mentioned = %v", snd.mentioned)
	}
	if !strings.Contains(snd.text, "@4917012345678") || strings.Contains(snd.text, testWirtNr) {
		t.Errorf("Text markiert nicht (nur) die eigene Nummer:\n%s", snd.text)
	}
	if out.DryRun || out.PreviewTo != testPreview {
		t.Errorf("Outcome = %+v", out)
	}
}

// Es gibt noch keinen echten Versand: ohne Parameter bleibt es beim Dry-Run.
func TestWirtOhneParameterSendetNicht(t *testing.T) {
	s, snd := wirtServer(testPreview)
	_, out := postWirt(t, s, "")
	if snd.called || !out.DryRun {
		t.Fatalf("ohne dryRun/preview darf nichts rausgehen (called=%v, out=%+v)", snd.called, out)
	}
}

func TestWirtVorschauOhnePreviewJIDLehntAb(t *testing.T) {
	s, snd := wirtServer("")
	rec, _ := postWirt(t, s, "?preview=true")
	if rec.Code != http.StatusServiceUnavailable || snd.called {
		t.Fatalf("Status = %d, called = %v", rec.Code, snd.called)
	}
}

func TestWirtOhneWirtJIDBleibtNeutral(t *testing.T) {
	s, _ := wirtServer(testPreview)
	s.WirtJID = ""
	_, out := postWirt(t, s, "?dryRun=true")
	if out.Mentioned != "" || !strings.Contains(out.Message, "Servus 👋") {
		t.Errorf("Outcome = %+v", out)
	}
}

// Mit previewTarget=group geht die Vorschau in die Testgruppe – markiert
// bleibt die eigene Nummer, die Stammtisch-Gruppe bekommt nichts.
func TestWirtVorschauGehtInDieTestgruppe(t *testing.T) {
	const testgruppe = "120363000000000000@g.us"
	s, snd := wirtServer(testPreview)
	s.PreviewGroupJID = testgruppe
	_, out := postWirt(t, s, "?preview=true&previewTarget=group")
	if snd.number != testgruppe || out.PreviewTo != testgruppe {
		t.Fatalf("Empfänger = %q, PreviewTo = %q", snd.number, out.PreviewTo)
	}
	if len(snd.mentioned) != 1 || snd.mentioned[0] != "4917012345678" {
		t.Errorf("mentioned = %v", snd.mentioned)
	}
}

// Ohne previewTarget bleibt es beim 1:1-Chat, auch wenn eine Testgruppe
// konfiguriert ist; ohne konfigurierte Testgruppe lehnt group ab.
func TestWirtVorschauZielIstWaehlbar(t *testing.T) {
	s, snd := wirtServer(testPreview)
	s.PreviewGroupJID = "120363000000000000@g.us"
	if _, out := postWirt(t, s, "?preview=true"); snd.number != testPreview || out.PreviewTo != testPreview {
		t.Errorf("ohne previewTarget: Empfänger = %q", snd.number)
	}

	s, snd = wirtServer(testPreview)
	rec, _ := postWirt(t, s, "?preview=true&previewTarget=group")
	if rec.Code != http.StatusServiceUnavailable || snd.called {
		t.Errorf("ohne Testgruppe: Status = %d, called = %v", rec.Code, snd.called)
	}
}

// Die Testgruppe ist ein Vorschau-Ziel für alle Testwege, nicht nur für die
// Tisch-Erinnerung – hier der Wochenreport.
func TestWochenreportVorschauGehtInDieTestgruppe(t *testing.T) {
	const testgruppe = "120363000000000000@g.us"
	s, _, snd := newTestServer(classifier.Invalid, thursday)
	s.PreviewJID = testPreview
	s.PreviewGroupJID = testgruppe

	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/weekly-report?preview=true&previewTarget=group", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d: %s", rec.Code, rec.Body.String())
	}
	if !snd.called || snd.number != testgruppe {
		t.Fatalf("Empfänger = %q, erwartet die Testgruppe", snd.number)
	}
	if !strings.Contains(rec.Body.String(), `"previewTo":"`+testgruppe+`"`) {
		t.Errorf("Antwort = %s", rec.Body.String())
	}
}
