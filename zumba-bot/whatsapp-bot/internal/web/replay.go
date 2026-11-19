package web

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/michael/zumba-whatsapp-bot/internal/evolution"
	"github.com/michael/zumba-whatsapp-bot/internal/tracestore"
)

// handleReplay verarbeitet ein aufgezeichnetes Event nochmal (Admin-UI,
// Verlauf → „Erneut verarbeiten"), z. B. wenn die Klassifizierung live an
// einem Ausfall hing. Es laufen dieselben Guards und dieselbe
// Klassifizierung wie live; es entscheidet, wer im Admin-UI jetzt
// eingestellt ist. Verarbeitungstag ist der Tag der Originalnachricht, nicht
// heute.
//
// Die Payload kommt aus bot_trace, nie aus dem Request – der Request nennt
// nur die Trace-ID. Gesendet wird nichts: "statistik" wird deshalb nicht
// nachverarbeitet (das würde die Rangliste nochmal in die Gruppe schicken).
// Ins ML-Protokoll kommt die Nachricht nur, wenn sie dort am Tag noch fehlt.
func (s *Server) handleReplay(w http.ResponseWriter, r *http.Request) {
	if s.Traces == nil {
		http.Error(w, "Trace-Aufzeichnung ist aus", http.StatusServiceUnavailable)
		return
	}
	id, err := strconv.ParseInt(r.URL.Query().Get("trace"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "trace=<id> fehlt oder ist ungültig", http.StatusBadRequest)
		return
	}
	t, err := s.Traces.Get(r.Context(), id)
	if errors.Is(err, tracestore.ErrNotFound) {
		http.Error(w, "Event nicht gefunden (Verlauf hält 21 Tage)", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("⚠️  replay #%d: %v", id, err)
		http.Error(w, "Event nicht lesbar", http.StatusInternalServerError)
		return
	}
	if len(t.RawPayload) == 0 {
		http.Error(w, "zu diesem Event ist keine Payload gespeichert", http.StatusUnprocessableEntity)
		return
	}
	var ev evolution.WebhookEvent
	if err := json.Unmarshal(t.RawPayload, &ev); err != nil {
		http.Error(w, "gespeicherte Payload nicht lesbar: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	if strings.EqualFold(strings.TrimSpace(ev.Message()), "statistik") {
		http.Error(w, "„statistik“ wird nicht nachverarbeitet – das schickt die Rangliste nochmal in die Gruppe",
			http.StatusUnprocessableEntity)
		return
	}

	day := s.dayOf(t.CreatedAt)
	rec := tracestore.NewRecorder()
	out := s.process(r.Context(), ev, runOpts{asOf: day, rec: rec, replay: true})
	out.TraceID = s.recordTrace(r.Context(), ev, t.RawPayload, out, rec, day, t.ID)
	log.Printf("🔁 Event #%d nachverarbeitet (%s): %s → %s/%s", t.ID, day.Format("02.01."),
		ev.UserName(), out.Classification, out.Action)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// dayOf ist der Tag (Mitternacht) eines Zeitpunkts in der konfigurierten
// Zeitzone.
func (s *Server) dayOf(t time.Time) time.Time {
	l := t.In(s.location)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, s.location)
}
