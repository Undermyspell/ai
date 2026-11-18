package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	sharedstore "github.com/michael/zumba-shared/store"

	"github.com/michael/zumba-admin-ui/internal/config"
	"github.com/michael/zumba-admin-ui/internal/store"
)

// wirt_test.go: Seite /tisch-erinnerung und das Übernehmen eines KI-Texts
// aus dem Bot-Test.

const wirtKIText = "Griaß di {wirt}! Halt uns heute Abend bitte unseren Tisch frei 🙏 Danke dir!"

// Die Uhr der Seite: Donnerstag, 19.11.2026.
func wirtUhr(t *testing.T, stunde int) {
	t.Helper()
	alt := wirtNow
	wirtNow = func() time.Time { return time.Date(2026, 11, 19, stunde, 30, 0, 0, time.Local) }
	t.Cleanup(func() { wirtNow = alt })
}

func wirtEntwurf() *store.WirtNachricht {
	return &store.WirtNachricht{
		Tag: mustDate("2026-11-19"), Text: wirtKIText, Model: "gemma-4-31b-it", Quelle: sharedstore.WirtQuelleAuto,
		ErzeugtAm: time.Date(2026, 11, 19, 10, 2, 0, 0, time.Local),
	}
}

func getWirt(spy *spyStore) string {
	rec := httptest.NewRecorder()
	New(spy, testCfg(), false).Routes().ServeHTTP(rec, httptest.NewRequest("GET", "/tisch-erinnerung", nil))
	return rec.Body.String()
}

func mustContain(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("Seite enthält %q nicht", want)
		}
	}
}

// Vor 10 Uhr gibt es noch keinen Text: die Seite zeigt die Standardnachricht
// als Fallback und nichts zum Ablehnen.
func TestWirtSeiteWartetAufDenText(t *testing.T) {
	wirtUhr(t, 8)
	spy := newSpyStore()
	body := getWirt(spy)
	mustContain(t, body, "Do., 19. November 2026 · 16:00", "wartet auf Do 10:00", "is-ghost",
		"bitte halt uns heute Abend unseren Tisch frei", "Gemma 4 31B", `<span class="mention">@Wirt</span>`)
	if strings.Contains(body, "/tisch-erinnerung/ablehnen") {
		t.Error("ohne KI-Text gibt es nichts abzulehnen")
	}
	if spy.wirtGefragt != "2026-11-19" {
		t.Errorf("gefragt nach %q, erwartet der heutige Donnerstag", spy.wirtGefragt)
	}
}

// Der vorgenerierte Text steht mit Rahmen und markiertem Wirt da und lässt
// sich ablehnen; danach zeigt die Seite die Standardnachricht.
func TestWirtSeiteZeigtDenTextUndLehntAb(t *testing.T) {
	wirtUhr(t, 11)
	spy := newSpyStore()
	spy.wirtTag = wirtEntwurf()
	body := getWirt(spy)
	mustContain(t, body, "Griaß di <span class=\"mention\">@Wirt</span>! Halt uns heute Abend",
		"🍻 *Stammtisch heute Abend*", "Automatische Erinnerung vom Zumba-Bot",
		"Gemma 4 31B · 10:02", "geht der Text heute um 16:00", `hx-post="/tisch-erinnerung/ablehnen"`)
	if strings.Contains(body, "{wirt}") {
		t.Error("Platzhalter steht roh auf der Seite")
	}

	h := New(spy, testCfg(), false).Routes()
	rec := postForm(t, h, "/tisch-erinnerung/ablehnen", url.Values{})
	if spy.wirtAblehnen != "2026-11-19=true" {
		t.Fatalf("SetWirtAbgelehnt = %q", spy.wirtAblehnen)
	}
	mustContain(t, rec.Body.String(), `id="wirt-region"`, "is-abgelehnt", "abgelehnt – stattdessen geht raus",
		"bitte halt uns heute Abend unseren Tisch frei", "Doch den KI-Text senden")

	// … und zurück.
	rec = postForm(t, h, "/tisch-erinnerung/ablehnen", url.Values{"zurueck": {"1"}})
	if spy.wirtAblehnen != "2026-11-19=false" || !strings.Contains(rec.Body.String(), "is-ki") {
		t.Errorf("Zurücknehmen: %q", spy.wirtAblehnen)
	}
}

// Ohne KI-Text nach 10 Uhr: die Seite sagt, dass die Standardnachricht geht –
// mit Grund, wenn der Bot einen festgehalten hat.
func TestWirtSeiteOhneTextNachZehn(t *testing.T) {
	wirtUhr(t, 11)
	spy := newSpyStore()
	mustContain(t, getWirt(spy), "Heute wurde kein KI-Text vorgeneriert")

	spy.wirtTag = &store.WirtNachricht{Tag: mustDate("2026-11-19"), Fehler: "Tonprüfung nicht bestanden"}
	mustContain(t, getWirt(spy), "nicht entstanden (Tonprüfung nicht bestanden)", "is-standard")
}

// Nach 16 Uhr geht es um den nächsten Donnerstag; der heutige steht im Verlauf.
func TestWirtSeiteNachDemVersand(t *testing.T) {
	wirtUhr(t, 17)
	spy := newSpyStore()
	gesendet := time.Date(2026, 11, 19, 16, 0, 0, 0, time.Local)
	heute := wirtEntwurf()
	heute.GesendetAm, heute.GesendetAls = &gesendet, sharedstore.WirtAlsKI
	spy.wirtVerlauf = []store.WirtNachricht{*heute, {Tag: mustDate("2026-11-12"), GesendetAm: &gesendet, GesendetAls: sharedstore.WirtAlsStandard}}
	body := getWirt(spy)
	mustContain(t, body, "Do., 26. November 2026 · 16:00", "Verlauf", "19.11.", "Griaß di @Wirt!", "12.11.")
	if spy.wirtGefragt != "2026-11-26" {
		t.Errorf("gefragt nach %q", spy.wirtGefragt)
	}
}

func TestWirtVersandSchalter(t *testing.T) {
	wirtUhr(t, 8)
	spy := newSpyStore()
	h := New(spy, testCfg(), false).Routes()

	rec := postForm(t, h, "/tisch-erinnerung/versand", url.Values{"versand": {"aus"}})
	if spy.wirt.Versand {
		t.Fatal("Versand ist noch an")
	}
	mustContain(t, rec.Body.String(), "Versand aus", "geht nichts an den Wirt")
	if strings.Contains(rec.Body.String(), "bubble-out") {
		t.Error("bei ausgeschaltetem Versand darf keine Nachricht als anstehend dastehen")
	}

	postForm(t, h, "/tisch-erinnerung/versand", url.Values{"versand": {"an"}})
	if !spy.wirt.Versand {
		t.Fatal("Versand ist noch aus")
	}
	if rec := postForm(t, h, "/tisch-erinnerung/versand", url.Values{"versand": {"vielleicht"}}); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("unbekannter Wert: Status %d", rec.Code)
	}
}

// Welches Modell den Text schreibt, wählt die Seite KI-Modell: dieselbe
// Auswahl wie beim Klassifizieren, etwas anderes speichert sie nicht. Die
// Seite Tisch-Erinnerung zeigt die Wahl nur an.
func TestWirtModellAufDerSeiteKIModell(t *testing.T) {
	wirtUhr(t, 8)
	spy := newSpyStore()
	h := New(spy, testCfg(), false).Routes()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/ki-modell", nil))
	body := rec.Body.String()
	mustContain(t, body, `id="tisch-erinnerung"`, "Tisch-Erinnerung an den Wirt", "Tisch-Text")
	for _, o := range modelOptions {
		mustContain(t, body, `data-wirt-model="`+o.ID+`"`)
	}

	rec = postForm(t, h, "/ki-modell/tisch-erinnerung", url.Values{"model": {"gemini-3.8-flash"}})
	if spy.wirt.Model != "gemini-3.8-flash" {
		t.Fatalf("Modell = %q", spy.wirt.Model)
	}
	// Die Region kommt neu: das gewählte Modell ist aktiv, die Kontingent-Zeile
	// rechnet mit ihm (4 von 20 Aufrufen).
	mustContain(t, rec.Body.String(), `id="model-region"`, "bis zu 4 von 20 Aufrufen", "5/min · 20/Tag")
	i := strings.Index(rec.Body.String(), `data-wirt-model="gemini-3.8-flash"`)
	if i < 0 || !strings.Contains(rec.Body.String()[i-120:i], "is-active") {
		t.Error("gewähltes Modell ist nicht als aktiv markiert")
	}

	if rec := postForm(t, h, "/ki-modell/tisch-erinnerung", url.Values{"model": {"gpt-unbekannt"}}); rec.Code != http.StatusUnprocessableEntity || spy.wirt.Model != "gemini-3.8-flash" {
		t.Errorf("unbekanntes Modell: Status %d, Modell %q", rec.Code, spy.wirt.Model)
	}

	seite := getWirt(spy)
	mustContain(t, seite, "Gemini 3.8 Flash", `href="/ki-modell#tisch-erinnerung"`)
	if strings.Contains(seite, "<select") {
		t.Error("die Seite Tisch-Erinnerung soll das Modell nur anzeigen")
	}
}

// Ein Sperrtag am nächsten Donnerstag: keine Erinnerung.
func TestWirtSeiteSperrtag(t *testing.T) {
	wirtUhr(t, 8)
	spy := newSpyStore()
	spy.sperrtage = []time.Time{mustDate("2026-11-19")}
	mustContain(t, getWirt(spy), "ist ein Sperrtag", "Sperrtag")
}

// wirtNeuBot steht für den Bot, der einen frischen KI-Text liefert.
func wirtNeuBot(t *testing.T, calls *[]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls = append(*calls, r.URL.Path+"?"+r.URL.RawQuery)
		switch r.URL.Query().Get("nachricht") {
		case "neu":
			_, _ = w.Write([]byte(`{"message":"🍻 Kopf\n\nGriaß di @498912345678!","mentioned":"498912345678","dryRun":true,
				"variante":"ki","model":"gemma-4-31b-it","text":"` + wirtKIText + `"}`))
		default:
			_, _ = w.Write([]byte(`{"message":"Servus @498912345678 👋","mentioned":"498912345678","dryRun":true,
				"variante":"standard","hinweis":"kein KI-Text für heute"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// Bot-Test: "Neu (KI)" lässt schreiben und bietet den Text zum Übernehmen
// an; "wie live" und "Standard" tun das nicht.
func TestBotTestWirtNeuUndUebernehmen(t *testing.T) {
	wirtUhr(t, 9)
	var calls []string
	bot := wirtNeuBot(t, &calls)
	spy := newSpyStore()
	h := New(spy, config.Config{BotURL: bot.URL}, false).Routes()

	body := runBotTest(h, url.Values{"szenario": {"wirt"}, "mode": {"preview"}, "wirtNachricht": {"neu"}})
	mustContain(t, body, "Griaß di @498912345678!", `hx-post="/bot-test/wirt/uebernehmen"`, "Für heute übernehmen",
		"KI schreibt den Text", "Gemma 4 31B")

	body = runBotTest(h, url.Values{"szenario": {"wirt"}, "mode": {"dryrun"}, "wirtNachricht": {"live"}})
	mustContain(t, body, "Standardnachricht – kein KI-Text für heute")
	if strings.Contains(body, "uebernehmen") {
		t.Error("die Standardnachricht lässt sich nicht übernehmen")
	}
	runBotTest(h, url.Values{"szenario": {"wirt"}, "mode": {"group"}, "wirtNachricht": {"standard"}})

	want := []string{
		"/wirt-reminder?preview=true&nachricht=neu",
		"/wirt-reminder?dryRun=true",
		"/wirt-reminder?preview=true&previewTarget=group&nachricht=standard",
	}
	if strings.Join(calls, " ") != strings.Join(want, " ") {
		t.Errorf("Bot-Aufrufe = %q", calls)
	}

	// Übernehmen: gilt für den nächsten Versand, der Tag kommt nicht aus dem Formular.
	rec := postForm(t, h, "/bot-test/wirt/uebernehmen", url.Values{"text": {wirtKIText}, "model": {"gemma-4-31b-it"}, "tag": {"2030-01-01"}})
	if spy.wirtAdopt != "2026-11-19|gemma-4-31b-it|"+wirtKIText {
		t.Fatalf("AdoptWirtText = %q", spy.wirtAdopt)
	}
	mustContain(t, rec.Body.String(), "Übernommen – geht heute um 16:00 raus", `href="/tisch-erinnerung"`)
}

// Was die feste Prüfung nicht besteht, wird nicht übernommen – und nach dem
// Versand ändert niemand mehr etwas.
func TestBotTestWirtUebernehmenLehntAb(t *testing.T) {
	wirtUhr(t, 9)
	spy := newSpyStore()
	h := New(spy, testCfg(), false).Routes()

	for name, text := range map[string]string{
		"ohne Markierung": "Servus! Halt uns heute Abend bitte unseren Tisch frei. Danke dir!",
		"fremde Nummer":   "Servus {wirt} und @4917099999999, haltet uns bitte den Tisch frei!",
		"leer":            "",
	} {
		rec := postForm(t, h, "/bot-test/wirt/uebernehmen", url.Values{"text": {text}, "model": {"gemma-4-31b-it"}})
		if rec.Code != http.StatusUnprocessableEntity || spy.wirtAdopt != "" {
			t.Errorf("%s: Status %d, übernommen %q", name, rec.Code, spy.wirtAdopt)
		}
	}

	spy.wirtAdoptNo = true
	if rec := postForm(t, h, "/bot-test/wirt/uebernehmen", url.Values{"text": {wirtKIText}}); rec.Code != http.StatusConflict {
		t.Errorf("schon gesendet: Status %d", rec.Code)
	}
}

// Die Seite Tisch-Erinnerung verlinkt in den Bot-Test mit vorgewähltem Szenario.
func TestBotTestSzenarioVorauswahl(t *testing.T) {
	rec := httptest.NewRecorder()
	New(newSpyStore(), testCfg(), false).Routes().ServeHTTP(rec, httptest.NewRequest("GET", "/bot-test?szenario=wirt", nil))
	body := rec.Body.String()
	mustContain(t, body, `id="wn-neu"`, `id="wn-live"`)
	i := strings.Index(body, `id="sz-wirt"`)
	if i < 0 || !strings.Contains(body[i:i+40], "checked") {
		t.Error("Szenario wirt ist nicht vorgewählt")
	}
}
