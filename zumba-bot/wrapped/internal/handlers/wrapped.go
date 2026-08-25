package handlers

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/michael/stammtisch-wrapped/data"
	"github.com/michael/stammtisch-wrapped/internal/database"
	eval2026 "github.com/michael/stammtisch-wrapped/internal/evaluations/2026"
	"github.com/michael/zumba-shared/domain"

	"github.com/michael/stammtisch-wrapped/internal/repository"
	"github.com/michael/stammtisch-wrapped/internal/viewbuilder"
	year2026 "github.com/michael/stammtisch-wrapped/web/templates/years/2026"
	"github.com/michael/stammtisch-wrapped/web/templates/years/2026/viewmodels"
)

// cacheTTL bounds how long an evaluated page is served without hitting the
// database. The underlying data changes at most weekly, while every render
// runs the full evaluation pipeline — no need to redo that per request.
const cacheTTL = 15 * time.Minute

// seasonLabel2026 ist der Slug des Stammtischjahres in public.seasons. Der
// Zeitraum kommt aus der Tabelle – dieselbe Quelle wie für Bot und Admin-UI.
const seasonLabel2026 = "2026"

// fallbackSeason2026 greift nur, wenn public.seasons (noch) nicht gepflegt
// ist: 01.12.2025 – 30.11.2026, der bisher fest verdrahtete Zeitraum.
var fallbackSeason2026 = domain.Season{
	Label: seasonLabel2026,
	Period: domain.Period{
		Start: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 11, 30, 23, 59, 59, 0, time.UTC),
	},
}

// WrappedHandler handles requests for the Wrapped pages
type WrappedHandler struct {
	repo  *repository.RejectionRepository
	useDB bool

	mu       sync.Mutex
	cachedVM *viewmodels.PageViewModel
	cachedAt time.Time
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
	vm := h.viewModel(r.Context())

	// Render the templ component
	err := year2026.Page(vm).Render(r.Context(), w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// viewModel returns the page view model, served from a short-lived cache on
// the DB path. The mock path stays uncached (dev only, and it randomizes).
func (h *WrappedHandler) viewModel(ctx context.Context) viewmodels.PageViewModel {
	if !h.useDB {
		return viewbuilder.Build(h.loadFromMock(), "2026")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cachedVM != nil && time.Since(h.cachedAt) < cacheTTL {
		return *h.cachedVM
	}

	vm := viewbuilder.Build(h.loadFromDatabase(ctx), "2026")
	h.cachedVM = &vm
	h.cachedAt = time.Now()
	return vm
}

// loadFromDatabase loads data from PostgreSQL and evaluates it
func (h *WrappedHandler) loadFromDatabase(ctx context.Context) *viewbuilder.EvalData {
	season, err := h.repo.SeasonByLabel(ctx, seasonLabel2026)
	if err != nil {
		log.Printf("Stammtischjahr %s nicht gepflegt (%v), nutze festen Zeitraum", seasonLabel2026, err)
		season = fallbackSeason2026
	}

	rawData, err := h.repo.GetRawDataBySeason(ctx, season)
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
		ThursdayStats:          result.ThursdayStats,
		StrafenStats:           result.StrafenStats,
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
		ThursdayStats:          data.GetThursdayStats(),
		StrafenStats:           data.GetStrafenStats(),
		Awards:                 awards,
		Cancellations:          allCancellations,
	}
}
