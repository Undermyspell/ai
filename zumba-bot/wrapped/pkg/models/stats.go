package models

import "time"

// GlobalStats contains overall Stammtisch statistics
type GlobalStats struct {
	TotalThursdays        int `json:"totalThursdays"`
	TotalUsers            int `json:"totalUsers"`
	TotalCancellations    int `json:"totalCancellations"`
	TotalAttendances      int `json:"totalAttendances"`
	AverageAttendanceRate int `json:"averageAttendanceRate"`
}

// CategoryStats maps category names to their count
type CategoryStats map[string]int

// MonthStats maps month keys (e.g., "2025-01") to cancellation counts
type MonthStats map[string]int

// MonthlyAttendanceStats maps month keys (e.g., "2025-01") to average attendance rate (0-100)
type MonthlyAttendanceStats map[string]int

// ThursdayStat contains attendance numbers for a single Thursday
type ThursdayStat struct {
	Date      time.Time `json:"date"`
	Attendees int       `json:"attendees"`
	Total     int       `json:"total"` // active users on that Thursday
	Rate      int       `json:"rate"`  // 0-100
}

// StrafenEntry is a single penalty attributed to a user
type StrafenEntry struct {
	UserName string    `json:"userName"`
	Art      string    `json:"art"` // "fehltage" | "noshow"
	Betrag   int       `json:"betrag"`
	Tage     int       `json:"tage"` // fehltage only: length of the streak
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"` // fehltage only: last Thursday of the streak
	Status   string    `json:"status"`
}

// StrafenUserTotal aggregates all penalties of one user
type StrafenUserTotal struct {
	UserName      string         `json:"userName"`
	Total         int            `json:"total"`
	FehltageCount int            `json:"fehltageCount"`
	NoShowCount   int            `json:"noShowCount"`
	Entries       []StrafenEntry `json:"entries"`
}

// StrafenStats contains the penalty evaluation for the wrapped period
type StrafenStats struct {
	TotalSum   int                `json:"totalSum"`
	TotalCount int                `json:"totalCount"`
	UserTotals []StrafenUserTotal `json:"userTotals"` // sorted by Total descending
}

// Award represents a special recognition
type Award struct {
	Emoji    string    `json:"emoji"`
	Title    string    `json:"title"`
	Subtitle string    `json:"subtitle"`
	Winner   UserStats `json:"winner"`
	Color    string    `json:"color"`
}
