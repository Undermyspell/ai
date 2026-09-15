package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/michael/zumba-whatsapp-bot/internal/classifier"
)

func notifyServer(t *testing.T, previewJID string) (http.Handler, *fakeSender) {
	t.Helper()
	s, _, snd := newTestServer(classifier.Invalid, time.Now())
	s.PreviewJID = previewJID
	return s.Routes(), snd
}

func postNotify(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/notify", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestNotifyGehtAnDieVorschauNummer(t *testing.T) {
	h, snd := notifyServer(t, "4917012345678@s.whatsapp.net")

	rec := postNotify(t, h, `{"text":"Wrapped ist offen: https://abc.ngrok-free.app"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d: %s", rec.Code, rec.Body.String())
	}
	if !snd.called {
		t.Fatal("es wurde nichts gesendet")
	}
	if snd.number != "4917012345678@s.whatsapp.net" {
		t.Fatalf("Empfänger = %q", snd.number)
	}
	if !strings.Contains(snd.text, "https://abc.ngrok-free.app") {
		t.Fatalf("Text = %q", snd.text)
	}
}

// Der Empfänger ist fest verdrahtet. Ein Request darf keinen eigenen
// mitbringen können — sonst wäre das ein offener Versandweg in die Gruppe.
func TestNotifyIgnoriertEinenEmpfaengerImRequest(t *testing.T) {
	h, snd := notifyServer(t, "4917012345678@s.whatsapp.net")

	rec := postNotify(t, h, `{"text":"hallo","number":"1234@g.us","to":"1234@g.us"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d", rec.Code)
	}
	if snd.number != "4917012345678@s.whatsapp.net" {
		t.Fatalf("Empfänger = %q, erwartet die Vorschau-Nummer", snd.number)
	}
}

func TestNotifyOhnePreviewJIDLehntAb(t *testing.T) {
	h, snd := notifyServer(t, "")

	rec := postNotify(t, h, `{"text":"hallo"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("Status = %d, erwartet 503", rec.Code)
	}
	if snd.called {
		t.Fatal("ohne Vorschau-Nummer darf nichts rausgehen")
	}
}

func TestNotifyLehntLeerenUndZuLangenTextAb(t *testing.T) {
	h, snd := notifyServer(t, "4917012345678@s.whatsapp.net")

	if rec := postNotify(t, h, `{"text":"   "}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("leerer Text: Status = %d, erwartet 400", rec.Code)
	}
	long := `{"text":"` + strings.Repeat("x", maxNotifyLen+1) + `"}`
	if rec := postNotify(t, h, long); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("langer Text: Status = %d, erwartet 413", rec.Code)
	}
	if snd.called {
		t.Fatal("es hätte nichts gesendet werden dürfen")
	}
}

func TestNotifyLehntKaputtesJsonAb(t *testing.T) {
	h, _ := notifyServer(t, "4917012345678@s.whatsapp.net")
	if rec := postNotify(t, h, `{`); rec.Code != http.StatusBadRequest {
		t.Fatalf("Status = %d, erwartet 400", rec.Code)
	}
}
