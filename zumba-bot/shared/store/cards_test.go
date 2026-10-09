package store

import (
	"context"
	"database/sql"
	"os"
	"slices"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// Braucht eine Wegwerf-DB: ZUMBA_TEST_DSN=postgres://... go test ./store/
func TestCardSettingsRundlauf(t *testing.T) {
	dsn := os.Getenv("ZUMBA_TEST_DSN")
	if dsn == "" {
		t.Skip("ZUMBA_TEST_DSN nicht gesetzt")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	// Tabelle im Stand des ersten Entwurfs – EnsureCardSchema muss sie nachziehen.
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS card_settings;
		CREATE TABLE card_settings (id INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
		  rotation TEXT[] NOT NULL DEFAULT '{}', naechster_tag DATE, naechster_style TEXT,
		  updated_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		t.Fatal(err)
	}
	if err := EnsureCardSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	if s, err := GetCardSettings(ctx, db); err != nil || s.RotationGesetzt || s.Naechster != nil {
		t.Fatalf("leer: %+v, %v", s, err)
	}

	// Erst die Einmal-Auswahl, dann ein Bot ohne CARD_STYLES: beides darf die
	// Rotation nicht festnageln – der Seed mit CARD_STYLES greift danach noch.
	do := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	if err := SetNextCard(ctx, db, do, "kassenbon"); err != nil {
		t.Fatal(err)
	}
	if err := SeedCardRotation(ctx, db, nil); err != nil {
		t.Fatal(err)
	}
	if s, _ := GetCardSettings(ctx, db); s.RotationGesetzt {
		t.Fatalf("Rotation durch Einmal-Auswahl/leeren Seed gesetzt: %+v", s)
	}
	if err := SeedCardRotation(ctx, db, []string{"arena", "wrapped"}); err != nil {
		t.Fatal(err)
	}
	if s, _ := GetCardSettings(ctx, db); !s.RotationGesetzt || !slices.Equal(s.Rotation, []string{"arena", "wrapped"}) {
		t.Fatalf("Seed nicht übernommen: %+v", s)
	}

	// Eine Änderung im Admin-UI überlebt den nächsten Seed.
	if err := SetCardRotation(ctx, db, []string{"gipfelbuch"}); err != nil {
		t.Fatal(err)
	}
	if err := SeedCardRotation(ctx, db, []string{"arena", "wrapped"}); err != nil {
		t.Fatal(err)
	}
	s, err := GetCardSettings(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(s.Rotation, []string{"gipfelbuch"}) || s.Naechster == nil ||
		s.Naechster.Style != "kassenbon" || s.Naechster.Tag.Format("2006-01-02") != "2026-10-15" {
		t.Fatalf("gelesen: %+v / %+v", s, s.Naechster)
	}

	if err := SetNextCard(ctx, db, do, ""); err != nil {
		t.Fatal(err)
	}
	if s, _ := GetCardSettings(ctx, db); s.Naechster != nil {
		t.Errorf("Einmal-Auswahl nicht aufgehoben: %+v", s.Naechster)
	}
}

// Nach dem Versand wandert das oberste Design ans Ende; derselbe Tag ein
// zweites Mal ändert nichts, eine Einmal-Auswahl bewegt die Schlange nicht.
func TestAdvanceCardQueue(t *testing.T) {
	dsn := os.Getenv("ZUMBA_TEST_DSN")
	if dsn == "" {
		t.Skip("ZUMBA_TEST_DSN nicht gesetzt")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS card_settings`); err != nil {
		t.Fatal(err)
	}
	if err := EnsureCardSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := SetCardRotation(ctx, db, []string{"wetterbericht", "arena", "zeitung"}); err != nil {
		t.Fatal(err)
	}
	queue := func() []string {
		s, err := GetCardSettings(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
		return s.Rotation
	}
	do := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)

	if err := AdvanceCardQueue(ctx, db, do, "wetterbericht", false); err != nil {
		t.Fatal(err)
	}
	if got := queue(); !slices.Equal(got, []string{"arena", "zeitung", "wetterbericht"}) {
		t.Fatalf("nach Versand: %v", got)
	}
	// Wiederholungslauf am selben Tag.
	if err := AdvanceCardQueue(ctx, db, do, "arena", false); err != nil {
		t.Fatal(err)
	}
	if got := queue(); !slices.Equal(got, []string{"arena", "zeitung", "wetterbericht"}) {
		t.Fatalf("Wiederholung hat weitergerückt: %v", got)
	}
	s, _ := GetCardSettings(ctx, db)
	if s.Zuletzt == nil || s.Zuletzt.Style != "wetterbericht" || s.Zuletzt.Tag.Format("2006-01-02") != "2026-10-15" {
		t.Fatalf("zuletzt: %+v", s.Zuletzt)
	}
	// Einmal-Auswahl eine Woche später: Schlange bleibt stehen.
	if err := AdvanceCardQueue(ctx, db, do.AddDate(0, 0, 7), "kassenbon", true); err != nil {
		t.Fatal(err)
	}
	if got := queue(); !slices.Equal(got, []string{"arena", "zeitung", "wetterbericht"}) {
		t.Fatalf("Einmal-Auswahl hat die Schlange bewegt: %v", got)
	}
}
