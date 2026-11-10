package web

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/michael/zumba-whatsapp-bot/internal/report"
)

// MentionSender verschickt einen Text, in dem Nummern markiert sind
// (Evolution: mentioned). Ein Sender ohne diese Fähigkeit (stdout/file)
// bekommt den Text über SendText – "@<nummer>" bleibt dann einfach Text.
type MentionSender interface {
	SendTextMention(ctx context.Context, number, text string, mentioned []string) error
}

// WirtOutcome ist die Antwort von /wirt-reminder.
type WirtOutcome struct {
	Message   string `json:"message"`
	Recipient string `json:"recipient,omitempty"` // Ziel des echten Versands: die Stammtisch-Gruppe
	Mentioned string `json:"mentioned,omitempty"` // in der Nachricht markierte Nummer (leer = WIRT_JID nicht gesetzt)
	DryRun    bool   `json:"dryRun"`
	PreviewTo string `json:"previewTo,omitempty"`
}

// jidNumber: "491701234567@s.whatsapp.net" → "491701234567".
func jidNumber(jid string) string {
	if i := strings.IndexByte(jid, '@'); i >= 0 {
		return jid[:i]
	}
	return jid
}

// handleWirtReminder baut die wöchentliche Bitte an den Wirt, den Tisch
// freizuhalten. Sie soll in die Stammtisch-Gruppe gehen, der Wirt wird dort
// nur markiert – einen Direktchat mit ihm gibt es nicht. Bisher gibt es nur
// zwei Wege: ?dryRun=true berechnet den Text, ?preview=true schickt ihn
// zusätzlich an PREVIEW_JID und markiert dort die Vorschau-Nummer anstelle
// des Wirts. Mit &previewTarget=group geht die Vorschau stattdessen in die
// Testgruppe (PREVIEW_GROUP_JID) – nur in Gruppen zeigt WhatsApp eine
// Markierung als Namen. Der Parameter wählt nur zwischen den beiden fest
// konfigurierten Zielen; eine Adresse kommt nie aus dem Request. Einen echten Versand in die Gruppe gibt es noch NICHT – ein
// Aufruf ohne Parameter ist deshalb ebenfalls ein Dry-Run. Geplant ist der
// Versand donnerstags um 16 Uhr.
func (s *Server) handleWirtReminder(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	preview := q.Get("preview") == "true"
	previewTo := s.previewTarget(q)
	// Markiert wird in der Vorschau immer PREVIEW_JID – ohne sie gibt es
	// keine Vorschau, auch nicht in die Testgruppe.
	if preview && (s.PreviewJID == "" || previewTo == "") {
		http.Error(w, "Vorschau-Ziel nicht konfiguriert (PREVIEW_JID bzw. PREVIEW_GROUP_JID)", http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()

	// In der Vorschau wird die eigene Nummer markiert – der Wirt bekommt von
	// einem Test nichts mit.
	mention := jidNumber(s.WirtJID)
	if preview {
		mention = jidNumber(s.PreviewJID)
	}

	out := WirtOutcome{
		Message:   report.BuildWirtReminder(mention),
		Recipient: s.groupJID,
		Mentioned: mention,
		DryRun:    true,
	}

	if preview {
		target := previewTo
		var err error
		if ms, ok := s.sender.(MentionSender); ok {
			err = ms.SendTextMention(ctx, target, out.Message, []string{mention})
		} else {
			err = s.sender.SendText(ctx, target, out.Message)
		}
		if err != nil {
			log.Printf("⚠️  Wirt-Vorschau(%s): %v", target, err)
			http.Error(w, "Versand fehlgeschlagen: "+err.Error(), http.StatusBadGateway)
			return
		}
		out.PreviewTo = target
		out.DryRun = false
		log.Printf("📱 Wirt-Erinnerung als Vorschau gesendet an %s", target)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
