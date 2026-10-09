package domain

// CompetitionRanks vergibt Plätze wie im Sport ("1-2-2-4"): Gleichplatzierte
// teilen sich einen Platz, die folgenden Plätze entfallen – nach zwei Zweiten
// geht es mit Platz 4 weiter, einen Dritten gibt es dann nicht.
//
// n ist die Anzahl der bereits sortierten Einträge, tie(i) meldet, ob Eintrag
// i mit seinem Vorgänger i-1 gleichauf liegt (wird für i ≥ 1 gefragt).
func CompetitionRanks(n int, tie func(i int) bool) []int {
	ranks := make([]int, n)
	for i := range ranks {
		if i > 0 && tie(i) {
			ranks[i] = ranks[i-1]
		} else {
			ranks[i] = i + 1
		}
	}
	return ranks
}
