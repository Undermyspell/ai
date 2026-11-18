// Package wirt bündelt, was Bot und Admin-UI über die Tisch-Erinnerung an den
// Wirt gemeinsam wissen müssen: den festen Rahmen der Nachricht (Kopf,
// Fußzeile), die Standardnachricht, den Platzhalter für die Markierung, die
// Prüfung eines KI-Texts und den nächsten Versandtermin.
//
// Die Nachricht besteht immer aus drei Teilen: Kopf, Text, Fußzeile. Nur der
// Text in der Mitte kommt von einem Sprachmodell – Markierung und Fußzeile
// setzt der Code, sie können also nie fehlen.
package wirt

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Platzhalter steht im gespeicherten Text an der Stelle, an der der Wirt
// markiert wird. Die echte Nummer kennt nur der Bot (WIRT_JID) – sie steht
// weder in der DB noch im Admin-UI.
const Platzhalter = "{wirt}"

const (
	// Kopf und Fuss rahmen jede Erinnerung, egal woher der Text kommt.
	Kopf = "🍻 *Stammtisch heute Abend*"
	// Fuss ist die Fußzeile wie in der Statistik-Nachricht des Bots.
	Fuss = "🤖🍺 *Automatische Erinnerung vom Zumba-Bot*"

	// StandardText ist der feste Text: er geht raus, wenn es keinen KI-Text
	// gibt, er abgelehnt wurde oder die Prüfung nicht besteht.
	StandardText = "Servus " + Platzhalter + " 👋\n\n" +
		"bitte halt uns heute Abend unseren Tisch frei 🙏\n\n" +
		"Danke dir – bis später! 😉"
)

// Stunden der beiden CronJobs (Europe/Berlin): vormittags wird der Text
// vorgeneriert, nachmittags geht die Erinnerung raus.
const (
	GenerateHour = 10
	SendHour     = 16
)

// Grenzen für einen KI-Text.
const (
	minZeichen = 30
	maxZeichen = 400
	maxZeilen  = 8
)

// Compose setzt Kopf, Text und Fußzeile zusammen und ersetzt den Platzhalter
// durch die Markierung "@<nummer>" – WhatsApp zeigt daraus den Kontaktnamen,
// wenn dieselbe Nummer in der mentioned-Liste des Versands steht. Ohne
// Nummer bleibt die Anrede neutral.
func Compose(text, mention string) string {
	if mention != "" {
		text = strings.ReplaceAll(text, Platzhalter, "@"+mention)
	} else {
		text = strings.ReplaceAll(text, " "+Platzhalter, "")
		text = strings.ReplaceAll(text, Platzhalter, "")
	}
	return Kopf + "\n\n" + strings.TrimSpace(text) + "\n\n" + Fuss
}

// Standard ist die feste Erinnerung.
func Standard(mention string) string { return Compose(StandardText, mention) }

// Validate prüft einen KI-Text auf alles, was sich ohne Sprachgefühl prüfen
// lässt: der Wirt wird genau einmal markiert, es geht um den Tisch, der Text
// ist kurz und enthält weder weitere Markierungen noch Links. Den Ton prüft
// der Bot zusätzlich mit einem zweiten Modell-Aufruf.
func Validate(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("Text ist leer")
	}
	if n := strings.Count(text, Platzhalter); n != 1 {
		return fmt.Errorf("Wirt muss genau einmal markiert sein (%s kommt %d-mal vor)", Platzhalter, n)
	}
	rest := strings.Replace(text, Platzhalter, "", 1)
	if strings.ContainsAny(rest, "@{}") {
		return errors.New("Text enthält weitere Markierungen oder Platzhalter")
	}
	if n := utf8.RuneCountInString(text); n < minZeichen || n > maxZeichen {
		return fmt.Errorf("Text hat %d Zeichen (erlaubt %d–%d)", n, minZeichen, maxZeichen)
	}
	zeilen := 0
	for _, z := range strings.Split(text, "\n") {
		if strings.TrimSpace(z) != "" {
			zeilen++
		}
	}
	if zeilen > maxZeilen {
		return fmt.Errorf("Text hat %d Zeilen (erlaubt %d)", zeilen, maxZeilen)
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "tisch") {
		return errors.New("die Bitte um den Tisch fehlt")
	}
	for _, link := range []string{"http://", "https://", "www."} {
		if strings.Contains(lower, link) {
			return errors.New("Text enthält einen Link")
		}
	}
	return nil
}

// NextVersand ist der Donnerstag der nächsten Erinnerung ab now (in der
// Zeitzone von now): heute, wenn Donnerstag ist und es noch vor 16 Uhr ist,
// sonst der folgende Donnerstag. Ergebnis: Mitternacht dieses Tages.
func NextVersand(now time.Time) time.Time {
	tag := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	bis := (int(time.Thursday) - int(now.Weekday()) + 7) % 7
	if bis == 0 && now.Hour() >= SendHour {
		bis = 7
	}
	return tag.AddDate(0, 0, bis)
}
