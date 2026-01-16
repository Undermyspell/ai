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
	"github.com/michael/stammtisch-wrapped/internal/viewbuilder"
	year2026 "github.com/michael/stammtisch-wrapped/web/templates/years/2026"
)

// Date range for 2026 Wrapped: 01.12.2025 - 30.11.2026
var dateRange2026 = repository.DateRange{
	Start: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
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
	var evalData *viewbuilder.EvalData

	if h.useDB {
		evalData = h.loadFromDatabase(r.Context())
	} else {
		evalData = h.loadFromMock()
	}

	// Build view model using the viewbuilder
	vm := viewbuilder.Build(evalData, "2026")

	// Render the templ component
	err := year2026.Page(vm).Render(r.Context(), w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// loadFromDatabase loads data from PostgreSQL and evaluates it
func (h *WrappedHandler) loadFromDatabase(ctx context.Context) *viewbuilder.EvalData {
	rawData, err := h.repo.GetRawDataByDateRange(ctx, dateRange2026)
	if err != nil {
		log.Printf("Error loading data from database: %v, falling back to mock data", err)
		return h.loadFromMock()
	}

	// Run evaluation
	evaluator := eval2026.NewEvaluator(rawData)
	result := evaluator.Evaluate()

	return &viewbuilder.EvalData{
		UserStats:              result.UserStats,
		GlobalStats:            result.GlobalStats,
		CategoryStats:          result.CategoryStats,
		MonthStats:             result.MonthStats,
		MonthlyAttendanceStats: result.MonthlyAttendanceStats,
		Awards:                 result.Awards,
		Cancellations:          result.Cancellations,
	}
}

// loadFromMock loads data from the mock generator (fallback)
func (h *WrappedHandler) loadFromMock() *viewbuilder.EvalData {
	userStats := data.CalculateUserStats()
	globalStats := data.GetGlobalStats()
	awards := data.GetAwards()
	categoryStats := data.GetCategoryStats()
	monthStats := data.GetMonthStats()
	allCancellations := data.GenerateCancellations()

	return &viewbuilder.EvalData{
		UserStats:              userStats,
		GlobalStats:            globalStats,
		CategoryStats:          categoryStats,
		MonthStats:             monthStats,
		MonthlyAttendanceStats: data.GetMonthlyAttendanceStats(),
		Awards:                 awards,
		Cancellations:          allCancellations,
	}
}
