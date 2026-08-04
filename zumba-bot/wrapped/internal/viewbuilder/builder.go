// Package viewbuilder transforms evaluation results into view models for templ rendering.
package viewbuilder

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/michael/stammtisch-wrapped/pkg/models"
	"github.com/michael/stammtisch-wrapped/web/templates/years/2026/viewmodels"
)

// EvalData contains the raw evaluation data to transform
type EvalData struct {
	UserStats              []models.UserStats
	GlobalStats            models.GlobalStats
	CategoryStats          models.CategoryStats
	MonthStats             models.MonthStats
	MonthlyAttendanceStats models.MonthlyAttendanceStats
	ThursdayStats          []models.ThursdayStat
	StrafenStats           models.StrafenStats
	Awards                 []models.Award
	Cancellations          []models.Cancellation
}

// Build transforms evaluation data into a PageViewModel ready for templ rendering
func Build(data *EvalData, year string) viewmodels.PageViewModel {
	vm := viewmodels.PageViewModel{
		Year: year,
	}

	// Build YearStats
	vm.YearStats = buildYearStats(data.GlobalStats)

	// Build Rankings (3 tiers of 5)
	vm.Top5Rankings = buildRankings(data.UserStats, 0, 5, "top")
	vm.MidRankings = buildRankings(data.UserStats, 5, 10, "mid")
	vm.BottomRankings = buildRankings(data.UserStats, 10, 15, "bottom")

	// Build Streaks
	vm.AttendanceStreaks, vm.CancellationStreaks = buildStreaks(data.UserStats)

	// Build Excuse Categories
	vm.CategoryStats = buildCategoryStats(data.CategoryStats)

	// Build Best Excuses (creative ones) + verbatim recycled ones
	vm.BestExcuses = buildBestExcuses(data.Cancellations)
	vm.RecycledExcuses = buildRecycledExcuses(data.Cancellations)

	// Build Attendance Heatmap (rate colors + cancellation counts merged)
	vm.AttendanceHeatmapMonths, vm.AttendanceHeatmapInsight = buildAttendanceHeatmap(data.MonthlyAttendanceStats, data.MonthStats)

	// Build best/worst Thursdays
	vm.BestThursdays, vm.WorstThursdays = buildThursdayTopFlop(data.ThursdayStats)

	// Build full house Thursdays
	vm.FullHouse = buildFullHouse(data.ThursdayStats)

	// Build fun-card slides (duos, squad, forensics, muffel)
	vm.DuoCards = buildDuoCards(data.Cancellations, data.UserStats, data.ThursdayStats)
	vm.SquadCards = buildSquadCards(data.Cancellations, data.UserStats, data.ThursdayStats)
	vm.ForensikCards = buildForensikCards(data.Cancellations, data.UserStats)
	vm.MuffelCards = buildMuffelCards(data.Cancellations, data.UserStats)

	// Build quiz
	vm.Quiz = buildQuiz(data.UserStats)

	// Build Strafen
	vm.Strafen = buildStrafen(data.StrafenStats, data.UserStats)

	// Build AI Stats for client-side randomization
	vm.AIStats = buildAIStats(data.UserStats, data.GlobalStats, data.MonthStats)

	// Build share card payload
	vm.ShareJSON = buildShareJSON(year, data.GlobalStats, data.UserStats, data.StrafenStats)

	// Build Personality Types
	vm.PersonalityTypes = buildPersonalityTypes(data.UserStats)

	// Build Awards
	vm.Awards = buildAwards(data.Awards)

	// Build Confetti (pre-generated particles for SSR)
	vm.Confetti = buildConfetti()

	return vm
}

// buildYearStats creates the year stats view
func buildYearStats(gs models.GlobalStats) viewmodels.YearStatsView {
	return viewmodels.YearStatsView{
		TotalThursdays:        gs.TotalThursdays,
		TotalUsers:            gs.TotalUsers,
		TotalAttendances:      gs.TotalAttendances,
		TotalCancellations:    gs.TotalCancellations,
		AverageAttendanceRate: gs.AverageAttendanceRate,
	}
}

// buildRankings creates ranked user cards for a tier
func buildRankings(users []models.UserStats, start, end int, tier string) []viewmodels.RankedUser {
	if end > len(users) {
		end = len(users)
	}
	if start >= end {
		return nil
	}

	result := make([]viewmodels.RankedUser, 0, end-start)
	for i := start; i < end; i++ {
		user := users[i]
		idx := i - start // Index within this tier (0-4)

		result = append(result, viewmodels.RankedUser{
			Rank:            user.Rank,
			RankDisplay:     getRankDisplay(user.Rank),
			Name:            user.Name,
			Emoji:           user.Emoji,
			Title:           user.Title,
			TitleEmoji:      user.TitleEmoji,
			AttendanceRate:  user.AttendanceRate,
			BarColor:        getBarColor(user.AttendanceRate),
			TierBgColor:     getTierBgColor(tier),
			FunFact:         getFunFact(user),
			PersonalMessage: getPersonalMessage(user.AttendanceRate),
			DelayClass:      fmt.Sprintf("delay-%d", idx*100+200),
		})
	}
	return result
}

// buildStreaks creates the top 3 attendance and cancellation streaks
func buildStreaks(users []models.UserStats) ([]viewmodels.StreakUser, []viewmodels.StreakUser) {
	// Sort by attendance streak (descending)
	attendanceSorted := make([]models.UserStats, len(users))
	copy(attendanceSorted, users)
	sort.Slice(attendanceSorted, func(i, j int) bool {
		return attendanceSorted[i].MaxAttendanceStreak > attendanceSorted[j].MaxAttendanceStreak
	})

	attendanceStreaks := make([]viewmodels.StreakUser, 0, 3)
	for i := 0; i < 3 && i < len(attendanceSorted); i++ {
		user := attendanceSorted[i]
		attendanceStreaks = append(attendanceStreaks, viewmodels.StreakUser{
			Name:                user.Name,
			Emoji:               user.Emoji,
			MaxAttendanceStreak: user.MaxAttendanceStreak,
			DateRange:           formatDateRange(user.MaxAttendanceStreakStart, user.MaxAttendanceStreakEnd),
			DelayClass:          fmt.Sprintf("delay-%d", i*200+200),
		})
	}

	// Sort by cancellation streak (descending), filter out zeros
	cancellationSorted := make([]models.UserStats, 0)
	for _, u := range users {
		if u.MaxCancellationStreak > 0 {
			cancellationSorted = append(cancellationSorted, u)
		}
	}
	sort.Slice(cancellationSorted, func(i, j int) bool {
		return cancellationSorted[i].MaxCancellationStreak > cancellationSorted[j].MaxCancellationStreak
	})

	cancellationStreaks := make([]viewmodels.StreakUser, 0, 3)
	for i := 0; i < 3 && i < len(cancellationSorted); i++ {
		user := cancellationSorted[i]
		cancellationStreaks = append(cancellationStreaks, viewmodels.StreakUser{
			Name:                  user.Name,
			Emoji:                 user.Emoji,
			MaxCancellationStreak: user.MaxCancellationStreak,
			DateRange:             formatDateRange(user.MaxCancellationStreakStart, user.MaxCancellationStreakEnd),
			DelayClass:            fmt.Sprintf("delay-%d", i*200+700),
		})
	}

	return attendanceStreaks, cancellationStreaks
}

// buildCategoryStats creates sorted category statistics with percentages
func buildCategoryStats(cs models.CategoryStats) []viewmodels.CategoryStat {
	// Get category labels
	allCategories := models.GetAllExcuseCategories()

	// Convert to slice and sort by count
	type catEntry struct {
		key   string
		count int
	}
	entries := make([]catEntry, 0, len(cs))
	for key, count := range cs {
		if count > 0 {
			entries = append(entries, catEntry{key, count})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].count > entries[j].count
	})

	maxCount := 1
	if len(entries) > 0 {
		maxCount = entries[0].count
	}

	result := make([]viewmodels.CategoryStat, 0, len(entries))
	for i, entry := range entries {
		cat := allCategories[entry.key]
		percentage := (entry.count * 100) / maxCount

		result = append(result, viewmodels.CategoryStat{
			Key:        entry.key,
			Label:      cat.Label,
			Emoji:      cat.Emoji,
			Count:      entry.count,
			Percentage: percentage,
			DelayClass: fmt.Sprintf("delay-%d", i*100+200),
		})
	}
	return result
}

// buildBestExcuses gets the top creative excuses
func buildBestExcuses(cancellations []models.Cancellation) []viewmodels.Excuse {
	creative := make([]viewmodels.Excuse, 0)
	for _, c := range cancellations {
		if c.Category == "kreativ" {
			creative = append(creative, viewmodels.Excuse{
				Message:  c.Message,
				UserName: c.UserName,
			})
		}
	}

	// Limit to 5 and add delays
	if len(creative) > 5 {
		creative = creative[:5]
	}
	for i := range creative {
		creative[i].DelayClass = fmt.Sprintf("delay-%d", i*200+200)
	}

	return creative
}

// periodMonth is one month of the wrapped period (Dec 2025 - Nov 2026)
type periodMonth struct {
	Key   string // e.g. "2025-12"
	Label string // e.g. "Dez"
}

// periodMonths returns the 12 months of the wrapped period in chronological
// order: Dez 2025, Jan 2026, ..., Nov 2026
func periodMonths() []periodMonth {
	labels := []string{"Jan", "Feb", "Mär", "Apr", "Mai", "Jun", "Jul", "Aug", "Sep", "Okt", "Nov", "Dez"}
	out := make([]periodMonth, 0, 12)
	out = append(out, periodMonth{Key: "2025-12", Label: "Dez"})
	for i := 1; i <= 11; i++ {
		out = append(out, periodMonth{Key: fmt.Sprintf("2026-%02d", i), Label: labels[i-1]})
	}
	return out
}

// buildAttendanceHeatmap creates monthly heatmap data: cell color from the
// attendance rate, cancellation count from MonthStats as secondary info
func buildAttendanceHeatmap(mas models.MonthlyAttendanceStats, ms models.MonthStats) ([]viewmodels.AttendanceHeatmapMonth, viewmodels.AttendanceHeatmapInsight) {
	// Collect month data
	type monthData struct {
		label string
		key   string
		rate  int
	}
	data := make([]monthData, 12)
	for i, m := range periodMonths() {
		data[i] = monthData{
			label: m.Label,
			key:   m.Key,
			rate:  mas[m.Key],
		}
	}

	// Build heatmap months
	heatmapMonths := make([]viewmodels.AttendanceHeatmapMonth, 12)
	for i, d := range data {
		heatmapMonths[i] = viewmodels.AttendanceHeatmapMonth{
			Label:      d.label,
			Rate:       d.rate,
			Count:      ms[d.key],
			BgColor:    getAttendanceHeatmapColor(d.rate),
			DelayClass: fmt.Sprintf("delay-%d", i*50+200),
		}
	}

	// Find best and worst months (only consider months with data)
	var best, worst monthData
	best.rate = -1
	worst.rate = 101

	for _, d := range data {
		if d.rate > 0 { // Only consider months with data
			if d.rate > best.rate {
				best = d
			}
			if d.rate < worst.rate {
				worst = d
			}
		}
	}

	// Handle case where no months have data
	if best.rate == -1 {
		best = data[0]
	}
	if worst.rate == 101 {
		worst = data[0]
	}

	insight := viewmodels.AttendanceHeatmapInsight{
		BestMonth:  best.label,
		BestRate:   best.rate,
		WorstMonth: worst.label,
		WorstRate:  worst.rate,
	}

	return heatmapMonths, insight
}

// buildThursdayTopFlop picks the 3 best and 3 worst attended Thursdays
func buildThursdayTopFlop(stats []models.ThursdayStat) ([]viewmodels.ThursdayCard, []viewmodels.ThursdayCard) {
	if len(stats) == 0 {
		return nil, nil
	}

	best := make([]models.ThursdayStat, len(stats))
	copy(best, stats)
	sort.SliceStable(best, func(i, j int) bool {
		if best[i].Attendees != best[j].Attendees {
			return best[i].Attendees > best[j].Attendees
		}
		return best[i].Date.Before(best[j].Date)
	})

	worst := make([]models.ThursdayStat, len(stats))
	copy(worst, stats)
	sort.SliceStable(worst, func(i, j int) bool {
		if worst[i].Attendees != worst[j].Attendees {
			return worst[i].Attendees < worst[j].Attendees
		}
		return worst[i].Date.Before(worst[j].Date)
	})

	topMedals := []string{"🥇", "🥈", "🥉"}
	flopMedals := []string{"💀", "🕸️", "🦗"}

	buildCards := func(src []models.ThursdayStat, medals []string, delayOffset int) []viewmodels.ThursdayCard {
		n := 3
		if n > len(src) {
			n = len(src)
		}
		cards := make([]viewmodels.ThursdayCard, 0, n)
		for i := 0; i < n; i++ {
			s := src[i]
			cards = append(cards, viewmodels.ThursdayCard{
				RankDisplay: medals[i],
				DateDisplay: formatDateWithYear(s.Date),
				Attendees:   s.Attendees,
				Total:       s.Total,
				Rate:        s.Rate,
				BarColor:    getBarColor(s.Rate),
				DelayClass:  fmt.Sprintf("delay-%d", i*150+delayOffset),
			})
		}
		return cards
	}

	return buildCards(best, topMedals, 200), buildCards(worst, flopMedals, 700)
}

// buildStrafen aggregates penalty stats into the strafen slide view
func buildStrafen(stats models.StrafenStats, users []models.UserStats) viewmodels.StrafenView {
	view := viewmodels.StrafenView{
		HasStrafen: stats.TotalCount > 0,
		TotalSum:   stats.TotalSum,
		TotalCount: stats.TotalCount,
		MassBier:   stats.TotalSum / 5,
	}
	if !view.HasStrafen {
		return view
	}

	emojiByName := make(map[string]string, len(users))
	for _, u := range users {
		emojiByName[u.Name] = u.Emoji
	}

	medals := []string{"🥇", "🥈", "🥉"}
	n := 3
	if n > len(stats.UserTotals) {
		n = len(stats.UserTotals)
	}

	for i := 0; i < n; i++ {
		ut := stats.UserTotals[i]

		entries := make([]viewmodels.StrafenEntryView, 0, len(ut.Entries))
		for _, e := range ut.Entries {
			ev := viewmodels.StrafenEntryView{
				Betrag: fmt.Sprintf("%d €", e.Betrag),
			}
			if e.Art == "fehltage" {
				ev.ArtEmoji = "🪑"
				ev.Label = fmt.Sprintf("%d Wochen gefehlt", e.Tage)
				ev.DateRange = formatDateRange(e.Start, e.End)
			} else {
				ev.ArtEmoji = "👻"
				ev.Label = "No-Show"
				ev.DateRange = formatDateWithYear(e.Start)
			}
			if e.Status == "beglichen" {
				ev.StatusEmoji = "✅"
			} else {
				ev.StatusEmoji = "⏳"
			}
			entries = append(entries, ev)
		}

		view.TopPayers = append(view.TopPayers, viewmodels.StrafenUserView{
			RankDisplay: medals[i],
			Name:        ut.UserName,
			Emoji:       emojiByName[ut.UserName],
			Total:       fmt.Sprintf("%d €", ut.Total),
			TotalEuro:   ut.Total,
			Entries:     entries,
			DelayClass:  fmt.Sprintf("delay-%d", i*200+400),
		})
	}

	return view
}

// buildAIStats creates the pre-rendered AI summary (server-side randomization)
func buildAIStats(users []models.UserStats, gs models.GlobalStats, ms models.MonthStats) viewmodels.AIStats {
	topUser := ""
	bottomUser := ""
	if len(users) > 0 {
		topUser = users[0].Name
		bottomUser = users[len(users)-1].Name
	}

	// Find worst/best month within the wrapped period
	fullNames := map[string]string{
		"Jan": "Januar", "Feb": "Februar", "Mär": "März", "Apr": "April",
		"Mai": "Mai", "Jun": "Juni", "Jul": "Juli", "Aug": "August",
		"Sep": "September", "Okt": "Oktober", "Nov": "November", "Dez": "Dezember",
	}
	var worstMonth, bestMonth string
	var worstCount, bestCount int
	bestCount = 999999

	for _, m := range periodMonths() {
		count := ms[m.Key]
		if count > worstCount {
			worstCount = count
			worstMonth = fullNames[m.Label]
		}
		if count > 0 && count < bestCount {
			bestCount = count
			bestMonth = fullNames[m.Label]
		}
	}
	if bestCount == 999999 {
		bestCount = 0
		bestMonth = "Dezember"
	}

	avgRate := gs.AverageAttendanceRate
	totalAttendances := gs.TotalAttendances

	// Pre-select one of the 3 summary variants server-side
	summaries := []string{
		fmt.Sprintf(`2026 war ein Jahr der Hingabe – mit einer durchschnittlichen Teilnahme von <span class="text-biergold font-bold">%d%%</span>. %s führte das Feld an, während %s noch Potenzial nach oben hat. Im %s war die Motivation am niedrigsten, aber im %s zeigte sich wahre Stammtisch-Treue!`,
			avgRate, topUser, bottomUser, worstMonth, bestMonth),
		fmt.Sprintf(`Der Stammtisch 2026: Eine Geschichte von Bier, Freundschaft und... kreativen Ausreden. <span class="text-biergold font-bold">%s</span> war der unerschütterliche Fels, während <span class="text-biergold font-bold">%s</span> eher spirituell dabei war. Der %s forderte uns heraus – aber wir haben durchgehalten!`,
			topUser, bottomUser, worstMonth),
		fmt.Sprintf(`Was für ein Jahr! <span class="text-biergold font-bold">%d</span> mal wurde am Stammtisch angestoßen. %s verpasste kaum einen Donnerstag, während %s den Begriff "Stammtisch" eher flexibel interpretierte. Der Sommer war stark, der %s war eine Herausforderung.`,
			totalAttendances, topUser, bottomUser, worstMonth),
	}

	selectedSummary := summaries[rand.Intn(len(summaries))]

	return viewmodels.AIStats{
		SummaryHTML: selectedSummary,
	}
}

// buildRecycledExcuses finds messages a user sent verbatim more than once
func buildRecycledExcuses(cancellations []models.Cancellation) []viewmodels.RecycledExcuse {
	type key struct {
		user string
		msg  string
	}
	counts := make(map[key]int)
	original := make(map[key]string)
	for _, c := range cancellations {
		msg := strings.TrimSpace(c.Message)
		if msg == "" {
			continue
		}
		k := key{user: c.UserName, msg: strings.ToLower(msg)}
		counts[k]++
		original[k] = msg
	}

	var result []viewmodels.RecycledExcuse
	for k, n := range counts {
		if n >= 2 {
			result = append(result, viewmodels.RecycledExcuse{
				UserName: k.user,
				Message:  original[k],
				Count:    n,
			})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].UserName < result[j].UserName
	})
	if len(result) > 3 {
		result = result[:3]
	}
	for i := range result {
		result[i].DelayClass = fmt.Sprintf("delay-%d", i*200+1200)
	}
	return result
}

// buildFullHouse collects Thursdays where everyone attended
func buildFullHouse(stats []models.ThursdayStat) viewmodels.FullHouseView {
	var dates []string
	for _, s := range stats {
		if s.Total > 0 && s.Attendees == s.Total {
			dates = append(dates, formatDateWithYear(s.Date))
		}
	}
	view := viewmodels.FullHouseView{
		HasAny: len(dates) > 0,
		Count:  len(dates),
	}
	if len(dates) > 5 {
		view.More = len(dates) - 5
		dates = dates[:5]
	}
	view.Dates = dates
	return view
}

// buildQuiz picks one quiz question answerable from the data
func buildQuiz(users []models.UserStats) viewmodels.QuizView {
	for _, u := range users {
		if u.NeverCancelled {
			return viewmodels.QuizView{
				Question:     "Wer hat dieses Jahr kein einziges Mal abgesagt?",
				AnswerName:   u.Name,
				AnswerEmoji:  u.Emoji,
				AnswerDetail: fmt.Sprintf("%d von %d Donnerstagen da – null Absagen.", u.AttendanceCount, u.AttendanceCount+u.CancellationCount),
			}
		}
	}
	if len(users) == 0 {
		return viewmodels.QuizView{}
	}
	top := users[0]
	return viewmodels.QuizView{
		Question:     "Wer stand dieses Jahr am häufigsten am Tisch?",
		AnswerName:   top.Name,
		AnswerEmoji:  top.Emoji,
		AnswerDetail: fmt.Sprintf("%d%% Anwesenheit, %d Donnerstage.", top.AttendanceRate, top.AttendanceCount),
	}
}

// buildShareJSON serializes the key numbers for the client-side share card
func buildShareJSON(year string, gs models.GlobalStats, users []models.UserStats, strafen models.StrafenStats) string {
	topName := ""
	if len(users) > 0 {
		topName = users[0].Emoji + " " + users[0].Name
	}
	payload := map[string]any{
		"year":       year,
		"thursdays":  gs.TotalThursdays,
		"users":      gs.TotalUsers,
		"avgRate":    gs.AverageAttendanceRate,
		"topUser":    topName,
		"strafenSum": strafen.TotalSum,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// buildPersonalityTypes groups users by personality type
func buildPersonalityTypes(users []models.UserStats) []viewmodels.PersonalityType {
	types := []struct {
		emoji       string
		name        string
		description string
		filter      func(u models.UserStats) bool
	}{
		{
			emoji:       "🪨",
			name:        "Der Fels",
			description: "Immer da, immer zuverlässig",
			filter:      func(u models.UserStats) bool { return u.AttendanceRate >= 85 },
		},
		{
			emoji:       "🎲",
			name:        "Der Spontane",
			description: "Kommt wenn der Vibe stimmt",
			filter:      func(u models.UserStats) bool { return u.AttendanceRate >= 50 && u.AttendanceRate < 70 },
		},
		{
			emoji:       "🎨",
			name:        "Der Kreative",
			description: "Hat die besten Ausreden",
			filter:      func(u models.UserStats) bool { return u.FavoriteExcuseCategory == "kreativ" },
		},
		{
			emoji:       "👻",
			name:        "Das Phantom",
			description: "Selten gesichtet, aber legendär",
			filter:      func(u models.UserStats) bool { return u.AttendanceRate < 50 },
		},
	}

	result := make([]viewmodels.PersonalityType, 0)
	for i, t := range types {
		// Filter users
		matchingUsers := make([]viewmodels.PersonalityUser, 0)
		for _, u := range users {
			if t.filter(u) {
				matchingUsers = append(matchingUsers, viewmodels.PersonalityUser{
					Name:  u.Name,
					Emoji: u.Emoji,
				})
			}
		}

		if len(matchingUsers) == 0 {
			continue
		}

		pt := viewmodels.PersonalityType{
			Emoji:       t.emoji,
			Name:        t.name,
			Description: t.description,
			DelayClass:  fmt.Sprintf("delay-%d", i*150+200),
		}

		if len(matchingUsers) > 5 {
			pt.Users = matchingUsers[:5]
			pt.HasMore = true
			pt.MoreCount = len(matchingUsers) - 5
		} else {
			pt.Users = matchingUsers
		}

		result = append(result, pt)
	}

	return result
}

// buildAwards creates award views
func buildAwards(awards []models.Award) []viewmodels.AwardView {
	result := make([]viewmodels.AwardView, len(awards))
	for i, a := range awards {
		result[i] = viewmodels.AwardView{
			Emoji:       a.Emoji,
			Title:       a.Title,
			Subtitle:    a.Subtitle,
			WinnerName:  a.Winner.Name,
			WinnerEmoji: a.Winner.Emoji,
			Color:       a.Color,
			DelayClass:  fmt.Sprintf("delay-%d", i*200+200),
		}
	}
	return result
}

// buildConfetti creates pre-generated confetti particles for SSR
func buildConfetti() viewmodels.ConfettiView {
	colors := []string{"#F59E0B", "#FEF3C7", "#D97706", "#92400E", "#ffffff"}
	confettiCount := 50

	particles := make([]viewmodels.ConfettiParticle, confettiCount)
	for i := range confettiCount {
		color := colors[rand.Intn(len(colors))]
		left := rand.Float64() * 100
		delay := rand.Float64() * 3
		duration := 3 + rand.Float64()*2
		size := 5 + rand.Float64()*10
		borderRadius := "0"
		if rand.Float64() > 0.5 {
			borderRadius = "50%"
		}

		particles[i] = viewmodels.ConfettiParticle{
			Color:        color,
			Left:         fmt.Sprintf("%.2f%%", left),
			Size:         fmt.Sprintf("%.0fpx", size),
			BorderRadius: borderRadius,
			Delay:        fmt.Sprintf("%.2fs", delay),
			Duration:     fmt.Sprintf("%.2fs", duration),
		}
	}

	return viewmodels.ConfettiView{
		Particles: particles,
	}
}

// Helper functions

// getRankDisplay returns medal emoji for top 3, or "#N" for others
func getRankDisplay(rank int) string {
	medals := []string{"🥇", "🥈", "🥉"}
	if rank <= 3 {
		return medals[rank-1]
	}
	return fmt.Sprintf("#%d", rank)
}

// getBarColor returns the appropriate color class based on attendance rate
func getBarColor(rate int) string {
	switch {
	case rate >= 80:
		return "bg-green-500"
	case rate >= 60:
		return "bg-biergold"
	case rate >= 40:
		return "bg-orange-500"
	default:
		return "bg-red-400"
	}
}

// getTierBgColor returns the background gradient class for a tier
func getTierBgColor(tier string) string {
	switch tier {
	case "top":
		return "bg-gradient-to-r from-biergold/30 to-holz-light/50"
	case "mid":
		return "bg-holz-light/40"
	default:
		return "bg-holz-light/20"
	}
}

// getAttendanceHeatmapColor returns the background color class based on attendance rate
// Higher attendance = greener (good), lower attendance = redder (bad)
func getAttendanceHeatmapColor(rate int) string {
	if rate == 0 {
		return "bg-holz-light/30"
	}

	switch {
	case rate >= 80:
		return "bg-green-500"
	case rate >= 65:
		return "bg-green-500/50"
	case rate >= 50:
		return "bg-yellow-500"
	case rate >= 35:
		return "bg-orange-500"
	default:
		return "bg-red-500"
	}
}

// getPersonalMessage returns a message based on attendance rate
func getPersonalMessage(rate int) string {
	switch {
	case rate >= 90:
		return "Legende! 🏆"
	case rate >= 80:
		return "Stark dabei! 💪"
	case rate >= 70:
		return "Solide! 📈"
	case rate >= 60:
		return "Wenn's passt 🤷"
	case rate >= 50:
		return "Spontan 🎲"
	default:
		return "Mysteriös 👻"
	}
}

// getFunFact returns a fun fact about the user
func getFunFact(user models.UserStats) string {
	switch {
	case user.NeverCancelled:
		return "👑 Nie abgesagt!"
	case user.MaxAttendanceStreak >= 10:
		return fmt.Sprintf("🔥 %der Serie", user.MaxAttendanceStreak)
	case user.MaxCancellationStreak >= 4:
		return fmt.Sprintf("🧊 %d Wochen Pause", user.MaxCancellationStreak)
	case user.FavoriteExcuseCategory == "kreativ":
		return "🎨 Kreativ-Ausreder"
	case user.FavoriteExcuseCategory == "arbeit":
		return "💼 Workaholic"
	default:
		return fmt.Sprintf("%dx gefehlt", user.CancellationCount)
	}
}

// formatDateWithYear formats a single date for display (e.g., "12. Feb 2026")
func formatDateWithYear(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	germanMonths := []string{
		"Jan", "Feb", "Mär", "Apr", "Mai", "Jun",
		"Jul", "Aug", "Sep", "Okt", "Nov", "Dez",
	}
	return fmt.Sprintf("%d. %s %d", t.Day(), germanMonths[t.Month()-1], t.Year())
}

// formatDateRange formats a date range for display (e.g., "12. Jan - 9. Feb")
func formatDateRange(start, end time.Time) string {
	if start.IsZero() || end.IsZero() {
		return ""
	}

	germanMonths := []string{
		"Jan", "Feb", "Mär", "Apr", "Mai", "Jun",
		"Jul", "Aug", "Sep", "Okt", "Nov", "Dez",
	}

	startMonth := germanMonths[start.Month()-1]
	endMonth := germanMonths[end.Month()-1]

	if start.Equal(end) {
		return fmt.Sprintf("%d. %s", start.Day(), startMonth)
	}

	return fmt.Sprintf("%d. %s – %d. %s", start.Day(), startMonth, end.Day(), endMonth)
}
