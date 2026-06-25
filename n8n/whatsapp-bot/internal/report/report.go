// Package report portiert den n8n-Code-Node "Generate Whatsapp Message":
// aus der Per-User-Statistik wird der WhatsApp-Ranglisten-Text gebaut.
package report

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

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

// Build erzeugt den WhatsApp-Text. rows wird in DB-Reihenfolge erwartet
// (ORDER BY attendance_count DESC, attend_percentage DESC).
func Build(rows []store.Stat) string {
	if len(rows) == 0 {
		return "🍻 *ZUMBA STATS*\n\n_Keine Daten._"
	}

	// total: vor dem Sortieren aus der ersten DB-Zeile (wie im JS).
	total := rows[0].Attendance + rows[0].Away

	users := make([]store.Stat, len(rows))
	copy(users, rows)
	sort.SliceStable(users, func(i, j int) bool {
		if users[i].Attendance != users[j].Attendance {
			return users[i].Attendance > users[j].Attendance
		}
		return users[i].Percent > users[j].Percent
	})

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
	mvp := rankedUsers[0]
	var sumPercent float64
	for _, u := range users {
		sumPercent += u.Percent
	}
	avgPercent := int(math.Round(sumPercent / float64(len(users))))

	hottest := rankedUsers[0]
	coldest := rankedUsers[0]
	for _, u := range rankedUsers {
		if u.Streak > hottest.Streak {
			hottest = u
		}
		if u.Streak < coldest.Streak {
			coldest = u
		}
	}

	var b strings.Builder
	b.WriteString("🍻 *ZUMBA STATS*\n")
	b.WriteString("_Weihnachtsfeier → Weihnachtsfeier_\n\n")
	b.WriteString(fmt.Sprintf("📊 *%d* Stammtische\n\n", total))
	b.WriteString(fmt.Sprintf("👑 *MVP:* %s (%s%%)\n", mvp.Name, fmtNum(mvp.Percent)))
	if hottest.Streak > 0 {
		flame := "🔥"
		if hottest.Streak > 7 {
			flame = "❤️‍🔥"
		}
		b.WriteString(fmt.Sprintf("%s *Heißeste Serie:* %s (%dx)\n", flame, hottest.Name, hottest.Streak))
	}
	if coldest.Streak < 0 {
		ice := "❄️"
		if coldest.Streak < -3 {
			ice = "🧊"
		}
		b.WriteString(fmt.Sprintf("%s *Längste Pause:* %s (%dx)\n", ice, coldest.Name, abs(coldest.Streak)))
	}
	b.WriteString(fmt.Sprintf("📈 *Durchschnitt:* %d%%\n\n", avgPercent))
	b.WriteString("── *RANGLISTE* ──\n\n")

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
	b.WriteString("\n\n🤖🍺 *Automatisch erstellt vom Zumba-Bot*")

	return b.String()
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
