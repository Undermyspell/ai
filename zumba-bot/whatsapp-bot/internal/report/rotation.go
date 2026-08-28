package report

import (
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"time"
)

// rotation.go bestimmt, welches Bild-Design eine Karte bekommt. Die Auswahl
// kommt aus der Env CARD_STYLES; die "statistik" auf Zuruf zieht daraus
// zufällig, der Wochenreport arbeitet die Liste in Durchläufen ab.

// ParseCardStyles liest die Komma-Liste aus CARD_STYLES. Leereinträge fallen
// weg, unbekannte IDs sind ein Fehler (Tippfehler soll beim Start auffallen,
// nicht erst donnerstags um 21:00). Leere Liste = nur das Live-Design.
func ParseCardStyles(list string) ([]string, error) {
	bekannt := make(map[string]bool, len(CardStyles()))
	for _, s := range CardStyles() {
		bekannt[s.ID] = true
	}

	var styles []string
	for _, raw := range strings.Split(list, ",") {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if !bekannt[id] {
			return nil, fmt.Errorf("unbekanntes Bild-Design %q", id)
		}
		if !slices.Contains(styles, id) {
			styles = append(styles, id)
		}
	}
	return styles, nil
}

// CardRotation wählt das Design einer Karte. Der Nullwert (leere Liste)
// liefert immer das Live-Design – damit bleibt der Bot ohne CARD_STYLES bei
// seinem bisherigen Verhalten.
type CardRotation struct {
	styles []string
}

// NewCardRotation baut die Rotation über der geprüften Liste (ParseCardStyles).
func NewCardRotation(styles []string) CardRotation {
	return CardRotation{styles: slices.Clone(styles)}
}

// Styles ist die konfigurierte Auswahl (leer = nur Live-Design).
func (r CardRotation) Styles() []string { return r.styles }

// Random zieht gleichverteilt. Für die "statistik" auf Zuruf, die mehrmals am
// Tag kommen kann – da wäre ein fester Durchlauf nur berechenbar.
func (r CardRotation) Random() string {
	switch len(r.styles) {
	case 0:
		return DefaultCardStyle
	case 1:
		return r.styles[0]
	}
	return r.styles[rand.Intn(len(r.styles))]
}

// ForWeek liefert das Design des Wochenreports zum Stichtag t: die Liste wird
// der Reihe nach durchlaufen, Position = Wochenindex modulo Länge. Damit ist
// ein Design erst wieder dran, wenn alle anderen einmal dran waren — auch über
// die Durchlauf-Grenze hinweg, was ein je Durchlauf neu gewürfelter Zufall
// nicht garantieren kann (dort kann dasselbe Design als Letztes und gleich
// wieder als Zweites kommen).
//
// Die Reihenfolge bestimmt CARD_STYLES. Es braucht keinen gespeicherten
// Zustand: Neustart, Wiederholungslauf oder Dry-Run im Admin-UI liefern
// dasselbe Design wie der echte Versand.
func (r CardRotation) ForWeek(t time.Time) string {
	n := len(r.styles)
	if n == 0 {
		return DefaultCardStyle
	}
	woche := weekIndex(t)
	return r.styles[woche-floorDiv(woche, n)*n] // Modulo, auch für Daten vor 1970
}

// weekIndex zählt Wochen seit der Unix-Epoche. Tag 0 (01.01.1970) war ein
// Donnerstag — die Wochen wechseln also donnerstags, genau im Takt des
// Reports. Ein Nachhol-Lauf am Freitag bleibt damit beim Design des Vortags.
func weekIndex(t time.Time) int {
	y, m, d := t.Date()
	tage := int(time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400)
	return floorDiv(tage, 7)
}

// floorDiv rundet immer ab (Go rundet bei negativen Zahlen Richtung null).
func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}
