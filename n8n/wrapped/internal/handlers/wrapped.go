package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/michael/stammtisch-wrapped/data"
	"github.com/michael/stammtisch-wrapped/pkg/models"
)

type PageData struct {
	UserStatsJSON      template.JS
	GlobalStatsJSON    template.JS
	AwardsJSON         template.JS
	CategoryStatsJSON  template.JS
	MonthStatsJSON     template.JS
	CancellationsJSON  template.JS
	CategoryLabelsJSON template.JS
}

func HandleIndex(w http.ResponseWriter, r *http.Request) {
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

	// Serialize data to JSON for JavaScript
	userStatsJSON, err := json.Marshal(userStats)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	globalStatsJSON, err := json.Marshal(globalStats)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	awardsJSON, err := json.Marshal(awards)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	categoryStatsJSON, err := json.Marshal(categoryStats)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	monthStatsJSON, err := json.Marshal(monthStats)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cancellationsJSON, err := json.Marshal(allCancellations)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	categoryLabelsJSON, err := json.Marshal(categoryLabels)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Prepare template data
	pageData := PageData{
		UserStatsJSON:      template.JS(userStatsJSON),
		GlobalStatsJSON:    template.JS(globalStatsJSON),
		AwardsJSON:         template.JS(awardsJSON),
		CategoryStatsJSON:  template.JS(categoryStatsJSON),
		MonthStatsJSON:     template.JS(monthStatsJSON),
		CancellationsJSON:  template.JS(cancellationsJSON),
		CategoryLabelsJSON: template.JS(categoryLabelsJSON),
	}

	// Parse and execute template
	tmpl, err := template.ParseFiles("web/templates/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, pageData); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
