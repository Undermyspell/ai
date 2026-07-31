package penalty

import (
	"testing"
	"time"
)

// Donnerstage ab 2026-01-01 (ein Donnerstag).
func thursday(n int) time.Time {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return base.AddDate(0, 0, 7*n)
}

func ts(t time.Time, hour int) *time.Time {
	v := time.Date(t.Year(), t.Month(), t.Day(), hour, 0, 0, 0, time.UTC)
	return &v
}

func user(absences ...time.Time) UserData {
	return UserData{
		UserID: "u1", Name: "Hans",
		EffectiveStart: thursday(0),
		Absences:       absences,
	}
}

func TestBetrag(t *testing.T) {
	cases := []struct{ tage, want int }{
		{0, 0}, {4, 0}, {5, 25}, {6, 30}, {10, 50},
	}
	for _, c := range cases {
		if got := Betrag(c.tage); got != c.want {
			t.Errorf("Betrag(%d) = %d, want %d", c.tage, got, c.want)
		}
	}
}

func TestAssessKeineStrafeUnterFuenf(t *testing.T) {
	u := user(thursday(0), thursday(1), thursday(2), thursday(3))
	got := Assess(Input{Users: []UserData{u}}, thursday(3))
	if len(got) != 0 {
		t.Fatalf("erwartet keine Strafe, got %+v", got)
	}
}

func TestAssessFuenfFehltage(t *testing.T) {
	u := user(thursday(0), thursday(1), thursday(2), thursday(3), thursday(4))
	got := Assess(Input{Users: []UserData{u}}, thursday(4))
	if len(got) != 1 {
		t.Fatalf("erwartet 1 Strafe, got %+v", got)
	}
	e := got[0]
	if e.ID != 0 || e.Art != ArtFehltage || e.Tage != 5 || e.Betrag != 25 || !e.Datum.Equal(thursday(0)) {
		t.Errorf("unerwarteter Entry: %+v", e)
	}
}

func TestAssessSechsterFehltagErhoeht(t *testing.T) {
	u := user(thursday(0), thursday(1), thursday(2), thursday(3), thursday(4), thursday(5))
	got := Assess(Input{Users: []UserData{u}}, thursday(5))
	if len(got) != 1 || got[0].Betrag != 30 || got[0].Tage != 6 {
		t.Fatalf("erwartet 30€/6 Tage, got %+v", got)
	}
}

// Kniffliger Fall aus der Anforderung: 6x gefehlt → 30€ → am Abend beglichen →
// nächste Woche wieder gefehlt → Zähler steht bei 1, KEINE neue Strafe.
func TestAssessBeglichenResettetZaehler(t *testing.T) {
	u := user(
		thursday(0), thursday(1), thursday(2), thursday(3), thursday(4), thursday(5),
		thursday(6), // Fehltag nach Begleichung
	)
	row := Row{
		ID: 1, UserID: "u1", Art: ArtFehltage, Datum: thursday(0),
		Status: StatusBeglichen, BeglichenAm: ts(thursday(5), 22), // Abend des 6. Fehltags
	}
	got := Assess(Input{Users: []UserData{u}, Rows: []Row{row}}, thursday(6))
	if len(got) != 1 {
		t.Fatalf("erwartet nur die beglichene Strafe, got %+v", got)
	}
	e := got[0]
	if e.ID != 1 || e.Status != StatusBeglichen || e.Betrag != 30 || e.Tage != 6 {
		t.Errorf("beglichene Strafe falsch bewertet: %+v", e)
	}
}

// Anwesenheit friert die offene Strafe ein; eine spätere neue 5er-Serie
// erzeugt eine zweite, separate Strafe.
func TestAssessSerienbruchErzeugtZweiteStrafe(t *testing.T) {
	u := user(
		thursday(0), thursday(1), thursday(2), thursday(3), thursday(4), thursday(5), // Serie 1: 6 Tage
		// thursday(6): anwesend
		thursday(7), thursday(8), thursday(9), thursday(10), thursday(11), // Serie 2: 5 Tage
	)
	row := Row{ID: 1, UserID: "u1", Art: ArtFehltage, Datum: thursday(0), Status: StatusOffen}
	got := Assess(Input{Users: []UserData{u}, Rows: []Row{row}}, thursday(11))
	if len(got) != 2 {
		t.Fatalf("erwartet 2 Strafen, got %+v", got)
	}
	if got[0].ID != 1 || got[0].Betrag != 30 || got[0].Tage != 6 {
		t.Errorf("eingefrorene Strafe falsch: %+v", got[0])
	}
	if got[1].ID != 0 || got[1].Betrag != 25 || !got[1].Datum.Equal(thursday(7)) {
		t.Errorf("neue Strafe falsch: %+v", got[1])
	}
}

// Gelöschte Strafe: unsichtbar, aber ihr Lösch-Zeitpunkt resettet den Zähler –
// die Serie wird nicht sofort wieder als Strafe erkannt.
func TestAssessGeloeschtResettetUndVerschwindet(t *testing.T) {
	u := user(thursday(0), thursday(1), thursday(2), thursday(3), thursday(4), thursday(5))
	row := Row{
		ID: 1, UserID: "u1", Art: ArtFehltage, Datum: thursday(0),
		Status: StatusGeloescht, GeloeschtAm: ts(thursday(4), 12), // nach dem 5. Fehltag gelöscht
	}
	got := Assess(Input{Users: []UserData{u}, Rows: []Row{row}}, thursday(5))
	// Serie 1 [0..4] gehört zur gelöschten Strafe, Serie 2 [5] hat erst 1 Tag.
	for _, e := range got {
		if e.Status == StatusGeloescht {
			continue
		}
		t.Errorf("nichts Sichtbares erwartet, got %+v", e)
	}
	if len(got) != 1 || got[0].Status != StatusGeloescht {
		t.Fatalf("erwartet genau die gelöschte Row als Entry, got %+v", got)
	}
	if VisibleAt(got[0], thursday(5)) {
		t.Errorf("gelöschte Strafe darf nie sichtbar sein")
	}
}

// Sperrtage zählen nirgends: sie unterbrechen eine Serie NICHT.
func TestAssessSperrtagUeberbruecktSerie(t *testing.T) {
	u := user(thursday(0), thursday(1), thursday(3), thursday(4), thursday(5))
	in := Input{Users: []UserData{u}, Excluded: []time.Time{thursday(2)}}
	got := Assess(in, thursday(5))
	if len(got) != 1 || got[0].Tage != 5 || got[0].Betrag != 25 {
		t.Fatalf("erwartet durchgehende 5er-Serie über Sperrtag, got %+v", got)
	}
}

func TestVisibleAtBeglichenFenster(t *testing.T) {
	beglichen := ts(thursday(5).AddDate(0, 0, 1), 10) // Freitag nach Donnerstag 5
	e := Entry{Status: StatusBeglichen, BeglichenAm: beglichen}

	if VisibleAt(e, thursday(5)) {
		t.Error("vor der Begleichung nicht sichtbar")
	}
	if !VisibleAt(e, *beglichen) {
		t.Error("am Tag der Begleichung sichtbar")
	}
	if !VisibleAt(e, thursday(6)) {
		t.Error("am Folgedonnerstag sichtbar")
	}
	if VisibleAt(e, thursday(6).AddDate(0, 0, 1)) {
		t.Error("am Freitag nach dem Folgedonnerstag nicht mehr sichtbar")
	}
}

func TestVisibleAtBeglichenAmDonnerstagSelbst(t *testing.T) {
	// Am Donnerstagabend beglichen → Folgedonnerstag ist +7 Tage.
	e := Entry{Status: StatusBeglichen, BeglichenAm: ts(thursday(5), 22)}
	if !VisibleAt(e, thursday(6)) {
		t.Error("eine Woche später noch sichtbar")
	}
	if VisibleAt(e, thursday(6).AddDate(0, 0, 1)) {
		t.Error("danach nicht mehr")
	}
}

func TestNextThursday(t *testing.T) {
	if got := NextThursday(thursday(0)); !got.Equal(thursday(1)) {
		t.Errorf("Donnerstag → +7: got %v", got)
	}
	friday := thursday(0).AddDate(0, 0, 1)
	if got := NextThursday(friday); !got.Equal(thursday(1)) {
		t.Errorf("Freitag → nächster Donnerstag: got %v", got)
	}
}
