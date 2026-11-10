package report

import (
	"strings"
	"testing"
)

func TestBuildWirtReminderMarkiertDieNummer(t *testing.T) {
	text := BuildWirtReminder("491701234567")
	for _, want := range []string{"Servus @491701234567 👋", "heute Abend unseren Tisch frei", "🤖🍺 *Automatische Erinnerung vom Zumba-Bot*"} {
		if !strings.Contains(text, want) {
			t.Errorf("Text enthält %q nicht:\n%s", want, text)
		}
	}
}

func TestBuildWirtReminderOhneNummerBleibtNeutral(t *testing.T) {
	if text := BuildWirtReminder(""); !strings.Contains(text, "Servus 👋") {
		t.Errorf("neutrale Anrede fehlt:\n%s", text)
	}
}
