package wirttext

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/michael/zumba-shared/wirt"
)

const guterText = "Servus {wirt} 👋\nmagst du uns heute Abend bitte unseren Tisch freihalten? 🙏\nDanke dir!"

// fakeLLM antwortet der Reihe nach: Schreib-Aufrufe aus texte, Prüf-Aufrufe
// aus urteile. calls hält "schreiben"/"pruefen" in Reihenfolge fest.
type fakeLLM struct {
	texte   []string
	urteile []string
	err     error
	calls   []string
	auftrag string // letzte Nachricht eines Schreib-Aufrufs
	temps   []float64
}

func (f *fakeLLM) Generate(_ context.Context, _, system, message string, temperature float64) (string, error) {
	f.temps = append(f.temps, temperature)
	if f.err != nil {
		return "", f.err
	}
	if system == pruefPrompt {
		f.calls = append(f.calls, "pruefen")
		u := f.urteile[0]
		f.urteile = f.urteile[1:]
		return u, nil
	}
	f.calls = append(f.calls, "schreiben")
	f.auftrag = message
	t := f.texte[0]
	f.texte = f.texte[1:]
	return t, nil
}

var donnerstag = time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)

func generator(llm LLM) *Generator {
	g := New(llm)
	g.pick = func(int) int { return 0 }
	return g
}

func TestTextBestehtBeidePruefungen(t *testing.T) {
	llm := &fakeLLM{texte: []string{"```\n" + guterText + "\n```"}, urteile: []string{"OK."}}
	text, err := generator(llm).Text(context.Background(), "m", donnerstag, []string{"Alter Text {wirt}"})
	if err != nil {
		t.Fatal(err)
	}
	if text != guterText {
		t.Errorf("Text = %q", text)
	}
	if got := strings.Join(llm.calls, ","); got != "schreiben,pruefen" {
		t.Errorf("Aufrufe = %s", got)
	}
	// Schreiben mit Temperatur (Abwechslung), prüfen ohne.
	if llm.temps[0] == 0 || llm.temps[1] != 0 {
		t.Errorf("Temperaturen = %v", llm.temps)
	}
	for _, want := range []string{"Donnerstag, der 19. November 2026", aufhaenger[0], "Alter Text {wirt}"} {
		if !strings.Contains(llm.auftrag, want) {
			t.Errorf("Auftrag enthält %q nicht:\n%s", want, llm.auftrag)
		}
	}
}

// Ein Text ohne Markierung kommt gar nicht erst zur Tonprüfung; der zweite
// Versuch darf es richten.
func TestTextZweiterVersuchNachUngueltigemText(t *testing.T) {
	llm := &fakeLLM{
		texte:   []string{"Servus! Halt uns heute Abend bitte unseren Tisch frei. Danke!", guterText},
		urteile: []string{"ok"},
	}
	text, err := generator(llm).Text(context.Background(), "m", donnerstag, nil)
	if err != nil || text != guterText {
		t.Fatalf("Text = %q, err = %v", text, err)
	}
	if got := strings.Join(llm.calls, ","); got != "schreiben,schreiben,pruefen" {
		t.Errorf("Aufrufe = %s", got)
	}
}

// Sagt die Tonprüfung zweimal nein, gibt es keinen Text – dann geht die
// Standardnachricht raus.
func TestTextTonpruefungLehntAb(t *testing.T) {
	llm := &fakeLLM{texte: []string{guterText, guterText}, urteile: []string{"nein", "ok, aber frech"}}
	text, err := generator(llm).Text(context.Background(), "m", donnerstag, nil)
	if err == nil || text != "" {
		t.Fatalf("Text = %q, err = %v – erwartet ein Fehler", text, err)
	}
	if !strings.Contains(err.Error(), "Tonprüfung") {
		t.Errorf("Fehler nennt den Grund nicht: %v", err)
	}
}

func TestTextModellFehler(t *testing.T) {
	llm := &fakeLLM{err: errors.New("503")}
	if _, err := generator(llm).Text(context.Background(), "gemma", donnerstag, nil); err == nil || !strings.Contains(err.Error(), "gemma") {
		t.Fatalf("err = %v", err)
	}
}

func TestClean(t *testing.T) {
	cases := map[string]string{
		"  \"Servus {wirt}\"  ":              "Servus {wirt}",
		"„Servus {wirt}“":                    "Servus {wirt}",
		"Servus @{Wirt} 👋":                   "Servus {wirt} 👋",
		"Servus { wirt }":                    "Servus {wirt}",
		"Zeile 1  \r\n\r\n\r\n\r\nZeile 2":   "Zeile 1\n\nZeile 2",
		"```\nServus {wirt}\n```":            "Servus {wirt}",
		"Er sagte \"Servus\" zu {wirt} gern": "Er sagte \"Servus\" zu {wirt} gern",
	}
	for raw, want := range cases {
		if got := Clean(raw); got != want {
			t.Errorf("Clean(%q) = %q, erwartet %q", raw, got, want)
		}
	}
}

func TestBestanden(t *testing.T) {
	for urteil, want := range map[string]bool{"ok": true, " OK.\n": true, "`ok`": true, "nein": false, "ok, aber": false, "": false} {
		if bestanden(urteil) != want {
			t.Errorf("bestanden(%q) != %v", urteil, want)
		}
	}
}

// Die Beispiele im Prompt sind Vorbilder – sie müssen selbst gültig sein.
func TestPromptBeispieleSindGueltig(t *testing.T) {
	_, beispiele, ok := strings.Cut(schreibPrompt, "(nicht abschreiben):")
	if !ok {
		t.Fatal("Beispiel-Abschnitt nicht gefunden")
	}
	n := 0
	for _, b := range strings.Split(strings.TrimSpace(beispiele), "\n\n") {
		if err := wirt.Validate(b); err != nil {
			t.Errorf("Beispiel fällt durch (%v):\n%s", err, b)
		}
		n++
	}
	if n < 2 {
		t.Errorf("nur %d Beispiele gefunden", n)
	}
}
