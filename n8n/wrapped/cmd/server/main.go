package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

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
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	addr := fmt.Sprintf(":%s", port)
	log.Printf("🍺 Stammtisch Wrapped läuft auf http://localhost%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
