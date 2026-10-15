package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	sharedstore "github.com/michael/zumba-shared/store"

	"github.com/michael/zumba-admin-ui/internal/store"
	"github.com/michael/zumba-admin-ui/web/templates/kimodell"
)

type modelOption struct {
	ID    string
	Label string
	// PerMin/PerDay: kostenloses Kontingent von AI Studio (Stand 10/2026). Sie
	// entscheiden, wie viele Nachrichten ein Donnerstag verträgt.
	PerMin int
	PerDay int
}

// callsPerThursday: so viele Klassifizierungen braucht ein Donnerstag etwa,
// fehlgeschlagene Aufrufe zählen mit.
const callsPerThursday = 10

// modelOptions sind die Modelle, die die Seite anbietet – und die einzigen,
// die sie speichert.
var modelOptions = []modelOption{
	{ID: "gemma-4-31b-it", Label: "Gemma 4 31B", PerMin: 30, PerDay: 14400},
	{ID: "gemini-3.8-flash", Label: "Gemini 3.8 Flash", PerMin: 5, PerDay: 20},
	{ID: "gemini-3.5-flash-lite", Label: "Gemini 3.5 Flash Lite", PerMin: 15, PerDay: 500},
}

// limits: "30/min · 14.400/Tag".
func (o modelOption) limits() string {
	return fmt.Sprintf("%d/min · %s/Tag", o.PerMin, thousands(o.PerDay))
}

// thousands setzt deutsche Tausenderpunkte: 14400 → "14.400".
func thousands(n int) string {
	s := fmt.Sprintf("%d", n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	return s
}

func findModel(id string) (modelOption, bool) {
	for _, o := range modelOptions {
		if o.ID == id {
			return o, true
		}
	}
	return modelOption{}, false
}

var roleTitles = map[string]string{
	sharedstore.ClassifierPrimary:  "Hauptmodell",
	sharedstore.ClassifierFallback: "Fallback",
}

// quotaClass: wie knapp ein Donnerstag das Tageslimit macht.
func quotaClass(perDay int) string {
	switch {
	case perDay < 50:
		return "is-red"
	case perDay < 1000:
		return "is-amber"
	default:
		return "is-green"
	}
}

func kiModellVM(m store.ClassifierModels, shadow store.MLShadowStats) kimodell.VM {
	vm := kimodell.VM{}
	ref := func(rolle string, own sharedstore.ClassifierModel) kimodell.Ref {
		if own.Model == "" {
			vm.Unknown = append(vm.Unknown, roleTitles[rolle]+": noch nicht gesetzt – der Bot nimmt den Wert aus dem Deployment.")
			return kimodell.Ref{}
		}
		o, ok := findModel(own.Model)
		if !ok {
			vm.Unknown = append(vm.Unknown, roleTitles[rolle]+": aktiv ist "+own.Model+" (nicht in der Auswahl).")
			return kimodell.Ref{Label: own.Model}
		}
		return kimodell.Ref{Label: o.Label, Limits: o.limits()}
	}
	vm.Primary = ref(sharedstore.ClassifierPrimary, m.Primary)
	vm.Fallback = ref(sharedstore.ClassifierFallback, m.Fallback)

	since := m.Primary.UpdatedAt
	if m.Fallback.UpdatedAt.After(since) {
		since = m.Fallback.UpdatedAt
	}
	if !since.IsZero() {
		vm.Since = since.Local().Format("02.01.2006, 15:04")
	}

	for _, o := range modelOptions {
		card := kimodell.Model{
			ID: o.ID, Label: o.Label,
			PerMin:     fmt.Sprintf("%d", o.PerMin),
			PerDay:     thousands(o.PerDay),
			Meter:      max(3, min(100, callsPerThursday*100/o.PerDay)),
			MeterClass: quotaClass(o.PerDay),
			IsPrimary:  o.ID == m.Primary.Model,
			IsFallback: o.ID == m.Fallback.Model,
		}
		thursdays := o.PerDay / callsPerThursday
		switch {
		case o.PerDay < 50:
			card.Note = fmt.Sprintf("⚠ höchstens %d Donnerstage pro Tag – als Hauptmodell knapp", thursdays)
			card.NoteRed = true
		case o.PerDay < 1000:
			card.Note = fmt.Sprintf("~%d Donnerstage pro Tag", thursdays)
		default:
			card.Note = "praktisch unbegrenzt für einen Donnerstag"
		}
		vm.Models = append(vm.Models, card)
	}

	for _, r := range []struct {
		role string
		own  sharedstore.ClassifierModel
	}{{"Haupt", m.Primary}, {"Fallback", m.Fallback}} {
		o, ok := findModel(r.own.Model)
		if !ok {
			continue
		}
		p := float64(callsPerThursday) / float64(o.PerDay) * 100
		q := kimodell.Quota{
			Role: r.role, Label: o.Label,
			Text: fmt.Sprintf("%d von %s Aufrufen am Donnerstag", callsPerThursday, thousands(o.PerDay)),
			Pct:  fmt.Sprintf("%.0f %%", p),
		}
		if p < 1 {
			q.Pct = "<1 %"
		}
		switch {
		case p > 40:
			q.Class = "is-red"
		case p > 5:
			q.Class = "is-amber"
		default:
			q.Class = "is-green"
		}
		vm.Quota = append(vm.Quota, q)
	}

	vm.Shadow = kimodell.Shadow{AgreePct: "—", Verified: shadow.Verified, Total: shadow.Total}
	if shadow.WithModel > 0 {
		vm.Shadow.AgreePct = fmt.Sprintf("%.0f %%", float64(shadow.Agree)/float64(shadow.WithModel)*100)
	}
	return vm
}

func otherRole(rolle string) string {
	if rolle == sharedstore.ClassifierPrimary {
		return sharedstore.ClassifierFallback
	}
	return sharedstore.ClassifierPrimary
}

// shadowStats für die Seitenspalte – fehlt die Tabelle, bleibt die Karte leer.
func (s *Server) shadowStats(ctx context.Context) store.MLShadowStats {
	st, err := s.store.MLShadowStats(ctx)
	if err != nil {
		log.Printf("ml stats: %v", err)
	}
	return st
}

// classifierInfo ist die Antwort von GET /info am classifier-service.
type classifierInfo struct {
	Model     string  `json:"model"`
	Threshold float64 `json:"threshold"`
	Meta      struct {
		TrainiertAm    string `json:"trainiert_am"`
		Merkmale       string `json:"merkmale"`
		Gewichtung     string `json:"gewichtung"`
		Kopf           string `json:"kopf"`
		AnzahlMerkmale struct {
			Zeichen int `json:"zeichen"`
			Woerter int `json:"woerter"`
		} `json:"anzahl_merkmale"`
		Training struct {
			Synthetics  int    `json:"synthetics"`
			Whatsapp    int    `json:"whatsapp"`
			WhatsappBis string `json:"whatsapp_bis"`
		} `json:"training"`
		Test struct {
			MacroF1    float64   `json:"macro_f1"`
			Bereich    []float64 `json:"macro_f1_bereich"`
			Pruefaelle string    `json:"pruefaelle"`
		} `json:"test"`
	} `json:"meta"`
}

var infoClient = &http.Client{Timeout: 2 * time.Second}

// mockClassifierInfo zeigt im Mock-Modus (ohne classifier-service), wie die
// Karte aussieht.
var mockClassifierInfo = func() *classifierInfo {
	var ci classifierInfo
	_ = json.Unmarshal([]byte(`{"model":"NB-LogReg 0.2.0","threshold":0.45,"meta":{"trainiert_am":"2026-10-15",
		"merkmale":"Zeichen-n-Gramme 2–4 + Wort-n-Gramme 1–2, binär","gewichtung":"Naive-Bayes-Log-Verhältnis je Klasse",
		"kopf":"logistische Regression je Klasse (one-vs-rest), Sigmoid, auf Summe 1 normiert",
		"anzahl_merkmale":{"zeichen":18490,"woerter":13621},
		"training":{"synthetics":1596,"whatsapp":130,"whatsapp_bis":"2026-10-15"},
		"test":{"macro_f1":0.9185,"macro_f1_bereich":[0.829,0.9801],"pruefaelle":"10/10"}}}`), &ci)
	return &ci
}()

// fetchClassifierInfo fragt den classifier-service nach seinem Modell.
// configured=false: keine CLASSIFIER_URL (im Mock-Modus gibt es ein Beispiel).
func (s *Server) fetchClassifierInfo(ctx context.Context) (info *classifierInfo, configured bool) {
	if s.cfg.ClassifierURL == "" {
		if s.mockMode {
			return mockClassifierInfo, true
		}
		return nil, false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(s.cfg.ClassifierURL, "/")+"/info", nil)
	if err != nil {
		return nil, true
	}
	resp, err := infoClient.Do(req)
	if err != nil {
		log.Printf("classifier info: %v", err)
		return nil, true
	}
	defer resp.Body.Close()
	var ci classifierInfo
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&ci) != nil || ci.Model == "" {
		log.Printf("classifier info: HTTP %d bzw. unlesbar", resp.StatusCode)
		return nil, true
	}
	return &ci, true
}

// germanNum: 0.45 → „0,45".
func germanNum(f float64) string { return strings.Replace(fmt.Sprintf("%.2f", f), ".", ",", 1) }

// germanDate: „2026-10-15" → „15.10.2026" (sonst unverändert).
func germanDate(iso string) string {
	if t, err := time.Parse("2006-01-02", iso); err == nil {
		return t.Format("02.01.2006")
	}
	return iso
}

func mlInfoVM(info *classifierInfo, configured bool) kimodell.MLInfo {
	ml := kimodell.MLInfo{Configured: configured, OK: info != nil}
	if info == nil {
		return ml
	}
	m := info.Meta
	ml.Name = info.Model
	ml.Trained = germanDate(m.TrainiertAm)
	ml.Threshold = germanNum(info.Threshold)
	ml.Facts = []kimodell.Fact{
		{Label: "Merkmale", Value: fmt.Sprintf("%s (%s Zeichen-, %s Wort-n-Gramme)", m.Merkmale,
			thousands(m.AnzahlMerkmale.Zeichen), thousands(m.AnzahlMerkmale.Woerter))},
		{Label: "Gewichtung", Value: m.Gewichtung},
		{Label: "Kopf", Value: m.Kopf},
		{Label: "Schwelle", Value: ml.Threshold + " – darunter invalid, der Bot tut nichts"},
		{Label: "Training", Value: fmt.Sprintf("%s Synthetics + %d echte Nachrichten (vor dem %s)",
			thousands(m.Training.Synthetics), m.Training.Whatsapp, germanDate(m.Training.WhatsappBis))},
	}
	if m.Test.MacroF1 > 0 {
		test := "Macro-F1 " + germanNum(m.Test.MacroF1)
		if len(m.Test.Bereich) == 2 {
			test += fmt.Sprintf(" (%s–%s)", germanNum(m.Test.Bereich[0]), germanNum(m.Test.Bereich[1]))
		}
		if m.Test.Pruefaelle != "" {
			test += " · Prüffälle " + m.Test.Pruefaelle
		}
		ml.Facts = append(ml.Facts, kimodell.Fact{Label: "Test", Value: test})
	}
	return ml
}

// buildKIModellVM ergänzt die Anzeige um Entscheider und eigenes Modell.
func (s *Server) buildKIModellVM(ctx context.Context, m store.ClassifierModels) kimodell.VM {
	vm := kiModellVM(m, s.shadowStats(ctx))
	e, err := s.store.ClassifierEntscheider(ctx)
	if err != nil {
		log.Printf("classifier entscheider: %v", err)
	}
	vm.Entscheider = e.Wer
	if vm.Entscheider == "" {
		vm.Entscheider = sharedstore.EntscheiderGemini
	}
	if !e.UpdatedAt.IsZero() {
		vm.EntscheiderSince = e.UpdatedAt.Local().Format("02.01.2006, 15:04")
	}
	vm.ML = mlInfoVM(s.fetchClassifierInfo(ctx))
	return vm
}

func (s *Server) handleKIModell(w http.ResponseWriter, r *http.Request) {
	m, err := s.store.ClassifierModels(r.Context())
	if err != nil {
		s.fail(w, "classifier models", err)
		return
	}
	s.render(w, r, s.meta("KI-Modell", "kimodell"), kimodell.Page(s.buildKIModellVM(r.Context(), m)))
}

// handleKIModellEntscheider stellt um, wer entscheidet. Ist der
// classifier-service gerade nicht erreichbar, wird trotzdem gespeichert – der
// Bot fällt dann pro Nachricht auf Gemini zurück –, aber der Toast warnt.
func (s *Server) handleKIModellEntscheider(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wer := r.FormValue("wer")
	if wer != sharedstore.EntscheiderGemini && wer != sharedstore.EntscheiderModell {
		s.triggerToast(w, "error", "Unbekannte Auswahl.")
		http.Error(w, "unbekannt", http.StatusUnprocessableEntity)
		return
	}
	if err := s.store.SetClassifierEntscheider(ctx, wer); err != nil {
		s.fail(w, "set entscheider", err)
		return
	}
	log.Printf("⚖️  Entscheider → %s", wer)
	m, err := s.store.ClassifierModels(ctx)
	if err != nil {
		s.fail(w, "classifier models", err)
		return
	}
	vm := s.buildKIModellVM(ctx, m)
	switch {
	case wer == sharedstore.EntscheiderGemini:
		s.triggerToast(w, "success", "Ab der nächsten Nachricht entscheidet Gemini – das eigene Modell läuft mit.")
	case !vm.ML.OK:
		s.triggerToast(w, "error", "Gespeichert – aber der classifier-service antwortet nicht. Bis er da ist, entscheidet Gemini.")
	default:
		s.triggerToast(w, "success", "Ab der nächsten Nachricht entscheidet "+vm.ML.Name+" – Gemini läuft mit.")
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := kimodell.Region(vm).Render(ctx, w); err != nil {
		log.Printf("render ki-modell region: %v", err)
	}
}

func (s *Server) handleKIModellSet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rolle, model := r.FormValue("rolle"), r.FormValue("model")
	if _, ok := roleTitles[rolle]; !ok {
		s.triggerToast(w, "error", "Unbekannte Rolle.")
		http.Error(w, "unbekannte Rolle", http.StatusUnprocessableEntity)
		return
	}
	opt, ok := findModel(model)
	if !ok {
		s.triggerToast(w, "error", "Unbekanntes Modell.")
		http.Error(w, "unbekanntes Modell", http.StatusUnprocessableEntity)
		return
	}
	m, err := s.store.ClassifierModels(ctx)
	if err != nil {
		s.fail(w, "classifier models", err)
		return
	}
	other := m.Primary.Model
	if rolle == sharedstore.ClassifierPrimary {
		other = m.Fallback.Model
	}
	if model == other {
		// Kein 422: die Region kommt neu, damit die Auswahl wieder den
		// gespeicherten Stand zeigt statt des abgelehnten Klicks.
		s.triggerToast(w, "error", opt.Label+" ist schon "+roleTitles[otherRole(rolle)]+" – Haupt- und Fallback-Modell müssen sich unterscheiden.")
		s.renderKIModellRegion(w, r, m)
		return
	}
	if err := s.store.SetClassifierModel(ctx, rolle, model); err != nil {
		s.fail(w, "set classifier model", err)
		return
	}
	log.Printf("🧠 KI-Modell: %s → %s", rolle, model)
	if m, err = s.store.ClassifierModels(ctx); err != nil {
		s.fail(w, "classifier models", err)
		return
	}
	s.triggerToast(w, "success", roleTitles[rolle]+": "+opt.Label+" – gilt ab der nächsten Nachricht.")
	s.renderKIModellRegion(w, r, m)
}

func (s *Server) renderKIModellRegion(w http.ResponseWriter, r *http.Request, m store.ClassifierModels) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := kimodell.Region(s.buildKIModellVM(r.Context(), m)).Render(r.Context(), w); err != nil {
		log.Printf("render ki-modell region: %v", err)
	}
}
