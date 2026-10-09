package web

import (
	"slices"
	"testing"

	"github.com/michael/zumba-admin-ui/internal/store"
)

// Dashboard und Mitglied-Detail zählen wie die Bot-Statistik: nach zwei
// Zweiten geht es mit Platz 4 weiter.
func TestBoardRanksWieImSport(t *testing.T) {
	board := []store.LeaderboardRow{
		{UserName: "A", AttendanceCount: 10, AttendPercent: 100},
		{UserName: "B", AttendanceCount: 8, AttendPercent: 80},
		{UserName: "C", AttendanceCount: 8, AttendPercent: 80},
		{UserName: "D", AttendanceCount: 8, AttendPercent: 72.7}, // gleich oft da, aber später eingestiegen
		{UserName: "E", AttendanceCount: 6, AttendPercent: 60},
	}
	if got := boardRanks(board); !slices.Equal(got, []int{1, 2, 2, 4, 5}) {
		t.Errorf("boardRanks = %v, will [1 2 2 4 5]", got)
	}
}
