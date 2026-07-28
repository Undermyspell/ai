package repository

import "time"

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

// RawData contains all raw data needed for evaluations
type RawData struct {
	Users        []RawUser
	Rejections   []RawRejection
	ExcludedDays []ExcludedDay
	Thursdays    []time.Time // All valid Thursdays for the year (excluding excluded_days)
}
