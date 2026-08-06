package report

import (
	"strings"
	"testing"
	"time"

	"github.com/michael/zumba-shared/penalty"
	"github.com/michael/zumba-whatsapp-bot/internal/store"
)

func TestBuildCardHTML(t *testing.T) {
	rows := []store.Stat{
		{Name: "Anna", Attendance: 28, Away: 3, Percent: 90.3, Streak: 9},
		{Name: "Börni", Attendance: 26, Away: 5, Percent: 83.9, Streak: 4},
		{Name: "Chris", Attendance: 24, Away: 7, Percent: 77.4, Streak: -1},
		{Name: "Didi", Attendance: 20, Away: 11, Percent: 64.5, Streak: -5},
		{Name: "Emil", Attendance: 20, Away: 11, Percent: 64.5, Streak: 2},
	}
	entries := []penalty.Entry{
		{Name: "Didi", Betrag: 30, Tage: 6, Art: penalty.ArtFehltage, Status: penalty.StatusOffen},
	}
	asOf := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)

	html, err := BuildCardHTML(rows, entries, asOf, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Automatischer Wochenreport",     // weekly-Header
		"31",                             // total aus erster Zeile (28+3)
		"Anna", "🥇",                      // Rangliste mit Medaille
		"width: 90.3%",                   // Balkenbreite
		"❤️‍🔥&#43;9",                      // Streak-Tag > 7 ("+" HTML-escaped)
		"6x in Folge gefehlt", "30€",     // Strafenblock
		"data:font/woff2;base64,",        // eingebetteter Font
	} {
		if !strings.Contains(html, want) {
			t.Errorf("Karte enthält %q nicht", want)
		}
	}
}

func TestBuildCardHTMLLeer(t *testing.T) {
	html, err := BuildCardHTML(nil, nil, time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "Keine Daten.") {
		t.Error("Leer-Karte ohne 'Keine Daten.'")
	}
	if !strings.Contains(html, "Keine offenen Strafen") {
		t.Error("Leer-Karte ohne Strafen-Fallback")
	}
	if strings.Contains(html, "Wochenreport") {
		t.Error("weekly=false darf keinen Wochenreport-Header haben")
	}
}

func TestAlleCardStylesRendern(t *testing.T) {
	rows := []store.Stat{
		{Name: "Anna", Attendance: 26, Away: 5, Percent: 83.9, Streak: 9},
		{Name: "Börni", Attendance: 10, Away: 21, Percent: 32.3, Streak: -4},
	}
	entries := []penalty.Entry{
		{Name: "Börni", Betrag: 30, Tage: 6, Art: penalty.ArtFehltage, Status: penalty.StatusOffen},
	}
	asOf := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)

	for _, s := range CardStyles() {
		html, err := BuildCardHTMLByStyle(s.ID, rows, entries, asOf, true)
		if err != nil {
			t.Fatalf("%s: %v", s.ID, err)
		}
		for _, want := range []string{"Anna", "Börni", "30", "data:font/woff2;base64,"} {
			if !strings.Contains(html, want) {
				t.Errorf("%s: %q fehlt", s.ID, want)
			}
		}
		// Keine "-3"-Doppelminus: die Designs setzen das Vorzeichen selbst.
		if strings.Contains(html, "Pause -") {
			t.Errorf("%s: doppeltes Minus in der Pausen-Anzeige", s.ID)
		}
		if _, err := BuildCardHTMLByStyle(s.ID, nil, nil, asOf, false); err != nil {
			t.Errorf("%s (leer): %v", s.ID, err)
		}
	}
}

func TestUnbekannterCardStyleFaelltAufLiveZurueck(t *testing.T) {
	rows := []store.Stat{{Name: "Anna", Attendance: 1, Away: 0, Percent: 100}}
	asOf := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)

	fallback, err := BuildCardHTMLByStyle("gibtsnicht", rows, nil, asOf, false)
	if err != nil {
		t.Fatal(err)
	}
	live, err := BuildCardHTMLByStyle(DefaultCardStyle, rows, nil, asOf, false)
	if err != nil {
		t.Fatal(err)
	}
	if fallback != live {
		t.Error("unbekannter Stil muss das Live-Design liefern")
	}
}
