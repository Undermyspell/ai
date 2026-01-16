package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/michael/stammtisch-wrapped/data"
	"github.com/michael/stammtisch-wrapped/internal/database"
	eval2026 "github.com/michael/stammtisch-wrapped/internal/evaluations/2026"
	"github.com/michael/stammtisch-wrapped/internal/repository"
	"github.com/michael/stammtisch-wrapped/pkg/models"
	"github.com/michael/stammtisch-wrapped/web/templates"
	year2026 "github.com/michael/stammtisch-wrapped/web/templates/years/2026"
)

// Date range for 2026 Wrapped: 01.01.2025 - 30.11.2026
var dateRange2026 = repository.DateRange{
	Start: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	End:   time.Date(2026, 11, 30, 23, 59, 59, 0, time.UTC),
}

// WrappedHandler handles requests for the Wrapped pages
type WrappedHandler struct {
	repo  *repository.RejectionRepository
	useDB bool
}

// NewWrappedHandler creates a new handler with optional database connection
func NewWrappedHandler(db *database.PostgresDB) *WrappedHandler {
	if db == nil {
		return &WrappedHandler{useDB: false}
	}
	return &WrappedHandler{
		repo:  repository.NewRejectionRepository(db),
		useDB: true,
	}
}

// HandleIndex redirects to the current year
func (h *WrappedHandler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/2026", http.StatusTemporaryRedirect)
}

// Handle2026 renders the 2026 Wrapped page
func (h *WrappedHandler) Handle2026(w http.ResponseWriter, r *http.Request) {
	var pageData templates.PageData

	if h.useDB {
		pageData = h.loadFromDatabase(r.Context())
	} else {
		pageData = h.loadFromMock()
	}

	// Render the templ component
	err := year2026.Page(pageData).Render(r.Context(), w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// loadFromDatabase loads data from PostgreSQL and evaluates it
func (h *WrappedHandler) loadFromDatabase(ctx context.Context) templates.PageData {
	rawData, err := h.repo.GetRawDataByDateRange(ctx, dateRange2026)
	if err != nil {
		log.Printf("Error loading data from database: %v, falling back to mock data", err)
		return h.loadFromMock()
	}

	// Run evaluation
	evaluator := eval2026.NewEvaluator(rawData)
	result := evaluator.Evaluate()

	// Prepare category labels for frontend
	categoryLabels := buildCategoryLabels()

	return templates.PageData{
		UserStatsJSON:      result.UserStats,
		GlobalStatsJSON:    result.GlobalStats,
		AwardsJSON:         result.Awards,
		CategoryStatsJSON:  result.CategoryStats,
		CategoryLabelsJSON: categoryLabels,
		MonthStatsJSON:     result.MonthStats,
		CancellationsJSON:  result.Cancellations,
		Year:               "2026",
	}
}

// loadFromMock loads data from the mock generator (fallback)
func (h *WrappedHandler) loadFromMock() templates.PageData {
	userStats := data.CalculateUserStats()
	globalStats := data.GetGlobalStats()
	awards := data.GetAwards()
	categoryStats := data.GetCategoryStats()
	monthStats := data.GetMonthStats()
	allCancellations := data.GenerateCancellations()

	categoryLabels := buildCategoryLabels()

	return templates.PageData{
		UserStatsJSON:      userStats,
		GlobalStatsJSON:    globalStats,
		AwardsJSON:         awards,
		CategoryStatsJSON:  categoryStats,
		CategoryLabelsJSON: categoryLabels,
		MonthStatsJSON:     monthStats,
		CancellationsJSON:  allCancellations,
		Year:               "2026",
	}
}

// buildCategoryLabels creates category label map for frontend
func buildCategoryLabels() map[string]map[string]string {
	categoryLabels := make(map[string]map[string]string)
	allCategories := models.GetAllExcuseCategories()
	for key, cat := range allCategories {
		categoryLabels[key] = map[string]string{
			"label": cat.Label,
			"emoji": cat.Emoji,
		}
	}
	return categoryLabels
}
