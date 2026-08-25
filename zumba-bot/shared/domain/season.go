package domain

import (
	"errors"
	"fmt"
	"time"
)

// ErrNoSeason meldet, dass zu einem Zeitpunkt kein Stammtischjahr gepflegt
// ist. Bewusst ein harter Fehler statt eines stillen Fallbacks: sonst laufen
// Bot und Admin-UI nach dem Ende des letzten gepflegten Jahres unbemerkt auf
// leere Auswertungen.
var ErrNoSeason = errors.New("kein Stammtischjahr für diesen Zeitpunkt gepflegt")

// Season ist ein Stammtischjahr: ein benannter Auswertungszeitraum mit
// gepflegtem Start- und Enddatum (beide inklusive). Jahre überlappen nicht,
// die Grenzen sind keine Kalenderjahresgrenzen (2026 = 01.12.2025–30.11.2026).
// Alle Auswertungen laufen gegen genau eine Season; Fehltage-Serien enden an
// der Jahresgrenze.
type Season struct {
	ID    int64
	Label string // "2026" – zugleich der Pfad-Slug in Admin-UI und Wrapped
	Period
}

func (s Season) String() string {
	return fmt.Sprintf("%s (%s–%s)", s.Label,
		s.Start.Format("02.01.2006"), s.End.Format("02.01.2006"))
}

// Contains prüft auf Tagesbasis, ob t in das Jahr fällt (Grenzen inklusive).
func (s Season) Contains(t time.Time) bool {
	d := DateOnly(t)
	return !d.Before(DateOnly(s.Start)) && !d.After(DateOnly(s.End))
}

// ClampAsOf begrenzt einen Stichtag auf das Jahr: nie vor dem Start, nie nach
// dem Ende. Damit liefert ein abgelaufenes Jahr im Archiv denselben Stand wie
// am letzten Tag seiner Laufzeit, statt bis heute weiterzurechnen.
func (s Season) ClampAsOf(asOf time.Time) time.Time {
	d := DateOnly(asOf)
	if d.Before(DateOnly(s.Start)) {
		return DateOnly(s.Start)
	}
	if d.After(DateOnly(s.End)) {
		return DateOnly(s.End)
	}
	return d
}

// ClampStart liefert den effektiven Startpunkt eines Users in diesem Jahr:
// GREATEST(startDate, Jahresstart). Wer vor dem Jahr eingetreten ist (oder gar
// kein startDate hat), beginnt mit dem Jahr bei null.
func (s Season) ClampStart(startDate *time.Time) time.Time {
	start := DateOnly(s.Start)
	if startDate == nil {
		return start
	}
	if d := DateOnly(*startDate); d.After(start) {
		return d
	}
	return start
}

// DateOnly schneidet die Uhrzeit ab (UTC-Tagesbasis) – die Domäne rechnet
// ausschließlich in Tagen.
func DateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
