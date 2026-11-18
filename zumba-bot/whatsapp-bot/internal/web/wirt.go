package web

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	sharedstore "github.com/michael/zumba-shared/store"
	"github.com/michael/zumba-shared/wirt"
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

// WirtStore hält Einstellungen und Tagesstand der Tisch-Erinnerung: ob der
// Versand an ist, welches Modell den Text schreibt, den vorgenerierten
// KI-Text des Tages und was rausging. Vor jedem Lauf gelesen, damit eine
// Änderung im Admin-UI ohne Neustart greift.
type WirtStore interface {
	WirtSettings(ctx context.Context) (sharedstore.WirtSettings, error)
	WirtNachricht(ctx context.Context, tag time.Time) (*sharedstore.WirtNachricht, error)
	WirtVorherige(ctx context.Context, limit int) ([]string, error)
	SaveWirtEntwurf(ctx context.Context, tag time.Time, text, model string) (bool, error)
	SaveWirtFehler(ctx context.Context, tag time.Time, fehler string) error
	MarkWirtGesendet(ctx context.Context, tag time.Time, als string) error
}

// WirtTexter lässt ein Sprachmodell den Text der Erinnerung schreiben
// (wirttext.Generator). Der Text enthält den Platzhalter für die Markierung
// und ist bereits geprüft.
type WirtTexter interface {
	Text(ctx context.Context, model string, day time.Time, vorherige []string) (string, error)
}

// wirtGedaechtnis: so viele frühere Texte bekommt das Modell zu sehen, damit
// es sich nicht wiederholt.
const wirtGedaechtnis = 5

// wirtGenerateTimeout deckt den schlechtesten Fall: zwei Modelle antworten
// nicht (je ein Aufruf bis zum Timeout), das dritte braucht zwei Versuche mit
// je zwei Aufrufen.
const wirtGenerateTimeout = 240 * time.Second

// Welche Nachricht ein Testlauf zeigt (?nachricht=). Der echte Versand kennt
// nur "wie live".
const (
	wirtWahlStandard = "standard" // immer die feste Standardnachricht
	wirtWahlNeu      = "neu"      // jetzt einen KI-Text schreiben lassen (wird nicht gespeichert)
)

// WirtOutcome ist die Antwort von /wirt-reminder.
type WirtOutcome struct {
	Message   string `json:"message"`
	Recipient string `json:"recipient,omitempty"` // Ziel des echten Versands: die Stammtisch-Gruppe
	Mentioned string `json:"mentioned,omitempty"` // in der Nachricht markierte Nummer (leer = WIRT_JID nicht gesetzt)
	DryRun    bool   `json:"dryRun"`
	PreviewTo string `json:"previewTo,omitempty"`
	Sent      bool   `json:"sent"`             // echter Versand in die Gruppe ist rausgegangen
	Reason    string `json:"reason,omitempty"` // warum ein echter Versand entfiel

	// Variante: "ki" (Text vom Sprachmodell) oder "standard" (fester Text).
	Variante string `json:"variante"`
	// Text ist der KI-Text ohne Kopf und Fußzeile, mit Platzhalter statt
	// Markierung – so übernimmt ihn das Admin-UI für den nächsten Versand.
	Text  string `json:"text,omitempty"`
	Model string `json:"model,omitempty"` // Modell, das den KI-Text geschrieben hat
	// Hinweis: warum es die Standardnachricht ist, obwohl der KI-Text gefragt war.
	Hinweis string `json:"hinweis,omitempty"`
}

// wirtAuswahl ist die Nachricht eines Laufs vor dem Einsetzen der Markierung.
type wirtAuswahl struct {
	text     string // mit Platzhalter, ohne Kopf/Fußzeile
	variante string
	model    string
	hinweis  string
}

func wirtStandard(hinweis string) wirtAuswahl {
	return wirtAuswahl{text: wirt.StandardText, variante: sharedstore.WirtAlsStandard, hinweis: hinweis}
}

// jidNumber: "491701234567@s.whatsapp.net" → "491701234567".
func jidNumber(jid string) string {
	if i := strings.IndexByte(jid, '@'); i >= 0 {
		return jid[:i]
	}
	return jid
}

// wirtTermin ist der Donnerstag der nächsten Erinnerung: heute, wenn
// Donnerstag ist und der Versand noch aussteht, sonst der folgende. Testläufe
// zeigen den Text dieses Tages – so sieht man auch am Mittwoch, was ein schon
// übernommener Text am Donnerstag ergibt.
func (s *Server) wirtTermin() time.Time {
	return wirt.NextVersand(s.Now().In(s.location))
}

// wirtStand liest den Stand des Tages tag; nil = nichts da oder nicht lesbar
// (dann gilt die Standardnachricht).
func (s *Server) wirtStand(ctx context.Context, tag time.Time) *sharedstore.WirtNachricht {
	if s.Wirt == nil {
		return nil
	}
	n, err := s.Wirt.WirtNachricht(ctx, tag)
	if err != nil {
		log.Printf("⚠️  wirt_nachrichten: %v – es gilt die Standardnachricht", err)
		return nil
	}
	return n
}

// wirtLive wählt, was am Tag von stand rausgeht: den vorgenerierten KI-Text,
// solange es ihn gibt, er nicht abgelehnt wurde und er die feste Prüfung
// (noch) besteht – sonst die Standardnachricht. Die muss immer möglich sein.
func (s *Server) wirtLive(stand *sharedstore.WirtNachricht, tag time.Time) wirtAuswahl {
	switch {
	case stand == nil || stand.Text == "":
		if stand != nil && stand.Fehler != "" {
			return wirtStandard("Vorgenerierung fehlgeschlagen: " + stand.Fehler)
		}
		return wirtStandard("kein KI-Text für den " + tag.Format("02.01."))
	case stand.AbgelehntAm != nil:
		return wirtStandard("KI-Text im Admin-UI abgelehnt")
	}
	if err := wirt.Validate(stand.Text); err != nil {
		log.Printf("⚠️  Wirt-Erinnerung: gespeicherter KI-Text ungültig (%v) – Standardnachricht", err)
		return wirtStandard("KI-Text ungültig: " + err.Error())
	}
	return wirtAuswahl{text: stand.Text, variante: sharedstore.WirtAlsKI, model: stand.Model}
}

// wirtModel ist das Modell, das den Text schreibt: die Wahl aus dem Admin-UI,
// sonst der Wert aus der Konfiguration.
func (s *Server) wirtModel(ctx context.Context) string {
	if s.Wirt != nil {
		set, err := s.Wirt.WirtSettings(ctx)
		if err != nil {
			log.Printf("⚠️  wirt_settings: %v (→ %s)", err, s.WirtModel)
		} else if set.Model != "" {
			return set.Model
		}
	}
	return s.WirtModel
}

// wirtModelle ist die Reihenfolge, in der Modelle den Text schreiben dürfen:
// zuerst das für die Tisch-Erinnerung gewählte, dann – falls es nicht
// antwortet oder nichts Brauchbares liefert – Haupt- und Fallback-Modell des
// Klassifikators. Doppelte fallen weg.
func (s *Server) wirtModelle(ctx context.Context) []string {
	kandidaten := []string{s.wirtModel(ctx)}
	if s.WirtAusweich != nil {
		p, f, err := s.WirtAusweich.ClassifierModels(ctx)
		if err != nil {
			log.Printf("⚠️  classifier_models: %v – Wirt-Text ohne Ausweichmodelle", err)
		}
		kandidaten = append(kandidaten, p, f)
	}
	var out []string
	for _, m := range kandidaten {
		if m != "" && !slices.Contains(out, m) {
			out = append(out, m)
		}
	}
	return out
}

// kurz macht aus einer Fehlermeldung eine Zeile – die Gemini-API antwortet
// mit mehrzeiligem JSON, das weder ins Log noch ins Admin-UI gehört.
func kurz(err error) string {
	msg := strings.Join(strings.Fields(err.Error()), " ")
	if r := []rune(msg); len(r) > 160 {
		msg = string(r[:160]) + "…"
	}
	return msg
}

// wirtNeu lässt jetzt einen KI-Text schreiben – vom gewählten Modell, sonst
// von den Ausweichmodellen (wirtModelle). Gespeichert wird hier nichts.
func (s *Server) wirtNeu(ctx context.Context) (wirtAuswahl, error) {
	if s.WirtText == nil {
		return wirtAuswahl{}, errors.New("kein Textgenerator konfiguriert")
	}
	ctx, cancel := context.WithTimeout(ctx, wirtGenerateTimeout)
	defer cancel()
	modelle := s.wirtModelle(ctx)
	if len(modelle) == 0 {
		return wirtAuswahl{}, errors.New("kein Modell gewählt")
	}
	var vorherige []string
	if s.Wirt != nil {
		var err error
		if vorherige, err = s.Wirt.WirtVorherige(ctx, wirtGedaechtnis); err != nil {
			log.Printf("⚠️  wirt_nachrichten: %v – Text ohne Rückblick", err)
		}
	}
	// Geschrieben wird für den Tag des Versands – ein am Mittwoch im Bot-Test
	// erzeugter Text soll vom Donnerstag reden.
	termin := s.wirtTermin()
	var fehler []string
	for i, model := range modelle {
		text, err := s.WirtText.Text(ctx, model, termin, vorherige)
		if err != nil {
			log.Printf("⚠️  Wirt-Text (%s): %s", model, kurz(err))
			fehler = append(fehler, kurz(err))
			continue
		}
		if i > 0 {
			log.Printf("ℹ️  Wirt-Text: %s ist für %s eingesprungen", model, modelle[0])
		}
		return wirtAuswahl{text: text, variante: sharedstore.WirtAlsKI, model: model}, nil
	}
	return wirtAuswahl{}, errors.New(strings.Join(fehler, " | "))
}

// sendWirt schickt die Erinnerung an target und markiert mention.
func (s *Server) sendWirt(ctx context.Context, target, message, mention string) error {
	if ms, ok := s.sender.(MentionSender); ok {
		return ms.SendTextMention(ctx, target, message, []string{mention})
	}
	return s.sender.SendText(ctx, target, message)
}

// handleWirtReminder baut die wöchentliche Bitte an den Wirt, den Tisch
// freizuhalten. Sie geht in die Stammtisch-Gruppe, der Wirt wird dort nur
// markiert – einen Direktchat mit ihm gibt es nicht.
//
// Der Text kommt, wenn möglich, von einem Sprachmodell: vormittags
// vorgeneriert (/wirt-reminder/generate), im Admin-UI einsehbar und
// ablehnbar. Gibt es keinen, wurde er abgelehnt oder ist er ungültig, geht
// die feste Standardnachricht raus. Kopf, Markierung und Fußzeile setzt in
// beiden Fällen der Bot.
//
// ?send=true ist der echte Versand (CronJob, donnerstags 16 Uhr). Er
// entfällt – mit Grund in der Antwort –, wenn der Versand im Admin-UI
// ausgeschaltet ist, heute kein Donnerstag ist, der Tag ein Sperrtag ist,
// WIRT_JID fehlt oder heute schon gesendet wurde. ?dryRun=true berechnet nur
// den Text, ?preview=true schickt ihn an PREVIEW_JID und markiert dort die
// Vorschau-Nummer anstelle des Wirts. Mit &previewTarget=group geht die
// Vorschau stattdessen in die Testgruppe (PREVIEW_GROUP_JID) – nur in Gruppen
// zeigt WhatsApp eine Markierung als Namen. Der Parameter wählt nur zwischen
// den beiden fest konfigurierten Zielen; eine Adresse kommt nie aus dem
// Request. Ein Aufruf ganz ohne Parameter ist ein Dry-Run – in die Gruppe
// geht nur etwas mit ausdrücklichem send=true.
//
// Testläufe wählen mit ?nachricht= den Text: ohne Angabe das, was beim
// nächsten Versand rausginge (wirtTermin), "standard" die feste Nachricht, "neu" einen jetzt geschriebenen
// KI-Text (nicht gespeichert – übernommen wird er im Admin-UI). Auch der Text
// einer Nachricht kommt nie aus dem Request.
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
	send := !preview && q.Get("send") == "true" && q.Get("dryRun") != "true"

	ctx := r.Context()
	// Der echte Versand gilt immer für heute; ein Testlauf zeigt den
	// nächsten Versand.
	tag := s.wirtTermin()
	if send {
		tag = s.today()
	}
	stand := s.wirtStand(ctx, tag)

	var wahl wirtAuswahl
	switch nachricht := q.Get("nachricht"); {
	case send:
		// Der echte Versand lässt sich nichts aussuchen.
		wahl = s.wirtLive(stand, tag)
	case nachricht == wirtWahlStandard:
		wahl = wirtStandard("")
	case nachricht == wirtWahlNeu:
		var err error
		if wahl, err = s.wirtNeu(ctx); err != nil {
			http.Error(w, "KI-Text nicht erzeugt: "+err.Error(), http.StatusBadGateway)
			return
		}
	default:
		wahl = s.wirtLive(stand, tag)
	}

	// In der Vorschau wird die eigene Nummer markiert – der Wirt bekommt von
	// einem Test nichts mit.
	mention := jidNumber(s.WirtJID)
	if preview {
		mention = jidNumber(s.PreviewJID)
	}

	out := WirtOutcome{
		Message:   wirt.Compose(wahl.text, mention),
		Recipient: s.groupJID,
		Mentioned: mention,
		DryRun:    true,
		Variante:  wahl.variante,
		Model:     wahl.model,
		Hinweis:   wahl.hinweis,
	}
	if wahl.variante == sharedstore.WirtAlsKI {
		out.Text = wahl.text
	}

	if preview {
		if err := s.sendWirt(ctx, previewTo, out.Message, mention); err != nil {
			log.Printf("⚠️  Wirt-Vorschau(%s): %v", previewTo, err)
			http.Error(w, "Versand fehlgeschlagen: "+err.Error(), http.StatusBadGateway)
			return
		}
		out.PreviewTo = previewTo
		out.DryRun = false
		log.Printf("📱 Wirt-Erinnerung als Vorschau gesendet an %s (%s)", previewTo, wahl.variante)
	}

	if send {
		out.Reason = s.wirtSkipReason(ctx)
		if out.Reason == "" && stand != nil && stand.GesendetAm != nil {
			out.Reason = "heute schon gesendet"
		}
		if out.Reason != "" {
			log.Printf("ℹ️  Wirt-Erinnerung entfällt: %s", out.Reason)
		} else {
			if err := s.sendWirt(ctx, s.groupJID, out.Message, mention); err != nil {
				log.Printf("⚠️  Wirt-Erinnerung(%s): %v", s.groupJID, err)
				http.Error(w, "Versand fehlgeschlagen: "+err.Error(), http.StatusBadGateway)
				return
			}
			out.Sent = true
			out.DryRun = false
			log.Printf("🍽️  Wirt-Erinnerung gesendet an %s (%s)", s.groupJID, wahl.variante)
			if wahl.hinweis != "" {
				log.Printf("ℹ️  Standardnachricht, weil: %s", wahl.hinweis)
			}
			if s.Wirt != nil {
				if err := s.Wirt.MarkWirtGesendet(ctx, tag, wahl.variante); err != nil {
					log.Printf("⚠️  wirt_nachrichten (gesendet): %v", err)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// WirtGenerateOutcome ist die Antwort von /wirt-reminder/generate.
type WirtGenerateOutcome struct {
	Tag    string `json:"tag"`              // Tag, für den der Text gilt (YYYY-MM-DD)
	Stored bool   `json:"stored"`           // ein neuer KI-Text ist gespeichert
	Text   string `json:"text,omitempty"`   // der KI-Text des Tages (neu oder schon vorhanden)
	Model  string `json:"model,omitempty"`  // Modell, das ihn geschrieben hat
	Reason string `json:"reason,omitempty"` // warum nichts erzeugt wurde
}

// handleWirtGenerate lässt den KI-Text für heute schreiben und legt ihn als
// Entwurf ab (CronJob, donnerstags 10 Uhr). Bis zum Versand um 16 Uhr ist er
// im Admin-UI einsehbar und ablehnbar. Es entsteht nichts, wenn heute ohnehin
// nichts rausgeht (wirtSkipReason) oder es für heute schon einen Text gibt –
// etwa einen aus dem Bot-Test übernommenen oder einen abgelehnten: ein
// Wiederholungslauf darf eine Ablehnung nicht aufheben.
//
// Liefert das Modell nichts Brauchbares, antwortet der Endpoint mit 502 (der
// CronJob versucht es dann noch einmal) und hält den Grund für das Admin-UI
// fest. Bleibt es dabei, geht nachmittags die Standardnachricht raus.
func (s *Server) handleWirtGenerate(w http.ResponseWriter, r *http.Request) {
	if s.Wirt == nil || s.WirtText == nil {
		http.Error(w, "KI-Text nicht konfiguriert", http.StatusServiceUnavailable)
		return
	}
	ctx := r.Context()
	tag := s.today()
	out := WirtGenerateOutcome{Tag: tag.Format("2006-01-02")}

	heute := s.wirtStand(ctx, tag)
	switch skip := s.wirtSkipReason(ctx); {
	case skip != "":
		out.Reason = skip
	case heute != nil && heute.GesendetAm != nil:
		out.Reason = "heute schon gesendet"
	case heute != nil && heute.Text != "":
		out.Reason = "für heute gibt es schon einen KI-Text"
		out.Text, out.Model = heute.Text, heute.Model
	}
	if out.Reason != "" {
		log.Printf("ℹ️  Wirt-Text entfällt: %s", out.Reason)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
		return
	}

	wahl, err := s.wirtNeu(ctx)
	if err != nil {
		log.Printf("⚠️  Wirt-Text: kein Modell hat geliefert – ohne neuen Versuch geht die Standardnachricht raus")
		if ferr := s.Wirt.SaveWirtFehler(ctx, tag, err.Error()); ferr != nil {
			log.Printf("⚠️  wirt_nachrichten (fehler): %v", ferr)
		}
		http.Error(w, "KI-Text nicht erzeugt: "+err.Error(), http.StatusBadGateway)
		return
	}
	stored, err := s.Wirt.SaveWirtEntwurf(ctx, tag, wahl.text, wahl.model)
	if err != nil {
		log.Printf("⚠️  wirt_nachrichten (entwurf): %v", err)
		http.Error(w, "KI-Text nicht gespeichert: "+err.Error(), http.StatusInternalServerError)
		return
	}
	out.Stored, out.Text, out.Model = stored, wahl.text, wahl.model
	if !stored {
		// Zwischen Lesen und Schreiben kam ein anderer Text dazu.
		out.Reason = "für heute gibt es schon einen KI-Text"
		out.Text, out.Model = "", ""
	} else {
		log.Printf("✍️  Wirt-Text für %s vorgeneriert (%s)", out.Tag, wahl.model)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// wirtSkipReason nennt den Grund, aus dem der echte Versand heute entfällt
// ("" = senden). Antwortet die DB bei der Frage nach dem Schalter oder dem
// Sperrtag nicht, geht die Erinnerung trotzdem raus: ein unnötig
// freigehaltener Tisch ist das kleinere Übel als ein Stammtisch ohne Tisch.
func (s *Server) wirtSkipReason(ctx context.Context) string {
	if s.WirtJID == "" {
		return "WIRT_JID nicht gesetzt – niemand zu markieren"
	}
	if !s.isThursday() {
		return "heute ist kein Donnerstag"
	}
	if s.Wirt != nil {
		set, err := s.Wirt.WirtSettings(ctx)
		if err != nil {
			log.Printf("⚠️  wirt_settings: %v – Erinnerung geht trotzdem raus", err)
		} else if !set.Versand {
			return "automatischer Versand ist im Admin-UI ausgeschaltet"
		}
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
