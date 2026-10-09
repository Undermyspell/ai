package eval2026

import (
	"sort"
	"time"

	"github.com/michael/stammtisch-wrapped/pkg/models"
)

// calculateAwards determines award winners based on user stats
func (e *Evaluator) calculateAwards(userStats []models.UserStats, strafen models.StrafenStats) []models.Award {
	return CalculateAwards(userStats, e.rawData.Thursdays, strafen)
}

// CalculateAwards vergibt Ehren- und Schmähpreise. Reine Funktion über die
// User-Statistiken, damit Mock und echte Auswertung dieselben Regeln nutzen.
// Gleichstand teilt sich den Award – wie bei den Plätzen (1-2-2-4).
func CalculateAwards(users []models.UserStats, thursdays []time.Time, strafen models.StrafenStats) []models.Award {
	if len(users) == 0 {
		return nil
	}
	sorted := make([]time.Time, len(thursdays))
	copy(sorted, thursdays)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })

	var awards []models.Award
	add := func(a models.Award, winners []models.UserStats, value int) {
		if len(winners) == 0 {
			return
		}
		a.Winners, a.Value = winners, value
		awards = append(awards, a)
	}

	// ── Ehrenpreise ──
	var kings []models.UserStats
	for _, u := range users {
		if u.Rank == 1 {
			kings = append(kings, u)
		}
	}
	if len(kings) > 0 {
		add(models.Award{ID: "koenig", Emoji: "👑", Title: "Stammtisch-König", Subtitle: "Höchste Anwesenheitsquote", Tone: "amber"},
			kings, kings[0].AttendanceRate)
	}

	w, v := best(users, 1, func(u models.UserStats) (int, bool) { return u.MaxAttendanceStreak, true })
	add(models.Award{ID: "streak", Emoji: "🔥", Title: "Streak-Meister", Subtitle: "Längste Anwesenheitsserie", Tone: "red"}, w, v)

	w, v = best(users, 1, func(u models.UserStats) (int, bool) { return countCategory(u, "kreativ"), true })
	add(models.Award{ID: "kreativ", Emoji: "🎨", Title: "Kreativster Absager", Subtitle: "Die besten Ausreden", Tone: "blue"}, w, v)
	if len(w) == 0 {
		// Ohne kreative Ausreden geht der Preis an die meisten Absagen –
		// dann ehrlich als Schmähpreis.
		w, v = best(users, 1, func(u models.UserStats) (int, bool) { return u.CancellationCount, true })
		add(models.Award{ID: "absagen", Emoji: "📵", Title: "Absage-Weltmeister", Subtitle: "Die meisten Absagen", Tone: "neutral", Shame: true}, w, v)
	}

	// Phantom vorab: wer den Schmähpreis für die längste Absage-Serie
	// bekommt, ist für dieselbe Serie nicht auch noch das Comeback.
	phantoms, phantomLen := best(users, 3, func(u models.UserStats) (int, bool) { return u.MaxCancellationStreak, true })
	isPhantom := make(map[int]bool, len(phantoms))
	for _, u := range phantoms {
		isPhantom[u.ID] = true
	}

	if len(sorted) > 0 {
		last := sorted[len(sorted)-1]
		w, v = best(users, 3, func(u models.UserStats) (int, bool) {
			returned := !u.MaxCancellationStreakEnd.IsZero() && u.MaxCancellationStreakEnd.Before(last)
			return u.MaxCancellationStreak, returned && !isPhantom[u.ID]
		})
		add(models.Award{ID: "comeback", Emoji: "🦅", Title: "Comeback des Jahres", Subtitle: "Lange weg – und wieder da", Tone: "green"}, w, v)
	}

	deltas := halfYearDeltas(users, sorted)
	w, v = best(users, 10, func(u models.UserStats) (int, bool) { d, ok := deltas[u.ID]; return d, ok })
	add(models.Award{ID: "rising", Emoji: "🌟", Title: "Rising Star", Subtitle: "Beste Entwicklung im Jahresverlauf", Tone: "cyan"}, w, v)

	// ── Schmähpreise ──
	add(models.Award{ID: "phantom", Emoji: "👻", Title: "Phantom des Jahres", Subtitle: "Längste Absage-Serie", Tone: "neutral", Shame: true},
		phantoms, phantomLen)

	w, v = strafenChamps(users, strafen)
	add(models.Award{ID: "strafen", Emoji: "💸", Title: "Strafen-Champ", Subtitle: "Bester Kunde der Kasse", Tone: "red", Shame: true}, w, v)

	w, v = best(users, 10, func(u models.UserStats) (int, bool) { d, ok := deltas[u.ID]; return -d, ok })
	add(models.Award{ID: "absturz", Emoji: "📉", Title: "Absturz des Jahres", Subtitle: "Größter Einbruch im zweiten Halbjahr", Tone: "amber", Shame: true}, w, v)

	runs := absenceRuns(users, sorted)
	w, v = best(users, 3, func(u models.UserStats) (int, bool) { return runs[u.ID], true })
	add(models.Award{ID: "wackel", Emoji: "🎢", Title: "Wackelkandidat", Subtitle: "Mal da, mal weg – nie berechenbar", Tone: "blue", Shame: true}, w, v)

	return awards
}

// best liefert alle User mit dem höchsten Wert (Gleichstand teilt sich den
// Award) und den Wert selbst. Wer ok=false liefert, ist nicht im Rennen;
// unter min gibt es keinen Gewinner.
func best(users []models.UserStats, min int, value func(models.UserStats) (int, bool)) ([]models.UserStats, int) {
	var winners []models.UserStats
	top := min - 1
	for _, u := range users {
		v, ok := value(u)
		if !ok || v < min {
			continue
		}
		switch {
		case v > top:
			top = v
			winners = []models.UserStats{u}
		case v == top:
			winners = append(winners, u)
		}
	}
	return winners, top
}

// countCategory zählt die Absagen eines Users in einer Ausreden-Kategorie.
func countCategory(u models.UserStats, category string) int {
	n := 0
	for _, c := range u.Cancellations {
		if c.Category == category {
			n++
		}
	}
	return n
}

// minHalfThursdays ist die Mindestzahl gezählter Donnerstage je Halbjahr –
// darunter ist eine Quote zu wackelig für einen Vergleich.
const minHalfThursdays = 4

// halfYearDeltas liefert je User die Änderung der Quote vom ersten zum
// zweiten Halbjahr in Prozentpunkten. Gezählt werden nur Donnerstage ab dem
// Eintritt – sonst sähe ein Späteinsteiger im ersten Halbjahr wie 100 % aus.
func halfYearDeltas(users []models.UserStats, thursdays []time.Time) map[int]int {
	if len(thursdays) < 2*minHalfThursdays {
		return nil
	}
	half := len(thursdays) / 2
	deltas := make(map[int]int, len(users))
	for _, u := range users {
		absent := make(map[string]bool, len(u.Cancellations))
		for _, c := range u.Cancellations {
			absent[c.Date.Format(time.DateOnly)] = true
		}
		var n, miss [2]int
		for i, t := range thursdays {
			if t.Before(u.Since) {
				continue
			}
			h := 0
			if i >= half {
				h = 1
			}
			n[h]++
			if absent[t.Format(time.DateOnly)] {
				miss[h]++
			}
		}
		if n[0] < minHalfThursdays || n[1] < minHalfThursdays {
			continue
		}
		rate := func(h int) int { return (n[h] - miss[h]) * 100 / n[h] }
		deltas[u.ID] = rate(1) - rate(0)
	}
	return deltas
}

// absenceRuns zählt je User, wie oft er abgetaucht ist: zusammenhängende
// Absage-Blöcke über die gezählten Donnerstage.
func absenceRuns(users []models.UserStats, thursdays []time.Time) map[int]int {
	index := make(map[string]int, len(thursdays))
	for i, t := range thursdays {
		index[t.Format(time.DateOnly)] = i
	}
	runs := make(map[int]int, len(users))
	for _, u := range users {
		absent := make(map[int]bool, len(u.Cancellations))
		for _, c := range u.Cancellations {
			if i, ok := index[c.Date.Format(time.DateOnly)]; ok {
				absent[i] = true
			}
		}
		for i := range absent {
			if !absent[i-1] {
				runs[u.ID]++
			}
		}
	}
	return runs
}

// strafenChamps liefert die User mit der höchsten Strafensumme (in Euro).
// Die Strafen kennen nur den Namen – darüber kommt der User mit Emoji dazu.
func strafenChamps(users []models.UserStats, strafen models.StrafenStats) ([]models.UserStats, int) {
	byName := make(map[string]models.UserStats, len(users))
	for _, u := range users {
		byName[u.Name] = u
	}
	var winners []models.UserStats
	top := 0
	for _, t := range strafen.UserTotals {
		if t.Total <= 0 || t.Total < top {
			continue
		}
		u, ok := byName[t.UserName]
		if !ok {
			u = models.UserStats{User: models.User{Name: t.UserName}}
		}
		if t.Total > top {
			top = t.Total
			winners = nil
		}
		winners = append(winners, u)
	}
	return winners, top
}
