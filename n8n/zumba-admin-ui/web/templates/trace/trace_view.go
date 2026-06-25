package trace

import (
	"fmt"
	"strconv"
	"time"

	"github.com/michael/zumba-admin-ui/internal/store"
)

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

var weekdayDE = map[time.Weekday]string{
	time.Monday: "Mo", time.Tuesday: "Di", time.Wednesday: "Mi",
	time.Thursday: "Do", time.Friday: "Fr", time.Saturday: "Sa", time.Sunday: "So",
}

// FmtTime → "Do, 25.06. 20:12"
func FmtTime(t time.Time) string {
	return fmt.Sprintf("%s, %s", weekdayDE[t.Weekday()], t.Format("02.01. 15:04"))
}

// PathIcon / PathLabel beschreiben den genommenen Hauptpfad.
func PathIcon(p string) string {
	switch p {
	case "statistik":
		return "📊"
	case "classify":
		return "🤖"
	default:
		return "🚫"
	}
}

func PathLabel(p string) string {
	switch p {
	case "statistik":
		return "Statistik"
	case "classify":
		return "Klassifizierung"
	case "ignored":
		return "Ignoriert"
	default:
		return p
	}
}

// ClassLabel übersetzt das Classifier-Ergebnis.
func ClassLabel(c string) string {
	switch c {
	case "true":
		return "Zusage"
	case "false":
		return "Absage"
	case "invalid":
		return "invalid"
	default:
		return ""
	}
}

// ActionLabel übersetzt die ausgeführte DB-Aktion.
func ActionLabel(a string) string {
	switch a {
	case "marked_absent":
		return "Absage eingetragen"
	case "marked_present":
		return "Absage entfernt"
	case "would_mark_absent":
		return "würde Absage eintragen"
	case "would_mark_present":
		return "würde Absage entfernen"
	case "", "none":
		return "—"
	default:
		return a
	}
}

// Truncate kürzt lange Nachrichten für die Listenansicht.
func Truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// Summary fasst eine Liste für die Übersicht zusammen.
func Summary(t store.Trace) string { return PathLabel(t.Path) }
