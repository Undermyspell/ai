// Package viewmodels contains view-specific data structures for templ rendering.
// These structs contain pre-calculated display values (colors, percentages, delays)
// to enable server-side rendering without client-side JavaScript.
package viewmodels

// PageViewModel contains all data needed to render the Wrapped page
type PageViewModel struct {
	Year string

	// YearStats - for the statistics slide
	YearStats YearStatsView

	// Rankings (3 groups of 5)
	Top5Rankings   []RankedUser
	MidRankings    []RankedUser
	BottomRankings []RankedUser

	// Streaks
	AttendanceStreaks   []StreakUser
	CancellationStreaks []StreakUser

	// Excuses
	CategoryStats   []CategoryStat
	BestExcuses     []Excuse
	RecycledExcuses []RecycledExcuse

	// Attendance Heatmap (average attendance rate + cancellations per month)
	AttendanceHeatmapMonths  []AttendanceHeatmapMonth
	AttendanceHeatmapInsight AttendanceHeatmapInsight

	// Best/worst Thursdays
	BestThursdays  []ThursdayCard
	WorstThursdays []ThursdayCard

	// Full house Thursdays (everyone attended)
	FullHouse FullHouseView

	// Fun-card slides (rendered only when non-empty)
	DuoCards      []FunCard // Unzertrennliche, Zwillinge, Wachablösung, Ping-Pong
	SuspectCards  []FunCard // Magnet & Schreck, Alibi-Duo, Copy-Paste-Duo, Todesduo
	SquadCards    []FunCard // Dreamteam, Retter in der Not, Mitläufer
	ForensikCards []FunCard // Romanautor, Minimalist, Emoji-König
	MuffelCards   []FunCard // Sommermuffel, Wintermuffel

	// Quiz (interactive reveal)
	Quiz QuizView

	// Strafen (penalties)
	Strafen StrafenView

	// AI Summary (data for client-side randomization)
	AIStats AIStats

	// Share card data (JSON consumed by the canvas renderer)
	ShareJSON string

	// Personality types
	PersonalityTypes []PersonalityType

	// Awards
	Awards []AwardView

	// Confetti for finale (pre-generated particles)
	Confetti ConfettiView
}

// YearStatsView contains counter target values for the year stats slide
type YearStatsView struct {
	TotalThursdays        int
	TotalUsers            int
	TotalAttendances      int
	TotalCancellations    int
	AverageAttendanceRate int
}

// RankedUser contains pre-calculated display data for ranking cards
type RankedUser struct {
	Rank            int
	RankDisplay     string // "🥇", "🥈", "🥉", or "#4" etc.
	Name            string
	Emoji           string
	Title           string
	TitleEmoji      string
	AttendanceRate  int
	BarColor        string // "bg-green-500", "bg-biergold", "bg-orange-500", "bg-red-400"
	TierBgColor     string // gradient class for tier
	FunFact         string // e.g. "👑 Nie abgesagt!"
	PersonalMessage string // e.g. "Legende! 🏆"
	DelayClass      string // "delay-200", "delay-300", etc.
}

// StreakUser contains data for streak displays
type StreakUser struct {
	Name                  string
	Emoji                 string
	MaxAttendanceStreak   int
	MaxCancellationStreak int
	DateRange             string // Formatted date range like "12. Jan - 9. Feb"
	DelayClass            string
}

// CategoryStat contains sorted category data with percentage
type CategoryStat struct {
	Key        string
	Label      string
	Emoji      string
	Count      int
	Percentage int // 0-100 relative to max
	DelayClass string
}

// Excuse contains data for best excuses display
type Excuse struct {
	Message    string
	UserName   string
	DelayClass string
}

// RecycledExcuse is a message a user sent verbatim more than once
type RecycledExcuse struct {
	UserName   string
	Message    string
	Count      int
	DelayClass string
}

// AttendanceHeatmapMonth contains pre-calculated attendance rate heatmap cell data
type AttendanceHeatmapMonth struct {
	Label      string // "Jan", "Feb", etc.
	Rate       int    // Average attendance rate 0-100
	Count      int    // Cancellations in that month
	BgColor    string // color class based on rate
	DelayClass string
}

// AttendanceHeatmapInsight contains best/worst month data for attendance
type AttendanceHeatmapInsight struct {
	BestMonth  string
	BestRate   int
	WorstMonth string
	WorstRate  int
}

// ThursdayCard contains display data for a single best/worst Thursday
type ThursdayCard struct {
	RankDisplay string // "🥇", "🥈", "🥉" (top) or "💀", "🕸️", "🦗" (flop)
	DateDisplay string // e.g. "12. Feb 2026"
	Attendees   int
	Total       int
	Rate        int    // 0-100, used as bar width
	BarColor    string // color class based on rate
	DelayClass  string
}

// StrafenEntryView is one display-ready penalty line
type StrafenEntryView struct {
	ArtEmoji    string // "🪑" fehltage, "👻" noshow
	Label       string // e.g. "6 Wochen gefehlt" or "No-Show"
	DateRange   string // e.g. "12. Feb – 19. Mär" (fehltage) or "5. Mär" (noshow)
	Betrag      string // e.g. "30 €"
	StatusEmoji string // "✅" beglichen, "⏳" offen
}

// StrafenUserView contains display data for one penalty payer
type StrafenUserView struct {
	RankDisplay string
	Name        string
	Emoji       string
	Total       string // e.g. "55 €"
	TotalEuro   int    // raw euro value for counters
	Entries     []StrafenEntryView
	DelayClass  string
}

// StrafenView contains all penalty slide data
type StrafenView struct {
	HasStrafen bool
	TotalSum   int // Euro, counter target
	TotalCount int
	MassBier   int // TotalSum / 5 € — what the Kasse buys in Maß
	TopPayers  []StrafenUserView
}

// AIStats contains pre-rendered AI summary (server-side selected)
type AIStats struct {
	// SummaryHTML is the pre-selected and rendered summary text
	SummaryHTML string
}

// FullHouseView contains the "everyone attended" stats
type FullHouseView struct {
	HasAny bool
	Count  int
	Dates  []string // formatted, at most 5
	More   int      // dates beyond the shown ones
}

// FunCard is a generic finding card: a category label, the people/values
// involved, and an explanatory line
type FunCard struct {
	Emoji      string // category emoji, e.g. "👯"
	Title      string // e.g. "Die Absage-Zwillinge"
	Headline   string // e.g. "🎵 Martin & 💻 Sebastian"
	Detail     string // e.g. "8× am selben Donnerstag gefehlt"
	Quote      string // optional: verbatim message, rendered italic
	Gradient   string // background gradient classes
	DelayClass string
}

// QuizView contains one interactive quiz question with hidden answer
type QuizView struct {
	Question     string
	AnswerName   string
	AnswerEmoji  string
	AnswerDetail string
}

// PersonalityType contains pre-grouped personality type data
type PersonalityType struct {
	Emoji       string
	Name        string
	Description string
	Users       []PersonalityUser
	HasMore     bool // true if more than 5 users
	MoreCount   int  // count of users beyond 5
	DelayClass  string
}

// PersonalityUser contains minimal user data for personality type display
type PersonalityUser struct {
	Name  string
	Emoji string
}

// AwardView contains display-ready award data
type AwardView struct {
	Emoji       string
	Title       string
	Subtitle    string
	WinnerName  string
	WinnerEmoji string
	Color       string // gradient class
	DelayClass  string
}

// ConfettiParticle contains pre-computed confetti particle data for SSR
type ConfettiParticle struct {
	Color        string // hex color
	Left         string // percentage
	Size         string // pixels
	BorderRadius string // "50%" or "0"
	Delay        string // seconds
	Duration     string // seconds
}

// ConfettiView contains all confetti particles for the finale slide
type ConfettiView struct {
	Particles []ConfettiParticle
}
