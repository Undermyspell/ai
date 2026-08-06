package repository

import (
	"time"

	sharedstore "github.com/michael/zumba-shared/store"
)

// RawRejection represents a row from stammtisch_abwesenheit table
type RawRejection struct {
	UserID  string
	Date    time.Time
	Message *string // nullable - can be nil if no message provided
}

// RawUser represents a row from users table
type RawUser struct {
	UserID    string
	UserName  string
	StartDate *time.Time // nullable - user might not have a start date
}

// ExcludedDay represents a row from excluded_days table
type ExcludedDay struct {
	Date time.Time
}

// StrafenRow represents a row from the strafen table (created by whatsapp-bot/admin-ui)
type StrafenRow struct {
	ID          int64
	UserID      string
	Art         string // "fehltage" | "noshow"
	Datum       time.Time
	Betrag      *int // nullable; only set for noshow
	Status      string
	BeglichenAm *time.Time
	GeloeschtAm *time.Time
}

// MaxStreak ist die längste Serie eines Users in einem Zustand
// (Absent=false: Anwesenheit, Absent=true: Absagen) — aus max_streaks.sql.
type MaxStreak struct {
	UserID string
	Absent bool
	Len    int
	Start  time.Time
	End    time.Time
}

// ThursdayAttendance ist die SQL-berechnete Anwesenheit eines Donnerstags
// (aus thursday_stats.sql; Attendees = Aktive - Abgemeldete).
type ThursdayAttendance struct {
	Day       time.Time
	Active    int
	Attendees int
}

// RawData contains all raw data needed for evaluations
type RawData struct {
	Users        []RawUser
	Rejections   []RawRejection
	ExcludedDays []ExcludedDay
	Thursdays    []time.Time  // All valid Thursdays for the year (excluding excluded_days)
	StrafenRows  []StrafenRow // All penalty rows (incl. beglichen/geloescht — needed as reset markers)

	// In SQL vorberechnete Auswertungen (gleiche Snapshot-Transaktion):
	Leaderboard   []sharedstore.LeaderboardRow // geteilte Rangliste-Query (shared/store)
	MaxStreaks    []MaxStreak                  // längste Serien je User
	ThursdayStats []ThursdayAttendance         // Anwesenheit je Donnerstag
}
