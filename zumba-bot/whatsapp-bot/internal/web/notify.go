package web

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// maxNotifyLen deckelt die Nachricht. Der Aufrufer ist das Admin-UI mit ein
// paar Zeilen Text — alles darüber ist ein Fehler, kein Anwendungsfall.
const maxNotifyLen = 2000

type notifyRequest struct {
	Text string `json:"text"`
}

type notifyResponse struct {
	SentTo string `json:"sentTo"`
}

// handleNotify schickt eine Nachricht an die Vorschau-Nummer — und nur dorthin.
// Der Empfänger kommt aus der Konfiguration (PREVIEW_JID), niemals aus dem
// Request: sonst wäre das hier ein offener Versandweg in die Stammtisch-Gruppe.
func (s *Server) handleNotify(w http.ResponseWriter, r *http.Request) {
	if s.PreviewJID == "" {
		http.Error(w, "kein PREVIEW_JID konfiguriert", http.StatusServiceUnavailable)
		return
	}

	var req notifyRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "ungültiges JSON", http.StatusBadRequest)
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		http.Error(w, "leerer Text", http.StatusBadRequest)
		return
	}
	if len(text) > maxNotifyLen {
		http.Error(w, "Text zu lang", http.StatusRequestEntityTooLarge)
		return
	}

	if err := s.sender.SendText(r.Context(), s.PreviewJID, text); err != nil {
		log.Printf("⚠️  Notiz-Versand(%s): %v", s.PreviewJID, err)
		http.Error(w, "Versand fehlgeschlagen: "+err.Error(), http.StatusBadGateway)
		return
	}
	log.Printf("📱 Notiz gesendet an %s", s.PreviewJID)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(notifyResponse{SentTo: s.PreviewJID}); err != nil {
		log.Printf("Antwort schreiben: %v", err)
	}
}
