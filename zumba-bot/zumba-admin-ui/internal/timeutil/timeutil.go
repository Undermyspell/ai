package timeutil

import (
	"fmt"
	"time"

	"github.com/michael/zumba-shared/domain"
)

const dateLayout = "2006-01-02"

func IsThursday(t time.Time) bool {
	return t.Weekday() == time.Thursday
}

// Period is the active Stammtisch evaluation window – geteilter Typ aus dem
// shared-Modul (EffectiveEnd kappt das Ende am heutigen Tag).
type Period = domain.Period

func StartOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// FormatISO returns YYYY-MM-DD.
func FormatISO(t time.Time) string {
	return t.Format(dateLayout)
}

// ParseISO parses YYYY-MM-DD.
func ParseISO(s string) (time.Time, error) {
	return time.Parse(dateLayout, s)
}

// FormatDE returns "Do., 7. Mai 2026".
func FormatDE(t time.Time) string {
	weekdays := []string{"So.", "Mo.", "Di.", "Mi.", "Do.", "Fr.", "Sa."}
	months := []string{
		"Januar", "Februar", "März", "April", "Mai", "Juni",
		"Juli", "August", "September", "Oktober", "November", "Dezember",
	}
	return fmt.Sprintf("%s, %d. %s %d",
		weekdays[t.Weekday()], t.Day(), months[t.Month()-1], t.Year())
}

// FormatDEShort returns "7. Mai".
func FormatDEShort(t time.Time) string {
	months := []string{
		"Jan", "Feb", "Mär", "Apr", "Mai", "Jun",
		"Jul", "Aug", "Sep", "Okt", "Nov", "Dez",
	}
	return fmt.Sprintf("%d. %s", t.Day(), months[t.Month()-1])
}
