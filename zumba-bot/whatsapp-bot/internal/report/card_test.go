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
		"Automatischer Wochenreport", // weekly-Header
		"31",                         // total aus erster Zeile (28+3)
		"Anna", "🥇",                  // Rangliste mit Medaille
		"width: 90.3%",               // Balkenbreite
		"❤️‍🔥&#43;9",                 // Streak-Tag > 7 ("+" HTML-escaped)
		"6x in Folge gefehlt", "30€", // Strafenblock
		"data:font/woff2;base64,", // eingebetteter Font
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
		html, err := BuildCardHTMLByStyle(s.ID, rows, entries, asOf, "", true)
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
		if _, err := BuildCardHTMLByStyle(s.ID, nil, nil, asOf, "", false); err != nil {
			t.Errorf("%s (leer): %v", s.ID, err)
		}
	}
}

func TestUnbekannterCardStyleFaelltAufLiveZurueck(t *testing.T) {
	rows := []store.Stat{{Name: "Anna", Attendance: 1, Away: 0, Percent: 100}}
	asOf := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)

	fallback, err := BuildCardHTMLByStyle("gibtsnicht", rows, nil, asOf, "", false)
	if err != nil {
		t.Fatal(err)
	}
	live, err := BuildCardHTMLByStyle(DefaultCardStyle, rows, nil, asOf, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if fallback != live {
		t.Error("unbekannter Stil muss das Live-Design liefern")
	}
}

// Die fünf Designs aus dem Statistik-Karten-Handoff rechnen sich ein paar
// Kennzahlen selbst (Ø-Quote, Sitzverteilung, offene Summe, Saison).
func TestHandoffCardStyles(t *testing.T) {
	rows := []store.Stat{
		{Name: "Anna", Attendance: 26, Away: 5, Percent: 83.9, Streak: 9},
		{Name: "Börni", Attendance: 10, Away: 21, Percent: 32.3, Streak: -4},
	}
	beglichen := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	entries := []penalty.Entry{
		{Name: "Börni", Betrag: 30, Tage: 6, Art: penalty.ArtFehltage, Status: penalty.StatusOffen},
		{Name: "Anna", Betrag: 50, Art: penalty.ArtNoShow, Datum: beglichen, Status: penalty.StatusBeglichen, BeglichenAm: &beglichen},
	}
	asOf := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)

	for style, wants := range map[string][]string{
		"abfahrtstafel": {"ZUMBA HBF", "❤️‍🔥 pünktlich +9", "🧊 fällt aus −4", "84%"},
		// OFFEN zählt nur die offene Strafe; Strichcode = Jahr Total Offen Ø.
		"kassenbon":     {"Bon-Nr. 31", "<span>OFFEN</span><span>30 EUR</span>", "2027 031 0030 058", "[−4]"},
		"gipfelbuch":    {"Saison 2027", "26 da · 5 weg", "❤️‍🔥 9 Etappen", "🧊 4 Wo. im Tal", "clip-path: polygon(0% 100%, 0% 22.1%, 0.0% 16.1%, 100.0% 67.7%, 100% 75.7%, 100% 100%)"},
		"wetterbericht": {"Ø 58 % im Saisonmittel", "🔥", "Rekordhitze", "🌨️", "Schnee", "26 da · 5 weg", "Serie +9", "Pause −4", "84°", "warnung stufe-2", "Stufe 2 · Markantes Wetter"},
		// 36 da / 26 weg → 105° des Halbkreises gelb.
		"hochrechnung": {"Stammtischwahl 2027", "#ffd23f 105deg", "DA 36", "WEG 26", "10 da · 21 weg", "−4,0", "+9,0", "absolute Mehrheit"},
	} {
		html, err := BuildCardHTMLByStyle(style, rows, entries, asOf, "2027", true)
		if err != nil {
			t.Fatalf("%s: %v", style, err)
		}
		for _, want := range wants {
			if !strings.Contains(html, want) {
				t.Errorf("%s: %q fehlt", style, want)
			}
		}

		leer, err := BuildCardHTMLByStyle(style, nil, nil, asOf, "", false)
		if err != nil {
			t.Fatalf("%s (leer): %v", style, err)
		}
		for _, want := range []string{"Keine Daten", "Keine offenen Strafen"} {
			if !strings.Contains(leer, want) {
				t.Errorf("%s (leer): %q fehlt", style, want)
			}
		}
	}
}

func TestCardOhneSaisonNimmtKalenderjahr(t *testing.T) {
	rows := []store.Stat{{Name: "Anna", Attendance: 1, Away: 0, Percent: 100}}
	html, err := BuildCardHTMLByStyle("gipfelbuch", rows, nil, time.Date(2026, 12, 3, 0, 0, 0, 0, time.UTC), "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "Saison 2026") {
		t.Error("ohne Saison-Label muss das Kalenderjahr von asOf erscheinen")
	}
}

func TestWetterSkala(t *testing.T) {
	for streak, want := range map[int]string{
		13: "Vulkan", 8: "Rekordhitze", 5: "Hitzewelle", 3: "sonnig", 2: "heiter", 1: "Regenbogen", 0: "wechselhaft",
		-1: "Schauer", -2: "Regen", -3: "Gewitter", -4: "Schnee", -5: "Dauerfrost", -9: "Dauerfrost", -10: "Eiszeit",
	} {
		if got := wetterFuer(streak).Text; got != want {
			t.Errorf("wetterFuer(%d) = %q, will %q", streak, got, want)
		}
	}
	for betrag, want := range map[int]int{25: 1, 30: 2, 45: 2, 50: 3, 95: 3, 100: 4} {
		if got := warnstufe(betrag); got != want {
			t.Errorf("warnstufe(%d) = %d, will %d", betrag, got, want)
		}
	}
}
