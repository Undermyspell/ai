// Package report portiert den n8n-Code-Node "Generate Whatsapp Message":
// aus der Per-User-Statistik wird der WhatsApp-Ranglisten-Text gebaut.
package report

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/michael/zumba-whatsapp-bot/internal/penalty"
	"github.com/michael/zumba-whatsapp-bot/internal/store"
)

// showStartDate bildet eine Eigenheit des Original-Workflows ab: der JS-Code
// las `item.json.start_date`, die Stats-Query liefert die Spalte aber als
// "startDate" (camelCase). Dadurch war das Feld immer undefined und das
// Startdatum `(d.m.)` wurde nie gerendert. Für 1:1-Parität bleibt es aus;
// auf true setzen, um die ursprünglich beabsichtigte Anzeige zu aktivieren.
const showStartDate = false

// WeeklyNote ist der Hinweis, der dem automatischen Donnerstag-Wochenreport
// vorangestellt wird. Ansonsten identisch zum on-demand "statistik"-Text.
const WeeklyNote = "📅 *Automatischer Wochenreport (Do 21:00)*\n\n"

// BuildWeekly entspricht Build, stellt aber den Wochenreport-Hinweis voran.
func BuildWeekly(rows []store.Stat) string {
	return WeeklyNote + Build(rows)
}

// BuildWeeklyWithStrafen ist BuildWeekly inkl. Strafenblock.
func BuildWeeklyWithStrafen(rows []store.Stat, strafenBlock string) string {
	return WeeklyNote + BuildWithStrafen(rows, strafenBlock)
}

// Build erzeugt den WhatsApp-Text ohne Strafenblock (Styles/Tests).
func Build(rows []store.Stat) string {
	return BuildWithStrafen(rows, "")
}

// BuildWithStrafen erzeugt den WhatsApp-Text; ein nicht-leerer strafenBlock
// (siehe StrafenBlock) wird zwischen Rangliste und Abschlusszeile eingefügt.
// rows wird in DB-Reihenfolge erwartet
// (ORDER BY attendance_count DESC, attend_percentage DESC).
func BuildWithStrafen(rows []store.Stat, strafenBlock string) string {
	if len(rows) == 0 {
		return "🍻 *ZUMBA STATS*\n\n_Keine Daten._"
	}

	// total: aus der ersten DB-Zeile (wie im JS).
	total := rows[0].Attendance + rows[0].Away

	// rows kommen bereits sortiert aus stats.sql
	// (ORDER BY attendance_count DESC, attend_percentage DESC).
	users := rows

	// Medaillen mit Gleichstand-Logik.
	medals := []string{"🥇", "🥈", "🥉"}
	type ranked struct {
		store.Stat
		medal string
	}
	var (
		rankedUsers []ranked
		lastAttend  = math.MinInt
		lastPercent = math.NaN()
		rank        int
	)
	for _, u := range users {
		if u.Attendance != lastAttend || u.Percent != lastPercent {
			rank++
		}
		var medal string
		if rank <= len(medals) {
			medal = medals[rank-1]
		} else {
			medal = fmt.Sprintf("%d ", rank)
		}
		lastAttend = u.Attendance
		lastPercent = u.Percent
		rankedUsers = append(rankedUsers, ranked{Stat: u, medal: medal})
	}

	// Highlights.
	goat := rankedUsers[0]

	// Heißeste Serie / längste Pause: höchste bzw. niedrigste Streak – bei
	// Gleichstand werden ALLE Namen genannt (in Ranglisten-Reihenfolge).
	maxStreak, minStreak := rankedUsers[0].Streak, rankedUsers[0].Streak
	for _, u := range rankedUsers {
		if u.Streak > maxStreak {
			maxStreak = u.Streak
		}
		if u.Streak < minStreak {
			minStreak = u.Streak
		}
	}
	streakNames := func(streak int) string {
		var names []string
		for _, u := range rankedUsers {
			if u.Streak == streak {
				names = append(names, u.Name)
			}
		}
		return strings.Join(names, ", ")
	}

	var b strings.Builder
	b.WriteString("🍻 *ZUMBA STATS*\n")
	b.WriteString("_Weihnachtsfeier → Weihnachtsfeier_\n\n")
	b.WriteString(fmt.Sprintf("📊 *%d* Stammtische\n\n", total))
	b.WriteString(fmt.Sprintf("🐐 *GOAT:* %s (%s%%)\n", goat.Name, fmtNum(goat.Percent)))
	if maxStreak > 0 {
		flame := "🔥"
		if maxStreak > 7 {
			flame = "❤️‍🔥"
		}
		b.WriteString(fmt.Sprintf("%s *Heißeste Serie:* %s (%dx)\n", flame, streakNames(maxStreak), maxStreak))
	}
	if minStreak < 0 {
		ice := "❄️"
		if minStreak < -3 {
			ice = "🧊"
		}
		b.WriteString(fmt.Sprintf("%s *Längste Pause:* %s (%dx)\n", ice, streakNames(minStreak), abs(minStreak)))
	}
	b.WriteString("\n── *RANGLISTE* ──\n\n")

	lines := make([]string, 0, len(rankedUsers))
	for _, u := range rankedUsers {
		startDateText := ""
		if showStartDate && u.StartDate != nil {
			d := *u.StartDate
			startDateText = fmt.Sprintf(" _(%d.%d.)_", d.Day(), int(d.Month()))
		}
		bar := barChart(u.Percent, 6)
		streakLabel := hotTag(u.Streak) + coldTag(u.Streak)
		lines = append(lines, fmt.Sprintf("%s *%s* %s %d-%d (%s%%)%s%s",
			u.medal, u.Name, bar, u.Attendance, u.Away, fmtNum(u.Percent), streakLabel, startDateText))
	}
	b.WriteString(strings.Join(lines, "\n"))
	if strafenBlock != "" {
		b.WriteString("\n\n")
		b.WriteString(strafenBlock)
	}
	b.WriteString("\n\n🤖🍺 *Automatisch erstellt vom Zumba-Bot*")

	return b.String()
}

// StrafenBlock rendert den Strafen-Abschnitt für den Stichtag asOf: offene
// Strafen immer, beglichene bis einschließlich zum Folgedonnerstag der
// Begleichung, gelöschte nie. Ohne sichtbare Strafen gibt es die
// "Keine offenen Strafen"-Zeile.
func StrafenBlock(entries []penalty.Entry, asOf time.Time) string {
	var visible []penalty.Entry
	for _, e := range entries {
		if penalty.VisibleAt(e, asOf) {
			visible = append(visible, e)
		}
	}
	// Offene zuerst, innerhalb der Gruppen stabil (Assess sortiert nach Name).
	sort.SliceStable(visible, func(i, j int) bool {
		oi, oj := visible[i].Status == penalty.StatusOffen, visible[j].Status == penalty.StatusOffen
		return oi && !oj
	})

	var b strings.Builder
	b.WriteString("── 💸 *STRAFEN* ──\n")
	if len(visible) == 0 {
		b.WriteString("\n_Keine offenen Strafen_ 🎉")
		return b.String()
	}
	for _, e := range visible {
		b.WriteString("\n")
		b.WriteString(strafenLine(e))
	}
	return b.String()
}

// strafenLine baut die Zeile einer Strafe; die Klammer weist die Strafart aus.
func strafenLine(e penalty.Entry) string {
	var grund string
	switch e.Art {
	case penalty.ArtNoShow:
		grund = fmt.Sprintf("nicht abgemeldet, %s", fmtDate(e.Datum))
	default:
		grund = fmt.Sprintf("%dx in Folge gefehlt", e.Tage)
	}
	if e.Status == penalty.StatusBeglichen {
		return fmt.Sprintf("✅ *%s* – %d€ beglichen (%s)", e.Name, e.Betrag, grund)
	}
	return fmt.Sprintf("⚠️ *%s* – %d€ (%s)", e.Name, e.Betrag, grund)
}

// fmtDate rendert "12.3." (DE, ohne führende Nullen).
func fmtDate(t time.Time) string {
	return fmt.Sprintf("%d.%d.", t.Day(), int(t.Month()))
}

func barChart(percent float64, length int) string {
	filled := int(math.Round(percent / 100 * float64(length)))
	if filled < 0 {
		filled = 0
	}
	if filled > length {
		filled = length
	}
	return strings.Repeat("▰", filled) + strings.Repeat("▱", length-filled)
}

// fmtNum bildet die JS-Number-zu-String-Konvertierung nach: ganzzahlige Werte
// ohne Nachkommastellen, sonst die kürzeste Darstellung (z.B. 85.7 statt 85.70).
func fmtNum(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
