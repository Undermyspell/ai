package report

import (
	"strings"
	"testing"
	"time"

	"github.com/michael/zumba-shared/penalty"
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

func TestBuildWithStrafenPlatzierung(t *testing.T) {
	rows := []store.Stat{{Name: "Anna", Attendance: 10, Percent: 100}}
	asOf := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC) // Donnerstag
	entries := []penalty.Entry{
		{Name: "Ben", Art: penalty.ArtFehltage, Tage: 6, Betrag: 30, Status: penalty.StatusOffen},
		{Name: "Carl", Art: penalty.ArtNoShow, Betrag: 50, Status: penalty.StatusOffen,
			Datum: time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)},
	}
	got := BuildWithStrafen(rows, StrafenBlock(entries, asOf))

	blockIdx := strings.Index(got, "── 💸 *STRAFEN* ──")
	rangIdx := strings.Index(got, "── *RANGLISTE* ──")
	footIdx := strings.Index(got, "🤖🍺 *Automatisch erstellt vom Zumba-Bot*")
	if blockIdx == -1 || rangIdx == -1 || footIdx == -1 {
		t.Fatalf("Abschnitte fehlen:\n%s", got)
	}
	if !(rangIdx < blockIdx && blockIdx < footIdx) {
		t.Errorf("Strafenblock nicht zwischen Rangliste und Abschlusszeile:\n%s", got)
	}
	for _, want := range []string{
		"⚠️ *Ben* – 30€ (6x in Folge gefehlt)",
		"⚠️ *Carl* – 50€ (nicht abgemeldet, 23.7.)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Zeile fehlt: %q\n%s", want, got)
		}
	}
}

func TestStrafenBlockLeer(t *testing.T) {
	block := StrafenBlock(nil, time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(block, "_Keine offenen Strafen_") {
		t.Errorf("Leermeldung fehlt: %q", block)
	}
}

func TestStrafenBlockBeglichenNurImFenster(t *testing.T) {
	beglichen := time.Date(2026, 7, 24, 18, 0, 0, 0, time.UTC) // Freitag
	e := penalty.Entry{Name: "Dora", Art: penalty.ArtFehltage, Tage: 5, Betrag: 25,
		Status: penalty.StatusBeglichen, BeglichenAm: &beglichen}

	inWindow := StrafenBlock([]penalty.Entry{e}, time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(inWindow, "✅ *Dora* – 25€ beglichen (5x in Folge gefehlt)") {
		t.Errorf("beglichene Strafe fehlt am Folgedonnerstag: %q", inWindow)
	}
	after := StrafenBlock([]penalty.Entry{e}, time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC))
	if strings.Contains(after, "Dora") {
		t.Errorf("beglichene Strafe nach dem Folgedonnerstag noch sichtbar: %q", after)
	}
}
