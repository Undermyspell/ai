package eval2026

import (
	"sort"

	"github.com/michael/stammtisch-wrapped/pkg/models"
)

// User emojis for display (can be extended or made configurable)
var userEmojis = []string{
	"👑", "🎯", "🔥", "⭐", "🎸", "🎮", "🍕", "🚀",
	"💪", "🎲", "🎭", "🌟", "🎪", "🎬", "🎵", "🎹",
	"🏆", "⚽", "🎱", "🎳", "🎯", "🎰", "🎼", "🎧",
}

// Titles based on attendance rate
var titleThresholds = []struct {
	MinRate    int
	Title      string
	TitleEmoji string
}{
	{90, "Stammtisch-König", "👑"},
	{80, "Zuverlässiger Stammgast", "⭐"},
	{70, "Regelmäßiger Teilnehmer", "🎯"},
	{60, "Gelegentlicher Gast", "🍺"},
	{50, "Sporadischer Besucher", "👀"},
	{0, "Seltener Gast", "👻"},
}

// calculateUserStats baut die User-Statistiken aus den SQL-Ergebnissen
// zusammen: Anwesenheit/Rate aus der geteilten Leaderboard-Query, längste
// Serien aus max_streaks.sql. In Go bleibt nur Präsentation (Titel, Emoji,
// Lieblings-Ausrede) und die Zuordnung der klassifizierten Absagen.
func (e *Evaluator) calculateUserStats(userLookup map[string]int, cancellations []models.Cancellation) []models.UserStats {
	userCancellations := make(map[int][]models.Cancellation)
	for _, c := range cancellations {
		userCancellations[c.UserID] = append(userCancellations[c.UserID], c)
	}

	// Längste Serien je User und Zustand (false = Anwesenheit, true = Absagen)
	streaks := make(map[string]map[bool]streakOf, len(e.rawData.MaxStreaks))
	for _, s := range e.rawData.MaxStreaks {
		if streaks[s.UserID] == nil {
			streaks[s.UserID] = make(map[bool]streakOf, 2)
		}
		streaks[s.UserID][s.Absent] = streakOf{Len: s.Len, Start: s.Start, End: s.End}
	}

	var userStats []models.UserStats

	for _, row := range e.rawData.Leaderboard {
		idx, ok := userLookup[row.UserID]
		if !ok {
			continue
		}
		userID := idx + 1 // 1-based ID, stabil über die Users-Reihenfolge

		// Absagen vor dem effektiven Start zählen nicht (SQL rechnet genauso).
		msgs := userCancellations[userID][:0:0]
		for _, c := range userCancellations[userID] {
			if !c.Date.Before(row.EffectiveStart) {
				msgs = append(msgs, c)
			}
		}

		rate := int(row.AttendPercent)
		title, titleEmoji := getTitleForRate(rate)
		att := streaks[row.UserID][false]
		canc := streaks[row.UserID][true]

		userStats = append(userStats, models.UserStats{
			User: models.User{
				ID:    userID,
				Name:  row.UserName,
				Emoji: userEmojis[idx%len(userEmojis)],
			},
			CancellationCount:          row.AwayCount,
			AttendanceCount:            row.AttendanceCount,
			AttendanceRate:             rate,
			MaxAttendanceStreak:        att.Len,
			MaxAttendanceStreakStart:   att.Start,
			MaxAttendanceStreakEnd:     att.End,
			MaxCancellationStreak:      canc.Len,
			MaxCancellationStreakStart: canc.Start,
			MaxCancellationStreakEnd:   canc.End,
			NeverCancelled:             row.AwayCount == 0,
			FavoriteExcuseCategory:     findFavoriteCategory(msgs),
			Title:                      title,
			TitleEmoji:                 titleEmoji,
			Cancellations:              msgs,
		})
	}

	// Wrapped sortiert nach Rate (nicht nach absoluter Anwesenheit wie die
	// Rangliste des Bots), dann Name — Präsentationsentscheidung, bleibt Go.
	sort.Slice(userStats, func(i, j int) bool {
		if userStats[i].AttendanceRate != userStats[j].AttendanceRate {
			return userStats[i].AttendanceRate > userStats[j].AttendanceRate
		}
		return userStats[i].Name < userStats[j].Name
	})

	// Assign ranks
	for i := range userStats {
		userStats[i].Rank = i + 1
	}

	return userStats
}

// findFavoriteCategory returns the most common category for a user's cancellations
func findFavoriteCategory(cancellations []models.Cancellation) string {
	if len(cancellations) == 0 {
		return ""
	}

	categoryCount := make(map[string]int)
	for _, c := range cancellations {
		categoryCount[c.Category]++
	}

	maxCount := 0
	favorite := ""
	for category, count := range categoryCount {
		if count > maxCount {
			maxCount = count
			favorite = category
		}
	}

	return favorite
}

// getTitleForRate returns title and emoji based on attendance rate
func getTitleForRate(rate int) (string, string) {
	for _, t := range titleThresholds {
		if rate >= t.MinRate {
			return t.Title, t.TitleEmoji
		}
	}
	return "Teilnehmer", "🍺"
}
