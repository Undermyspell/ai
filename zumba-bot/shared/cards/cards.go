// Package cards ist der gemeinsame Katalog der Bild-Designs der
// Statistik-Karte und die Regel, welches Design ein Wochenreport bekommt.
// Der whatsapp-bot rendert die Designs (Templates in internal/report), das
// Admin-UI stellt sie ein – beide brauchen dieselben IDs und dieselbe
// Rotationsrechnung, sonst zeigt die Vorschau etwas anderes als der Versand.
package cards

import (
	"slices"
	"time"
)

// Style ist ein Bild-Design: ID wie in CARD_STYLES, Label für die Auswahl.
type Style struct {
	ID    string
	Label string
}

// Default ist das Live-Design – es kommt, wenn keine Rotation gepflegt ist.
const Default = "wrapped"

// catalog in Anzeige-Reihenfolge. Ein neues Design braucht hier einen
// Eintrag und im Bot ein Template (ein Test im Bot prüft beides).
var catalog = []Style{
	{"wrapped", "Wrapped (live)"},
	{"bierdeckel", "Bierdeckel hell"},
	{"bierdeckel-dunkel", "Bierdeckel dunkel"},
	{"tafel", "Kreidetafel"},
	{"masskrug", "Maßkrug"},
	{"zeitung", "Zeitung"},
	{"arena", "Arena"},
	{"stempelkarte", "Treuekarte"},
	{"sammelkarten", "Sammelalbum"},
	{"formular", "Amtsformular"},
	{"abfahrtstafel", "Abfahrtstafel"},
	{"kassenbon", "Kassenbon"},
	{"gipfelbuch", "Gipfelbuch"},
	{"wetterbericht", "Wetterbericht"},
	{"hochrechnung", "Hochrechnung"},
}

// Catalog liefert alle Bild-Designs (Kopie).
func Catalog() []Style { return slices.Clone(catalog) }

// Known meldet, ob id ein Bild-Design ist.
func Known(id string) bool {
	return slices.ContainsFunc(catalog, func(s Style) bool { return s.ID == id })
}

// Label liefert die Anzeige eines Designs (unbekannt → die ID selbst).
func Label(id string) string {
	for _, s := range catalog {
		if s.ID == id {
			return s.Label
		}
	}
	return id
}

// Clean wirft unbekannte IDs und Duplikate aus einer Rotation – etwa ein
// Design, das es nach einem Update nicht mehr gibt. Die Reihenfolge bleibt.
func Clean(ids []string) []string {
	var out []string
	for _, id := range ids {
		if Known(id) && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// ForWeek liefert das Design des Wochenreports zum Stichtag t, solange nur
// CARD_STYLES gilt (keine Warteschlange gespeichert): die Rotation wird der
// Reihe nach durchlaufen, Position = Wochenindex modulo Länge. Ein
// Design ist so erst wieder dran, wenn alle anderen einmal dran waren — auch
// über die Durchlauf-Grenze hinweg. Es braucht keinen gespeicherten Zustand:
// Neustart, Wiederholungslauf oder Dry-Run liefern dasselbe Design wie der
// echte Versand. Leere Rotation → Live-Design.
func ForWeek(rotation []string, t time.Time) string {
	n := len(rotation)
	if n == 0 {
		return Default
	}
	woche := weekIndex(t)
	return rotation[woche-floorDiv(woche, n)*n] // Modulo, auch für Daten vor 1970
}

// Next ist die Einmal-Auswahl für einen bestimmten Wochenreport.
type Next struct {
	Tag   time.Time // der Donnerstag, für den sie gilt
	Style string
}

// Last ist der zuletzt an die Gruppe gesendete Wochenreport.
type Last struct {
	Tag   time.Time
	Style string
}

// Woher das Design eines Wochenreports kommt.
const (
	QuelleSchlange   = "schlange"   // oberstes Design der Warteschlange
	QuelleFest       = "fest"       // Einmal-Auswahl für genau diesen Donnerstag
	QuelleWiederholt = "wiederholt" // an diesem Tag schon gesendet – dasselbe nochmal
)

// ForQueue ist das Design des Wochenreports am Tag t, wenn die Rotation als
// Warteschlange gepflegt ist: das oberste Design kommt als nächstes. Nach dem
// Versand rückt der Bot die Schlange weiter (gesendetes Design nach hinten,
// siehe store.AdvanceCardQueue) – so stimmt "oberste kommt als nächstes" jede
// Woche. Eine Einmal-Auswahl ersetzt den Donnerstag, ohne die Schlange zu
// bewegen; ein Wiederholungslauf am selben Tag liefert das schon gesendete
// Design.
func ForQueue(queue []string, next *Next, last *Last, t time.Time) (style, quelle string) {
	if last != nil && SameDay(last.Tag, t) && Known(last.Style) {
		return last.Style, QuelleWiederholt
	}
	if next != nil && SameDay(next.Tag, t) && Known(next.Style) {
		return next.Style, QuelleFest
	}
	if q := Clean(queue); len(q) > 0 {
		return q[0], QuelleSchlange
	}
	return Default, QuelleSchlange
}

// Woche ist eine Zeile der Vorschau.
type Woche struct {
	Tag   time.Time
	Style string
	Fest  bool
}

// Upcoming sind die Designs der nächsten n Wochenreports ab erster: die
// Schlange der Reihe nach, eine Einmal-Auswahl schiebt sie um eine Woche.
func Upcoming(queue []string, next *Next, erster time.Time, n int) []Woche {
	q := Clean(queue)
	out := make([]Woche, 0, n)
	pos := 0
	for w := 0; w < n; w++ {
		tag := erster.AddDate(0, 0, 7*w)
		if next != nil && SameDay(next.Tag, tag) && Known(next.Style) {
			out = append(out, Woche{Tag: tag, Style: next.Style, Fest: true})
			continue
		}
		style := Default
		if len(q) > 0 {
			style = q[pos%len(q)]
		}
		pos++
		out = append(out, Woche{Tag: tag, Style: style})
	}
	return out
}

// ReportHour ist die Stunde des Wochenreport-CronJobs (Do 21:00).
const ReportHour = 21

// NextReport ist der Donnerstag des nächsten Wochenreports ab now (in der
// Zeitzone von now): heute, wenn Donnerstag ist und es noch vor 21 Uhr ist,
// sonst der folgende Donnerstag. Ergebnis: Mitternacht dieses Tages.
func NextReport(now time.Time) time.Time {
	tag := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	bis := (int(time.Thursday) - int(now.Weekday()) + 7) % 7
	if bis == 0 && now.Hour() >= ReportHour {
		bis = 7
	}
	return tag.AddDate(0, 0, bis)
}

// SameDay vergleicht nur das Kalenderdatum (jeweils in der eigenen Zone).
func SameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
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
