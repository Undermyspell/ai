// Package wirttext lässt ein Sprachmodell den Text der Tisch-Erinnerung an
// den Wirt schreiben – jede Woche anders, aber immer höflich.
//
// Das Modell liefert nur den Mittelteil der Nachricht. Kopf, Markierung des
// Wirts und Fußzeile setzt der Bot (shared/wirt.Compose), sie können also
// nicht fehlen. Jeder Text muss zwei Prüfungen bestehen, bevor er als Entwurf
// gilt: die feste Prüfung (wirt.Validate: genau eine Markierung, es geht um
// den Tisch, kurz, keine Links) und eine Tonprüfung durch einen zweiten
// Modell-Aufruf. Fällt er durch, gibt es einen zweiten Versuch; danach einen
// Fehler – dann geht am Nachmittag die Standardnachricht raus.
package wirttext

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/michael/zumba-shared/wirt"
)

//go:embed prompt.txt
var schreibPrompt string

//go:embed pruef-prompt.txt
var pruefPrompt string

// LLM ruft ein Modell mit Systemprompt und Nachricht auf (classifier.Gemini).
type LLM interface {
	Generate(ctx context.Context, model, system, message string, temperature float64) (string, error)
}

// versuche: so oft darf das Modell schreiben, bis ein Text beide Prüfungen
// besteht. Jeder Versuch kostet zwei Aufrufe (schreiben + prüfen).
const versuche = 2

// aufhaenger geben dem Modell jede Woche einen anderen Einstieg – ohne sie
// landen Sprachmodelle bei gleichem Prompt immer wieder beim selben Satz.
var aufhaenger = []string{
	"die Vorfreude auf den Feierabend",
	"die Jahreszeit",
	"der Donnerstag als bester Tag der Woche",
	"unser Stammplatz, an dem wir uns wie daheim fühlen",
	"die lange Tradition unserer Runde",
	"der Hunger nach einer guten Brotzeit",
	"die Woche, die sich schon nach Wochenende anfühlt",
	"ein kleiner Reim",
	"die Pünktlichkeit, mit der diese Nachricht jeden Donnerstag kommt",
	"ganz schlicht und herzlich, ohne Schnörkel",
	"die gute Stube, in der es am gemütlichsten ist",
	"die Geschichten, die es heute Abend wieder zu erzählen gibt",
}

var wochentage = [...]string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"}
var monate = [...]string{"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"}

// Generator schreibt den Text der Tisch-Erinnerung.
type Generator struct {
	llm LLM
	// pick wählt den Aufhänger (0 ≤ n); in Tests fest.
	pick func(n int) int
}

func New(llm LLM) *Generator {
	return &Generator{llm: llm, pick: rand.IntN}
}

// Text lässt model den Text für den Tag day schreiben. vorherige sind die
// zuletzt verwendeten Texte – das Modell soll sich nicht wiederholen. Das
// Ergebnis enthält den Platzhalter wirt.Platzhalter und hat beide Prüfungen
// bestanden.
func (g *Generator) Text(ctx context.Context, model string, day time.Time, vorherige []string) (string, error) {
	var gruende []string
	for i := 0; i < versuche; i++ {
		raw, err := g.llm.Generate(ctx, model, schreibPrompt, g.auftrag(day, vorherige), 1)
		if err != nil {
			// Antwortet das Modell nicht, hilft ein zweiter Versuch selten –
			// und jeder kostet Kontingent.
			return "", fmt.Errorf("Modell %s: %w", model, err)
		}
		text := Clean(raw)
		if err := wirt.Validate(text); err != nil {
			// Verworfene Texte stehen im Log – nur so lässt sich sehen, ob
			// der Prompt oder die Prüfung nachgeschärft werden muss.
			log.Printf("ℹ️  Wirt-Text (%s) verworfen – %v: %q", model, err, text)
			gruende = append(gruende, err.Error())
			continue
		}
		urteil, err := g.llm.Generate(ctx, model, pruefPrompt, text, 0)
		if err != nil {
			return "", fmt.Errorf("Tonprüfung mit %s: %w", model, err)
		}
		if !bestanden(urteil) {
			log.Printf("ℹ️  Wirt-Text (%s) verworfen – Tonprüfung antwortet %q auf: %q", model, gekuerzt(urteil, 200), text)
			gruende = append(gruende, "Tonprüfung nicht bestanden")
			continue
		}
		return text, nil
	}
	return "", errors.New("kein brauchbarer Text nach " + fmt.Sprint(versuche) + " Versuchen: " + strings.Join(gruende, "; "))
}

// auftrag ist die Nachricht an das Modell: Datum, Aufhänger der Woche und was
// es zuletzt geschrieben hat.
func (g *Generator) auftrag(day time.Time, vorherige []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Heute ist %s, der %d. %s %d.\n", wochentage[day.Weekday()], day.Day(), monate[day.Month()-1], day.Year())
	fmt.Fprintf(&b, "Aufhänger für diese Woche (locker einbauen, nicht erzwingen): %s.\n", aufhaenger[g.pick(len(aufhaenger))])
	if len(vorherige) > 0 {
		b.WriteString("\nDie letzten Nachrichten – schreib etwas spürbar anderes, mit anderem Einstieg und anderen Formulierungen:\n")
		for i, v := range vorherige {
			fmt.Fprintf(&b, "\n%d.\n%s\n", i+1, v)
		}
	}
	b.WriteString("\nSchreib jetzt die Nachricht für heute.")
	return b.String()
}

var (
	platzhalterRe = regexp.MustCompile(`(?i)@?\{\s*wirt\s*\}`)
	leerzeilenRe  = regexp.MustCompile(`\n{3,}`)
)

// Clean räumt die Antwort des Modells auf: Codeblock-Zäune und umschließende
// Anführungszeichen weg, Schreibvarianten des Platzhalters ("{Wirt}",
// "@{wirt}") vereinheitlicht, mehrfache Leerzeilen auf eine gekürzt.
func Clean(raw string) string {
	s := strings.ReplaceAll(raw, "\r\n", "\n")
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// Zaun samt Sprachangabe ("```text") – die ganze erste Zeile.
		_, s, _ = strings.Cut(s, "\n")
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}
	s = strings.TrimSpace(s)
	for _, q := range [][2]string{{`"`, `"`}, {"„", "“"}, {"„", "\""}, {"“", "”"}} {
		if strings.HasPrefix(s, q[0]) && strings.HasSuffix(s, q[1]) && len(s) > len(q[0])+len(q[1]) {
			s = strings.TrimSpace(s[len(q[0]) : len(s)-len(q[1])])
			break
		}
	}
	s = platzhalterRe.ReplaceAllString(s, wirt.Platzhalter)
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return leerzeilenRe.ReplaceAllString(strings.Join(lines, "\n"), "\n\n")
}

// bestanden: nur ein eindeutiges "ok" zählt – alles andere ist ein Nein.
// Satzzeichen und Markdown um das Wort herum ("**OK**", "Ok.") stören nicht,
// jedes weitere Wort ("ok, aber …") schon.
func bestanden(urteil string) bool {
	var b strings.Builder
	for _, r := range strings.ToLower(urteil) {
		if unicode.IsLetter(r) {
			b.WriteRune(r)
		}
	}
	return b.String() == "ok"
}

// gekuerzt bringt eine Modell-Antwort fürs Log auf eine Zeile mit höchstens
// max Zeichen.
func gekuerzt(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}
