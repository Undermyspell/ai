package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sharedstore "github.com/michael/zumba-shared/store"
	"github.com/michael/zumba-shared/wirt"
)

// wirt_ki_test.go: KI-Text der Tisch-Erinnerung – vorgenerierter Entwurf,
// Ablehnung, Schalter im Admin-UI und der Endpoint /wirt-reminder/generate.

const kiText = "Griaß di {wirt}! Halt uns heute Abend bitte unseren Tisch frei 🙏 Danke dir!"

// fakeWirtStore hält Einstellungen und den Stand eines Tages im Speicher.
type fakeWirtStore struct {
	settings    sharedstore.WirtSettings
	settingsErr error
	heute       *sharedstore.WirtNachricht
	vorherige   []string

	gefragt     string // Tag (YYYY-MM-DD) des letzten WirtNachricht
	gesendetAls string
	fehler      string
	saved       int
}

func newFakeWirtStore() *fakeWirtStore {
	return &fakeWirtStore{settings: sharedstore.WirtSettings{Versand: true, Model: "db-model"}}
}

func (f *fakeWirtStore) WirtSettings(context.Context) (sharedstore.WirtSettings, error) {
	return f.settings, f.settingsErr
}
func (f *fakeWirtStore) WirtNachricht(_ context.Context, tag time.Time) (*sharedstore.WirtNachricht, error) {
	f.gefragt = tag.Format("2006-01-02")
	return f.heute, nil
}
func (f *fakeWirtStore) WirtVorherige(context.Context, int) ([]string, error) {
	return f.vorherige, nil
}
func (f *fakeWirtStore) SaveWirtEntwurf(_ context.Context, tag time.Time, text, model string) (bool, error) {
	if f.heute != nil && f.heute.Text != "" {
		return false, nil
	}
	f.saved++
	f.heute = &sharedstore.WirtNachricht{Tag: tag, Text: text, Model: model, Quelle: sharedstore.WirtQuelleAuto}
	return true, nil
}
func (f *fakeWirtStore) SaveWirtFehler(_ context.Context, _ time.Time, fehler string) error {
	f.fehler = fehler
	return nil
}
func (f *fakeWirtStore) MarkWirtGesendet(_ context.Context, _ time.Time, als string) error {
	f.gesendetAls = als
	return nil
}

// fakeTexter liefert einen festen Text und merkt sich, womit er gefragt wurde.
type fakeTexter struct {
	text      string
	err       error
	model     string
	vorherige []string
	calls     int
}

func (f *fakeTexter) Text(_ context.Context, model string, _ time.Time, vorherige []string) (string, error) {
	f.calls++
	f.model, f.vorherige = model, vorherige
	return f.text, f.err
}

// kiServer ist ein wirtServer am Donnerstag mit Tagesstand und Textgenerator.
func kiServer() (*Server, *mentionSender, *fakeWirtStore, *fakeTexter) {
	s, snd := wirtServerAm(thursday)
	st := newFakeWirtStore()
	tx := &fakeTexter{text: kiText}
	s.Wirt, s.WirtText, s.WirtModel = st, tx, "cfg-model"
	return s, snd, st, tx
}

func entwurf() *sharedstore.WirtNachricht {
	return &sharedstore.WirtNachricht{Text: kiText, Model: "gemma", Quelle: sharedstore.WirtQuelleAuto}
}

// Der vorgenerierte Text geht raus, wenn ihn niemand abgelehnt hat – mit
// Kopf, markiertem Wirt und Fußzeile, die der Bot setzt.
func TestWirtSendetDenVorgeneriertenText(t *testing.T) {
	s, snd, st, tx := kiServer()
	st.heute = entwurf()
	_, out := postWirt(t, s, "?send=true")

	if !out.Sent || out.Variante != sharedstore.WirtAlsKI || st.gesendetAls != sharedstore.WirtAlsKI {
		t.Fatalf("Outcome = %+v, gesendetAls = %q", out, st.gesendetAls)
	}
	if !strings.HasPrefix(snd.text, wirt.Kopf) || !strings.HasSuffix(snd.text, wirt.Fuss) {
		t.Errorf("Rahmen fehlt:\n%s", snd.text)
	}
	if !strings.Contains(snd.text, "Griaß di @"+testWirtNr+"!") || strings.Contains(snd.text, wirt.Platzhalter) {
		t.Errorf("Wirt nicht markiert:\n%s", snd.text)
	}
	if len(snd.mentioned) != 1 || snd.mentioned[0] != testWirtNr {
		t.Errorf("mentioned = %v", snd.mentioned)
	}
	if tx.calls != 0 {
		t.Error("der Versand darf nichts neu schreiben lassen")
	}
}

// Abgelehnt, nie erzeugt, fehlgeschlagen oder kaputt: dann die
// Standardnachricht – sie muss immer möglich sein.
func TestWirtFaelltAufDieStandardnachrichtZurueck(t *testing.T) {
	jetzt := time.Now()
	cases := map[string]*sharedstore.WirtNachricht{
		"kein Entwurf":    nil,
		"abgelehnt":       {Text: kiText, AbgelehntAm: &jetzt},
		"fehlgeschlagen":  {Fehler: "Modell antwortet nicht"},
		"ohne Markierung": {Text: "Servus! Halt uns bitte heute Abend den Tisch frei. Danke dir!"},
	}
	for name, heute := range cases {
		s, snd, st, _ := kiServer()
		st.heute = heute
		_, out := postWirt(t, s, "?send=true")
		if !out.Sent || out.Variante != sharedstore.WirtAlsStandard || st.gesendetAls != sharedstore.WirtAlsStandard {
			t.Errorf("%s: Outcome = %+v, gesendetAls = %q", name, out, st.gesendetAls)
		}
		if snd.text != wirt.Standard(testWirtNr) {
			t.Errorf("%s: nicht die Standardnachricht:\n%s", name, snd.text)
		}
		if out.Hinweis == "" {
			t.Errorf("%s: Hinweis fehlt, warum es die Standardnachricht ist", name)
		}
	}
}

func TestWirtVersandAusgeschaltet(t *testing.T) {
	s, snd, st, _ := kiServer()
	st.heute = entwurf()
	st.settings.Versand = false
	_, out := postWirt(t, s, "?send=true")
	if snd.called || out.Sent || !strings.Contains(out.Reason, "ausgeschaltet") || st.gesendetAls != "" {
		t.Fatalf("called = %v, Outcome = %+v", snd.called, out)
	}
	// Ein Test bleibt trotzdem möglich.
	if _, out := postWirt(t, s, "?preview=true"); out.PreviewTo != testPreview {
		t.Errorf("Vorschau trotz Schalter aus erwartet: %+v", out)
	}
}

// Ist der Schalter nicht lesbar, geht die Erinnerung raus – wie beim Sperrtag.
func TestWirtSchalterNichtLesbarSendetTrotzdem(t *testing.T) {
	s, snd, st, _ := kiServer()
	st.settingsErr = errors.New("db weg")
	if _, out := postWirt(t, s, "?send=true"); !snd.called || !out.Sent {
		t.Fatalf("Outcome = %+v", out)
	}
}

func TestWirtSendetNichtZweimalAmSelbenTag(t *testing.T) {
	s, snd, st, _ := kiServer()
	jetzt := time.Now()
	st.heute = &sharedstore.WirtNachricht{GesendetAm: &jetzt, GesendetAls: sharedstore.WirtAlsStandard}
	_, out := postWirt(t, s, "?send=true")
	if snd.called || out.Sent || !strings.Contains(out.Reason, "schon gesendet") {
		t.Fatalf("called = %v, Outcome = %+v", snd.called, out)
	}
}

// Bot-Test: "wie live" zeigt den Entwurf, "standard" die feste Nachricht,
// "neu" lässt schreiben – gespeichert wird dabei nichts.
func TestWirtTestlaufWaehltDieNachricht(t *testing.T) {
	s, snd, st, tx := kiServer()
	st.heute = entwurf()
	st.vorherige = []string{"alt {wirt}"}
	tx.text = "Servus {wirt}, ganz frisch: halt uns bitte heute Abend den Tisch frei. Danke!"

	_, live := postWirt(t, s, "?dryRun=true")
	if live.Variante != sharedstore.WirtAlsKI || live.Text != kiText || live.Model != "gemma" {
		t.Errorf("wie live: %+v", live)
	}
	_, std := postWirt(t, s, "?dryRun=true&nachricht=standard")
	if std.Variante != sharedstore.WirtAlsStandard || std.Text != "" || std.Message != wirt.Standard(testWirtNr) {
		t.Errorf("standard: %+v", std)
	}
	_, neu := postWirt(t, s, "?dryRun=true&nachricht=neu")
	if neu.Variante != sharedstore.WirtAlsKI || neu.Text != tx.text || neu.Model != "db-model" {
		t.Errorf("neu: %+v", neu)
	}
	if tx.model != "db-model" || len(tx.vorherige) != 1 {
		t.Errorf("Texter gefragt mit model=%q vorherige=%v", tx.model, tx.vorherige)
	}
	if snd.called || st.saved != 0 || st.heute.Text != kiText {
		t.Error("ein Dry-Run darf weder senden noch den Entwurf ändern")
	}

	// Vorschau des neuen Texts: an die eigene Nummer, markiert bin ich.
	_, vor := postWirt(t, s, "?preview=true&nachricht=neu")
	if snd.number != testPreview || !strings.Contains(snd.text, "Servus @4917012345678,") || vor.Text != tx.text {
		t.Errorf("Vorschau: Empfänger %q, Text:\n%s", snd.number, snd.text)
	}

	// send=true lässt sich nichts aussuchen: es geht der Entwurf raus.
	if _, out := postWirt(t, s, "?send=true&nachricht=standard"); out.Variante != sharedstore.WirtAlsKI {
		t.Errorf("send mit nachricht=standard: %+v", out)
	}
}

// Ein Testlauf zeigt den nächsten Versand: am Freitag den Text des kommenden
// Donnerstags – so sieht man einen schon übernommenen Text vorab. Der echte
// Versand schaut nur auf heute.
func TestWirtTestlaufZeigtDenNaechstenVersand(t *testing.T) {
	s, _, st, _ := kiServer()
	s.Now = func() time.Time { return friday }
	st.heute = entwurf()

	_, out := postWirt(t, s, "?dryRun=true")
	if st.gefragt != "2026-01-08" || out.Variante != sharedstore.WirtAlsKI {
		t.Errorf("Dry-Run am Freitag: gefragt %q, Outcome %+v", st.gefragt, out)
	}
	postWirt(t, s, "?send=true")
	if st.gefragt != "2026-01-02" {
		t.Errorf("send am Freitag: gefragt %q", st.gefragt)
	}

	// Donnerstag nach 16 Uhr: der heutige Versand ist durch, es zählt der nächste.
	s.Now = func() time.Time { return time.Date(2026, 1, 1, 18, 0, 0, 0, s.location) }
	postWirt(t, s, "?dryRun=true")
	if st.gefragt != "2026-01-08" {
		t.Errorf("Dry-Run Do 18 Uhr: gefragt %q", st.gefragt)
	}
}

// Ohne Wahl im Admin-UI schreibt das Modell aus der Konfiguration.
func TestWirtNeuNimmtDasModellAusDerKonfiguration(t *testing.T) {
	s, _, st, tx := kiServer()
	st.settings.Model = ""
	postWirt(t, s, "?dryRun=true&nachricht=neu")
	if tx.model != "cfg-model" {
		t.Errorf("model = %q", tx.model)
	}
}

func TestWirtNeuFehlerIst502(t *testing.T) {
	s, snd, _, tx := kiServer()
	tx.err = errors.New("Tonprüfung nicht bestanden")
	rec, _ := postWirt(t, s, "?preview=true&nachricht=neu")
	if rec.Code != http.StatusBadGateway || snd.called || !strings.Contains(rec.Body.String(), "Tonprüfung") {
		t.Fatalf("Status = %d, called = %v: %s", rec.Code, snd.called, rec.Body.String())
	}
}

func postGenerate(t *testing.T, s *Server) (*httptest.ResponseRecorder, WirtGenerateOutcome) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/wirt-reminder/generate", nil))
	var out WirtGenerateOutcome
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("Antwort nicht lesbar: %v", err)
		}
	}
	return rec, out
}

func TestWirtGenerateLegtDenEntwurfAb(t *testing.T) {
	s, snd, st, tx := kiServer()
	rec, out := postGenerate(t, s)
	if rec.Code != http.StatusOK || !out.Stored || out.Text != kiText || out.Model != "db-model" || out.Tag != "2026-01-01" {
		t.Fatalf("Status = %d, Outcome = %+v", rec.Code, out)
	}
	if st.heute == nil || st.heute.Text != kiText || tx.calls != 1 {
		t.Errorf("Entwurf = %+v, calls = %d", st.heute, tx.calls)
	}
	if snd.called {
		t.Error("Vorgenerieren darf nichts senden")
	}
}

// Kein neuer Text, wenn heute ohnehin nichts rausgeht oder es schon einen
// gibt – auch ein abgelehnter bleibt abgelehnt.
func TestWirtGenerateEntfaellt(t *testing.T) {
	jetzt := time.Now()
	cases := []struct {
		name   string
		setup  func(*Server, *fakeWirtStore)
		reason string
	}{
		{"kein Donnerstag", func(s *Server, _ *fakeWirtStore) { s.Now = func() time.Time { return friday } }, "kein Donnerstag"},
		{"Sperrtag", func(s *Server, _ *fakeWirtStore) { s.Sperrtage = fakeSperrtage{excluded: true} }, "Sperrtag"},
		{"Versand aus", func(_ *Server, st *fakeWirtStore) { st.settings.Versand = false }, "ausgeschaltet"},
		{"ohne WIRT_JID", func(s *Server, _ *fakeWirtStore) { s.WirtJID = "" }, "WIRT_JID"},
		{"schon übernommen", func(_ *Server, st *fakeWirtStore) { st.heute = entwurf() }, "schon einen KI-Text"},
		{"abgelehnt", func(_ *Server, st *fakeWirtStore) {
			st.heute = entwurf()
			st.heute.AbgelehntAm = &jetzt
		}, "schon einen KI-Text"},
		{"schon gesendet", func(_ *Server, st *fakeWirtStore) {
			st.heute = &sharedstore.WirtNachricht{GesendetAm: &jetzt}
		}, "schon gesendet"},
	}
	for _, c := range cases {
		s, _, st, tx := kiServer()
		c.setup(s, st)
		rec, out := postGenerate(t, s)
		if rec.Code != http.StatusOK || out.Stored || tx.calls != 0 || st.saved != 0 || !strings.Contains(out.Reason, c.reason) {
			t.Errorf("%s: Status = %d, calls = %d, Outcome = %+v", c.name, rec.Code, tx.calls, out)
		}
	}
}

// Scheitert das Modell, hält der Bot den Grund fest und meldet 502 – der
// CronJob versucht es dann noch einmal.
func TestWirtGenerateFehlerWirdFestgehalten(t *testing.T) {
	s, _, st, tx := kiServer()
	tx.err = errors.New("Modell antwortet nicht")
	rec, _ := postGenerate(t, s)
	if rec.Code != http.StatusBadGateway || st.fehler != "Modell antwortet nicht" || st.saved != 0 {
		t.Fatalf("Status = %d, fehler = %q", rec.Code, st.fehler)
	}
	// Ein fehlgeschlagener Versuch blockiert den nächsten nicht.
	st.heute = &sharedstore.WirtNachricht{Fehler: st.fehler}
	tx.err = nil
	if _, out := postGenerate(t, s); !out.Stored {
		t.Errorf("zweiter Versuch: %+v", out)
	}
}

func TestWirtGenerateOhneGeneratorIst503(t *testing.T) {
	s, _ := wirtServerAm(thursday)
	if rec, _ := postGenerate(t, s); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("Status = %d", rec.Code)
	}
}
