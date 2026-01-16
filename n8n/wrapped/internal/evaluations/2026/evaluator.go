package eval2026

import (
	"github.com/michael/stammtisch-wrapped/internal/repository"
	"github.com/michael/stammtisch-wrapped/pkg/models"
)

// Evaluator orchestrates all 2025 evaluations
type Evaluator struct {
	rawData *repository.RawData
}

// NewEvaluator creates a new Evaluator with raw data
func NewEvaluator(rawData *repository.RawData) *Evaluator {
	return &Evaluator{rawData: rawData}
}

// EvaluationResult contains all computed statistics for 2025
type EvaluationResult struct {
	UserStats     []models.UserStats
	GlobalStats   models.GlobalStats
	CategoryStats models.CategoryStats
	MonthStats    models.MonthStats
	Awards        []models.Award
	Cancellations []models.Cancellation
}

// Evaluate computes all statistics from raw data
func (e *Evaluator) Evaluate() *EvaluationResult {
	// Step 1: Build user lookup map (userId -> index)
	userLookup := e.buildUserLookup()

	// Step 2: Classify all rejections into cancellations with categories
	cancellations := e.classifyCancellations(userLookup)

	// Step 3: Calculate per-user statistics
	userStats := e.calculateUserStats(userLookup, cancellations)

	// Step 4: Calculate global statistics
	globalStats := e.calculateGlobalStats(userStats)

	// Step 5: Calculate category statistics
	categoryStats := e.calculateCategoryStats(cancellations)

	// Step 6: Calculate monthly statistics
	monthStats := e.calculateMonthStats(cancellations)

	// Step 7: Determine awards
	awards := e.calculateAwards(userStats)

	return &EvaluationResult{
		UserStats:     userStats,
		GlobalStats:   globalStats,
		CategoryStats: categoryStats,
		MonthStats:    monthStats,
		Awards:        awards,
		Cancellations: cancellations,
	}
}

// buildUserLookup creates a map from userId string to sequential index
func (e *Evaluator) buildUserLookup() map[string]int {
	lookup := make(map[string]int)
	for i, user := range e.rawData.Users {
		lookup[user.UserID] = i
	}
	return lookup
}
