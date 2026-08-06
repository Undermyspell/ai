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
