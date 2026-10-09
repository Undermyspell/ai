package report

import (
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"time"

	"github.com/michael/zumba-shared/cards"
)

// rotation.go: die Rotation aus CARD_STYLES. Gepflegt wird die Auswahl im
// Admin-UI als Warteschlange (Seite "Bild-Designs", Tabelle card_settings,
// siehe cards.ForQueue); CARD_STYLES ist nur der Startwert und der Fallback,
// solange keine Schlange gespeichert ist oder die DB nicht antwortet. Die
// "statistik" auf Zuruf zieht zufällig, der Wochenreport rechnet dann per
// Datum (cards.ForWeek).

// ParseCardStyles liest die Komma-Liste aus CARD_STYLES. Leereinträge fallen
// weg, unbekannte IDs sind ein Fehler (Tippfehler soll beim Start auffallen,
// nicht erst donnerstags um 21:00). Leere Liste = nur das Live-Design.
func ParseCardStyles(list string) ([]string, error) {
	var styles []string
	for _, raw := range strings.Split(list, ",") {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if !cards.Known(id) {
			return nil, fmt.Errorf("unbekanntes Bild-Design %q", id)
		}
		if !slices.Contains(styles, id) {
			styles = append(styles, id)
		}
	}
	return styles, nil
}

// CardRotation wählt das Design einer Karte. Der Nullwert (leere Liste)
// liefert immer das Live-Design.
type CardRotation struct {
	styles []string
}

// NewCardRotation baut die Rotation über der Liste; unbekannte IDs (etwa
// aus der DB nach dem Entfernen eines Designs) fallen dabei weg.
func NewCardRotation(styles []string) CardRotation {
	return CardRotation{styles: cards.Clean(styles)}
}

// Styles ist die Auswahl (leer = nur Live-Design).
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

// ForWeek liefert das Design des Wochenreports zum Stichtag t (Durchlauf der
// Reihe nach, siehe cards.ForWeek).
func (r CardRotation) ForWeek(t time.Time) string {
	return cards.ForWeek(r.styles, t)
}
