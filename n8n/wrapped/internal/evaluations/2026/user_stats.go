package eval2026

import (
	"sort"
	"time"

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

// calculateUserStats computes statistics for each user
func (e *Evaluator) calculateUserStats(userLookup map[string]int, cancellations []models.Cancellation) []models.UserStats {
	totalThursdays := len(e.rawData.Thursdays)

	// Build cancellation map per user: userId -> []dates
	userCancellations := make(map[int][]time.Time)
	userCancellationMessages := make(map[int][]models.Cancellation)

	for _, c := range cancellations {
		userCancellations[c.UserID] = append(userCancellations[c.UserID], c.Date)
		userCancellationMessages[c.UserID] = append(userCancellationMessages[c.UserID], c)
	}

	var userStats []models.UserStats

	for i, user := range e.rawData.Users {
		userID := i + 1 // 1-based ID
		cancellationDates := userCancellations[userID]
		cancellationCount := len(cancellationDates)
		attendanceCount := totalThursdays - cancellationCount

		// Calculate attendance rate (0-100)
		attendanceRate := 0
		if totalThursdays > 0 {
			attendanceRate = (attendanceCount * 100) / totalThursdays
		}

		// Calculate streaks
		attendanceStreak, cancellationStreak := e.calculateStreaks(userID, cancellationDates)

		// Find favorite excuse category
		favoriteCategory := findFavoriteCategory(userCancellationMessages[userID])

		// Get title based on attendance rate
		title, titleEmoji := getTitleForRate(attendanceRate)

		// Get emoji for user
		emoji := userEmojis[i%len(userEmojis)]

		stats := models.UserStats{
			User: models.User{
				ID:    userID,
				Name:  user.UserName,
				Emoji: emoji,
			},
			CancellationCount:          cancellationCount,
			AttendanceCount:            attendanceCount,
			AttendanceRate:             attendanceRate,
			MaxAttendanceStreak:        attendanceStreak.Count,
			MaxAttendanceStreakStart:   attendanceStreak.Start,
			MaxAttendanceStreakEnd:     attendanceStreak.End,
			MaxCancellationStreak:      cancellationStreak.Count,
			MaxCancellationStreakStart: cancellationStreak.Start,
			MaxCancellationStreakEnd:   cancellationStreak.End,
			NeverCancelled:             cancellationCount == 0,
			FavoriteExcuseCategory:     favoriteCategory,
			Title:                      title,
			TitleEmoji:                 titleEmoji,
			Cancellations:              userCancellationMessages[userID],
		}

		userStats = append(userStats, stats)
	}

	// Sort by attendance rate (descending), then by name
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
