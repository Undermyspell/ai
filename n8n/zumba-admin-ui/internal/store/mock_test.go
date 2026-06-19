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
	p := timeutil.Period{Start: mustDate("2025-12-01"), End: mustDate("2026-11-30")}
	m := NewMock(p)
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
	p := timeutil.Period{Start: mustDate("2025-12-01"), End: mustDate("2026-11-30")}
	m := NewMock(p)
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

func mustDate(s string) time.Time {
	d, err := timeutil.ParseISO(s)
	if err != nil {
		panic(err)
	}
	return d
}
