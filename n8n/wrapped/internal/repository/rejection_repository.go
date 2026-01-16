package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/michael/stammtisch-wrapped/internal/database"
)

// DateRange represents a time span for evaluations
type DateRange struct {
	Start time.Time
	End   time.Time
}

// EffectiveEnd returns the end date capped at today's date
// This ensures we don't consider future Thursdays in evaluations
func (d DateRange) EffectiveEnd() time.Time {
	today := time.Now().Truncate(24 * time.Hour)
	if d.End.After(today) {
		return today
	}
	return d.End
}

// RejectionRepository handles data access for rejection/absence data
type RejectionRepository struct {
	db *database.PostgresDB
}

// NewRejectionRepository creates a new RejectionRepository
func NewRejectionRepository(db *database.PostgresDB) *RejectionRepository {
	return &RejectionRepository{db: db}
}

// GetAllUsers fetches all users from the database
func (r *RejectionRepository) GetAllUsers(ctx context.Context) ([]RawUser, error) {
	query := `
		SELECT "userId", "userName", "startDate"
		FROM users
		ORDER BY "userName"
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []RawUser
	for rows.Next() {
		var user RawUser
		if err := rows.Scan(&user.UserID, &user.UserName, &user.StartDate); err != nil {
			return nil, fmt.Errorf("failed to scan user row: %w", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user rows: %w", err)
	}

	return users, nil
}

// GetRejectionsByDateRange fetches all rejections within a date range (only Thursdays, excluding excluded_days)
// The end date is capped at today to exclude future dates
func (r *RejectionRepository) GetRejectionsByDateRange(ctx context.Context, dateRange DateRange) ([]RawRejection, error) {
	effectiveEnd := dateRange.EffectiveEnd()
	query := `
		SELECT "userId", date, message
		FROM stammtisch_abwesenheit
		WHERE date >= $1 AND date <= $2
		  AND EXTRACT(DOW FROM date) = 4
		  AND date NOT IN (SELECT date FROM excluded_days)
		ORDER BY date, "userId"
	`

	rows, err := r.db.Query(ctx, query, dateRange.Start, effectiveEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to query rejections: %w", err)
	}
	defer rows.Close()

	var rejections []RawRejection
	for rows.Next() {
		var rejection RawRejection
		if err := rows.Scan(&rejection.UserID, &rejection.Date, &rejection.Message); err != nil {
			return nil, fmt.Errorf("failed to scan rejection row: %w", err)
		}
		rejections = append(rejections, rejection)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rejection rows: %w", err)
	}

	return rejections, nil
}

// GetExcludedDaysByDateRange fetches all excluded days within a date range
// The end date is capped at today to exclude future dates
func (r *RejectionRepository) GetExcludedDaysByDateRange(ctx context.Context, dateRange DateRange) ([]ExcludedDay, error) {
	effectiveEnd := dateRange.EffectiveEnd()
	query := `
		SELECT date
		FROM excluded_days
		WHERE date >= $1 AND date <= $2
		ORDER BY date
	`

	rows, err := r.db.Query(ctx, query, dateRange.Start, effectiveEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to query excluded days: %w", err)
	}
	defer rows.Close()

	var excludedDays []ExcludedDay
	for rows.Next() {
		var day ExcludedDay
		if err := rows.Scan(&day.Date); err != nil {
			return nil, fmt.Errorf("failed to scan excluded day row: %w", err)
		}
		excludedDays = append(excludedDays, day)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating excluded day rows: %w", err)
	}

	return excludedDays, nil
}

// GetThursdaysByDateRange returns all Thursdays within a date range, excluding excluded_days
// The end date is capped at today to exclude future Thursdays
func (r *RejectionRepository) GetThursdaysByDateRange(ctx context.Context, dateRange DateRange) ([]time.Time, error) {
	effectiveEnd := dateRange.EffectiveEnd()
	query := `
		WITH all_thursdays AS (
			SELECT d::date AS thursday
			FROM generate_series($1::date, $2::date, interval '1 day') AS d
			WHERE EXTRACT(DOW FROM d) = 4
		)
		SELECT thursday
		FROM all_thursdays
		WHERE thursday NOT IN (SELECT date FROM excluded_days WHERE date >= $1 AND date <= $2)
		ORDER BY thursday
	`

	rows, err := r.db.Query(ctx, query, dateRange.Start, effectiveEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to query thursdays: %w", err)
	}
	defer rows.Close()

	var thursdays []time.Time
	for rows.Next() {
		var thursday time.Time
		if err := rows.Scan(&thursday); err != nil {
			return nil, fmt.Errorf("failed to scan thursday row: %w", err)
		}
		thursdays = append(thursdays, thursday)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating thursday rows: %w", err)
	}

	return thursdays, nil
}

// GetRawDataByDateRange fetches all raw data needed for evaluations within a date range
func (r *RejectionRepository) GetRawDataByDateRange(ctx context.Context, dateRange DateRange) (*RawData, error) {
	users, err := r.GetAllUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	rejections, err := r.GetRejectionsByDateRange(ctx, dateRange)
	if err != nil {
		return nil, fmt.Errorf("failed to get rejections: %w", err)
	}

	excludedDays, err := r.GetExcludedDaysByDateRange(ctx, dateRange)
	if err != nil {
		return nil, fmt.Errorf("failed to get excluded days: %w", err)
	}

	thursdays, err := r.GetThursdaysByDateRange(ctx, dateRange)
	if err != nil {
		return nil, fmt.Errorf("failed to get thursdays: %w", err)
	}

	return &RawData{
		Users:        users,
		Rejections:   rejections,
		ExcludedDays: excludedDays,
		Thursdays:    thursdays,
	}, nil
}
