package eval2026

import (
	"sort"
	"time"

	"github.com/michael/stammtisch-wrapped/internal/penalty"
	"github.com/michael/stammtisch-wrapped/pkg/models"
)

// calculateStrafenStats evaluates the strafen table with the shared penalty
// domain logic (same rules as whatsapp-bot / admin-ui) and aggregates the
// result per user for the wrapped page. Deleted penalties are excluded from
// the recap (they only act as reset markers inside penalty.Assess).
func (e *Evaluator) calculateStrafenStats() models.StrafenStats {
	if len(e.rawData.Thursdays) == 0 {
		return models.StrafenStats{}
	}
	// asOf = last valid Thursday of the (today-capped) period
	asOf := e.rawData.Thursdays[len(e.rawData.Thursdays)-1]

	excluded := make([]time.Time, 0, len(e.rawData.ExcludedDays))
	excludedSet := make(map[string]bool, len(e.rawData.ExcludedDays))
	for _, d := range e.rawData.ExcludedDays {
		excluded = append(excluded, d.Date)
		excludedSet[d.Date.Format("2006-01-02")] = true
	}

	absencesByUser := make(map[string][]time.Time)
	for _, r := range e.rawData.Rejections {
		absencesByUser[r.UserID] = append(absencesByUser[r.UserID], r.Date)
	}

	users := make([]penalty.UserData, 0, len(e.rawData.Users))
	for _, u := range e.rawData.Users {
		users = append(users, penalty.UserData{
			UserID:         u.UserID,
			Name:           u.UserName,
			EffectiveStart: penalty.ClampStart(u.StartDate),
			Absences:       absencesByUser[u.UserID],
		})
	}

	rows := make([]penalty.Row, 0, len(e.rawData.StrafenRows))
	for _, s := range e.rawData.StrafenRows {
		betrag := 0
		if s.Betrag != nil {
			betrag = *s.Betrag
		}
		rows = append(rows, penalty.Row{
			ID:          s.ID,
			UserID:      s.UserID,
			Art:         penalty.Art(s.Art),
			Datum:       s.Datum,
			Betrag:      betrag,
			Status:      penalty.Status(s.Status),
			BeglichenAm: s.BeglichenAm,
			GeloeschtAm: s.GeloeschtAm,
		})
	}

	entries := penalty.Assess(penalty.Input{
		Users:    users,
		Excluded: excluded,
		Rows:     rows,
	}, asOf)

	byUser := make(map[string]*models.StrafenUserTotal)
	stats := models.StrafenStats{}

	for _, entry := range entries {
		if entry.Status == penalty.StatusGeloescht || entry.Betrag == 0 {
			continue
		}

		me := models.StrafenEntry{
			UserName: entry.Name,
			Art:      string(entry.Art),
			Betrag:   entry.Betrag,
			Tage:     entry.Tage,
			Start:    entry.Datum,
			Status:   string(entry.Status),
		}
		if entry.Art == penalty.ArtFehltage {
			me.End = streakEnd(entry.Datum, entry.Tage, asOf, excludedSet)
		}

		ut, ok := byUser[entry.UserID]
		if !ok {
			ut = &models.StrafenUserTotal{UserName: entry.Name}
			byUser[entry.UserID] = ut
		}
		ut.Total += me.Betrag
		if entry.Art == penalty.ArtFehltage {
			ut.FehltageCount++
		} else {
			ut.NoShowCount++
		}
		ut.Entries = append(ut.Entries, me)

		stats.TotalSum += me.Betrag
		stats.TotalCount++
	}

	for _, ut := range byUser {
		sort.Slice(ut.Entries, func(i, j int) bool {
			return ut.Entries[i].Start.Before(ut.Entries[j].Start)
		})
		stats.UserTotals = append(stats.UserTotals, *ut)
	}
	sort.Slice(stats.UserTotals, func(i, j int) bool {
		if stats.UserTotals[i].Total != stats.UserTotals[j].Total {
			return stats.UserTotals[i].Total > stats.UserTotals[j].Total
		}
		return stats.UserTotals[i].UserName < stats.UserTotals[j].UserName
	})

	return stats
}

// streakEnd returns the last Thursday of a fehltage streak that starts at
// start and spans tage valid Thursdays (excluded days don't count).
func streakEnd(start time.Time, tage int, asOf time.Time, excluded map[string]bool) time.Time {
	thursdays := penalty.Thursdays(start, asOf, excluded)
	if tage <= 0 || len(thursdays) == 0 {
		return start
	}
	if tage > len(thursdays) {
		tage = len(thursdays)
	}
	return thursdays[tage-1]
}
