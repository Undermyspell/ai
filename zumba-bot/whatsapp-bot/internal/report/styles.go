// Package report: zusätzliche, auswählbare Nachrichten-Designs für die
// Bot-Test-Seite. Die LIVE-Nachricht (Build / BuildWeekly in report.go) bleibt
// davon unberührt – diese Stile dienen nur der Vorschau/Auswahl im Admin-UI.
package report

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/michael/zumba-whatsapp-bot/internal/store"
)

// Style ist ein auswählbares Nachrichten-Design.
type Style struct {
	ID    string
	Label string
	Build func([]store.Stat) string
}

// Styles listet alle auswählbaren Designs. "klassik" = die unveränderte
// Live-Nachricht; die übrigen sind Vorschau-Varianten.
func Styles() []Style {
	return []Style{
		{"klassik", "Klassik (live)", Build},
		{"podium", "Podium", buildPodium},
		{"kompakt", "Kompakt", buildKompakt},
		{"tabelle", "Scoreboard (Tabelle)", buildTabelle},
		{"race", "Rennen", buildRace},
		{"minimal", "Minimal", buildMinimal},
	}
}

// BuildByStyle rendert rows im gewählten Stil; unbekannt/leer → Live-Stil.
func BuildByStyle(id string, rows []store.Stat) string {
	for _, s := range Styles() {
		if s.ID == id {
			return s.Build(rows)
		}
	}
	return Build(rows)
}

// --- gemeinsame Auswertung (Ranking/Medaillen/Highlights) ---

type rankedUser struct {
	store.Stat
	medal string // 🥇/🥈/🥉 oder "4 ", "5 " …
	rank  int
}

type analysis struct {
	total      int
	users      []rankedUser
	mvp        rankedUser
	avgPercent int
	hottest    rankedUser
	coldest    rankedUser
}

func analyze(rows []store.Stat) analysis {
	total := rows[0].Attendance + rows[0].Away // wie Build: aus erster DB-Zeile

	users := make([]store.Stat, len(rows))
	copy(users, rows)
	sort.SliceStable(users, func(i, j int) bool {
		if users[i].Attendance != users[j].Attendance {
			return users[i].Attendance > users[j].Attendance
		}
		return users[i].Percent > users[j].Percent
	})

	medals := []string{"🥇", "🥈", "🥉"}
	var (
		ranked      []rankedUser
		lastAttend  = math.MinInt
		lastPercent = math.NaN()
		rank        int
		sumPercent  float64
	)
	for _, u := range users {
		if u.Attendance != lastAttend || u.Percent != lastPercent {
			rank++
		}
		medal := fmt.Sprintf("%d ", rank)
		if rank <= len(medals) {
			medal = medals[rank-1]
		}
		lastAttend, lastPercent = u.Attendance, u.Percent
		sumPercent += u.Percent
		ranked = append(ranked, rankedUser{Stat: u, medal: medal, rank: rank})
	}

	a := analysis{
		total:      total,
		users:      ranked,
		mvp:        ranked[0],
		avgPercent: int(math.Round(sumPercent / float64(len(ranked)))),
		hottest:    ranked[0],
		coldest:    ranked[0],
	}
	for _, u := range ranked {
		if u.Streak > a.hottest.Streak {
			a.hottest = u
		}
		if u.Streak < a.coldest.Streak {
			a.coldest = u
		}
	}
	return a
}

// --- neue Icon-Logik: lila Flamme ab Streak > 7, Eis ab Pause > 3 ---

// hotTag liefert das Streak-Suffix für laufende Anwesenheits-Serien.
func hotTag(streak int) string {
	switch {
	case streak > 7:
		return fmt.Sprintf(" ❤️‍🔥+%d", streak)
	case streak > 0:
		return fmt.Sprintf(" 🔥+%d", streak)
	}
	return ""
}

// coldTag liefert das Suffix für laufende Abwesenheits-Serien (streak < 0).
func coldTag(streak int) string {
	switch {
	case streak < -3:
		return fmt.Sprintf(" 🧊%d", streak)
	case streak < 0:
		return fmt.Sprintf(" ❄️%d", streak)
	}
	return ""
}

// hotEmoji/coldEmoji: nur das Symbol (für kompakte Designs).
func hotEmoji(streak int) string {
	switch {
	case streak > 7:
		return "❤️‍🔥"
	case streak > 0:
		return "🔥"
	}
	return ""
}

func coldEmoji(streak int) string {
	switch {
	case streak < -3:
		return "🧊"
	case streak < 0:
		return "❄️"
	}
	return ""
}

func empty() string { return "🍻 *ZUMBA STATS*\n\n_Keine Daten._" }

// Hinweis: "Klassik" (= Live-Nachricht, report.Build) nutzt jetzt selbst die neuen
// Icons (❤️‍🔥 ab Streak > 7, 🧊 ab Pause > 3) – daher keine separate "+Icons"-Variante mehr.

// --- Design: Podium (Top 3 im Fokus, Rest als Verfolger) ---

func buildPodium(rows []store.Stat) string {
	if len(rows) == 0 {
		return empty()
	}
	a := analyze(rows)

	var b strings.Builder
	fmt.Fprintf(&b, "🏆 *PODIUM* · %d Stammtische\n\n", a.total)
	var chasers []string
	for _, u := range a.users {
		if u.rank <= 3 {
			fmt.Fprintf(&b, "%s *%s* — %s%% · %d-%d%s%s\n",
				u.medal, u.Name, fmtNum(u.Percent), u.Attendance, u.Away, hotTag(u.Streak), coldTag(u.Streak))
		} else {
			chasers = append(chasers, fmt.Sprintf("%d. %s %s%%", u.rank, u.Name, fmtNum(u.Percent)))
		}
	}
	if len(chasers) > 0 {
		b.WriteString("\n— _Verfolger_ —\n")
		b.WriteString(strings.Join(chasers, " · "))
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\n🐐 %s", a.mvp.Name)
	return b.String()
}

// --- Design 3: Kompakt (dichte Liste, Prozent in Monospace ausgerichtet) ---

func buildKompakt(rows []store.Stat) string {
	if len(rows) == 0 {
		return empty()
	}
	a := analyze(rows)

	var b strings.Builder
	fmt.Fprintf(&b, "🍻 *ZUMBA* · %d×\n\n", a.total)
	for _, u := range a.users {
		tag := hotEmoji(u.Streak) + coldEmoji(u.Streak)
		if tag != "" {
			tag = " " + tag
		}
		fmt.Fprintf(&b, "%s *%s* `%3d%%` %s%s\n",
			u.medal, u.Name, int(math.Round(u.Percent)), barChart(u.Percent, 5), tag)
	}
	return strings.TrimRight(b.String(), "\n")
}

// --- Design 4: Scoreboard als Monospace-Tabelle (saubere Spalten) ---

func buildTabelle(rows []store.Stat) string {
	if len(rows) == 0 {
		return empty()
	}
	a := analyze(rows)

	var b strings.Builder
	b.WriteString("📊 *ZUMBA SCOREBOARD*\n")
	fmt.Fprintf(&b, "_%d Stammtische_\n", a.total)
	b.WriteString("```\n")
	b.WriteString(" #  Name         W-L     %\n")
	for _, u := range a.users {
		name := []rune(u.Name)
		if len(name) > 11 {
			name = name[:11]
		}
		wl := fmt.Sprintf("%d-%d", u.Attendance, u.Away)
		fmt.Fprintf(&b, "%2d  %-11s  %-6s %3d\n", u.rank, string(name), wl, int(math.Round(u.Percent)))
	}
	b.WriteString("```")
	return b.String()
}

// --- Design 5: Rennen (Läufer auf der Bahn entsprechend der Quote) ---

func buildRace(rows []store.Stat) string {
	if len(rows) == 0 {
		return empty()
	}
	a := analyze(rows)

	var b strings.Builder
	fmt.Fprintf(&b, "🏁 *ZUMBA RENNEN* · %d Etappen\n\n", a.total)
	const track = 10
	for _, u := range a.users {
		fill := int(math.Round(u.Percent / 100 * track))
		if fill < 0 {
			fill = 0
		}
		if fill > track {
			fill = track
		}
		lane := strings.Repeat("▰", fill) + "🏃" + strings.Repeat("▱", track-fill)
		tag := hotEmoji(u.Streak) + coldEmoji(u.Streak)
		if tag != "" {
			tag = " " + tag
		}
		fmt.Fprintf(&b, "%s *%s* %s `%d%%`%s\n", u.medal, u.Name, lane, int(math.Round(u.Percent)), tag)
	}
	return strings.TrimRight(b.String(), "\n")
}

// --- Design 6: Minimal (reduziert, wenig Emoji, schlanke Balken) ---

func buildMinimal(rows []store.Stat) string {
	if len(rows) == 0 {
		return empty()
	}
	a := analyze(rows)

	var b strings.Builder
	fmt.Fprintf(&b, "*ZUMBA* — nach %d Stammtischen\n", a.total)
	fmt.Fprintf(&b, "_GOAT %s_\n\n", a.mvp.Name)
	for _, u := range a.users {
		fmt.Fprintf(&b, "%s  %s%s  _%s%%_\n",
			slimBar(u.Percent), u.Name, hotColdMark(u.Streak), fmtNum(u.Percent))
	}
	return strings.TrimRight(b.String(), "\n")
}

// slimBar: 5er-Balken aus Achtel-Blöcken für ein feineres Minimal-Design.
func slimBar(percent float64) string {
	const width = 5
	full := int(percent / 100 * width)
	if full < 0 {
		full = 0
	}
	if full > width {
		full = width
	}
	return strings.Repeat("█", full) + strings.Repeat("░", width-full)
}

func hotColdMark(streak int) string {
	switch {
	case streak > 7:
		return " ❤️‍🔥"
	case streak < -3:
		return " 🧊"
	}
	return ""
}
