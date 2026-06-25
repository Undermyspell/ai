package report

import (
	"strings"
	"testing"

	"github.com/michael/zumba-whatsapp-bot/internal/store"
)

func TestBuild(t *testing.T) {
	rows := []store.Stat{
		{Name: "A", Attendance: 10, Away: 0, Percent: 100, Streak: 5},
		{Name: "B", Attendance: 8, Away: 2, Percent: 80, Streak: -2},
		{Name: "C", Attendance: 8, Away: 2, Percent: 80, Streak: 1},
	}

	want := "🍻 *ZUMBA STATS*\n" +
		"_Weihnachtsfeier → Weihnachtsfeier_\n\n" +
		"📊 *10* Stammtische\n\n" +
		"🐐 *GOAT:* A (100%)\n" +
		"🔥 *Heißeste Serie:* A (5x)\n" +
		"❄️ *Längste Pause:* B (2x)\n" +
		"\n── *RANGLISTE* ──\n\n" +
		"🥇 *A* ▰▰▰▰▰▰ 10-0 (100%) 🔥+5\n" +
		"🥈 *B* ▰▰▰▰▰▱ 8-2 (80%) ❄️-2\n" +
		"🥈 *C* ▰▰▰▰▰▱ 8-2 (80%) 🔥+1\n\n" +
		"🤖🍺 *Automatisch erstellt vom Zumba-Bot*"

	got := Build(rows)
	if got != want {
		t.Errorf("Build mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// Gleichstand bei der heißesten Serie: alle Namen müssen genannt werden.
func TestBuildTiedStreaks(t *testing.T) {
	rows := []store.Stat{
		{Name: "A", Attendance: 10, Away: 0, Percent: 100, Streak: 4},
		{Name: "B", Attendance: 9, Away: 1, Percent: 90, Streak: 4},
		{Name: "C", Attendance: 4, Away: 6, Percent: 40, Streak: -2},
		{Name: "D", Attendance: 4, Away: 6, Percent: 40, Streak: -2},
	}
	got := Build(rows)
	if !strings.Contains(got, "🔥 *Heißeste Serie:* A, B (4x)") {
		t.Errorf("erwartete 'A, B (4x)' in heißester Serie, bekam:\n%s", got)
	}
	if !strings.Contains(got, "❄️ *Längste Pause:* C, D (2x)") {
		t.Errorf("erwartete 'C, D (2x)' in längster Pause, bekam:\n%s", got)
	}
}

func TestBarChart(t *testing.T) {
	cases := []struct {
		percent float64
		want    string
	}{
		{100, "▰▰▰▰▰▰"},
		{80, "▰▰▰▰▰▱"}, // round(4.8) = 5
		{0, "▱▱▱▱▱▱"},
		{50, "▰▰▰▱▱▱"}, // round(3.0) = 3
	}
	for _, c := range cases {
		if got := barChart(c.percent, 6); got != c.want {
			t.Errorf("barChart(%v) = %q, want %q", c.percent, got, c.want)
		}
	}
}

func TestFmtNum(t *testing.T) {
	cases := map[float64]string{
		100:   "100",
		85:    "85",
		85.7:  "85.7",
		85.71: "85.71",
	}
	for in, want := range cases {
		if got := fmtNum(in); got != want {
			t.Errorf("fmtNum(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildEmpty(t *testing.T) {
	if got := Build(nil); got == "" {
		t.Error("Build(nil) should not panic or return empty")
	}
}
