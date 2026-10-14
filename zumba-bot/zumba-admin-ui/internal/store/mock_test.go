package store

import (
	"context"
	"testing"
	"time"

	"github.com/michael/zumba-admin-ui/internal/timeutil"
)

func thursdayIn(p timeutil.Period) time.Time {
	for d := p.Start; !d.After(p.End); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Thursday {
			return d
		}
	}
	return p.Start
}

func TestMockInsertDeleteAbsence(t *testing.T) {
	season := testSeason()
	p := season.Period
	m := NewMock(season, []Season{season})
	ctx := context.Background()
	day := thursdayIn(p)
	uid := m.users[0].ID

	before := countAbsences(ctx, t, m, p, uid, day)
	msg := "bin raus"
	if err := m.InsertAbsence(ctx, uid, day, &msg); err != nil {
		t.Fatal(err)
	}
	if got := countAbsences(ctx, t, m, p, uid, day); got != 1 {
		t.Fatalf("after insert = %d, want 1 (before %d)", got, before)
	}
	// idempotent upsert
	if err := m.InsertAbsence(ctx, uid, day, &msg); err != nil {
		t.Fatal(err)
	}
	if got := countAbsences(ctx, t, m, p, uid, day); got != 1 {
		t.Fatalf("after re-insert = %d, want 1", got)
	}
	if err := m.DeleteAbsence(ctx, uid, day); err != nil {
		t.Fatal(err)
	}
	if got := countAbsences(ctx, t, m, p, uid, day); got != 0 {
		t.Fatalf("after delete = %d, want 0", got)
	}
}

func TestMockInsertDeleteExcluded(t *testing.T) {
	season := testSeason()
	p := season.Period
	m := NewMock(season, []Season{season})
	ctx := context.Background()
	day := thursdayIn(p)

	if err := m.InsertExcludedDay(ctx, day); err != nil {
		t.Fatal(err)
	}
	days, _ := m.ListExcludedDays(ctx, p)
	if !containsDate(days, day) {
		t.Fatal("excluded day not added")
	}
	if err := m.DeleteExcludedDay(ctx, day); err != nil {
		t.Fatal(err)
	}
	days, _ = m.ListExcludedDays(ctx, p)
	if containsDate(days, day) {
		t.Fatal("excluded day not removed")
	}
}

func countAbsences(ctx context.Context, t *testing.T, m *Mock, p timeutil.Period, uid string, day time.Time) int {
	t.Helper()
	all, err := m.ListAbsences(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, a := range all {
		if a.UserID == uid && timeutil.FormatISO(a.Date) == timeutil.FormatISO(day) {
			n++
		}
	}
	return n
}

func containsDate(days []time.Time, d time.Time) bool {
	for _, x := range days {
		if timeutil.FormatISO(x) == timeutil.FormatISO(d) {
			return true
		}
	}
	return false
}

// testSeason: fester Zeitraum, unabhängig vom gepflegten Seed.
func testSeason() Season {
	s := Season{ID: 1, Label: "2026"}
	s.Start, s.End = mustDate("2025-12-01"), mustDate("2026-11-30")
	return s
}

func mustDate(s string) time.Time {
	d, err := timeutil.ParseISO(s)
	if err != nil {
		panic(err)
	}
	return d
}

// Im Mock bleibt ein Label bis zum Neustart erhalten: aus „offen" wird
// „gelabelt", ein zweites Label ändert das erste.
func TestMockMLLabelAendern(t *testing.T) {
	season := testSeason()
	m := NewMock(season, []Season{season})
	ctx := context.Background()

	offen, err := m.ListMLItems(ctx, MLWhatsapp, MLFilter{Status: MLOffen})
	if err != nil || len(offen) == 0 {
		t.Fatalf("offene Whatsapp-Einträge: %v, %d", err, len(offen))
	}
	if offen[0].Quelle != MLWhatsapp {
		t.Fatalf("Quelle = %q, want whatsapp", offen[0].Quelle)
	}
	id := offen[0].ID
	before, _ := m.MLCounts(ctx, MLWhatsapp)

	if _, err := m.LabelMLItem(ctx, MLWhatsapp, id, "true"); err != nil {
		t.Fatal(err)
	}
	it, err := m.LabelMLItem(ctx, MLWhatsapp, id, "invalid")
	if err != nil {
		t.Fatal(err)
	}
	if it.Offen() || it.Label == nil || *it.Label != "invalid" {
		t.Fatalf("nach Ändern: offen=%v label=%v", it.Offen(), it.Label)
	}
	after, _ := m.MLCounts(ctx, MLWhatsapp)
	if after.Offen != before.Offen-1 || after.Geprueft != before.Geprueft+1 {
		t.Errorf("Zähler vorher %+v, nachher %+v", before, after)
	}

	got, _ := m.ListMLItems(ctx, MLWhatsapp, MLFilter{Status: MLGeprueft, Label: "invalid"})
	found := false
	for _, g := range got {
		found = found || g.ID == id
	}
	if !found {
		t.Error("geänderter Eintrag fehlt im Filter gelabelt/invalid")
	}
}

func TestMockMLVerwerfen(t *testing.T) {
	season := testSeason()
	m := NewMock(season, []Season{season})
	ctx := context.Background()

	it, err := m.DiscardMLSynthetic(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !it.Verworfen || it.Label != nil || it.Offen() {
		t.Fatalf("verworfen=%v label=%v offen=%v", it.Verworfen, it.Label, it.Offen())
	}
	c, _ := m.MLCounts(ctx, MLSynthetics)
	if c.Verworfen != 1 || c.Geprueft != 0 {
		t.Errorf("Zähler %+v, want 1 verworfen, 0 gelabelt", c)
	}
	// Ein Label holt die Nachricht zurück ins Training.
	it, _ = m.LabelMLItem(ctx, MLSynthetics, 1, "true")
	if it.Verworfen {
		t.Error("nach Label noch verworfen")
	}
}
