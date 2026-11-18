package wirt

import (
	"strings"
	"testing"
	"time"
)

func TestStandardMarkiertDieNummer(t *testing.T) {
	text := Standard("491701234567")
	for _, want := range []string{Kopf, "Servus @491701234567 👋", "heute Abend unseren Tisch frei", Fuss} {
		if !strings.Contains(text, want) {
			t.Errorf("Text enthält %q nicht:\n%s", want, text)
		}
	}
	if !strings.HasSuffix(text, Fuss) {
		t.Errorf("Fußzeile steht nicht am Ende:\n%s", text)
	}
}

func TestStandardOhneNummerBleibtNeutral(t *testing.T) {
	text := Standard("")
	if !strings.Contains(text, "Servus 👋") || strings.Contains(text, Platzhalter) {
		t.Errorf("neutrale Anrede fehlt:\n%s", text)
	}
}

// Auch ein KI-Text bekommt Kopf, Markierung und Fußzeile vom Code.
func TestComposeRahmtDenKITextEin(t *testing.T) {
	text := Compose("Griaß di {wirt}! Halt uns bitte den Tisch frei 🙏", "498912345678")
	if !strings.HasPrefix(text, Kopf+"\n\n") || !strings.HasSuffix(text, "\n\n"+Fuss) {
		t.Errorf("Rahmen fehlt:\n%s", text)
	}
	if !strings.Contains(text, "Griaß di @498912345678!") {
		t.Errorf("Markierung fehlt:\n%s", text)
	}
}

func TestStandardTextBestehtDiePruefung(t *testing.T) {
	if err := Validate(StandardText); err != nil {
		t.Fatalf("Standardtext fällt durch: %v", err)
	}
}

func TestValidate(t *testing.T) {
	ok := "Servus {wirt} 👋 Halt uns heute Abend bitte unseren Tisch frei. Danke dir!"
	if err := Validate(ok); err != nil {
		t.Fatalf("gültiger Text fällt durch: %v", err)
	}
	cases := map[string]string{
		"leer":                "   ",
		"ohne Markierung":     "Servus! Halt uns heute Abend bitte unseren Tisch frei. Danke dir!",
		"doppelt markiert":    "Servus {wirt}, lieber {wirt}, halt uns bitte den Tisch frei. Danke!",
		"fremde Markierung":   "Servus {wirt} und @491701234567, haltet uns bitte den Tisch frei!",
		"fremder Platzhalter": "Servus {wirt}, {name} bittet dich: halt uns den Tisch frei. Danke!",
		"ohne Tisch":          "Servus {wirt} 👋 Wir kommen heute Abend wieder vorbei. Bis später!",
		"mit Link":            "Servus {wirt}, halt uns den Tisch frei, Infos auf https://example.org",
		"zu kurz":             "{wirt} Tisch!",
		"zu lang":             "Servus {wirt}, Tisch bitte. " + strings.Repeat("Sehr gern. ", 60),
		"zu viele Zeilen":     "Servus {wirt}\nTisch\nbitte\nheute\nAbend\nfrei\nhalten\ndanke\ndir\nsehr",
	}
	for name, text := range cases {
		if err := Validate(text); err == nil {
			t.Errorf("%s: sollte durchfallen:\n%s", name, text)
		}
	}
}

func TestNextVersand(t *testing.T) {
	loc := time.UTC
	at := func(day, hour int) time.Time { return time.Date(2026, 11, day, hour, 0, 0, 0, loc) }
	cases := []struct {
		now  time.Time
		want int // Tag im November 2026
	}{
		{at(16, 9), 19},  // Montag → Donnerstag derselben Woche
		{at(19, 9), 19},  // Donnerstag vormittags → heute
		{at(19, 15), 19}, // kurz vor dem Versand → heute
		{at(19, 16), 26}, // ab 16 Uhr → nächste Woche
		{at(20, 8), 26},  // Freitag → nächste Woche
	}
	for _, c := range cases {
		if got := NextVersand(c.now); got.Day() != c.want || got.Hour() != 0 {
			t.Errorf("NextVersand(%s) = %s, erwartet der %d.", c.now.Format("Mon 02.01. 15:04"), got.Format("02.01. 15:04"), c.want)
		}
	}
}
