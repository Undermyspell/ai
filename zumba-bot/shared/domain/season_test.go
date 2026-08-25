package domain

import (
	"testing"
	"time"
)

func d(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func season2026() Season {
	s := Season{ID: 2, Label: "2026"}
	s.Start, s.End = d("2025-12-01"), d("2026-11-30")
	return s
}

func TestSeasonContainsIncludesBothBounds(t *testing.T) {
	s := season2026()
	for _, tc := range []struct {
		day  string
		want bool
	}{
		{"2025-11-30", false}, // letzter Tag des Vorjahres
		{"2025-12-01", true},  // Jahresstart, inklusive
		{"2026-08-25", true},
		{"2026-11-30", true}, // Jahresende, inklusive
		{"2026-12-01", false},
	} {
		if got := s.Contains(d(tc.day)); got != tc.want {
			t.Errorf("Contains(%s) = %v, want %v", tc.day, got, tc.want)
		}
	}
}

func TestSeasonClampAsOf(t *testing.T) {
	s := season2026()
	if got := s.ClampAsOf(d("2025-06-01")); !got.Equal(d("2025-12-01")) {
		t.Errorf("vor dem Jahr: %s, want 2025-12-01", got.Format("2006-01-02"))
	}
	if got := s.ClampAsOf(d("2026-05-07")); !got.Equal(d("2026-05-07")) {
		t.Errorf("im Jahr: %s, want unverändert", got.Format("2006-01-02"))
	}
	// Archiv: ein späterer Stichtag darf nicht über das Jahresende hinaus
	// rechnen, sonst wüchse ein abgeschlossenes Jahr weiter.
	if got := s.ClampAsOf(d("2027-03-01")); !got.Equal(d("2026-11-30")) {
		t.Errorf("nach dem Jahr: %s, want 2026-11-30", got.Format("2006-01-02"))
	}
}

func TestSeasonClampStart(t *testing.T) {
	s := season2026()

	if got := s.ClampStart(nil); !got.Equal(d("2025-12-01")) {
		t.Errorf("ohne startDate: %s, want Jahresstart", got.Format("2006-01-02"))
	}
	// Eintritt vor dem Jahr: das Jahr beginnt für alle bei null.
	before := d("2024-05-01")
	if got := s.ClampStart(&before); !got.Equal(d("2025-12-01")) {
		t.Errorf("Eintritt vor dem Jahr: %s, want Jahresstart", got.Format("2006-01-02"))
	}
	// Eintritt im Jahr: zählt ab dem Eintritt.
	inside := d("2026-04-02")
	if got := s.ClampStart(&inside); !got.Equal(inside) {
		t.Errorf("Eintritt im Jahr: %s, want 2026-04-02", got.Format("2006-01-02"))
	}
}
