package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/michael/zumba-admin-ui/web/templates/bottest"
)

var botExampleKinds = map[string]bool{"statistik": true, "absage": true, "zusage": true}

func (s *Server) loadExample(kind string) (string, bool) {
	if !botExampleKinds[kind] {
		return "", false
	}
	raw, err := bottest.Examples.ReadFile("examples/" + kind + ".json")
	if err != nil {
		return "", false
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err != nil {
		return string(raw), true
	}
	return pretty.String(), true
}

func (s *Server) handleBotTest(w http.ResponseWriter, r *http.Request) {
	def, _ := s.loadExample("statistik")
	s.render(w, r, s.meta("Bot-Test", "bottest"),
		bottest.Page(bottest.PageVM{
			DefaultKind: "statistik", DefaultJSON: def,
			Model: s.primaryModel(r), Time: time.Now().Format("15:04"),
		}))
}

// primaryModel ist das aktive Hauptmodell – nur für die Anzeige im
// Verarbeitungspfad, ein Fehler lässt die Angabe einfach weg.
func (s *Server) primaryModel(r *http.Request) string {
	m, err := s.store.ClassifierModels(r.Context())
	if err != nil {
		return ""
	}
	return m.Primary.Model
}

func (s *Server) handleBotTestExample(w http.ResponseWriter, r *http.Request) {
	body, ok := s.loadExample(r.PathValue("kind"))
	if !ok {
		http.Error(w, "unbekannt", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(body))
}

// botOutcome mirrors the whatsapp-bot Outcome JSON.
type botOutcome struct {
	Path           string `json:"path"`
	Classification string `json:"classification"`
	Action         string `json:"action"`
	Message        string `json:"message"`
	Recipient      string `json:"recipient"`
	Date           string `json:"date"`
	UserID         string `json:"userId"`
	Reason         string `json:"reason"`
	DryRun         bool   `json:"dryRun"`
	PreviewTo      string `json:"previewTo"`
	Mentioned      string `json:"mentioned"` // nur /wirt-reminder
	ImageBase64    string `json:"imageBase64"`
}

// modeQuery übersetzt den Modus der Bot-Test-Seite in den Query-Parameter des Bots.
// Die Testseite kennt dryrun, preview (eigene Nummer) und group (Testgruppe)
// – sie löst NIE einen Versand in die Stammtisch-Gruppe aus (der passiert nur
// über echte Statistik-Webhooks + CronJob). Die Adressen der beiden
// Vorschau-Ziele kennt nur der Bot, hier geht bloß die Wahl mit.
func modeQuery(mode string) string {
	switch mode {
	case "preview":
		return "?preview=true"
	case "group":
		return "?preview=true&previewTarget=group"
	}
	return "?dryRun=true"
}

// botRun beschreibt einen Testlauf für den Verarbeitungspfad.
type botRun struct {
	weekly bool
	wirt   bool // Tisch-Erinnerung an den Wirt (/wirt-reminder)
	image  bool
	design string
	model  string
}

// handleBotTestRun führt den gewählten Testlauf aus. Das Szenario bestimmt den
// Bot-Endpoint: "wochenreport" ruft /weekly-report und "wirt" /wirt-reminder
// ohne Body auf, alles andere schickt das Beispiel-Event an /test. Modus, Stichtag und Ausgabeformat gelten
// für beide Wege – deshalb liegt alles in einem Formular.
func (s *Server) handleBotTestRun(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	run := botRun{
		weekly: r.FormValue("szenario") == "wochenreport",
		image:  r.FormValue("format") == "image",
		design: r.FormValue("style"),
	}
	if r.FormValue("szenario") == "wirt" {
		// Die Tisch-Erinnerung kennt weder Stichtag noch Formate: sie geht
		// immer donnerstags als Text raus.
		url := strings.TrimRight(s.cfg.BotURL, "/") + "/wirt-reminder" + modeQuery(r.FormValue("mode"))
		s.proxyBot(w, r, url, nil, botRun{wirt: true})
		return
	}
	endpoint := "/test"
	if run.weekly {
		endpoint = "/weekly-report"
	}
	url := strings.TrimRight(s.cfg.BotURL, "/") + endpoint + modeQuery(r.FormValue("mode"))

	// Stichtag gilt für den Statistik-Pfad wie für den Wochenreport.
	if date := r.FormValue("date"); date != "" {
		url += "&date=" + date
	}
	if run.image {
		url += "&format=image"
		if cs := r.FormValue("cardStyle"); cs != "" {
			url += "&cardStyle=" + cs
			run.design = cs
		}
	} else if style := r.FormValue("style"); style != "" && style != "klassik" {
		// Alternative Textdesigns kennt nur der /test-Endpoint.
		url += "&style=" + style
	}
	if run.design == "" {
		run.design = "klassik"
	}

	if run.weekly {
		s.proxyBot(w, r, url, nil, run)
		return
	}
	run.model = s.primaryModel(r)
	s.proxyBot(w, r, url, strings.NewReader(r.FormValue("payload")), run)
}

// proxyBot schickt eine POST-Anfrage an den Bot und rendert dessen Outcome
// (bzw. ein Fehler-Panel) als HTMX-Fragment.
func (s *Server) proxyBot(w http.ResponseWriter, r *http.Request, url string, body io.Reader, run botRun) {
	client := &http.Client{Timeout: 35 * time.Second}
	req, err := http.NewRequestWithContext(r.Context(), "POST", url, body)
	if err != nil {
		_ = bottest.ErrorPanel(err.Error()).Render(r.Context(), w)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		_ = bottest.ErrorPanel("Bot nicht erreichbar: "+err.Error()).Render(r.Context(), w)
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	took := time.Since(start)
	if resp.StatusCode != http.StatusOK {
		_ = bottest.ErrorPanel("Bot-Status "+resp.Status+": "+string(respBody)).Render(r.Context(), w)
		return
	}
	var out botOutcome
	if err := json.Unmarshal(respBody, &out); err != nil {
		_ = bottest.ErrorPanel("Antwort nicht lesbar: "+err.Error()).Render(r.Context(), w)
		return
	}
	if run.wirt {
		out.Path = "wirt" // der Bot liefert für die Erinnerung keinen Pfad
	}
	format := "Text"
	if run.image {
		format = "PNG via renderer-service"
	}
	meta := fmt.Sprintf("%s · %d ms", format, took.Milliseconds())
	if out.Path == "classify" && run.model != "" {
		meta = fmt.Sprintf("%s · %d ms", run.model, took.Milliseconds())
	}
	_ = bottest.Response(bottest.ResponseVM{
		Path: out.Path, Classification: out.Classification, Action: out.Action,
		Message: out.Message, Recipient: out.Recipient, Date: out.Date, UserID: out.UserID,
		DryRun: out.DryRun, PreviewTo: out.PreviewTo, ImageBase64: out.ImageBase64,
		Mentioned: out.Mentioned,
		Time:      time.Now().Format("15:04"),
		Steps:     botSteps(out, run),
		Meta:      meta,
	}).Render(r.Context(), w)
}

// botSteps übersetzt das Outcome des Bots in den Verarbeitungspfad.
func botSteps(out botOutcome, run botRun) []bottest.Step {
	done := func(label, detail string) bottest.Step {
		return bottest.Step{Label: label, Detail: detail, State: "done"}
	}
	skipped := func(label, detail string) bottest.Step {
		return bottest.Step{Label: label, Detail: detail, State: "skipped"}
	}
	render := done("Text rendern", "Design „"+run.design+"“")
	if run.image {
		render = done("PNG rendern", "renderer-service · „"+run.design+"“")
	}
	send := done("An Gruppe senden", "→ "+out.Recipient)
	switch {
	case out.PreviewTo != "":
		send = done("An Gruppe senden", "Vorschau → "+out.PreviewTo)
	case out.DryRun:
		send = skipped("An Gruppe senden", "Dry-Run: nicht gesendet")
	}

	if run.wirt {
		mark := skipped("Wirt markieren", "WIRT_JID nicht gesetzt")
		switch {
		case out.Mentioned != "" && out.PreviewTo != "":
			mark = done("Wirt markieren", "@"+out.Mentioned+" (du statt des Wirts)")
		case out.Mentioned != "":
			mark = done("Wirt markieren", "@"+out.Mentioned)
		}
		return []bottest.Step{
			done("CronJob ausgelöst", "Do 16:00 (simuliert)"),
			done("Nachricht bauen", "Text"),
			mark,
			send,
		}
	}
	if run.weekly {
		return []bottest.Step{
			done("CronJob ausgelöst", "Do 21:00 (simuliert)"),
			done("Rangliste + Strafen berechnen", out.Date),
			render,
			send,
		}
	}
	steps := []bottest.Step{done("Webhook empfangen", "Typ „conversation“")}
	switch out.Path {
	case "statistik":
		return append(steps, done("„statistik“?", "ja"), done("Rangliste berechnen", ""), render, send)
	case "classify":
		classifier := "Klassifikator"
		if run.model != "" {
			classifier += " (" + run.model + ")"
		}
		action := done("DB-Aktion", actionText(out.Action))
		if out.DryRun || strings.HasPrefix(out.Action, "would_") || out.Action == "" || out.Action == "none" {
			action = skipped("DB-Aktion", actionText(out.Action)+" (nicht geschrieben)")
		}
		return append(steps,
			done("„statistik“?", "nein"),
			done("Guards: Typ · Gruppe · Donnerstag", "umgangen (Bot-Test)"),
			done(classifier, "→ "+clsText(out.Classification)),
			action,
		)
	default:
		return append(steps,
			done("„statistik“?", "nein"),
			bottest.Step{Label: "Guards", Detail: orDash(out.Reason), State: "error"},
		)
	}
}

func clsText(c string) string {
	switch c {
	case "true":
		return "Zusage (true)"
	case "false":
		return "Absage (false)"
	default:
		return "invalid"
	}
}

func actionText(a string) string {
	switch a {
	case "marked_absent", "would_mark_absent":
		return "Absage eintragen"
	case "marked_present", "would_mark_present":
		return "Absage entfernen"
	default:
		return "keine"
	}
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
