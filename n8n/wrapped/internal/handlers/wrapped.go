package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/michael/stammtisch-wrapped/data"
	"github.com/michael/stammtisch-wrapped/pkg/models"
	"github.com/michael/stammtisch-wrapped/web/templates"
	year2025 "github.com/michael/stammtisch-wrapped/web/templates/years/2025"
)

// HandleIndex redirects to the current year
func HandleIndex(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/2025", http.StatusTemporaryRedirect)
}

// Handle2025 renders the 2025 Wrapped page
func Handle2025(w http.ResponseWriter, r *http.Request) {
	// Load data from mock generator
	userStats := data.CalculateUserStats()
	globalStats := data.GetGlobalStats()
	awards := data.GetAwards()
	categoryStats := data.GetCategoryStats()
	monthStats := data.GetMonthStats()
	allCancellations := data.GenerateCancellations()

	// Get category labels for frontend
	categoryLabels := make(map[string]map[string]string)
	allCategories := models.GetAllExcuseCategories()
	for key, cat := range allCategories {
		categoryLabels[key] = map[string]string{
			"label": cat.Label,
			"emoji": cat.Emoji,
		}
	}

	// Prepare template data - using any type for JSON serialization by templ
	pageData := templates.PageData{
		UserStatsJSON:      userStats,
		GlobalStatsJSON:    globalStats,
		AwardsJSON:         awards,
		CategoryStatsJSON:  categoryStats,
		CategoryLabelsJSON: categoryLabels,
		MonthStatsJSON:     monthStats,
		CancellationsJSON:  allCancellations,
		Year:               "2025",
	}

	// Render the templ component
	err := year2025.Page(pageData).Render(r.Context(), w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Helper function to serialize data to JSON (kept for reference)
func mustMarshal(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}
