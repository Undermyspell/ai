package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/joho/godotenv"

	"github.com/michael/zumba-admin-ui/internal/config"
	"github.com/michael/zumba-admin-ui/internal/db"
	"github.com/michael/zumba-admin-ui/internal/store"
	"github.com/michael/zumba-admin-ui/internal/timeutil"
	"github.com/michael/zumba-admin-ui/internal/web"
)

func main() {
	if err := godotenv.Load(); err == nil {
		log.Printf("📄 Loaded .env")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	period := timeutil.Period{Start: cfg.EvalPeriodStart, End: cfg.EvalPeriodEnd}

	var st store.Store
	mockMode := false
	pg, err := db.Open(cfg.DB)
	if err != nil {
		log.Printf("⚠️  DB unreachable (%v) – falling back to mock data", err)
		st = store.NewMock(period)
		mockMode = true
	} else {
		log.Printf("✅ Connected to PostgreSQL '%s' on %s:%s", cfg.DB.Name, cfg.DB.Host, cfg.DB.Port)
		pgStore := store.NewPostgres(pg)
		// Tabelle für den manuellen ML-Test (Schreiber ist das Admin-UI).
		if err := pgStore.EnsureMLTestSchema(context.Background()); err != nil {
			log.Printf("⚠️  ml_test_messages Schema: %v", err)
		}
		// Strafen-Tabelle (auch der Bot legt sie idempotent an).
		if err := pgStore.EnsureStrafenSchema(context.Background()); err != nil {
			log.Printf("⚠️  strafen Schema: %v", err)
		}
		st = pgStore
		defer pg.Close()
	}

	srv := web.New(st, cfg, mockMode)

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("🍻 Zumba Admin UI läuft auf http://localhost%s", addr)
	if err := http.ListenAndServe(addr, srv.Routes()); err != nil {
		log.Fatal(err)
	}
}
