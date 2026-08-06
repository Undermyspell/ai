// zumba-renderer: winziger HTTP-Dienst, der HTML per headless Chromium in ein
// PNG rendert. Wird vom whatsapp-bot genutzt, um die Statistik als Bild-Karte
// zu verschicken.
//
//	POST /render   {"html": "...", "width": 720, "scale": 2}  → image/png
//	GET  /healthz  → ok
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/michael/zumba-renderer/internal/render"
)

type renderRequest struct {
	HTML  string  `json:"html"`
	Width int     `json:"width"`
	Scale float64 `json:"scale"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := render.New()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /render", func(w http.ResponseWriter, req *http.Request) {
		var body renderRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 2<<20)).Decode(&body); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		if body.HTML == "" {
			http.Error(w, "html fehlt", http.StatusBadRequest)
			return
		}
		start := time.Now()
		png, err := r.PNG(req.Context(), body.HTML, render.Options{Width: body.Width, Scale: body.Scale})
		if err != nil {
			log.Printf("⚠️  render: %v", err)
			http.Error(w, "render: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("🖼  %d bytes PNG in %s", len(png), time.Since(start).Round(time.Millisecond))
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	log.Printf("🚀 zumba-renderer auf :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
