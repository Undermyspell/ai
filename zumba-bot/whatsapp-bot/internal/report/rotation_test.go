package report

import (
	"slices"
	"testing"
	"time"
)

var testStyles = []string{"wrapped", "bierdeckel", "zeitung", "arena", "formular"}

func TestParseCardStyles(t *testing.T) {
	got, err := ParseCardStyles(" wrapped , zeitung ,, arena , zeitung ")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"wrapped", "zeitung", "arena"} // getrimmt, ohne Leere, ohne Duplikat
	if !slices.Equal(got, want) {
		t.Errorf("ParseCardStyles = %v, will %v", got, want)
	}

	if got, err := ParseCardStyles("  "); err != nil || got != nil {
		t.Errorf("leere Liste: %v, %v – erwartet nil, nil", got, err)
	}
	if _, err := ParseCardStyles("wrapped,gibtsnicht"); err == nil {
		t.Error("unbekannte Design-ID muss einen Fehler geben")
	}
}

// Der Wochenreport soll ein Design erst wiedersehen, wenn alle anderen dran
// waren: jedes Fenster aus len(styles) aufeinanderfolgenden Wochen enthält
// jedes Design genau einmal — egal, wo das Fenster anfängt.
func TestForWeekJedesFensterEnthaeltAlleDesigns(t *testing.T) {
	r := NewCardRotation(testStyles)
	n := len(testStyles)
	do := donnerstag(2026, 1, 1)

	var folge []string
	for w := 0; w < 200; w++ {
		folge = append(folge, r.ForWeek(do.AddDate(0, 0, 7*w)))
	}
	want := slices.Sorted(slices.Values(testStyles))
	for i := 0; i+n <= len(folge); i++ {
		fenster := slices.Clone(folge[i : i+n])
		slices.Sort(fenster)
		if !slices.Equal(fenster, want) {
			t.Fatalf("Wochen %d..%d: %v – jedes Design genau einmal erwartet", i, i+n-1, folge[i:i+n])
		}
	}
}

// Kein gespeicherter Zustand: derselbe Stichtag muss immer dasselbe Design
// liefern (Dry-Run im Admin-UI == echter Lauf, auch nach einem Neustart).
func TestForWeekIstDeterministisch(t *testing.T) {
	r := NewCardRotation(testStyles)
	do := donnerstag(2026, 8, 6)

	if a, b := r.ForWeek(do), r.ForWeek(do); a != b {
		t.Errorf("zwei Aufrufe, zwei Designs: %q vs %q", a, b)
	}
	// Tag 0 der Epoche war ein Donnerstag: der Wochen-Bucket wechselt genau
	// donnerstags. Ein Nachhol-Lauf am Freitag bleibt beim selben Design.
	if a, b := r.ForWeek(do), r.ForWeek(do.AddDate(0, 0, 1)); a != b {
		t.Errorf("Freitag darauf: %q statt %q", b, a)
	}
	if a, b := r.ForWeek(do), r.ForWeek(do.AddDate(0, 0, 7)); a == b {
		t.Errorf("nächster Donnerstag: %q unverändert", b)
	}
}

func TestRotationLiefertNurKonfigurierteDesigns(t *testing.T) {
	r := NewCardRotation(testStyles)
	for i := 0; i < 200; i++ {
		if got := r.Random(); !slices.Contains(testStyles, got) {
			t.Fatalf("Random lieferte %q – nicht in der Auswahl", got)
		}
	}
}

// Ohne CARD_STYLES bleibt alles beim Live-Design.
func TestLeereRotationFaelltAufLiveDesignZurueck(t *testing.T) {
	var r CardRotation // Nullwert wie ohne Konfiguration
	if got := r.Random(); got != DefaultCardStyle {
		t.Errorf("Random = %q, will %q", got, DefaultCardStyle)
	}
	if got := r.ForWeek(donnerstag(2026, 8, 6)); got != DefaultCardStyle {
		t.Errorf("ForWeek = %q, will %q", got, DefaultCardStyle)
	}
}

func TestRotationMitEinemDesign(t *testing.T) {
	r := NewCardRotation([]string{"zeitung"})
	do := donnerstag(2026, 8, 6)
	for w := 0; w < 5; w++ {
		if got := r.ForWeek(do.AddDate(0, 0, 7*w)); got != "zeitung" {
			t.Fatalf("Woche %d: %q", w, got)
		}
	}
	if got := r.Random(); got != "zeitung" {
		t.Errorf("Random = %q", got)
	}
}

// donnerstag liefert den ersten Donnerstag ab dem angegebenen Datum.
func donnerstag(jahr int, monat time.Month, tag int) time.Time {
	d := time.Date(jahr, monat, tag, 0, 0, 0, 0, time.UTC)
	for d.Weekday() != time.Thursday {
		d = d.AddDate(0, 0, 1)
	}
	return d
}
