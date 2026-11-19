package web

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/michael/zumba-admin-ui/web/templates/trace"
)

// handleTraceList zeigt die letzten aufgezeichneten Bot-Events (max. 3 Wochen).
func (s *Server) handleTraceList(w http.ResponseWriter, r *http.Request) {
	traces, err := s.store.ListTraces(r.Context(), 200)
	if err != nil {
		s.fail(w, "ListTraces", err)
		return
	}
	s.render(w, r, s.meta("Verlauf", "trace"), trace.List(traces))
}

// handleTraceDetail zeigt den Flow-Graphen einer einzelnen Aufzeichnung.
func (s *Server) handleTraceDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "ungültige ID", http.StatusBadRequest)
		return
	}
	t, err := s.store.GetTrace(r.Context(), id)
	if err != nil {
		http.Error(w, "Aufzeichnung nicht gefunden", http.StatusNotFound)
		return
	}
	s.render(w, r, s.meta(fmt.Sprintf("Verlauf · #%d", t.ID), "trace"), trace.Detail(*t))
}

// replayClient: der Bot klassifiziert live – entscheidet Gemini, bis zu
// zweimal 30 s (Haupt- und Fallback-Modell).
var replayClient = &http.Client{Timeout: 70 * time.Second}

// handleTraceReplay schickt ein aufgezeichnetes Event nochmal durch den Bot
// (POST BOT_URL/replay?trace=<id>). Der Bot liest die Payload selbst aus
// bot_trace; von hier geht nur die ID mit.
func (s *Server) handleTraceReplay(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "ungültige ID", http.StatusBadRequest)
		return
	}
	fail := func(msg string) {
		s.triggerToast(w, "error", fmt.Sprintf("Event #%d nicht verarbeitet.", id))
		_ = trace.ReplayError(msg).Render(r.Context(), w)
	}
	if s.cfg.BotURL == "" {
		fail("BOT_URL ist nicht gesetzt.")
		return
	}
	url := strings.TrimRight(s.cfg.BotURL, "/") + "/replay?trace=" + strconv.FormatInt(id, 10)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, nil)
	if err != nil {
		fail(err.Error())
		return
	}
	resp, err := replayClient.Do(req)
	if err != nil {
		fail("Bot nicht erreichbar: " + err.Error())
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != http.StatusOK {
		fail(strings.TrimSpace(string(body)))
		return
	}
	var out trace.ReplayVM
	if err := json.Unmarshal(body, &out); err != nil {
		fail("Antwort nicht lesbar: " + err.Error())
		return
	}
	log.Printf("🔁 Event #%d erneut verarbeitet → %s/%s (Trace #%d)", id, out.Classification, out.Action, out.TraceID)
	s.triggerToast(w, "success", fmt.Sprintf("Event #%d erneut verarbeitet: %s.", id, trace.ReplaySummary(out)))
	_ = trace.ReplayResult(out).Render(r.Context(), w)
}
