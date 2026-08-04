package main

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/michael/stammtisch-wrapped/assets"
	"github.com/michael/stammtisch-wrapped/internal/database"
	"github.com/michael/stammtisch-wrapped/internal/handlers"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize database connection (optional - falls back to mock data if not available)
	var db *database.PostgresDB
	var err error

	dbConfig := database.ConfigFromEnv()
	db, err = database.New(dbConfig)
	if err != nil {
		log.Printf("⚠️  Database connection failed: %v", err)
		log.Printf("📦 Using mock data instead")
		db = nil
	} else {
		log.Printf("✅ Connected to PostgreSQL database '%s'", dbConfig.DBName)
		defer db.Close()
	}

	// Create handler with optional database
	handler := handlers.NewWrappedHandler(db)

	// Routes
	http.HandleFunc("/", handler.HandleIndex)
	http.HandleFunc("/2026", handler.Handle2026)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	fsys, _ := fs.Sub(assets.Static, "static")
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(fsys))))

	addr := fmt.Sprintf(":%s", port)
	log.Printf("🍺 Stammtisch Wrapped läuft auf http://localhost%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
