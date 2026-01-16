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
	CategoryStats []CategoryStat
	BestExcuses   []Excuse

	// Heatmap
	HeatmapMonths  []HeatmapMonth
	HeatmapInsight HeatmapInsight

	// Attendance Heatmap (average attendance rate per month)
	AttendanceHeatmapMonths  []AttendanceHeatmapMonth
	AttendanceHeatmapInsight AttendanceHeatmapInsight

	// AI Summary (data for client-side randomization)
	AIStats AIStats

	// Personal slides
	PersonalTop5    []PersonalCard
	PersonalMid5    []PersonalCard
	PersonalBottom5 []PersonalCard

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
	Rank           int
	RankDisplay    string // "🥇", "🥈", "🥉", or "#4" etc.
	Name           string
	Emoji          string
	Title          string
	TitleEmoji     string
	AttendanceRate int
	BarColor       string // "bg-green-500", "bg-biergold", "bg-orange-500", "bg-red-400"
	TierBgColor    string // gradient class for tier
	DelayClass     string // "delay-200", "delay-300", etc.
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

// HeatmapMonth contains pre-calculated heatmap cell data
type HeatmapMonth struct {
	Label      string // "Jan", "Feb", etc.
	Count      int
	BgColor    string // "bg-red-500", "bg-orange-500", "bg-yellow-500", "bg-green-500/50", "bg-holz-light/30"
	DelayClass string
}

// HeatmapInsight contains worst/best month data
type HeatmapInsight struct {
	WorstMonth string
	WorstCount int
	BestMonth  string
	BestCount  int
}

// AttendanceHeatmapMonth contains pre-calculated attendance rate heatmap cell data
type AttendanceHeatmapMonth struct {
	Label      string // "Jan", "Feb", etc.
	Rate       int    // Average attendance rate 0-100
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

// AIStats contains pre-rendered AI summary (server-side selected)
type AIStats struct {
	// SummaryHTML is the pre-selected and rendered summary text
	SummaryHTML string
}

// PersonalCard contains data for personal slide cards
type PersonalCard struct {
	Name            string
	Emoji           string
	TitleEmoji      string
	AttendanceRate  int
	BarColor        string
	FunFact         string
	PersonalMessage string
	TierBgColor     string
	DelayClass      string
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
