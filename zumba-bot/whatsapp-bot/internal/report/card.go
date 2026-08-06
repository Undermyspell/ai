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

//go:embed card-bierdeckel.tmpl
var cardBierdeckelSrc string

//go:embed card-zeitung.tmpl
var cardZeitungSrc string

//go:embed card-arena.tmpl
var cardArenaSrc string

//go:embed card-tafel.tmpl
var cardTafelSrc string

//go:embed card-masskrug.tmpl
var cardMasskrugSrc string

// Display-Fonts als eingebettete latin-Subsets: der Renderer-Container hat
// keinen Netzzugriff, Google-Fonts-Links scheiden aus. Im Container selbst
// liegen nur Noto Sans/Serif, DejaVu Mono und Noto Color Emoji.
//
//go:embed assets/anton-latin.woff2
var antonWoff2 []byte

//go:embed assets/playfair-latin.woff2
var playfairWoff2 []byte

//go:embed assets/caveat-latin.woff2
var caveatWoff2 []byte

// Das offizielle Stammtisch-Emblem, kreisrund freigestellt (256px, PNG mit
// Alpha), damit es sich im Live-Design als Wappen neben den Titel setzen
// lässt, ohne als Kachel aufzufallen.
//
//go:embed assets/logo.png
var logoPNG []byte

// CardWidth ist die Viewport-Breite, mit der die Karte gerendert werden muss.
const CardWidth = 720

// DefaultCardStyle ist das Design des Live-Betriebs (Gruppen-Statistik und
// Wochenreport). Die übrigen Stile sind reine Bot-Test-Spielwiese.
const DefaultCardStyle = "wrapped"

// CardStyle ist ein auswählbares Design der Bild-Karte.
type CardStyle struct {
	ID    string
	Label string

	tmpl  *template.Template
	fonts func(*cardFonts)
	skin  string // Farbwelt innerhalb eines Templates (leer = Standard)
}

// CardStyles listet alle Bild-Designs; "wrapped" ist das Live-Design, der
// Rest ist Bot-Test-Spielwiese.
func CardStyles() []CardStyle {
	return []CardStyle{
		{ID: "wrapped", Label: "Wrapped (live)", tmpl: parseCard("wrapped", cardTmplSrc), fonts: withAnton},
		{ID: "bierdeckel", Label: "Bierdeckel hell", tmpl: parseCard("bierdeckel", cardBierdeckelSrc), fonts: withCaveat},
		{ID: "bierdeckel-dunkel", Label: "Bierdeckel dunkel", tmpl: parseCard("bierdeckel", cardBierdeckelSrc), fonts: withCaveat, skin: "dunkel"},
		{ID: "tafel", Label: "Kreidetafel", tmpl: parseCard("tafel", cardTafelSrc), fonts: withCaveat},
		{ID: "masskrug", Label: "Maßkrug", tmpl: parseCard("masskrug", cardMasskrugSrc), fonts: withCaveat},
		{ID: "zeitung", Label: "Zeitung", tmpl: parseCard("zeitung", cardZeitungSrc), fonts: withPlayfair},
		{ID: "arena", Label: "Arena", tmpl: parseCard("arena", cardArenaSrc), fonts: withAnton},
	}
}

func parseCard(name, src string) *template.Template {
	return template.Must(template.New(name).Parse(src))
}

// cardFonts hält die base64-Data-URLs der eingebetteten Fonts. Jeder Stil
// bekommt nur die, die sein Template braucht – sonst bläht das HTML unnötig.
type cardFonts struct {
	Anton    template.URL
	Playfair template.URL
	Caveat   template.URL
}

func fontURL(b []byte) template.URL {
	return template.URL("data:font/woff2;base64," + base64.StdEncoding.EncodeToString(b))
}

// logoURL ist das eingebettete Emblem als Data-URL (der Renderer-Container
// hat keinen Netzzugriff).
func logoURL() template.URL {
	return template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(logoPNG))
}

// monatDE liefert deutsche Monatsnamen für das ausgeschriebene Datum
// (time.Month.String() ist englisch).
var monatDE = map[time.Month]string{
	time.January: "Januar", time.February: "Februar", time.March: "März",
	time.April: "April", time.May: "Mai", time.June: "Juni",
	time.July: "Juli", time.August: "August", time.September: "September",
	time.October: "Oktober", time.November: "November", time.December: "Dezember",
}

func withAnton(f *cardFonts)    { f.Anton = fontURL(antonWoff2) }
func withPlayfair(f *cardFonts) { f.Playfair = fontURL(playfairWoff2) }
func withCaveat(f *cardFonts)   { f.Caveat = fontURL(caveatWoff2); f.Anton = fontURL(antonWoff2) }

type cardUser struct {
	Medal      string // 🥇/🥈/🥉, sonst leer (dann zählt Rank)
	Rank       int
	Name       string
	Attendance int
	Away       int
	Percent    string  // formatiert, ohne %-Zeichen
	PercentVal float64 // für die Balkenbreite
	Streak     int
	StreakAbs  int    // Betrag der Serie (Designs setzen das Vorzeichen selbst)
	StreakTag  string // "🔥+4" / "❄️-2" / leer
	Top3       bool

	// Go-Templates können nicht n-mal zählen, deshalb kommen die
	// Wiederholungen fertig aus dem Code:
	//   Bundles/Rest — Strichliste in Fünferbündeln ("bierdeckel"). Die
	//     Striche der laufenden Anwesenheitsserie sind markiert, sie sind
	//     definitionsgemäß die zuletzt gemachten.
	//   Pausen — ein Kreuz je Termin der laufenden Fehl-Serie.
	//   Lit/Dark — ein Slot je Stammtisch, anwesend/gefehlt ("arena").
	Bundles []cardBundle
	Rest    []cardStroke
	Pausen  []int
	Lit     []int
	Dark    []int
}

// cardStroke ist ein einzelner Strich der Strichliste.
type cardStroke struct{ Serie bool }

// cardBundle ist ein Fünferbündel; Serie markiert den Querstrich, wenn das
// ganze Bündel zur laufenden Serie gehört.
type cardBundle struct {
	Strokes []cardStroke
	Serie   bool
}

// strichliste zerlegt die Anwesenheiten in Fünferbündel plus Rest und
// markiert die letzten streak Striche als laufende Serie.
func strichliste(attendance, streak int) ([]cardBundle, []cardStroke) {
	if streak < 0 {
		streak = 0
	}
	if streak > attendance {
		streak = attendance
	}
	abSerie := attendance - streak // Index, ab dem die Serie läuft

	strokes := make([]cardStroke, attendance)
	for i := range strokes {
		strokes[i] = cardStroke{Serie: i >= abSerie}
	}

	var bundles []cardBundle
	for i := 0; i+5 <= attendance; i += 5 {
		b := cardBundle{Strokes: strokes[i : i+5], Serie: i >= abSerie}
		bundles = append(bundles, b)
	}
	return bundles, strokes[attendance-attendance%5:]
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
	Datum      string // "6.8.2026"
	DatumLang  string // "6. August 2026"
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

	Skin  string       // Farbwelt-Variante des gewählten Designs
	Fonts cardFonts
	Logo  template.URL // Stammtisch-Emblem als Data-URL
}

// BuildCardHTML baut die Karte im Live-Design (siehe DefaultCardStyle).
func BuildCardHTML(rows []store.Stat, entries []penalty.Entry, asOf time.Time, weekly bool) (string, error) {
	return BuildCardHTMLByStyle(DefaultCardStyle, rows, entries, asOf, weekly)
}

// BuildCardHTMLByStyle baut das self-contained HTML der Statistik-Karte im
// gewählten Design (unbekannt/leer → Live-Design). entries dürfen leer sein
// (dann erscheint die "Keine offenen Strafen"-Zeile); weekly stellt den
// Wochenreport-Hinweis voran.
func BuildCardHTMLByStyle(style string, rows []store.Stat, entries []penalty.Entry, asOf time.Time, weekly bool) (string, error) {
	styles := CardStyles()
	sel := styles[0]
	for _, s := range styles {
		if s.ID == style {
			sel = s
			break
		}
	}

	data := cardData{
		WeeklyNote: weekly,
		Datum:      fmt.Sprintf("%d.%d.%d", asOf.Day(), int(asOf.Month()), asOf.Year()),
		DatumLang:  fmt.Sprintf("%d. %s %d", asOf.Day(), monatDE[asOf.Month()], asOf.Year()),
		Skin:       sel.skin,
		Logo:       logoURL(),
	}
	sel.fonts(&data.Fonts)

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
			bundles, rest := strichliste(u.Attendance, u.Streak)
			pausen := 0
			if u.Streak < 0 {
				pausen = abs(u.Streak)
			}
			data.Users = append(data.Users, cardUser{
				Medal: medal, Rank: u.rank, Name: u.Name,
				Attendance: u.Attendance, Away: u.Away,
				Percent: fmtNum(u.Percent), PercentVal: u.Percent,
				Streak: u.Streak, StreakAbs: abs(u.Streak), StreakTag: tag, Top3: u.rank <= 3,
				Bundles: bundles,
				Rest:    rest,
				Pausen:  make([]int, pausen),
				Lit:     make([]int, u.Attendance),
				Dark:    make([]int, u.Away),
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
	if err := sel.tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("card template %q: %w", sel.ID, err)
	}
	return buf.String(), nil
}
