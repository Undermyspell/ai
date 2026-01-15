package models

import "time"

// User represents a Stammtisch participant
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Emoji string `json:"emoji"`
}

// UserStats contains calculated statistics for a user
type UserStats struct {
	User
	CancellationCount      int            `json:"cancellationCount"`
	AttendanceCount        int            `json:"attendanceCount"`
	AttendanceRate         int            `json:"attendanceRate"`
	MaxAttendanceStreak    int            `json:"maxAttendanceStreak"`
	MaxCancellationStreak  int            `json:"maxCancellationStreak"`
	NeverCancelled         bool           `json:"neverCancelled"`
	FavoriteExcuseCategory string         `json:"favoriteExcuseCategory"`
	Rank                   int            `json:"rank"`
	Title                  string         `json:"title"`
	TitleEmoji             string         `json:"titleEmoji"`
	Cancellations          []Cancellation `json:"cancellations,omitempty"`
}

// Cancellation represents a user's absence
type Cancellation struct {
	Date     time.Time `json:"date"`
	UserID   int       `json:"userId"`
	UserName string    `json:"userName"`
	Message  string    `json:"message"`
	Category string    `json:"category"`
}

// ExcuseCategory holds excuse types and examples
type ExcuseCategory struct {
	Name     string
	Emoji    string
	Label    string
	Examples []string
}

// GetAllExcuseCategories returns all excuse categories with examples
func GetAllExcuseCategories() map[string]ExcuseCategory {
	return map[string]ExcuseCategory{
		"arbeit": {
			Name:  "arbeit",
			Emoji: "💼",
			Label: "Arbeit",
			Examples: []string{
				"Muss länger arbeiten, sorry Jungs 😔",
				"Meeting bis 20 Uhr, das wird nix heute",
				"Deadline morgen, sitze noch im Büro",
				"Chef hat spontan was reingedrückt...",
				"Überstunden ohne Ende, nächste Woche wieder!",
				"Projekt-Crunch, ihr kennt das 💼",
				"Kundenbesuch, muss leider absagen",
			},
		},
		"familie": {
			Name:  "familie",
			Emoji: "👨‍👩‍👧",
			Label: "Familie",
			Examples: []string{
				"Familienfeier, muss zur Schwiegermutter 😅",
				"Kind ist krank, bleibe daheim",
				"Hochzeitstag vergessen... muss was gutmachen",
				"Eltern kommen zu Besuch",
				"Kindergeburtstag, nächste Woche!",
				"Frau hat was geplant, sorry!",
				"Familiending, kann nicht weg",
			},
		},
		"gesundheit": {
			Name:  "gesundheit",
			Emoji: "🤒",
			Label: "Gesundheit",
			Examples: []string{
				"Bin flach, Erkältung hat mich erwischt 🤧",
				"Rücken macht nicht mit heute",
				"Migräne, liege im Dunkeln",
				"Magen-Darm, sag ich nur...",
				"Arzttermin morgen früh, muss fit sein",
				"Bin angeschlagen, will euch nicht anstecken",
			},
		},
		"muede": {
			Name:  "muede",
			Emoji: "😴",
			Label: "Müdigkeit",
			Examples: []string{
				"Komplett platt, sorry Leute 😴",
				"Null Energie heute, wird ne Couch-Session",
				"Die Woche war brutal, brauch Schlaf",
				"Bin durch, nächste Woche wieder fit!",
				"Einfach zu müde für alles",
			},
		},
		"wetter": {
			Name:  "wetter",
			Emoji: "🌧️",
			Label: "Wetter",
			Examples: []string{
				"Bei dem Wetter geh ich nicht raus 🌧️",
				"Schnee ohne Ende, Auto eingefroren",
				"Sturm angesagt, bleib lieber daheim",
				"40 Grad? Ich bleib in der Klimaanlage",
			},
		},
		"freizeit": {
			Name:  "freizeit",
			Emoji: "🎉",
			Label: "Andere Pläne",
			Examples: []string{
				"Champions League heute, sorry nicht sorry ⚽",
				"Konzert-Tickets seit Monaten, muss hin 🎸",
				"Kumpel von früher ist in der Stadt",
				"Geburtstag von nem Kollegen",
				"Andere Verabredung, war zuerst geplant",
			},
		},
		"kreativ": {
			Name:  "kreativ",
			Emoji: "🎨",
			Label: "Kreativ",
			Examples: []string{
				"Mein Goldfisch hat Geburtstag 🐟",
				"Muss meine Pflanzen gießen, die sehen traurig aus",
				"Hab mir vorgenommen heute mal früh ins Bett zu gehen (lol)",
				"Sitze in der Badewanne, kein Bock rauszugehen",
				"Mars steht ungünstig, Astrologe sagt nein 🔮",
				"Netflix hat neue Staffel released, ihr versteht",
				"Bin in einem Wikipedia-Rabbit-Hole gefangen",
				"Muss meinen Kühlschrank sortieren, dringend",
				"Hab mich ausgesperrt und warte auf den Schlüsseldienst (Spoiler: Lüge)",
				"Meine Katze braucht emotionale Unterstützung heute 🐱",
			},
		},
		"keine_lust": {
			Name:  "keine_lust",
			Emoji: "😬",
			Label: "Keine Lust",
			Examples: []string{
				"Hab heute einfach keinen Bock, sorry 😬",
				"Brauch mal ne Pause, nächste Woche!",
				"Heute nicht, Jungs",
				"Chill-Abend geplant, ohne Menschen",
			},
		},
	}
}
