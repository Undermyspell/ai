package eval2026

import (
	"sort"
	"time"

	"github.com/michael/stammtisch-wrapped/pkg/models"
)

// Award definitions (static selectors over user stats)
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

	// Comeback of the year: longest cancellation streak that ended with a return
	if winner := e.selectComeback(userStats); winner != nil {
		awards = append(awards, models.Award{
			Emoji:    "🦅",
			Title:    "Comeback des Jahres",
			Subtitle: "Lange weg – und wieder da",
			Color:    "from-green-500 to-emerald-600",
			Winner:   *winner,
		})
	}

	// Rising star: biggest attendance improvement second half vs. first half
	if winner := e.selectRisingStar(userStats); winner != nil {
		awards = append(awards, models.Award{
			Emoji:    "🌟",
			Title:    "Rising Star",
			Subtitle: "Beste Entwicklung im Jahresverlauf",
			Color:    "from-blue-500 to-cyan-600",
			Winner:   *winner,
		})
	}

	return awards
}

// selectComeback returns the user with the longest cancellation streak that
// ended before the last evaluated Thursday (i.e. they came back)
func (e *Evaluator) selectComeback(userStats []models.UserStats) *models.UserStats {
	if len(e.rawData.Thursdays) == 0 {
		return nil
	}
	lastThursday := e.rawData.Thursdays[len(e.rawData.Thursdays)-1]

	var winner *models.UserStats
	maxStreak := 0
	for i := range userStats {
		u := &userStats[i]
		// Streak must be substantial and finished (user attended afterwards)
		if u.MaxCancellationStreak >= 3 &&
			u.MaxCancellationStreak > maxStreak &&
			!u.MaxCancellationStreakEnd.IsZero() &&
			u.MaxCancellationStreakEnd.Before(lastThursday) {
			maxStreak = u.MaxCancellationStreak
			winner = u
		}
	}
	return winner
}

// selectRisingStar returns the user with the biggest rate improvement between
// the first and second half of the evaluated Thursdays
func (e *Evaluator) selectRisingStar(userStats []models.UserStats) *models.UserStats {
	thursdays := make([]time.Time, len(e.rawData.Thursdays))
	copy(thursdays, e.rawData.Thursdays)
	sort.Slice(thursdays, func(i, j int) bool { return thursdays[i].Before(thursdays[j]) })

	// Need enough data for two meaningful halves
	if len(thursdays) < 8 {
		return nil
	}
	half := len(thursdays) / 2
	firstHalf := make(map[string]bool, half)
	secondHalf := make(map[string]bool, len(thursdays)-half)
	for i, t := range thursdays {
		if i < half {
			firstHalf[t.Format("2006-01-02")] = true
		} else {
			secondHalf[t.Format("2006-01-02")] = true
		}
	}

	var winner *models.UserStats
	bestDelta := 0
	for i := range userStats {
		u := &userStats[i]
		miss1, miss2 := 0, 0
		for _, c := range u.Cancellations {
			key := c.Date.Format("2006-01-02")
			if firstHalf[key] {
				miss1++
			} else if secondHalf[key] {
				miss2++
			}
		}
		rate1 := ((half - miss1) * 100) / half
		rate2 := (((len(thursdays) - half) - miss2) * 100) / (len(thursdays) - half)
		delta := rate2 - rate1
		if delta > bestDelta {
			bestDelta = delta
			winner = u
		}
	}
	// Only award a real improvement
	if bestDelta < 10 {
		return nil
	}
	return winner
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

