package eval2026

import (
	"sort"

	"github.com/michael/zumba-shared/domain"

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
			AttendancePercent:          row.AttendPercent,
			Since:                      row.EffectiveStart,
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

	sortAndRank(userStats)
	return userStats
}

// sortAndRank ordnet die Rangliste und vergibt die Plätze. Wrapped sortiert
// nach Quote (nicht nach absoluter Anwesenheit wie die Rangliste des Bots –
// fairer für Späteinsteiger im Jahresrückblick), dann nach Anwesenheiten und
// Name. Plätze wie im Sport ("1-2-2-4", shared/domain.CompetitionRanks):
// gleichauf ist, wer dieselbe exakte Quote und gleich viele Anwesenheiten
// hat – dieselbe Regel wie in Bot und Admin-UI.
func sortAndRank(userStats []models.UserStats) {
	sort.SliceStable(userStats, func(i, j int) bool {
		a, b := userStats[i], userStats[j]
		if a.AttendancePercent != b.AttendancePercent {
			return a.AttendancePercent > b.AttendancePercent
		}
		if a.AttendanceCount != b.AttendanceCount {
			return a.AttendanceCount > b.AttendanceCount
		}
		return a.Name < b.Name
	})
	ranks := domain.CompetitionRanks(len(userStats), func(i int) bool {
		return userStats[i].AttendancePercent == userStats[i-1].AttendancePercent &&
			userStats[i].AttendanceCount == userStats[i-1].AttendanceCount
	})
	for i := range userStats {
		userStats[i].Rank = ranks[i]
	}
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
