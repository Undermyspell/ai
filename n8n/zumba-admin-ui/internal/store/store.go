// Package store is the data-access layer for the admin UI.
// Exposes a Store interface implemented by both Postgres and the mock backend
// so handlers don't care which one is wired up.
package store

import (
	"context"
	"time"

	"github.com/michael/zumba-admin-ui/internal/timeutil"
)

type User struct {
	ID        string
	Name      string
	Emoji     string
	StartDate *time.Time
}

type Absence struct {
	UserID  string
	Date    time.Time
	Message *string
}

type LeaderboardRow struct {
	UserID          string
	UserName        string
	StartDate       *time.Time
	EffectiveStart  time.Time
	ThursdayCount   int
	AttendanceCount int
	AwayCount       int
	AttendPercent   float64
	// Streak is signed: >0 = current attendance run, <0 = current absence run, 0 = no Thursdays yet.
	Streak int
}

// Store is the interface used by handlers. Phase 2 adds the write methods.
type Store interface {
	ListUsers(ctx context.Context) ([]User, error)
	ListThursdays(ctx context.Context, p timeutil.Period) ([]time.Time, error)
	ListExcludedDays(ctx context.Context, p timeutil.Period) ([]time.Time, error)
	ListAbsences(ctx context.Context, p timeutil.Period) ([]Absence, error)
	Leaderboard(ctx context.Context, p timeutil.Period) ([]LeaderboardRow, error)

	InsertAbsence(ctx context.Context, userID string, date time.Time, message *string) error
	DeleteAbsence(ctx context.Context, userID string, date time.Time) error
	InsertExcludedDay(ctx context.Context, date time.Time) error
	DeleteExcludedDay(ctx context.Context, date time.Time) error

	// Bot-Trace (Verlauf-Ansicht): ListTraces liefert Zusammenfassungen,
	// GetTrace die volle Aufzeichnung inkl. Schritte + Roh-Payload.
	ListTraces(ctx context.Context, limit int) ([]Trace, error)
	GetTrace(ctx context.Context, id int64) (*Trace, error)

	// ML-Shadow-Modus (ml_messages): Gemini- vs. eigenes Modell-Label.
	ListMLMessages(ctx context.Context, onlyDisagree bool, limit int) ([]MLMessage, error)
	MLShadowStats(ctx context.Context) (MLShadowStats, error)
	// VerifyMLMessage markiert einen Eintrag als handgeprüft; correctedLabel
	// ist das korrekte Label (nil = Gemini-Label war korrekt).
	VerifyMLMessage(ctx context.Context, id int64, correctedLabel *string) error
}

// MLMessage ist ein Eintrag aus ml_messages (Shadow-Modus-Protokoll des Bots).
type MLMessage struct {
	ID              int64
	CreatedAt       time.Time
	UserID          string
	UserName        string
	Message         string
	GeminiLabel     string
	ModelLabel      *string // nil = classifier-service war nicht erreichbar
	ModelConfidence *float64
	Agree           *bool
	Verified        bool
	CorrectedLabel  *string
}

// MLLabelStat aggregiert Übereinstimmung je Gemini-Label.
type MLLabelStat struct {
	Label string
	Total int
	Agree int
}

// MLShadowStats fasst den Stand des Shadow-Modus zusammen.
type MLShadowStats struct {
	Total     int // alle protokollierten Nachrichten
	WithModel int // davon mit Modell-Antwort
	Agree     int // davon Übereinstimmung Gemini == Modell
	PerLabel  []MLLabelStat
}

// Knoten-IDs des festen Bot-Flow-Graphen (Vertrag mit dem whatsapp-bot).
const (
	NodeReceived       = "received"
	NodeCheckStatistik = "check_statistik"
	NodeBuildStats     = "build_stats"
	NodeSendStats      = "send_stats"
	NodeGuardType      = "guard_type"
	NodeGuardGroup     = "guard_group"
	NodeGuardThursday  = "guard_thursday"
	NodeClassify       = "classify"
	NodeMarkAbsent     = "mark_absent"
	NodeMarkPresent    = "mark_present"
	NodeNoAction       = "no_action"
	NodeIgnored        = "ignored"
)

// TraceStep ist ein Entscheidungspunkt im Bot-Flow (gespiegelt aus dem Bot).
type TraceStep struct {
	Node    string `json:"node"`
	Outcome string `json:"outcome"` // pass | fail | info | error
	Label   string `json:"label"`
	Detail  string `json:"detail"`
}

// Trace ist eine aufgezeichnete Bot-Verarbeitung eines Gruppen-Events.
type Trace struct {
	ID             int64
	CreatedAt      time.Time
	RemoteJid      string
	UserID         string
	UserName       string
	Message        string
	MessageType    string
	Path           string // statistik | classify | ignored
	Classification string // true | false | invalid | ""
	Action         string
	HasError       bool
	RawPayload     string // hübsch eingerücktes JSON (nur in GetTrace befüllt)
	Steps          []TraceStep
}
