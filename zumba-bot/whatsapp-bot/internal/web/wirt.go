package web

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/michael/zumba-whatsapp-bot/internal/report"
)

// MentionSender verschickt einen Text, in dem Nummern markiert sind
// (Evolution: mentioned). Ein Sender ohne diese Fähigkeit (stdout/file)
// bekommt den Text über SendText – "@<nummer>" bleibt dann einfach Text.
type MentionSender interface {
	SendTextMention(ctx context.Context, number, text string, mentioned []string) error
}

// SperrtagSource sagt, ob an einem Tag kein Stammtisch stattfindet
// (excluded_days) – dann entfällt die Bitte an den Wirt.
type SperrtagSource interface {
	IsExcludedDay(ctx context.Context, day time.Time) (bool, error)
}

// WirtOutcome ist die Antwort von /wirt-reminder.
type WirtOutcome struct {
	Message   string `json:"message"`
	Recipient string `json:"recipient,omitempty"` // Ziel des echten Versands: die Stammtisch-Gruppe
	Mentioned string `json:"mentioned,omitempty"` // in der Nachricht markierte Nummer (leer = WIRT_JID nicht gesetzt)
	DryRun    bool   `json:"dryRun"`
	PreviewTo string `json:"previewTo,omitempty"`
	Sent      bool   `json:"sent"`             // echter Versand in die Gruppe ist rausgegangen
	Reason    string `json:"reason,omitempty"` // warum ein echter Versand entfiel
}

// jidNumber: "491701234567@s.whatsapp.net" → "491701234567".
func jidNumber(jid string) string {
	if i := strings.IndexByte(jid, '@'); i >= 0 {
		return jid[:i]
	}
	return jid
}

// handleWirtReminder baut die wöchentliche Bitte an den Wirt, den Tisch
// freizuhalten. Sie geht in die Stammtisch-Gruppe, der Wirt wird dort nur
// markiert – einen Direktchat mit ihm gibt es nicht.
//
// ?send=true ist der echte Versand (CronJob, donnerstags 16 Uhr). Er
// entfällt – mit Grund in der Antwort –, wenn heute kein Donnerstag ist, der
// Tag ein Sperrtag ist oder WIRT_JID fehlt. ?dryRun=true berechnet nur den
// Text, ?preview=true schickt ihn an PREVIEW_JID und markiert dort die
// Vorschau-Nummer anstelle des Wirts. Mit &previewTarget=group geht die Vorschau stattdessen in die
// Testgruppe (PREVIEW_GROUP_JID) – nur in Gruppen zeigt WhatsApp eine
// Markierung als Namen. Der Parameter wählt nur zwischen den beiden fest
// konfigurierten Zielen; eine Adresse kommt nie aus dem Request. Ein Aufruf
// ganz ohne Parameter ist ein Dry-Run – in die Gruppe geht nur etwas mit
// ausdrücklichem send=true.
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

	if !preview && q.Get("send") == "true" && q.Get("dryRun") != "true" {
		if out.Reason = s.wirtSkipReason(ctx); out.Reason != "" {
			log.Printf("ℹ️  Wirt-Erinnerung entfällt: %s", out.Reason)
		} else {
			var err error
			if ms, ok := s.sender.(MentionSender); ok {
				err = ms.SendTextMention(ctx, s.groupJID, out.Message, []string{mention})
			} else {
				err = s.sender.SendText(ctx, s.groupJID, out.Message)
			}
			if err != nil {
				log.Printf("⚠️  Wirt-Erinnerung(%s): %v", s.groupJID, err)
				http.Error(w, "Versand fehlgeschlagen: "+err.Error(), http.StatusBadGateway)
				return
			}
			out.Sent = true
			out.DryRun = false
			log.Printf("🍽️  Wirt-Erinnerung gesendet an %s", s.groupJID)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// wirtSkipReason nennt den Grund, aus dem der echte Versand heute entfällt
// ("" = senden). Antwortet die DB bei der Sperrtag-Frage nicht, geht die
// Erinnerung trotzdem raus: ein unnötig freigehaltener Tisch ist das kleinere
// Übel als ein Stammtisch ohne Tisch.
func (s *Server) wirtSkipReason(ctx context.Context) string {
	if s.WirtJID == "" {
		return "WIRT_JID nicht gesetzt – niemand zu markieren"
	}
	if !s.isThursday() {
		return "heute ist kein Donnerstag"
	}
	if s.Sperrtage != nil {
		excluded, err := s.Sperrtage.IsExcludedDay(ctx, s.today())
		if err != nil {
			log.Printf("⚠️  Sperrtag-Prüfung: %v – Erinnerung geht trotzdem raus", err)
		} else if excluded {
			return "heute ist ein Sperrtag"
		}
	}
	return ""
}
