package report

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"sort"
	"strings"
	"time"

	"github.com/michael/zumba-shared/penalty"
	"github.com/michael/zumba-whatsapp-bot/internal/store"
)

// card.go rendert die Statistik als HTML-"Bild-Karte" (720px breit) im
// Wrapped-Look (Holz/Biergold/Schaum). Das HTML ist self-contained
// (Inline-CSS, eingebetteter Display-Font) und wird vom renderer-service
// per headless Chromium zu einem PNG geschossen.

//go:embed card.tmpl
var cardTmplSrc string

// Anton (latin-Subset inkl. Umlaute) als eingebetteter Display-Font: der
// Renderer-Container hat keinen Netzzugriff, Google-Fonts-Links scheiden aus.
//
//go:embed assets/anton-latin.woff2
var antonWoff2 []byte

var cardTmpl = template.Must(template.New("card").Parse(cardTmplSrc))

// CardWidth ist die Viewport-Breite, mit der die Karte gerendert werden muss.
const CardWidth = 720

type cardUser struct {
	Medal      string // 🥇/🥈/🥉, sonst leer (dann zählt Rank)
	Rank       int
	Name       string
	Attendance int
	Away       int
	Percent    string  // formatiert, ohne %-Zeichen
	PercentVal float64 // für die Balkenbreite
	StreakTag  string  // "🔥+4" / "❄️-2" / leer
	Top3       bool
}

type cardStrafe struct {
	Icon      string
	Name      string
	Grund     string
	Betrag    int
	Beglichen bool
}

type cardData struct {
	WeeklyNote bool
	Datum      string
	Total      int

	GoatName    string
	GoatPercent string

	MaxStreak      int
	MaxStreakNames string
	MaxFlame       string
	MinStreak      int // als positive Zahl (Pause-Länge)
	MinStreakNames string
	MinIce         string

	Users   []cardUser
	Strafen []cardStrafe

	AntonDataURL template.URL
}

// BuildCardHTML baut das self-contained HTML der Statistik-Karte. entries
// dürfen leer sein (dann erscheint die "Keine offenen Strafen"-Zeile);
// weekly stellt die Wochenreport-Kopfzeile voran.
func BuildCardHTML(rows []store.Stat, entries []penalty.Entry, asOf time.Time, weekly bool) (string, error) {
	data := cardData{
		WeeklyNote:   weekly,
		Datum:        fmt.Sprintf("%d.%d.%d", asOf.Day(), int(asOf.Month()), asOf.Year()),
		AntonDataURL: template.URL("data:font/woff2;base64," + base64.StdEncoding.EncodeToString(antonWoff2)),
	}

	if len(rows) > 0 {
		a := analyze(rows)
		data.Total = a.total
		data.GoatName = a.mvp.Name
		data.GoatPercent = fmtNum(a.mvp.Percent)

		maxStreak, minStreak := a.hottest.Streak, a.coldest.Streak
		streakNames := func(streak int) string {
			var names []string
			for _, u := range a.users {
				if u.Streak == streak {
					names = append(names, u.Name)
				}
			}
			return strings.Join(names, ", ")
		}
		if maxStreak > 0 {
			data.MaxStreak = maxStreak
			data.MaxStreakNames = streakNames(maxStreak)
			data.MaxFlame = hotEmoji(maxStreak)
		}
		if minStreak < 0 {
			data.MinStreak = abs(minStreak)
			data.MinStreakNames = streakNames(minStreak)
			data.MinIce = coldEmoji(minStreak)
		}

		for _, u := range a.users {
			medal := ""
			if u.rank <= 3 {
				medal = u.medal
			}
			tag := strings.TrimSpace(hotTag(u.Streak) + coldTag(u.Streak))
			data.Users = append(data.Users, cardUser{
				Medal: medal, Rank: u.rank, Name: u.Name,
				Attendance: u.Attendance, Away: u.Away,
				Percent: fmtNum(u.Percent), PercentVal: u.Percent,
				StreakTag: tag, Top3: u.rank <= 3,
			})
		}
	}

	// Sichtbarkeit + Sortierung wie StrafenBlock (offene zuerst).
	var visible []penalty.Entry
	for _, e := range entries {
		if penalty.VisibleAt(e, asOf) {
			visible = append(visible, e)
		}
	}
	sort.SliceStable(visible, func(i, j int) bool {
		oi, oj := visible[i].Status == penalty.StatusOffen, visible[j].Status == penalty.StatusOffen
		return oi && !oj
	})
	for _, e := range visible {
		var grund string
		switch e.Art {
		case penalty.ArtNoShow:
			grund = fmt.Sprintf("nicht abgemeldet, %s", fmtDate(e.Datum))
		default:
			grund = fmt.Sprintf("%dx in Folge gefehlt", e.Tage)
		}
		s := cardStrafe{Name: e.Name, Grund: grund, Betrag: e.Betrag}
		if e.Status == penalty.StatusBeglichen {
			s.Icon, s.Beglichen = "✅", true
		} else {
			s.Icon = "⚠️"
		}
		data.Strafen = append(data.Strafen, s)
	}

	var buf bytes.Buffer
	if err := cardTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("card template: %w", err)
	}
	return buf.String(), nil
}
