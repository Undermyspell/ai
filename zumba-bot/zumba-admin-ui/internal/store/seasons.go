package store

import "time"

// DefaultSeasons spiegelt den Seed aus shared/store.EnsureSeasonsSchema. Sie
// wird nur im Mock-Modus gebraucht (keine DB, aus der die gepflegten Jahre
// kommen könnten) – im Normalbetrieb ist die Tabelle die Quelle.
//
// Bleibt der Seed dort stehen, muss diese Liste mitwandern; abweichen darf sie
// nicht, sonst zeigt der Mock-Modus andere Zeiträume als der Echtbetrieb.
func DefaultSeasons() []Season {
	d := func(s string) time.Time {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			panic("DefaultSeasons: " + err.Error())
		}
		return t
	}
	mk := func(id int64, label, start, end string) Season {
		s := Season{ID: id, Label: label}
		s.Start, s.End = d(start), d(end)
		return s
	}
	// Neuestes zuerst – Reihenfolge des Jahres-Umschalters.
	return []Season{
		mk(3, "2027", "2026-12-01", "2027-11-30"),
		mk(2, "2026", "2025-12-01", "2026-11-30"),
		mk(1, "2025", "2024-12-01", "2025-11-30"),
	}
}
