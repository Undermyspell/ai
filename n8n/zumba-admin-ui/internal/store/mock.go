package store

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/michael/zumba-admin-ui/internal/timeutil"
)

// Mock is an in-memory Store used when the real DB is unreachable.
// Data is generated deterministically (fixed seed) so the UI looks consistent
// across reloads.
type Mock struct {
	users        []User
	absences     []Absence   // only entries for valid Thursdays
	excludedDays []time.Time // Thursdays
}

func NewMock(p timeutil.Period) *Mock {
	users := []User{
		{ID: "u01", Name: "Max", Emoji: "🍺"},
		{ID: "u02", Name: "Thomas", Emoji: "🎸"},
		{ID: "u03", Name: "Stefan", Emoji: "⚽"},
		{ID: "u04", Name: "Andreas", Emoji: "🎮"},
		{ID: "u05", Name: "Michael", Emoji: "📚"},
		{ID: "u06", Name: "Christian", Emoji: "🏔️"},
		{ID: "u07", Name: "Markus", Emoji: "🚴"},
		{ID: "u08", Name: "Daniel", Emoji: "🎬"},
		{ID: "u09", Name: "Sebastian", Emoji: "💻"},
		{ID: "u10", Name: "Patrick", Emoji: "🎯"},
		{ID: "u11", Name: "Florian", Emoji: "🍕"},
		{ID: "u12", Name: "Tobias", Emoji: "🏋️"},
		{ID: "u13", Name: "Martin", Emoji: "🎵"},
		{ID: "u14", Name: "Philipp", Emoji: "🎨"},
		{ID: "u15", Name: "Jan", Emoji: "🏀"},
	}

	thursdays := generateThursdays(p.Start, p.EffectiveEnd())
	rng := rand.New(rand.NewSource(42))

	// Excluded days: pick a couple of Thursdays in the future-ish range
	var excluded []time.Time
	if len(thursdays) > 4 {
		excluded = []time.Time{thursdays[len(thursdays)/3]}
	}

	excludedSet := make(map[string]bool, len(excluded))
	for _, d := range excluded {
		excludedSet[timeutil.FormatISO(d)] = true
	}

	// Per user: assign a "reliability" tier and randomly mark them absent.
	excuses := []string{
		"bin raus", "muss arbeiten", "krank", "kind krank",
		"familienbesuch", "schaffs heut nicht",
		"komme heute leider nicht", "auswärtstermin",
	}

	var absences []Absence
	for i, u := range users {
		// reliability: ~ from 0.95 (always there) to 0.40 (often gone)
		reliability := 0.95 - float64(i)*0.04
		for _, day := range thursdays {
			if excludedSet[timeutil.FormatISO(day)] {
				continue
			}
			if rng.Float64() > reliability {
				var msg *string
				if rng.Float64() < 0.85 {
					m := excuses[rng.Intn(len(excuses))]
					msg = &m
				}
				absences = append(absences, Absence{
					UserID:  u.ID,
					Date:    day,
					Message: msg,
				})
			}
		}
	}

	return &Mock{users: users, absences: absences, excludedDays: excluded}
}

func generateThursdays(start, end time.Time) []time.Time {
	var out []time.Time
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Thursday {
			out = append(out, d)
		}
	}
	return out
}

func (m *Mock) ListUsers(_ context.Context) ([]User, error) {
	out := make([]User, len(m.users))
	copy(out, m.users)
	return out, nil
}

func (m *Mock) ListThursdays(_ context.Context, p timeutil.Period) ([]time.Time, error) {
	excluded := make(map[string]bool, len(m.excludedDays))
	for _, d := range m.excludedDays {
		excluded[timeutil.FormatISO(d)] = true
	}
	all := generateThursdays(p.Start, p.EffectiveEnd())
	out := make([]time.Time, 0, len(all))
	for _, d := range all {
		if !excluded[timeutil.FormatISO(d)] {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].After(out[j]) })
	return out, nil
}

func (m *Mock) ListExcludedDays(_ context.Context, p timeutil.Period) ([]time.Time, error) {
	out := make([]time.Time, 0, len(m.excludedDays))
	for _, d := range m.excludedDays {
		if (d.Equal(p.Start) || d.After(p.Start)) && (d.Equal(p.End) || d.Before(p.End)) {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].After(out[j]) })
	return out, nil
}

func (m *Mock) ListAbsences(_ context.Context, p timeutil.Period) ([]Absence, error) {
	end := p.EffectiveEnd()
	out := make([]Absence, 0)
	for _, a := range m.absences {
		if (a.Date.Equal(p.Start) || a.Date.After(p.Start)) && (a.Date.Equal(end) || a.Date.Before(end)) {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Date.Equal(out[j].Date) {
			return out[i].Date.After(out[j].Date)
		}
		return out[i].UserID < out[j].UserID
	})
	return out, nil
}

func (m *Mock) Leaderboard(ctx context.Context, p timeutil.Period) ([]LeaderboardRow, error) {
	thursdays, _ := m.ListThursdays(ctx, p)
	thursdayCount := len(thursdays)

	absenceByUser := make(map[string][]time.Time)
	for _, a := range m.absences {
		absenceByUser[a.UserID] = append(absenceByUser[a.UserID], a.Date)
	}

	rows := make([]LeaderboardRow, 0, len(m.users))
	for _, u := range m.users {
		away := len(absenceByUser[u.ID])
		attend := thursdayCount - away
		if attend < 0 {
			attend = 0
		}
		var pct float64
		if thursdayCount > 0 {
			pct = float64(attend) / float64(thursdayCount) * 100
		}
		streak := computeStreakMock(thursdays, absenceByUser[u.ID])
		rows = append(rows, LeaderboardRow{
			UserID:          u.ID,
			UserName:        u.Name,
			EffectiveStart:  p.Start,
			ThursdayCount:   thursdayCount,
			AttendanceCount: attend,
			AwayCount:       away,
			AttendPercent:   pct,
			Streak:          streak,
		})
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].AttendanceCount != rows[j].AttendanceCount {
			return rows[i].AttendanceCount > rows[j].AttendanceCount
		}
		if rows[i].AttendPercent != rows[j].AttendPercent {
			return rows[i].AttendPercent > rows[j].AttendPercent
		}
		return rows[i].UserName < rows[j].UserName
	})
	return rows, nil
}

func (m *Mock) InsertAbsence(_ context.Context, userID string, date time.Time, message *string) error {
	day := timeutil.StartOfDay(date)
	for i := range m.absences {
		if m.absences[i].UserID == userID && timeutil.FormatISO(m.absences[i].Date) == timeutil.FormatISO(day) {
			m.absences[i].Message = message // upsert
			return nil
		}
	}
	m.absences = append(m.absences, Absence{UserID: userID, Date: day, Message: message})
	return nil
}

func (m *Mock) DeleteAbsence(_ context.Context, userID string, date time.Time) error {
	out := m.absences[:0]
	for _, a := range m.absences {
		if a.UserID == userID && timeutil.FormatISO(a.Date) == timeutil.FormatISO(date) {
			continue
		}
		out = append(out, a)
	}
	m.absences = out
	return nil
}

func (m *Mock) InsertExcludedDay(_ context.Context, date time.Time) error {
	day := timeutil.StartOfDay(date)
	for _, d := range m.excludedDays {
		if timeutil.FormatISO(d) == timeutil.FormatISO(day) {
			return nil
		}
	}
	m.excludedDays = append(m.excludedDays, day)
	return nil
}

func (m *Mock) DeleteExcludedDay(_ context.Context, date time.Time) error {
	out := m.excludedDays[:0]
	for _, d := range m.excludedDays {
		if timeutil.FormatISO(d) == timeutil.FormatISO(date) {
			continue
		}
		out = append(out, d)
	}
	m.excludedDays = out
	return nil
}

// sampleTraces liefert ein paar Beispiel-Aufzeichnungen, damit die Verlauf-Ansicht
// auch ohne DB (Mock-Modus) etwas Sinnvolles zeigt.
func sampleTraces() []Trace {
	base := time.Date(2026, 6, 25, 20, 12, 0, 0, time.Local) // ein Donnerstag
	return []Trace{
		{
			ID: 3, CreatedAt: base, UserName: "Tobi", Message: "bin heute leider raus",
			MessageType: "conversation", Path: "classify", Classification: "false", Action: "marked_absent",
			RemoteJid: "000000000000-0000000000@g.us", UserID: "49170...@s.whatsapp.net",
			RawPayload: "{\n  \"data\": { \"messageType\": \"conversation\" }\n}",
			Steps: []TraceStep{
				{Node: "received", Outcome: "info", Label: "Webhook empfangen", Detail: "Tobi · Typ \"conversation\""},
				{Node: "check_statistik", Outcome: "info", Label: "\"statistik\"?", Detail: "nein"},
				{Node: "guard_type", Outcome: "pass", Label: "messageType == conversation?", Detail: "ja"},
				{Node: "guard_group", Outcome: "pass", Label: "Zumba-Gruppe?", Detail: "ja"},
				{Node: "guard_thursday", Outcome: "pass", Label: "Donnerstag?", Detail: "Thu, 2026-06-25"},
				{Node: "classify", Outcome: "info", Label: "Classifier (Gemini)", Detail: "→ false (roh: \"false\" · gemini-2.5-flash)"},
				{Node: "mark_absent", Outcome: "pass", Label: "Absage: DB-Insert", Detail: "eingetragen für 2026-06-25"},
			},
		},
		{
			ID: 2, CreatedAt: base.Add(-3 * time.Minute), UserName: "Hiller", Message: "statistik",
			MessageType: "conversation", Path: "statistik", RemoteJid: "000000000000-0000000000@g.us",
			Steps: []TraceStep{
				{Node: "received", Outcome: "info", Label: "Webhook empfangen", Detail: "Hiller · Typ \"conversation\""},
				{Node: "check_statistik", Outcome: "pass", Label: "\"statistik\"?", Detail: "ja"},
				{Node: "build_stats", Outcome: "pass", Label: "Statistik berechnen", Detail: "15 Nutzer"},
				{Node: "send_stats", Outcome: "pass", Label: "An Gruppe senden", Detail: "→ Zumba-Gruppe"},
			},
		},
		{
			ID: 1, CreatedAt: base.Add(-30 * time.Minute), UserName: "Michl", Message: "",
			MessageType: "imageMessage", Path: "ignored", RemoteJid: "000000000000-0000000000@g.us",
			Steps: []TraceStep{
				{Node: "received", Outcome: "info", Label: "Webhook empfangen", Detail: "Michl · Typ \"imageMessage\""},
				{Node: "check_statistik", Outcome: "info", Label: "\"statistik\"?", Detail: "nein"},
				{Node: "guard_type", Outcome: "fail", Label: "messageType == conversation?", Detail: "nein: imageMessage"},
				{Node: "ignored", Outcome: "info", Label: "Ignoriert", Detail: "kein conversation-Event"},
			},
		},
	}
}

func (m *Mock) ListTraces(_ context.Context, limit int) ([]Trace, error) {
	traces := sampleTraces()
	if limit > 0 && limit < len(traces) {
		traces = traces[:limit]
	}
	return traces, nil
}

func (m *Mock) GetTrace(_ context.Context, id int64) (*Trace, error) {
	for _, t := range sampleTraces() {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("GetTrace: trace %d nicht gefunden", id)
}

// computeStreakMock walks Thursdays newest-first; returns +N for an attendance
// run from now, -N for an absence run.
func computeStreakMock(thursdaysDesc []time.Time, absenceDates []time.Time) int {
	if len(thursdaysDesc) == 0 {
		return 0
	}
	absent := make(map[string]bool, len(absenceDates))
	for _, d := range absenceDates {
		absent[timeutil.FormatISO(d)] = true
	}
	first := absent[timeutil.FormatISO(thursdaysDesc[0])]
	count := 0
	for _, d := range thursdaysDesc {
		if absent[timeutil.FormatISO(d)] != first {
			break
		}
		count++
	}
	if first {
		return -count
	}
	return count
}

// --- ML-Shadow-Modus: Beispieldaten für den Mock-Betrieb ---

func sampleMLMessages() []MLMessage {
	s := func(v string) *string { return &v }
	f := func(v float64) *float64 { return &v }
	b := func(v bool) *bool { return &v }
	base := time.Date(2026, 7, 23, 18, 30, 0, 0, time.Local)
	return []MLMessage{
		{ID: 5, CreatedAt: base.Add(90 * time.Minute), UserID: "u03", UserName: "Sepp",
			Message: "Muss mi heut abmelden ❌", GeminiLabel: "false",
			ModelLabel: s("false"), ModelConfidence: f(0.97), Agree: b(true)},
		{ID: 4, CreatedAt: base.Add(60 * time.Minute), UserID: "u07", UserName: "Tobi",
			Message: "Bei mir wirds bissi später ✌🏻", GeminiLabel: "true",
			ModelLabel: s("invalid"), ModelConfidence: f(0.48), Agree: b(false)},
		{ID: 3, CreatedAt: base.Add(40 * time.Minute), UserID: "u01", UserName: "Max",
			Message: "Schau ma moi wie's Wetter wird", GeminiLabel: "invalid",
			ModelLabel: s("invalid"), ModelConfidence: f(0.88), Agree: b(true),
			Verified: true},
		{ID: 2, CreatedAt: base.Add(20 * time.Minute), UserID: "u05", UserName: "Flo",
			Message: "Gruzefix.... Nullinger", GeminiLabel: "false",
			ModelLabel: s("invalid"), ModelConfidence: f(0.61), Agree: b(false),
			Verified: true, CorrectedLabel: s("false")},
		{ID: 1, CreatedAt: base, UserID: "u02", UserName: "Basti",
			Message: "I bin dabei heit 🍻", GeminiLabel: "true",
			ModelLabel: nil, ModelConfidence: nil, Agree: nil},
	}
}

func (m *Mock) ListMLMessages(_ context.Context, onlyDisagree bool, limit int) ([]MLMessage, error) {
	var out []MLMessage
	for _, msg := range sampleMLMessages() {
		if onlyDisagree && msg.Agree != nil && *msg.Agree {
			continue
		}
		out = append(out, msg)
	}
	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}

func (m *Mock) MLShadowStats(_ context.Context) (MLShadowStats, error) {
	st := MLShadowStats{}
	per := map[string]*MLLabelStat{}
	for _, msg := range sampleMLMessages() {
		st.Total++
		if msg.ModelLabel != nil {
			st.WithModel++
		}
		if msg.Agree != nil && *msg.Agree {
			st.Agree++
		}
		ls, ok := per[msg.GeminiLabel]
		if !ok {
			ls = &MLLabelStat{Label: msg.GeminiLabel}
			per[msg.GeminiLabel] = ls
		}
		ls.Total++
		if msg.Agree != nil && *msg.Agree {
			ls.Agree++
		}
	}
	for _, label := range []string{"false", "invalid", "true"} {
		if ls, ok := per[label]; ok {
			st.PerLabel = append(st.PerLabel, *ls)
		}
	}
	return st, nil
}

func (m *Mock) VerifyMLMessage(_ context.Context, _ int64, _ *string) error { return nil }

// --- Manueller ML-Test: Mock ---

func (m *Mock) InsertMLTest(_ context.Context, _, _ string, _ float64) (int64, error) {
	return 1, nil
}

func (m *Mock) ListMLTests(_ context.Context, limit int) ([]MLTestMessage, error) {
	s := func(v string) *string { return &v }
	out := []MLTestMessage{
		{ID: 3, CreatedAt: time.Date(2026, 7, 27, 10, 12, 0, 0, time.Local),
			Message: "i kimm heid ned, sorry", ModelLabel: "false", ModelConfidence: 0.93},
		{ID: 2, CreatedAt: time.Date(2026, 7, 27, 10, 10, 0, 0, time.Local),
			Message: "mal schauen ob i's schaff", ModelLabel: "invalid", ModelConfidence: 0.81,
			ExpectedLabel: s("invalid")},
		{ID: 1, CreatedAt: time.Date(2026, 7, 27, 10, 8, 0, 0, time.Local),
			Message: "bin am start heit", ModelLabel: "invalid", ModelConfidence: 0.52,
			ExpectedLabel: s("true")},
	}
	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}

func (m *Mock) JudgeMLTest(_ context.Context, _ int64, _ string) error { return nil }
