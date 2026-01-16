package main

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/michael/stammtisch-wrapped/assets"
	"github.com/michael/stammtisch-wrapped/internal/handlers"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Routes
	http.HandleFunc("/", handlers.HandleIndex)
	http.HandleFunc("/2025", handlers.Handle2025)
	fsys, _ := fs.Sub(assets.Static, "static")
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(fsys))))

	addr := fmt.Sprintf(":%s", port)
	log.Printf("🍺 Stammtisch Wrapped läuft auf http://localhost%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
