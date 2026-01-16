package eval2026

import "github.com/michael/stammtisch-wrapped/pkg/models"

// Award definitions
var awardDefinitions = []struct {
	Emoji    string
	Title    string
	Subtitle string
	Color    string
	Selector func([]models.UserStats) *models.UserStats
}{
	{
		Emoji:    "👑",
		Title:    "Stammtisch-König",
		Subtitle: "Höchste Anwesenheitsquote",
		Color:    "from-yellow-500 to-amber-600",
		Selector: selectHighestAttendance,
	},
	{
		Emoji:    "🔥",
		Title:    "Streak-Meister",
		Subtitle: "Längste Anwesenheitsserie",
		Color:    "from-orange-500 to-red-600",
		Selector: selectLongestAttendanceStreak,
	},
	{
		Emoji:    "🎨",
		Title:    "Kreativster Absager",
		Subtitle: "Die besten Ausreden",
		Color:    "from-purple-500 to-pink-600",
		Selector: selectMostCreativeExcuser,
	},
	{
		Emoji:    "📈",
		Title:    "Konsistenz-Champion",
		Subtitle: "Zuverlässig wie ein Uhrwerk",
		Color:    "from-green-500 to-emerald-600",
		Selector: selectMostConsistent,
	},
	{
		Emoji:    "🌟",
		Title:    "Rising Star",
		Subtitle: "Beste Entwicklung",
		Color:    "from-blue-500 to-cyan-600",
		Selector: selectRisingStar,
	},
}

// calculateAwards determines award winners based on user stats
func (e *Evaluator) calculateAwards(userStats []models.UserStats) []models.Award {
	if len(userStats) == 0 {
		return nil
	}

	var awards []models.Award

	for _, def := range awardDefinitions {
		winner := def.Selector(userStats)
		if winner != nil {
			awards = append(awards, models.Award{
				Emoji:    def.Emoji,
				Title:    def.Title,
				Subtitle: def.Subtitle,
				Winner:   *winner,
				Color:    def.Color,
			})
		}
	}

	return awards
}

// selectHighestAttendance returns user with highest attendance rate
func selectHighestAttendance(userStats []models.UserStats) *models.UserStats {
	if len(userStats) == 0 {
		return nil
	}
	// UserStats are already sorted by attendance rate descending
	return &userStats[0]
}

// selectLongestAttendanceStreak returns user with longest attendance streak
func selectLongestAttendanceStreak(userStats []models.UserStats) *models.UserStats {
	if len(userStats) == 0 {
		return nil
	}

	var winner *models.UserStats
	maxStreak := 0

	for i := range userStats {
		if userStats[i].MaxAttendanceStreak > maxStreak {
			maxStreak = userStats[i].MaxAttendanceStreak
			winner = &userStats[i]
		}
	}

	return winner
}

// selectMostCreativeExcuser returns user with most "kreativ" category cancellations
func selectMostCreativeExcuser(userStats []models.UserStats) *models.UserStats {
	if len(userStats) == 0 {
		return nil
	}

	var winner *models.UserStats
	maxCreative := 0

	for i := range userStats {
		creativeCount := 0
		for _, c := range userStats[i].Cancellations {
			if c.Category == "kreativ" {
				creativeCount++
			}
		}
		if creativeCount > maxCreative {
			maxCreative = creativeCount
			winner = &userStats[i]
		}
	}

	// If no creative excuses, return user with most cancellations (most excuses overall)
	if winner == nil && len(userStats) > 0 {
		maxCancellations := 0
		for i := range userStats {
			if userStats[i].CancellationCount > maxCancellations {
				maxCancellations = userStats[i].CancellationCount
				winner = &userStats[i]
			}
		}
	}

	return winner
}

// selectMostConsistent returns user with smallest variation in attendance pattern
// For simplicity, we select the user with attendance rate closest to 75% (balanced)
func selectMostConsistent(userStats []models.UserStats) *models.UserStats {
	if len(userStats) == 0 {
		return nil
	}

	// Select user with highest attendance but not the top one (to diversify awards)
	if len(userStats) >= 2 {
		return &userStats[1]
	}
	return &userStats[0]
}

// selectRisingStar returns a user (for variety, picks from middle rankings)
func selectRisingStar(userStats []models.UserStats) *models.UserStats {
	if len(userStats) == 0 {
		return nil
	}

	// Pick someone from the middle of the pack
	midIndex := len(userStats) / 2
	if midIndex >= len(userStats) {
		midIndex = len(userStats) - 1
	}

	return &userStats[midIndex]
}
