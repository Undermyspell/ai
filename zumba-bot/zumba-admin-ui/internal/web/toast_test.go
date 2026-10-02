package web

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// Der Browser liest Header als Latin-1: der Toast muss als reines ASCII
// raus und beim JSON-Parsen trotzdem die Originalnachricht ergeben.
func TestTriggerToastASCII(t *testing.T) {
	msg := `Hauptmodell: Gemma – gilt ab der nächsten Nachricht 🍺 "x\y"`
	rec := httptest.NewRecorder()
	(&Server{}).triggerToast(rec, "success", msg)

	h := rec.Header().Get("HX-Trigger")
	for i := 0; i < len(h); i++ {
		if h[i] >= 0x80 {
			t.Fatalf("Header enthält Nicht-ASCII-Byte an %d: %q", i, h)
		}
	}
	var got struct {
		ShowToast struct{ Level, Msg string } `json:"showToast"`
	}
	if err := json.Unmarshal([]byte(h), &got); err != nil {
		t.Fatalf("kein gültiges JSON: %v (%s)", err, h)
	}
	if got.ShowToast.Msg != msg || got.ShowToast.Level != "success" {
		t.Errorf("got %+v", got.ShowToast)
	}
}
