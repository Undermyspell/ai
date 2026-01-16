package models

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

// Award represents a special recognition
type Award struct {
	Emoji    string    `json:"emoji"`
	Title    string    `json:"title"`
	Subtitle string    `json:"subtitle"`
	Winner   UserStats `json:"winner"`
	Color    string    `json:"color"`
}
