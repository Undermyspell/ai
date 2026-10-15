// Klassifikator-Service: stellt das in ml-classifier trainierte Modell als
// HTTP-Endpunkt bereit (POST /classify, GET /info). Wer entscheidet – Gemini
// oder dieses Modell –, stellt das Admin-UI ein; der whatsapp-bot protokolliert
// beide Ergebnisse in ml_messages.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/michael/zumba-classifier/internal/model"
	modeldata "github.com/michael/zumba-classifier/model"
)

type classifyRequest struct {
	Text string `json:"text"`
}

func main() {
	m, err := model.Load(modeldata.ModelGZ)
	if err != nil {
		log.Fatalf("Modell laden fehlgeschlagen: %v", err)
	}
	log.Printf("Modell geladen: %s, %d Merkmale, Klassen %v, Schwelle %.2f",
		m.Name(), m.NumFeatures(), m.Classes, m.Threshold)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	// /info beschreibt das geladene Modell (Admin-UI /ki-modell).
	mux.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model":     m.Name(),
			"classes":   m.Classes,
			"threshold": m.Threshold,
			"meta":      m.Meta,
		})
	})
	mux.HandleFunc("/classify", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "nur POST", http.StatusMethodNotAllowed)
			return
		}
		var req classifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "ungültiges JSON", http.StatusBadRequest)
			return
		}
		pred := m.Predict(req.Text)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(pred); err != nil {
			log.Printf("Antwort schreiben: %v", err)
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Klassifikator-Service auf :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
