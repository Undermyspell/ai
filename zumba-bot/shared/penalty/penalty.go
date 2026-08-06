// Package penalty enthält die Domain-Logik des Strafen-Features.
//
// Zwei Strafarten:
//   - "fehltage": automatisch – ab 5 abgemeldeten Donnerstagen in Folge 25 €,
//     jeder weitere Fehltag +5 €. Persistiert wird nur ein Marker (userId +
//     erster Fehltag der Serie); der Betrag wird immer dynamisch aus den
//     Abwesenheiten berechnet.
//   - "noshow": manuell im Admin-UI gepflegt (nicht abgemeldet und nicht
//     gekommen), fester Betrag (Default 50 €).
//
// Serien-Semantik: eine Serie ("Segment") ist eine Folge aufeinanderfolgender
// abgemeldeter Donnerstage. Sie wird beendet durch Anwesenheit ODER durch
// einen Reset-Zeitpunkt (Begleichen/Löschen einer Fehltage-Strafe des Users):
// nach dem Begleichen zählt der Zähler wieder bei 1 (kniffliger Fall aus der
// Anforderung: 6x gefehlt → 30 € → abends beglichen → nächster Fehltag ist
// Serie 1, nicht 35 €).
//
// Dieses Package lebt im shared-Modul und ist die EINZIGE Kopie der
// Strafen-Domain-Logik – whatsapp-bot, zumba-admin-ui und wrapped importieren
// es (früher wortgleich 3× dupliziert).
package penalty

import (
	"sort"
	"time"
)

type Art string

const (
	ArtFehltage Art = "fehltage"
	ArtNoShow   Art = "noshow"
)

type Status string

const (
	StatusOffen     Status = "offen"
	StatusBeglichen Status = "beglichen"
	StatusGeloescht Status = "geloescht"
)

// Beträge in ganzen Euro.
const (
	MinFehltage    = 5  // ab so vielen Fehltagen in Folge greift die Strafe
	BasisBetrag    = 25 // Betrag bei genau MinFehltage
	ProTagBetrag   = 5  // jeder weitere Fehltag
	NoShowDefault  = 50 // Default für manuelle No-Show-Strafen
	clampStartDate = "2025-12-01"
)

// Row ist eine persistierte Zeile der Tabelle strafen.
type Row struct {
	ID          int64
	UserID      string
	Art         Art
	Datum       time.Time // fehltage: 1. Fehltag der Serie; noshow: Tag des No-Shows
	Betrag      int       // Euro; nur für noshow gesetzt (fehltage: dynamisch)
	Status      Status
	CreatedAt   time.Time
	BeglichenAm *time.Time
	GeloeschtAm *time.Time
}

// UserData sind die Eingangsdaten eines Users für die Bewertung.
type UserData struct {
	UserID         string
	Name           string
	EffectiveStart time.Time // GREATEST(startDate, 2025-12-01)
	Absences       []time.Time
}

// Input bündelt alles, was Assess braucht.
type Input struct {
	Users    []UserData
	Excluded []time.Time
	Rows     []Row // alle strafen-Zeilen (inkl. beglichen/geloescht)
}

// Entry ist eine bewertete Strafe. ID == 0 bedeutet: automatische Strafe, die
// erkannt, aber noch nicht persistiert ist (Kandidat für den Marker-Insert).
type Entry struct {
	ID          int64
	UserID      string
	Name        string
	Art         Art
	Datum       time.Time
	Tage        int // nur fehltage: Länge der Serie
	Betrag      int // Euro, berechnet bzw. Row-Betrag
	Status      Status
	BeglichenAm *time.Time
}

// Segment ist eine Serie aufeinanderfolgender Fehltage.
type Segment struct {
	Start time.Time
	Tage  int
}

// ClampStart liefert den effektiven Startpunkt eines Users (Domänen-Konvention:
// nie vor dem 01.12.2025).
func ClampStart(startDate *time.Time) time.Time {
	clamp, _ := time.Parse("2006-01-02", clampStartDate)
	if startDate == nil || startDate.Before(clamp) {
		return clamp
	}
	return *startDate
}

// Betrag berechnet den Strafbetrag einer Fehltage-Serie (0 = keine Strafe).
func Betrag(tage int) int {
	if tage < MinFehltage {
		return 0
	}
	return BasisBetrag + (tage-MinFehltage)*ProTagBetrag
}

// Thursdays liefert alle gültigen Donnerstage in [start, asOf] aufsteigend,
// ohne Sperrtage.
func Thursdays(start, asOf time.Time, excluded map[string]bool) []time.Time {
	var out []time.Time
	d := dateOnly(start)
	end := dateOnly(asOf)
	for ; !d.After(end); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Thursday && !excluded[iso(d)] {
			out = append(out, d)
		}
	}
	return out
}

// NextThursday liefert den nächsten Donnerstag STRIKT nach t (Tagesbasis).
// Fällt t auf einen Donnerstag, ist das Ergebnis t+7 Tage. Bis einschließlich
// dieses Tages werden beglichene Strafen noch im Report ausgewiesen.
func NextThursday(t time.Time) time.Time {
	d := dateOnly(t).AddDate(0, 0, 1)
	for d.Weekday() != time.Thursday {
		d = d.AddDate(0, 0, 1)
	}
	return d
}

// Segments zerlegt die Donnerstage eines Users in Fehltag-Serien. resets sind
// Begleich-/Lösch-Zeitpunkte von Fehltage-Strafen des Users: liegt ein Reset
// (auf Tagesbasis) zwischen zwei aufeinanderfolgenden Fehltagen [a, b), wird
// die Serie zwischen a und b geschnitten.
func Segments(thursdays []time.Time, absent map[string]bool, resets []time.Time) []Segment {
	resetDays := make([]time.Time, 0, len(resets))
	for _, r := range resets {
		resetDays = append(resetDays, dateOnly(r))
	}
	sort.Slice(resetDays, func(i, j int) bool { return resetDays[i].Before(resetDays[j]) })

	var (
		segs       []Segment
		cur        *Segment
		lastAbsent time.Time
	)
	closeCur := func() {
		if cur != nil {
			segs = append(segs, *cur)
			cur = nil
		}
	}
	for _, t := range thursdays {
		if !absent[iso(t)] {
			closeCur()
			continue
		}
		if cur != nil && resetBetween(resetDays, lastAbsent, t) {
			closeCur()
		}
		if cur == nil {
			cur = &Segment{Start: t}
		}
		cur.Tage++
		lastAbsent = t
	}
	closeCur()
	return segs
}

// resetBetween: existiert ein Reset r mit a <= r < b?
// (Beglichen am Abend von Fehltag a schneidet damit VOR dem nächsten Fehltag –
// a zählt noch zur alten Strafe, b beginnt die neue Serie.)
func resetBetween(resetDays []time.Time, a, b time.Time) bool {
	for _, r := range resetDays {
		if !r.Before(a) && r.Before(b) {
			return true
		}
	}
	return false
}

// Assess bewertet alle User: persistierte Strafen bekommen ihren berechneten
// Betrag, erkannte aber noch nicht persistierte Fehltage-Strafen kommen als
// Kandidaten (ID == 0, Status offen) dazu.
func Assess(in Input, asOf time.Time) []Entry {
	excluded := make(map[string]bool, len(in.Excluded))
	for _, d := range in.Excluded {
		excluded[iso(d)] = true
	}
	rowsByUser := make(map[string][]Row)
	for _, r := range in.Rows {
		rowsByUser[r.UserID] = append(rowsByUser[r.UserID], r)
	}

	var out []Entry
	for _, u := range in.Users {
		rows := rowsByUser[u.UserID]

		absent := make(map[string]bool, len(u.Absences))
		for _, a := range u.Absences {
			absent[iso(a)] = true
		}
		var resets []time.Time
		for _, r := range rows {
			if r.Art != ArtFehltage {
				continue
			}
			if r.BeglichenAm != nil {
				resets = append(resets, *r.BeglichenAm)
			}
			if r.GeloeschtAm != nil {
				resets = append(resets, *r.GeloeschtAm)
			}
		}

		thursdays := Thursdays(u.EffectiveStart, asOf, excluded)
		segs := Segments(thursdays, absent, resets)
		segByStart := make(map[string]Segment, len(segs))
		for _, s := range segs {
			segByStart[iso(s.Start)] = s
		}

		claimed := make(map[string]bool)
		for _, r := range rows {
			e := Entry{
				ID: r.ID, UserID: u.UserID, Name: u.Name, Art: r.Art,
				Datum: r.Datum, Status: r.Status, BeglichenAm: r.BeglichenAm,
			}
			switch r.Art {
			case ArtNoShow:
				e.Betrag = r.Betrag
			case ArtFehltage:
				claimed[iso(r.Datum)] = true
				seg, ok := segByStart[iso(r.Datum)]
				if !ok {
					// Serie existiert nicht (mehr) – z. B. Abwesenheiten im
					// Admin-UI nachträglich korrigiert. Nicht ausweisen.
					continue
				}
				e.Tage = seg.Tage
				e.Betrag = Betrag(seg.Tage)
				if e.Betrag == 0 {
					continue
				}
			}
			out = append(out, e)
		}

		for _, seg := range segs {
			if seg.Tage < MinFehltage || claimed[iso(seg.Start)] {
				continue
			}
			out = append(out, Entry{
				UserID: u.UserID, Name: u.Name, Art: ArtFehltage,
				Datum: seg.Start, Tage: seg.Tage, Betrag: Betrag(seg.Tage),
				Status: StatusOffen,
			})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Datum.Before(out[j].Datum)
	})
	return out
}

// VisibleAt entscheidet, ob eine Strafe zum Stichtag im Report erscheint:
// offene immer, beglichene von der Begleichung bis einschließlich zum
// Folgedonnerstag, gelöschte nie.
func VisibleAt(e Entry, asOf time.Time) bool {
	switch e.Status {
	case StatusOffen:
		return true
	case StatusBeglichen:
		if e.BeglichenAm == nil {
			return false
		}
		day := dateOnly(asOf)
		return !day.Before(dateOnly(*e.BeglichenAm)) && !day.After(NextThursday(*e.BeglichenAm))
	default:
		return false
	}
}

func iso(t time.Time) string { return t.Format("2006-01-02") }

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
