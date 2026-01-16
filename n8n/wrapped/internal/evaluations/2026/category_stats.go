package eval2026

import (
	"strings"

	"github.com/michael/stammtisch-wrapped/pkg/models"
)

// categoryKeywords maps categories to their detection keywords (case-insensitive)
var categoryKeywords = map[string][]string{
	"arbeit": {
		"arbeit", "arbeiten", "job", "meeting", "büro", "buero", "office",
		"projekt", "deadline", "chef", "kunde", "kunden", "firma",
		"überstunden", "ueberstunden", "dienst", "geschäft", "termin",
		"beruflich", "kollege", "kollegin",
	},
	"familie": {
		"familie", "familien", "kind", "kinder", "eltern", "frau", "mann",
		"schwiegermutter", "schwiegervater", "hochzeit", "geburtstag",
		"verwandte", "oma", "opa", "tante", "onkel", "schwester", "bruder",
		"sohn", "tochter", "baby", "enkel",
	},
	"gesundheit": {
		"krank", "erkältet", "erkaeltet", "grippe", "arzt", "ärztin",
		"doktor", "krankenhaus", "op", "operation", "schmerz", "kopfschmerz",
		"migräne", "migraene", "magen", "rücken", "ruecken", "fieber",
		"erkältung", "erkaeltung", "husten", "schnupfen", "verletzt",
		"angeschlagen", "anstecken", "corona", "covid", "positiv",
	},
	"muede": {
		"müde", "muede", "erschöpft", "erschoepft", "kaputt", "platt",
		"schlaf", "energie", "fertig", "ausgepowert", "ko", "k.o.",
		"durch", "ausgelaugt",
	},
	"wetter": {
		"wetter", "regen", "regnet", "schnee", "sturm", "gewitter",
		"kalt", "hitze", "heiß", "heiss", "unwetter", "glatteis",
		"nebel", "frost",
	},
	"freizeit": {
		"konzert", "festival", "spiel", "fußball", "fussball", "champions",
		"bundesliga", "ticket", "kino", "theater", "veranstaltung",
		"party", "feier", "reise", "urlaub", "verreist", "unterwegs",
		"verabredet", "verabredung", "besuch", "besucher", "gast",
		"eingeladen", "einladung",
	},
	"keine_lust": {
		"kein bock", "keine lust", "keinen bock", "null bock",
		"unlust", "motivation", "motiviert", "antriebslos",
		"heute nicht", "nicht heute", "pause", "auszeit",
	},
}

// classifyCancellations converts raw rejections to categorized cancellations
func (e *Evaluator) classifyCancellations(userLookup map[string]int) []models.Cancellation {
	var cancellations []models.Cancellation

	for _, rejection := range e.rawData.Rejections {
		userIdx, exists := userLookup[rejection.UserID]
		if !exists {
			continue // Skip rejections for unknown users
		}

		user := e.rawData.Users[userIdx]
		message := ""
		if rejection.Message != nil {
			message = *rejection.Message
		}

		category := classifyMessage(message)

		cancellations = append(cancellations, models.Cancellation{
			Date:     rejection.Date,
			UserID:   userIdx + 1, // 1-based ID for frontend compatibility
			UserName: user.UserName,
			Message:  message,
			Category: category,
		})
	}

	return cancellations
}

// classifyMessage determines the category of a cancellation message
func classifyMessage(message string) string {
	if message == "" {
		return "keine_lust" // No message = no excuse = keine Lust
	}

	messageLower := strings.ToLower(message)

	// Check each category for keyword matches
	for category, keywords := range categoryKeywords {
		for _, keyword := range keywords {
			if strings.Contains(messageLower, keyword) {
				return category
			}
		}
	}

	// Check for creative/unusual excuses (messages that don't match any category)
	// If the message is long and doesn't match keywords, it might be creative
	if len(message) > 30 {
		return "kreativ"
	}

	// Default to keine_lust for short unclassified messages
	return "keine_lust"
}

// calculateCategoryStats counts cancellations by category
func (e *Evaluator) calculateCategoryStats(cancellations []models.Cancellation) models.CategoryStats {
	stats := make(models.CategoryStats)

	// Initialize all categories to 0
	for category := range models.GetAllExcuseCategories() {
		stats[category] = 0
	}

	// Count cancellations per category
	for _, c := range cancellations {
		stats[c.Category]++
	}

	return stats
}
