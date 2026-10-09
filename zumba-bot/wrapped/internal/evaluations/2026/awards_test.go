package eval2026

import (
	"testing"
	"time"

	"github.com/michael/stammtisch-wrapped/pkg/models"
)

// testThursdays liefert n aufeinanderfolgende Donnerstage ab dem 04.12.2025.
func testThursdays(n int) []time.Time {
	start := time.Date(2025, 12, 4, 0, 0, 0, 0, time.UTC)
	out := make([]time.Time, n)
	for i := range out {
		out[i] = start.AddDate(0, 0, 7*i)
	}
	return out
}

// absentOn baut Absagen an den angegebenen Donnerstag-Indizes.
func absentOn(ths []time.Time, category string, idx ...int) []models.Cancellation {
	out := make([]models.Cancellation, len(idx))
	for i, k := range idx {
		out[i] = models.Cancellation{Date: ths[k], Category: category}
	}
	return out
}

func awardByID(awards []models.Award, id string) *models.Award {
	for i := range awards {
		if awards[i].ID == id {
			return &awards[i]
		}
	}
	return nil
}

func winnerNames(a *models.Award) []string {
	if a == nil {
		return nil
	}
	names := make([]string, len(a.Winners))
	for i, w := range a.Winners {
		names[i] = w.Name
	}
	return names
}

func TestCalculateAwards(t *testing.T) {
	ths := testThursdays(16)
	since := ths[0]
	user := func(id int, name string, rank int) models.UserStats {
		return models.UserStats{User: models.User{ID: id, Name: name}, Rank: rank, Since: since}
	}

	// Anna & Ben: nie gefehlt, gleichauf auf Platz 1.
	anna, ben := user(1, "Anna", 1), user(2, "Ben", 1)
	anna.AttendanceRate, ben.AttendanceRate = 100, 100
	anna.MaxAttendanceStreak, ben.MaxAttendanceStreak = 16, 16

	// Chris: 6 Donnerstage am Stück weg (Index 2–7), danach wieder da.
	chris := user(3, "Chris", 4)
	chris.Cancellations = absentOn(ths, "arbeit", 2, 3, 4, 5, 6, 7)
	chris.MaxCancellationStreak, chris.MaxCancellationStreakEnd = 6, ths[7]

	// Dora: 4 am Stück weg (Index 1–4) und zurück → Comeback, weil Chris Phantom ist.
	dora := user(4, "Dora", 3)
	dora.Cancellations = absentOn(ths, "kreativ", 1, 2, 3, 4)
	dora.MaxCancellationStreak, dora.MaxCancellationStreakEnd = 4, ths[4]

	// Emil: erste Hälfte voll da, zweite Hälfte halb → Absturz; vier
	// einzelne Absagen → Wackelkandidat.
	emil := user(5, "Emil", 5)
	emil.Cancellations = absentOn(ths, "freizeit", 8, 10, 12, 14)
	emil.MaxCancellationStreak, emil.MaxCancellationStreakEnd = 1, ths[14]

	// Fritz: steigt erst im zweiten Halbjahr ein und fehlt dort oft – ohne
	// gezählte Donnerstage im ersten Halbjahr kein Absturz.
	fritz := user(6, "Fritz", 6)
	fritz.Since = ths[9]
	fritz.Cancellations = absentOn(ths, "arbeit", 10, 11)

	users := []models.UserStats{anna, ben, dora, chris, emil, fritz}
	strafen := models.StrafenStats{UserTotals: []models.StrafenUserTotal{
		{UserName: "Chris", Total: 30}, {UserName: "Emil", Total: 30}, {UserName: "Dora", Total: 25},
	}}

	awards := CalculateAwards(users, ths, strafen)

	cases := []struct {
		id    string
		want  []string
		value int
		shame bool
	}{
		{"koenig", []string{"Anna", "Ben"}, 100, false},
		{"streak", []string{"Anna", "Ben"}, 16, false},
		{"kreativ", []string{"Dora"}, 4, false},
		{"comeback", []string{"Dora"}, 4, false},
		{"phantom", []string{"Chris"}, 6, true},
		{"strafen", []string{"Chris", "Emil"}, 30, true},
		{"absturz", []string{"Emil"}, 50, true},
		{"wackel", []string{"Emil"}, 4, true},
	}
	for _, c := range cases {
		a := awardByID(awards, c.id)
		if a == nil {
			t.Errorf("%s: nicht vergeben", c.id)
			continue
		}
		if got := winnerNames(a); !equalStrings(got, c.want) {
			t.Errorf("%s: Gewinner %v, erwartet %v", c.id, got, c.want)
		}
		if a.Value != c.value {
			t.Errorf("%s: Wert %d, erwartet %d", c.id, a.Value, c.value)
		}
		if a.Shame != c.shame {
			t.Errorf("%s: Shame %v, erwartet %v", c.id, a.Shame, c.shame)
		}
	}
	if a := awardByID(awards, "absagen"); a != nil {
		t.Errorf("absagen: nur ohne kreative Ausreden erwartet, vergeben an %v", winnerNames(a))
	}
}

func TestCalculateAwardsFallbacks(t *testing.T) {
	ths := testThursdays(16)
	since := ths[0]
	a := models.UserStats{User: models.User{ID: 1, Name: "Anna"}, Rank: 1, Since: since}
	b := models.UserStats{User: models.User{ID: 2, Name: "Ben"}, Rank: 2, Since: since,
		CancellationCount: 2, Cancellations: absentOn(ths, "arbeit", 7, 8)}
	b.MaxCancellationStreak, b.MaxCancellationStreakEnd = 2, ths[8]

	awards := CalculateAwards([]models.UserStats{a, b}, ths, models.StrafenStats{})

	if got := winnerNames(awardByID(awards, "absagen")); !equalStrings(got, []string{"Ben"}) {
		t.Errorf("absagen: Gewinner %v, erwartet [Ben]", got)
	}
	// Unter den Schwellen gibt es keine Gewinner statt Zufallssieger.
	for _, id := range []string{"kreativ", "phantom", "comeback", "strafen", "wackel", "rising", "absturz"} {
		if a := awardByID(awards, id); a != nil {
			t.Errorf("%s: unerwartet vergeben an %v", id, winnerNames(a))
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
