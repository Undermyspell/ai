package store

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"

	_ "github.com/lib/pq"
)

// Bot und Admin-UI starten beim Deploy gleichzeitig und legen die Tabelle
// beide an. Zwei parallele CREATE TABLE IF NOT EXISTS kollidierten am
// 02.10. an pg_type ("duplicate key ... pg_type_typname_nsp_index").
//
// Braucht eine Wegwerf-DB: ZUMBA_TEST_DSN=postgres://... go test ./store/
func TestEnsureClassifierSchemaConcurrent(t *testing.T) {
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

	for round := 0; round < 30; round++ {
		if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS classifier_models`); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		errs := make(chan error, 4)
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := EnsureClassifierSchema(ctx, db); err != nil {
					errs <- err
					return
				}
				errs <- SeedClassifierModels(ctx, db, "haupt", "fallback")
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("Runde %d: %v", round, err)
			}
		}
	}

	m, err := GetClassifierModels(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if m.Primary.Model != "haupt" || m.Fallback.Model != "fallback" {
		t.Errorf("got %+v", m)
	}
	if err := SetClassifierModel(ctx, db, ClassifierPrimary, "neu"); err != nil {
		t.Fatal(err)
	}
	if err := SeedClassifierModels(ctx, db, "haupt", "fallback"); err != nil {
		t.Fatal(err)
	}
	if m, _ = GetClassifierModels(ctx, db); m.Primary.Model != "neu" {
		t.Errorf("Seed hat die Wahl überschrieben: %+v", m)
	}
}
