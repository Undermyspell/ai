package data

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/michael/stammtisch-wrapped/internal/penalty"
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

	// Local seeded source: rand.Seed is a no-op since Go 1.24, and every call
	// must return identical data so all slides describe the same mock year
	rng := rand.New(rand.NewSource(42))
	cancellations := []models.Cancellation{}

	for _, date := range thursdays {
		for _, user := range users {
			rate := cancellationRates[user.ID]
			if rng.Float64() < rate {
				// User cancels
				favCats := favoriteCategories[user.ID]
				catName := favCats[rng.Intn(len(favCats))]
				category := categories[catName]
				message := category.Examples[rng.Intn(len(category.Examples))]

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
		attendanceStreak, cancellationStreak := calculateStreaks(thursdays, userCancellations)

		// Find favorite excuse category
		favoriteCategory := findFavoriteExcuseCategory(userCancellations)

		userStats[i] = models.UserStats{
			User:                       user,
			CancellationCount:          cancellationCount,
			AttendanceCount:            attendanceCount,
			AttendanceRate:             attendanceRate,
			MaxAttendanceStreak:        attendanceStreak.count,
			MaxAttendanceStreakStart:   attendanceStreak.start,
			MaxAttendanceStreakEnd:     attendanceStreak.end,
			MaxCancellationStreak:      cancellationStreak.count,
			MaxCancellationStreakStart: cancellationStreak.start,
			MaxCancellationStreakEnd:   cancellationStreak.end,
			NeverCancelled:             cancellationCount == 0,
			FavoriteExcuseCategory:     favoriteCategory,
			Cancellations:              userCancellations,
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

// GetMonthlyAttendanceStats returns average attendance rate per month
func GetMonthlyAttendanceStats() models.MonthlyAttendanceStats {
	users := GetUsers()
	thursdays := GetThursdays2026()
	cancellations := GenerateCancellations()
	totalUsers := len(users)

	if totalUsers == 0 {
		return make(models.MonthlyAttendanceStats)
	}

	// Build a map of cancellations per Thursday
	cancellationsPerDay := make(map[string]int)
	for _, c := range cancellations {
		dateKey := c.Date.Format("2006-01-02")
		cancellationsPerDay[dateKey]++
	}

	// Group Thursdays by month and calculate attendance for each
	type monthData struct {
		totalAttendance int
		thursdayCount   int
	}
	monthlyData := make(map[string]*monthData)

	for _, thursday := range thursdays {
		monthKey := thursday.Format("2006-01")
		dateKey := thursday.Format("2006-01-02")

		if monthlyData[monthKey] == nil {
			monthlyData[monthKey] = &monthData{}
		}

		// Calculate attendance rate for this Thursday
		cancellationsOnDay := cancellationsPerDay[dateKey]
		attendees := totalUsers - cancellationsOnDay
		attendanceRate := (attendees * 100) / totalUsers

		monthlyData[monthKey].totalAttendance += attendanceRate
		monthlyData[monthKey].thursdayCount++
	}

	// Calculate average attendance rate per month
	stats := make(models.MonthlyAttendanceStats)
	for monthKey, data := range monthlyData {
		if data.thursdayCount > 0 {
			stats[monthKey] = data.totalAttendance / data.thursdayCount
		}
	}

	return stats
}

// GetThursdayStats returns per-Thursday attendance derived from the mock cancellations
func GetThursdayStats() []models.ThursdayStat {
	users := GetUsers()
	thursdays := GetThursdays2026()
	cancellations := GenerateCancellations()
	total := len(users)

	cancellationsPerDay := make(map[string]int)
	for _, c := range cancellations {
		cancellationsPerDay[c.Date.Format("2006-01-02")]++
	}

	stats := make([]models.ThursdayStat, 0, len(thursdays))
	for _, thursday := range thursdays {
		attendees := total - cancellationsPerDay[thursday.Format("2006-01-02")]
		stats = append(stats, models.ThursdayStat{
			Date:      thursday,
			Attendees: attendees,
			Total:     total,
			Rate:      (attendees * 100) / total,
		})
	}
	return stats
}

// GetStrafenStats evaluates the mock absences with the shared penalty logic,
// plus two hardcoded no-show penalties for variety
func GetStrafenStats() models.StrafenStats {
	users := GetUsers()
	thursdays := GetThursdays2026()
	cancellations := GenerateCancellations()

	if len(thursdays) == 0 {
		return models.StrafenStats{}
	}
	asOf := thursdays[len(thursdays)-1]

	absencesByUser := make(map[int][]time.Time)
	for _, c := range cancellations {
		absencesByUser[c.UserID] = append(absencesByUser[c.UserID], c.Date)
	}

	start := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
	userData := make([]penalty.UserData, 0, len(users))
	for _, u := range users {
		userData = append(userData, penalty.UserData{
			UserID:         fmt.Sprint(u.ID),
			Name:           u.Name,
			EffectiveStart: start,
			Absences:       absencesByUser[u.ID],
		})
	}

	// Two manual no-show penalties for variety
	noShowDate := thursdays[len(thursdays)/2]
	rows := []penalty.Row{
		{ID: 1, UserID: "8", Art: penalty.ArtNoShow, Datum: noShowDate, Betrag: penalty.NoShowDefault, Status: penalty.StatusOffen},
		{ID: 2, UserID: "13", Art: penalty.ArtNoShow, Datum: noShowDate, Betrag: penalty.NoShowDefault, Status: penalty.StatusBeglichen, BeglichenAm: &asOf},
	}

	entries := penalty.Assess(penalty.Input{Users: userData, Rows: rows}, asOf)

	excluded := map[string]bool{}
	byUser := make(map[string]*models.StrafenUserTotal)
	stats := models.StrafenStats{}

	for _, entry := range entries {
		if entry.Status == penalty.StatusGeloescht || entry.Betrag == 0 {
			continue
		}
		me := models.StrafenEntry{
			UserName: entry.Name,
			Art:      string(entry.Art),
			Betrag:   entry.Betrag,
			Tage:     entry.Tage,
			Start:    entry.Datum,
			Status:   string(entry.Status),
		}
		if entry.Art == penalty.ArtFehltage {
			seq := penalty.Thursdays(entry.Datum, asOf, excluded)
			if entry.Tage > 0 && len(seq) >= entry.Tage {
				me.End = seq[entry.Tage-1]
			} else {
				me.End = entry.Datum
			}
		}

		ut, ok := byUser[entry.UserID]
		if !ok {
			ut = &models.StrafenUserTotal{UserName: entry.Name}
			byUser[entry.UserID] = ut
		}
		ut.Total += me.Betrag
		if entry.Art == penalty.ArtFehltage {
			ut.FehltageCount++
		} else {
			ut.NoShowCount++
		}
		ut.Entries = append(ut.Entries, me)
		stats.TotalSum += me.Betrag
		stats.TotalCount++
	}

	for _, ut := range byUser {
		sort.Slice(ut.Entries, func(i, j int) bool {
			return ut.Entries[i].Start.Before(ut.Entries[j].Start)
		})
		stats.UserTotals = append(stats.UserTotals, *ut)
	}
	sort.Slice(stats.UserTotals, func(i, j int) bool {
		if stats.UserTotals[i].Total != stats.UserTotals[j].Total {
			return stats.UserTotals[i].Total > stats.UserTotals[j].Total
		}
		return stats.UserTotals[i].UserName < stats.UserTotals[j].UserName
	})

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

type mockStreak struct {
	count      int
	start, end time.Time
}

func calculateStreaks(thursdays []time.Time, userCancellations []models.Cancellation) (mockStreak, mockStreak) {
	cancellationMap := make(map[string]bool)
	for _, c := range userCancellations {
		cancellationMap[c.Date.Format("2006-01-02")] = true
	}

	var maxAttendance, maxCancellation mockStreak
	var currentAttendance, currentCancellation mockStreak

	for _, date := range thursdays {
		dateStr := date.Format("2006-01-02")
		if cancellationMap[dateStr] {
			if currentCancellation.count == 0 {
				currentCancellation.start = date
			}
			currentCancellation.count++
			currentCancellation.end = date
			if currentCancellation.count > maxCancellation.count {
				maxCancellation = currentCancellation
			}
			currentAttendance = mockStreak{}
		} else {
			if currentAttendance.count == 0 {
				currentAttendance.start = date
			}
			currentAttendance.count++
			currentAttendance.end = date
			if currentAttendance.count > maxAttendance.count {
				maxAttendance = currentAttendance
			}
			currentCancellation = mockStreak{}
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

// GetAwards returns special awards (same categories as the real evaluator)
func GetAwards() []models.Award {
	userStats := CalculateUserStats()
	thursdays := GetThursdays2026()

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

	awards := []models.Award{
		{
			Emoji:    "👑",
			Title:    "Stammtisch-König",
			Subtitle: "Höchste Anwesenheitsquote",
			Winner:   userStats[0],
			Color:    "from-yellow-500/30 to-amber-600/20",
		},
		{
			Emoji:    "🔥",
			Title:    "Streak-Meister",
			Subtitle: "Längste Anwesenheitsserie",
			Winner:   streakMaster,
			Color:    "from-orange-500/30 to-red-500/20",
		},
		{
			Emoji:    "🎨",
			Title:    "Kreativster Absager",
			Subtitle: "Die besten Ausreden",
			Winner:   excuseArtist,
			Color:    "from-purple-500/30 to-pink-500/20",
		},
	}

	// Comeback: longest finished cancellation streak (user returned)
	if len(thursdays) > 0 {
		lastThursday := thursdays[len(thursdays)-1]
		var comeback *models.UserStats
		maxStreak := 0
		for i := range userStats {
			u := &userStats[i]
			if u.MaxCancellationStreak >= 3 && u.MaxCancellationStreak > maxStreak &&
				!u.MaxCancellationStreakEnd.IsZero() && u.MaxCancellationStreakEnd.Before(lastThursday) {
				maxStreak = u.MaxCancellationStreak
				comeback = u
			}
		}
		if comeback != nil {
			awards = append(awards, models.Award{
				Emoji:    "🦅",
				Title:    "Comeback des Jahres",
				Subtitle: "Lange weg – und wieder da",
				Winner:   *comeback,
				Color:    "from-green-500/30 to-emerald-600/20",
			})
		}
	}

	// Rising star: biggest improvement second half vs. first half
	if len(thursdays) >= 8 {
		half := len(thursdays) / 2
		firstHalf := make(map[string]bool, half)
		for i, t := range thursdays {
			if i < half {
				firstHalf[t.Format("2006-01-02")] = true
			}
		}
		var rising *models.UserStats
		bestDelta := 0
		for i := range userStats {
			u := &userStats[i]
			miss1, miss2 := 0, 0
			for _, c := range u.Cancellations {
				if firstHalf[c.Date.Format("2006-01-02")] {
					miss1++
				} else {
					miss2++
				}
			}
			rate1 := ((half - miss1) * 100) / half
			rate2 := (((len(thursdays) - half) - miss2) * 100) / (len(thursdays) - half)
			if rate2-rate1 > bestDelta {
				bestDelta = rate2 - rate1
				rising = u
			}
		}
		if rising != nil && bestDelta >= 10 {
			awards = append(awards, models.Award{
				Emoji:    "🌟",
				Title:    "Rising Star",
				Subtitle: "Beste Entwicklung im Jahresverlauf",
				Winner:   *rising,
				Color:    "from-blue-500/30 to-cyan-500/20",
			})
		}
	}

	return awards
}
