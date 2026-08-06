package repository

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/lib/pq"

	"github.com/michael/zumba-shared/domain"
	sharedstore "github.com/michael/zumba-shared/store"

	"github.com/michael/stammtisch-wrapped/internal/database"
)

//go:embed queries/max_streaks.sql
var maxStreaksQ string

//go:embed queries/thursday_stats.sql
var thursdayStatsQ string

// DateRange ist der Auswertungszeitraum – geteilter Typ aus dem shared-Modul
// (EffectiveEnd kappt das Ende am heutigen Tag).
type DateRange = domain.Period

// queryer is satisfied by both *sql.DB and *sql.Tx so the fetch helpers can
// run inside one read-only transaction.
type queryer interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
}

// RejectionRepository handles data access for rejection/absence data
type RejectionRepository struct {
	db *database.PostgresDB
}

// NewRejectionRepository creates a new RejectionRepository
func NewRejectionRepository(db *database.PostgresDB) *RejectionRepository {
	return &RejectionRepository{db: db}
}

func getAllUsers(ctx context.Context, q queryer) ([]RawUser, error) {
	query := `
		SELECT "userId", "userName", "startDate"
		FROM users
		ORDER BY "userName"
	`

	rows, err := q.QueryContext(ctx, query)
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

// getRejections fetches all rejections within the date range (only Thursdays,
// excluding excluded_days).
func getRejections(ctx context.Context, q queryer, start, end time.Time) ([]RawRejection, error) {
	query := `
		SELECT "userId", date, message
		FROM stammtisch_abwesenheit
		WHERE date >= $1 AND date <= $2
		  AND EXTRACT(DOW FROM date) = 4
		  AND date NOT IN (SELECT date FROM excluded_days)
		ORDER BY date, "userId"
	`

	rows, err := q.QueryContext(ctx, query, start, end)
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

func getExcludedDays(ctx context.Context, q queryer, start, end time.Time) ([]ExcludedDay, error) {
	query := `
		SELECT date
		FROM excluded_days
		WHERE date >= $1 AND date <= $2
		ORDER BY date
	`

	rows, err := q.QueryContext(ctx, query, start, end)
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

// getThursdays returns all Thursdays within the date range, excluding excluded_days.
func getThursdays(ctx context.Context, q queryer, start, end time.Time) ([]time.Time, error) {
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

	rows, err := q.QueryContext(ctx, query, start, end)
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

// getStrafenRows fetches penalty rows up to the end date. Deleted and settled
// rows are included because they act as streak reset markers for the fehltage
// computation; rows after the evaluation window are irrelevant and filtered
// in SQL. If the strafen table does not exist yet (bot/admin-ui create it on
// startup), an empty slice is returned instead of an error.
func getStrafenRows(ctx context.Context, q queryer, end time.Time) ([]StrafenRow, error) {
	query := `
		SELECT id, "userId", art, datum, betrag, status, beglichen_am, geloescht_am
		FROM strafen
		WHERE datum <= $1
		ORDER BY "userId", datum
	`

	rows, err := q.QueryContext(ctx, query, end)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "42P01" { // undefined_table
			log.Printf("⚠️ strafen table does not exist yet, skipping penalty stats")
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query strafen: %w", err)
	}
	defer rows.Close()

	var strafen []StrafenRow
	for rows.Next() {
		var s StrafenRow
		if err := rows.Scan(&s.ID, &s.UserID, &s.Art, &s.Datum, &s.Betrag, &s.Status, &s.BeglichenAm, &s.GeloeschtAm); err != nil {
			return nil, fmt.Errorf("failed to scan strafen row: %w", err)
		}
		strafen = append(strafen, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating strafen rows: %w", err)
	}

	return strafen, nil
}

// getMaxStreaks liefert die längsten Serien je User (max_streaks.sql).
func getMaxStreaks(ctx context.Context, q queryer, start, end time.Time) ([]MaxStreak, error) {
	rows, err := q.QueryContext(ctx, maxStreaksQ, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query max streaks: %w", err)
	}
	defer rows.Close()

	var out []MaxStreak
	for rows.Next() {
		var s MaxStreak
		if err := rows.Scan(&s.UserID, &s.Absent, &s.Len, &s.Start, &s.End); err != nil {
			return nil, fmt.Errorf("failed to scan max streak row: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// getThursdayStats liefert die Anwesenheit je Donnerstag (thursday_stats.sql).
func getThursdayStats(ctx context.Context, q queryer, start, end time.Time) ([]ThursdayAttendance, error) {
	rows, err := q.QueryContext(ctx, thursdayStatsQ, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query thursday stats: %w", err)
	}
	defer rows.Close()

	var out []ThursdayAttendance
	for rows.Next() {
		var t ThursdayAttendance
		if err := rows.Scan(&t.Day, &t.Active, &t.Attendees); err != nil {
			return nil, fmt.Errorf("failed to scan thursday stat row: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetRawDataByDateRange fetches all raw data needed for evaluations within a
// date range. All fetches run in one read-only repeatable-read transaction so
// the evaluation sees a consistent snapshot.
func (r *RejectionRepository) GetRawDataByDateRange(ctx context.Context, dateRange DateRange) (*RawData, error) {
	effectiveEnd := dateRange.EffectiveEnd()

	tx, err := r.db.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	users, err := getAllUsers(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	rejections, err := getRejections(ctx, tx, dateRange.Start, effectiveEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get rejections: %w", err)
	}

	excludedDays, err := getExcludedDays(ctx, tx, dateRange.Start, effectiveEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get excluded days: %w", err)
	}

	thursdays, err := getThursdays(ctx, tx, dateRange.Start, effectiveEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get thursdays: %w", err)
	}

	strafenRows, err := getStrafenRows(ctx, tx, effectiveEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get strafen rows: %w", err)
	}

	leaderboard, err := sharedstore.Leaderboard(ctx, tx, domain.Period{Start: dateRange.Start, End: dateRange.End})
	if err != nil {
		return nil, fmt.Errorf("failed to get leaderboard: %w", err)
	}

	maxStreaks, err := getMaxStreaks(ctx, tx, dateRange.Start, effectiveEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get max streaks: %w", err)
	}

	thursdayStats, err := getThursdayStats(ctx, tx, dateRange.Start, effectiveEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get thursday stats: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit read transaction: %w", err)
	}

	return &RawData{
		Users:         users,
		Rejections:    rejections,
		ExcludedDays:  excludedDays,
		Thursdays:     thursdays,
		StrafenRows:   strafenRows,
		Leaderboard:   leaderboard,
		MaxStreaks:    maxStreaks,
		ThursdayStats: thursdayStats,
	}, nil
}
