package data

import (
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/michael/stammtisch-wrapped/pkg/models"
)

// GetUsers returns all 15 Stammtisch users
func GetUsers() []models.User {
	return []models.User{
		{ID: 1, Name: "Max", Emoji: "🍺"},
		{ID: 2, Name: "Thomas", Emoji: "🎸"},
		{ID: 3, Name: "Stefan", Emoji: "⚽"},
		{ID: 4, Name: "Andreas", Emoji: "🎮"},
		{ID: 5, Name: "Michael", Emoji: "📚"},
		{ID: 6, Name: "Christian", Emoji: "🏔️"},
		{ID: 7, Name: "Markus", Emoji: "🚴"},
		{ID: 8, Name: "Daniel", Emoji: "🎬"},
		{ID: 9, Name: "Sebastian", Emoji: "💻"},
		{ID: 10, Name: "Patrick", Emoji: "🎯"},
		{ID: 11, Name: "Florian", Emoji: "🍕"},
		{ID: 12, Name: "Tobias", Emoji: "🏋️"},
		{ID: 13, Name: "Martin", Emoji: "🎵"},
		{ID: 14, Name: "Philipp", Emoji: "🎨"},
		{ID: 15, Name: "Jan", Emoji: "🏀"},
	}
}

// GetThursdays2026 returns all Thursdays in the 2026 evaluation period (01.12.2025 - 30.11.2026)
// Limited to today's date to exclude future Thursdays
func GetThursdays2026() []time.Time {
	start := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC)
	today := time.Now().Truncate(24 * time.Hour)

	// Cap end date at today
	if end.After(today) {
		end = today
	}

	var thursdays []time.Time
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Thursday {
			thursdays = append(thursdays, d)
		}
	}
	return thursdays
}

// GenerateCancellations creates mock cancellations for all users
func GenerateCancellations() []models.Cancellation {
	users := GetUsers()
	thursdays := GetThursdays2026()
	categories := models.GetAllExcuseCategories()

	// Cancellation rates per user
	cancellationRates := map[int]float64{
		1: 0.08, 2: 0.12, 3: 0.18, 4: 0.35, 5: 0.15,
		6: 0.25, 7: 0.10, 8: 0.45, 9: 0.22, 10: 0.30,
		11: 0.14, 12: 0.20, 13: 0.55, 14: 0.28, 15: 0.38,
	}

	// Favorite excuse categories per user
	favoriteCategories := map[int][]string{
		1:  {"arbeit", "gesundheit"},
		2:  {"freizeit", "arbeit"},
		3:  {"freizeit", "familie"},
		4:  {"muede", "keine_lust", "kreativ"},
		5:  {"arbeit", "familie"},
		6:  {"wetter", "freizeit"},
		7:  {"arbeit", "gesundheit"},
		8:  {"keine_lust", "kreativ", "muede"},
		9:  {"arbeit", "muede"},
		10: {"familie", "freizeit"},
		11: {"gesundheit", "arbeit"},
		12: {"gesundheit", "muede"},
		13: {"keine_lust", "kreativ", "muede"},
		14: {"kreativ", "freizeit"},
		15: {"freizeit", "keine_lust"},
	}

	rand.Seed(42) // Fixed seed for consistent results
	cancellations := []models.Cancellation{}

	for _, date := range thursdays {
		for _, user := range users {
			rate := cancellationRates[user.ID]
			if rand.Float64() < rate {
				// User cancels
				favCats := favoriteCategories[user.ID]
				catName := favCats[rand.Intn(len(favCats))]
				category := categories[catName]
				message := category.Examples[rand.Intn(len(category.Examples))]

				cancellations = append(cancellations, models.Cancellation{
					Date:     date,
					UserID:   user.ID,
					UserName: user.Name,
					Message:  message,
					Category: catName,
				})
			}
		}
	}

	return cancellations
}

// CalculateUserStats computes statistics for all users
func CalculateUserStats() []models.UserStats {
	users := GetUsers()
	thursdays := GetThursdays2026()
	cancellations := GenerateCancellations()
	totalThursdays := len(thursdays)

	userStats := make([]models.UserStats, len(users))

	for i, user := range users {
		userCancellations := filterCancellationsByUser(cancellations, user.ID)
		cancellationCount := len(userCancellations)
		attendanceCount := totalThursdays - cancellationCount
		attendanceRate := int(math.Round(float64(attendanceCount) / float64(totalThursdays) * 100))

		// Calculate streaks
		maxAttendanceStreak, maxCancellationStreak := calculateStreaks(thursdays, userCancellations)

		// Find favorite excuse category
		favoriteCategory := findFavoriteExcuseCategory(userCancellations)

		userStats[i] = models.UserStats{
			User:                   user,
			CancellationCount:      cancellationCount,
			AttendanceCount:        attendanceCount,
			AttendanceRate:         attendanceRate,
			MaxAttendanceStreak:    maxAttendanceStreak,
			MaxCancellationStreak:  maxCancellationStreak,
			NeverCancelled:         cancellationCount == 0,
			FavoriteExcuseCategory: favoriteCategory,
			Cancellations:          userCancellations,
		}
	}

	// Sort by attendance rate and assign ranks and titles
	sort.Slice(userStats, func(i, j int) bool {
		return userStats[i].AttendanceRate > userStats[j].AttendanceRate
	})

	for i := range userStats {
		userStats[i].Rank = i + 1
		title, emoji := getTitle(&userStats[i])
		userStats[i].Title = title
		userStats[i].TitleEmoji = emoji
	}

	return userStats
}

// GetGlobalStats calculates overall statistics
func GetGlobalStats() models.GlobalStats {
	users := GetUsers()
	thursdays := GetThursdays2026()
	cancellations := GenerateCancellations()
	userStats := CalculateUserStats()

	totalAttendances := len(thursdays)*len(users) - len(cancellations)

	avgRate := 0
	for _, stat := range userStats {
		avgRate += stat.AttendanceRate
	}
	avgRate = avgRate / len(userStats)

	return models.GlobalStats{
		TotalThursdays:        len(thursdays),
		TotalUsers:            len(users),
		TotalCancellations:    len(cancellations),
		TotalAttendances:      totalAttendances,
		AverageAttendanceRate: avgRate,
	}
}

// GetCategoryStats returns cancellation counts by category
func GetCategoryStats() models.CategoryStats {
	cancellations := GenerateCancellations()
	stats := make(models.CategoryStats)

	for _, c := range cancellations {
		stats[c.Category]++
	}

	return stats
}

// GetMonthStats returns cancellation counts by month
func GetMonthStats() models.MonthStats {
	cancellations := GenerateCancellations()
	stats := make(models.MonthStats)

	for _, c := range cancellations {
		monthKey := c.Date.Format("2006-01")
		stats[monthKey]++
	}

	return stats
}

// Helper functions

func filterCancellationsByUser(cancellations []models.Cancellation, userID int) []models.Cancellation {
	filtered := []models.Cancellation{}
	for _, c := range cancellations {
		if c.UserID == userID {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func calculateStreaks(thursdays []time.Time, userCancellations []models.Cancellation) (int, int) {
	cancellationMap := make(map[string]bool)
	for _, c := range userCancellations {
		cancellationMap[c.Date.Format("2006-01-02")] = true
	}

	currentAttendance := 0
	maxAttendance := 0
	currentCancellation := 0
	maxCancellation := 0

	for _, date := range thursdays {
		dateStr := date.Format("2006-01-02")
		if cancellationMap[dateStr] {
			currentCancellation++
			if currentCancellation > maxCancellation {
				maxCancellation = currentCancellation
			}
			currentAttendance = 0
		} else {
			currentAttendance++
			if currentAttendance > maxAttendance {
				maxAttendance = currentAttendance
			}
			currentCancellation = 0
		}
	}

	return maxAttendance, maxCancellation
}

func findFavoriteExcuseCategory(cancellations []models.Cancellation) string {
	categoryCount := make(map[string]int)
	for _, c := range cancellations {
		categoryCount[c.Category]++
	}

	maxCount := 0
	favorite := ""
	for cat, count := range categoryCount {
		if count > maxCount {
			maxCount = count
			favorite = cat
		}
	}

	return favorite
}

func getTitle(stats *models.UserStats) (string, string) {
	if stats.NeverCancelled {
		return "Die Legende", "👑"
	}
	if stats.AttendanceRate >= 90 {
		return "Fels in der Brandung", "🪨"
	}
	if stats.AttendanceRate >= 80 {
		return "Der Wirtshaus-Veteran", "🍺"
	}
	if stats.AttendanceRate >= 70 {
		return "Stammgast mit Ausnahmen", "✅"
	}
	if stats.AttendanceRate >= 60 {
		return "Kommt wenn's passt", "🤷"
	}
	if stats.AttendanceRate >= 50 {
		return "Der Spontane", "🎲"
	}
	if stats.AttendanceRate >= 40 {
		return "Mystische Erscheinung", "👻"
	}
	if stats.FavoriteExcuseCategory == "kreativ" {
		return "Ausreden-Künstler", "🎨"
	}
	if stats.FavoriteExcuseCategory == "keine_lust" {
		return "Der Ehrliche", "😬"
	}
	return "Der Unsichtbare", "🫥"
}

// GetAwards returns special awards
func GetAwards() []models.Award {
	userStats := CalculateUserStats()

	// Find streak master
	streakMaster := userStats[0]
	for _, stat := range userStats {
		if stat.MaxAttendanceStreak > streakMaster.MaxAttendanceStreak {
			streakMaster = stat
		}
	}

	// Find excuse artist
	excuseArtist := userStats[0]
	maxCreative := 0
	for _, stat := range userStats {
		creativeCount := 0
		for _, c := range stat.Cancellations {
			if c.Category == "kreativ" {
				creativeCount++
			}
		}
		if creativeCount > maxCreative {
			maxCreative = creativeCount
			excuseArtist = stat
		}
	}

	return []models.Award{
		{
			Emoji:    "👑",
			Title:    "Stammtisch-König",
			Subtitle: "Höchste Teilnahme 2026",
			Winner:   userStats[0],
			Color:    "from-yellow-500/30 to-amber-600/20",
		},
		{
			Emoji:    "🥈",
			Title:    "Fast immer da",
			Subtitle: "Zweithöchste Teilnahme",
			Winner:   userStats[1],
			Color:    "from-gray-400/30 to-gray-500/20",
		},
		{
			Emoji:    "🔥",
			Title:    "Streak-Master",
			Subtitle: "Längste Anwesenheits-Serie",
			Winner:   streakMaster,
			Color:    "from-orange-500/30 to-red-500/20",
		},
		{
			Emoji:    "🎨",
			Title:    "Ausreden-Künstler",
			Subtitle: "Kreativste Entschuldigungen",
			Winner:   excuseArtist,
			Color:    "from-purple-500/30 to-pink-500/20",
		},
		{
			Emoji:    "🚀",
			Title:    "Potenzial 2027",
			Subtitle: "Raum nach oben",
			Winner:   userStats[len(userStats)-1],
			Color:    "from-blue-500/30 to-cyan-500/20",
		},
	}
}
