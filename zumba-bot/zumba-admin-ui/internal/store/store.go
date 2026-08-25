// Package store is the data-access layer for the admin UI.
// Exposes a Store interface implemented by both Postgres and the mock backend
// so handlers don't care which one is wired up.
package store

import (
	"context"
	"time"

	"github.com/michael/zumba-shared/domain"
	"github.com/michael/zumba-shared/penalty"
	sharedstore "github.com/michael/zumba-shared/store"

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

// LeaderboardRow ist eine Zeile der Rangliste (geteilter Typ, geteilte Query
// im shared-Modul). Streak ist vorzeichenbehaftet: >0 = aktuelle
// Anwesenheits-Serie, <0 = aktuelle Abwesenheits-Serie.
type LeaderboardRow = sharedstore.LeaderboardRow

// StripDay ist eine Kachel des Donnerstags-Strips: Datum, Sperrtag-Flag und
// Anzahl Abmeldungen – komplett in SQL aggregiert.
type StripDay struct {
	Date     time.Time
	Excluded bool
	Away     int
}

// DayAbsences sind die Abmeldungen eines gültigen Donnerstags (GROUP BY in SQL).
type DayAbsences struct {
	Date          time.Time
	AbsentUserIDs []string
}

// Season ist ein Stammtischjahr: gepflegter Auswertungszeitraum mit Label
// ("2026"). Jedes Jahr hat eigene Start-/Enddaten, keine Kalendergrenzen.
type Season = domain.Season

// Store is the interface used by handlers. Phase 2 adds the write methods.
type Store interface {
	// ListSeasons liefert alle Stammtischjahre, neuestes zuerst (Reihenfolge
	// des Jahres-Umschalters).
	ListSeasons(ctx context.Context) ([]Season, error)
	// SeasonAt liefert das Jahr, in das t fällt, sonst domain.ErrNoSeason.
	SeasonAt(ctx context.Context, t time.Time) (Season, error)
	// SeasonByLabel liefert das Jahr zum Slug aus ?jahr=.
	SeasonByLabel(ctx context.Context, label string) (Season, error)

	ListUsers(ctx context.Context) ([]User, error)
	// GetUser liefert einen einzelnen User (nil, wenn unbekannt).
	GetUser(ctx context.Context, userID string) (*User, error)
	ListThursdays(ctx context.Context, p timeutil.Period) ([]time.Time, error)
	ListExcludedDays(ctx context.Context, p timeutil.Period) ([]time.Time, error)
	// IsExcludedDay prüft einen einzelnen Tag (EXISTS statt Liste + Scan).
	IsExcludedDay(ctx context.Context, date time.Time) (bool, error)
	ListAbsences(ctx context.Context, p timeutil.Period) ([]Absence, error)
	// AbsencesOn liefert nur die Abmeldungen eines Datums (WHERE date = $1).
	AbsencesOn(ctx context.Context, date time.Time) ([]Absence, error)
	// ListUserAbsences liefert nur die Abmeldungen eines Users im Zeitraum.
	ListUserAbsences(ctx context.Context, p timeutil.Period, userID string) ([]Absence, error)
	Leaderboard(ctx context.Context, p timeutil.Period) ([]LeaderboardRow, error)
	// UserLeaderboardRow liefert die Leaderboard-Zeile eines einzelnen Users.
	UserLeaderboardRow(ctx context.Context, p timeutil.Period, userID string) (LeaderboardRow, error)
	// ThursdayStrip liefert die jüngsten Donnerstage (inkl. Sperrtage) bis
	// heute mit Abmelde-Zahl, aufsteigend sortiert; limit 0 = alle.
	ThursdayStrip(ctx context.Context, p timeutil.Period, limit int) ([]StripDay, error)
	// ListDayAbsences gruppiert Abmeldungen je gültigem Donnerstag (neueste zuerst).
	ListDayAbsences(ctx context.Context, p timeutil.Period) ([]DayAbsences, error)

	InsertAbsence(ctx context.Context, userID string, date time.Time, message *string) error
	DeleteAbsence(ctx context.Context, userID string, date time.Time) error
	// ToggleAbsence kippt den Abmelde-Status (true = jetzt abgemeldet) ohne
	// vorherigen Lese-Roundtrip.
	ToggleAbsence(ctx context.Context, userID string, date time.Time) (bool, error)
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
	// ist das korrekte Label (nil = Gemini-Label war korrekt). Liefert die
	// aktualisierte Zeile (UPDATE ... RETURNING).
	VerifyMLMessage(ctx context.Context, id int64, correctedLabel *string) (*MLMessage, error)

	// Manueller ML-Test (ml_test_messages): über das UI eingetippte Nachrichten,
	// vom classifier-service klassifiziert und per Hand bewertet.
	InsertMLTest(ctx context.Context, message, modelLabel string, confidence float64) (int64, error)
	ListMLTests(ctx context.Context, limit int) ([]MLTestMessage, error)
	// JudgeMLTest hinterlegt das erwartete Label ("passt" = Modell-Label) und
	// liefert die aktualisierte Zeile (UPDATE ... RETURNING).
	JudgeMLTest(ctx context.Context, id int64, expectedLabel string) (*MLTestMessage, error)
	DeleteMLTest(ctx context.Context, id int64) error

	// Strafen-Feature. ListSeasonStrafen liefert alle Zeilen EINES Jahres
	// (inkl. beglichen und geloescht – Lösch-/Begleich-Zeitpunkte resetten
	// den Fehltage-Zähler).
	ListSeasonStrafen(ctx context.Context, season Season) ([]penalty.Row, error)
	// InsertAutoStrafe persistiert den Marker einer erkannten Fehltage-Strafe
	// (idempotent auf userId + Serienstart).
	InsertAutoStrafe(ctx context.Context, userID string, datum time.Time) error
	// InsertNoShowStrafe legt eine manuelle Strafe an (nicht abgemeldet).
	InsertNoShowStrafe(ctx context.Context, userID string, datum time.Time, betrag int) error
	// BegleicheStrafe setzt status=beglichen + beglichen_am=now().
	BegleicheStrafe(ctx context.Context, id int64) error
	// LoescheStrafe ist ein Soft-Delete (status=geloescht): die Zeile bleibt
	// als Reset-Marker erhalten, taucht aber nirgends mehr auf.
	LoescheStrafe(ctx context.Context, id int64) error
}

// MLTestMessage ist ein manuell eingegebener Testfall aus dem Admin-UI.
type MLTestMessage struct {
	ID              int64
	CreatedAt       time.Time
	Message         string
	ModelLabel      string
	ModelConfidence float64
	ExpectedLabel   *string // nil = noch nicht bewertet
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
