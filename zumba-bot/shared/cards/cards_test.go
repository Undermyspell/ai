package cards

import (
	"slices"
	"testing"
	"time"
)

var berlin, _ = time.LoadLocation("Europe/Berlin")

func TestNextReport(t *testing.T) {
	for _, c := range []struct {
		now  time.Time
		want string
	}{
		{time.Date(2026, 10, 12, 9, 0, 0, 0, berlin), "2026-10-15"},   // Montag → Donnerstag
		{time.Date(2026, 10, 15, 20, 59, 0, 0, berlin), "2026-10-15"}, // Donnerstag vor dem Report
		{time.Date(2026, 10, 15, 21, 0, 0, 0, berlin), "2026-10-22"},  // Report läuft gerade → nächste Woche
		{time.Date(2026, 10, 16, 8, 0, 0, 0, berlin), "2026-10-22"},   // Freitag
	} {
		if got := NextReport(c.now).Format("2006-01-02"); got != c.want {
			t.Errorf("NextReport(%s) = %s, will %s", c.now.Format("Mon 02.01. 15:04"), got, c.want)
		}
	}
}

// Oberstes Design kommt als nächstes; die Einmal-Auswahl ersetzt nur ihren
// Donnerstag; ein Wiederholungslauf am selben Tag liefert das Gesendete.
func TestForQueue(t *testing.T) {
	queue := []string{"wetterbericht", "arena", "zeitung"}
	do := time.Date(2026, 10, 15, 0, 0, 0, 0, berlin)
	utc := func(d time.Time) time.Time { return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC) }

	if got, q := ForQueue(queue, nil, nil, do); got != "wetterbericht" || q != QuelleSchlange {
		t.Errorf("Schlange: %q/%q", got, q)
	}
	next := &Next{Tag: utc(do), Style: "kassenbon"}
	if got, q := ForQueue(queue, next, nil, do); got != "kassenbon" || q != QuelleFest {
		t.Errorf("Einmal-Auswahl: %q/%q", got, q)
	}
	if got, _ := ForQueue(queue, next, nil, do.AddDate(0, 0, 7)); got != "wetterbericht" {
		t.Errorf("Woche danach: %q – oberstes der Schlange erwartet", got)
	}
	last := &Last{Tag: utc(do), Style: "zeitung"}
	if got, q := ForQueue(queue, next, last, do); got != "zeitung" || q != QuelleWiederholt {
		t.Errorf("Wiederholung: %q/%q", got, q)
	}
	if got, _ := ForQueue(nil, nil, nil, do); got != Default {
		t.Errorf("leere Schlange: %q", got)
	}
	if got, _ := ForQueue([]string{"gibtsnicht", "arena"}, nil, nil, do); got != "arena" {
		t.Errorf("unbekanntes oben: %q", got)
	}
}

func TestUpcomingSchiebtUmEinmalAuswahl(t *testing.T) {
	queue := []string{"wetterbericht", "arena"}
	do := time.Date(2026, 10, 15, 0, 0, 0, 0, berlin)
	next := &Next{Tag: do, Style: "kassenbon"}
	var got []string
	for _, w := range Upcoming(queue, next, do, 4) {
		got = append(got, w.Style)
	}
	want := []string{"kassenbon", "wetterbericht", "arena", "wetterbericht"}
	if !slices.Equal(got, want) {
		t.Errorf("Upcoming = %v, will %v", got, want)
	}
}

func TestClean(t *testing.T) {
	got := Clean([]string{"arena", "gibtsnicht", "wrapped", "arena"})
	if !slices.Equal(got, []string{"arena", "wrapped"}) {
		t.Errorf("Clean = %v", got)
	}
}

// Jedes Fenster aus len(rotation) Wochen enthält jedes Design genau einmal.
func TestForWeekDurchlauf(t *testing.T) {
	rot := []string{"wrapped", "arena", "zeitung", "kassenbon"}
	do := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) // Donnerstag
	var folge []string
	for w := 0; w < 60; w++ {
		folge = append(folge, ForWeek(rot, do.AddDate(0, 0, 7*w)))
	}
	want := slices.Sorted(slices.Values(rot))
	for i := 0; i+len(rot) <= len(folge); i++ {
		f := slices.Clone(folge[i : i+len(rot)])
		slices.Sort(f)
		if !slices.Equal(f, want) {
			t.Fatalf("Wochen %d..: %v", i, folge[i:i+len(rot)])
		}
	}
	if got := ForWeek(nil, do); got != Default {
		t.Errorf("leere Rotation: %q", got)
	}
}
