package domain

import (
	"slices"
	"testing"
)

func TestCompetitionRanks(t *testing.T) {
	for _, c := range []struct {
		werte []int
		want  []int
	}{
		{[]int{9, 8, 8, 7}, []int{1, 2, 2, 4}},       // zwei Zweite → kein Dritter
		{[]int{9, 9, 9, 5, 4}, []int{1, 1, 1, 4, 5}}, // drei Erste
		{[]int{9, 8, 7, 7, 7, 6}, []int{1, 2, 3, 3, 3, 6}},
		{[]int{5}, []int{1}},
		{nil, []int{}},
	} {
		got := CompetitionRanks(len(c.werte), func(i int) bool { return c.werte[i] == c.werte[i-1] })
		if !slices.Equal(got, c.want) {
			t.Errorf("%v: %v, will %v", c.werte, got, c.want)
		}
	}
}
