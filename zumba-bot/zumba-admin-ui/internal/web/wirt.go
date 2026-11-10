package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/michael/zumba-admin-ui/web/templates/wirt"
)

func (s *Server) handleWirt(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, s.meta("Wirt", "wirt"), wirt.Page())
}

// wirtOutcome spiegelt die Antwort von POST /wirt-reminder am Bot.
type wirtOutcome struct {
	Message   string `json:"message"`
	Recipient string `json:"recipient"`
	Mentioned string `json:"mentioned"`
	PreviewTo string `json:"previewTo"`
}

// handleWirtTest lässt den Bot die Erinnerung bauen. Wie beim Bot-Test gibt
// es nur Dry-Run und Vorschau an die eigene Nummer – der Bot kennt für diese
// Nachricht ohnehin noch keinen echten Versand.
func (s *Server) handleWirtTest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fail := func(msg string) { _ = wirt.Error(msg).Render(ctx, w) }

	preview := r.FormValue("mode") == "preview"
	// Ein Doppelklick soll nicht zwei WhatsApp-Nachrichten schicken.
	if preview && !s.notify.allow(time.Now()) {
		fail("Gerade erst verschickt – kurz warten.")
		return
	}
	q := url.Values{}
	if preview {
		q.Set("preview", "true")
	} else {
		q.Set("dryRun", "true")
	}

	target := strings.TrimRight(s.cfg.BotURL, "/") + "/wirt-reminder?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, nil)
	if err != nil {
		fail(err.Error())
		return
	}
	resp, err := (&http.Client{Timeout: 35 * time.Second}).Do(req)
	if err != nil {
		fail("Bot nicht erreichbar: " + err.Error())
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode != http.StatusOK {
		fail("Bot-Status " + resp.Status + ": " + strings.TrimSpace(string(body)))
		return
	}
	var out wirtOutcome
	if err := json.Unmarshal(body, &out); err != nil {
		fail("Antwort nicht lesbar: " + err.Error())
		return
	}

	_ = wirt.Result(wirt.ResultVM{
		Message: out.Message, Recipient: out.Recipient,
		Mentioned: out.Mentioned, PreviewTo: out.PreviewTo,
		Time: time.Now().Format("15:04"),
	}).Render(ctx, w)
}
