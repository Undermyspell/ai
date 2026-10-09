package report

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/michael/zumba-shared/cards"
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

//go:embed card-stempelkarte.tmpl
var cardStempelkarteSrc string

//go:embed card-sammelkarten.tmpl
var cardSammelkartenSrc string

//go:embed card-formular.tmpl
var cardFormularSrc string

//go:embed card-abfahrtstafel.tmpl
var cardAbfahrtstafelSrc string

//go:embed card-kassenbon.tmpl
var cardKassenbonSrc string

//go:embed card-gipfelbuch.tmpl
var cardGipfelbuchSrc string

//go:embed card-wetterbericht.tmpl
var cardWetterberichtSrc string

//go:embed card-hochrechnung.tmpl
var cardHochrechnungSrc string

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

// Barlow Condensed und Space Mono gibt es nur als statische Schnitte – je
// Gewicht eine Datei.
//
//go:embed assets/barlow-condensed-500-latin.woff2
var barlow500Woff2 []byte

//go:embed assets/barlow-condensed-600-latin.woff2
var barlow600Woff2 []byte

//go:embed assets/barlow-condensed-700-latin.woff2
var barlow700Woff2 []byte

//go:embed assets/barlow-condensed-800-latin.woff2
var barlow800Woff2 []byte

//go:embed assets/space-mono-400-latin.woff2
var spaceMono400Woff2 []byte

//go:embed assets/space-mono-700-latin.woff2
var spaceMono700Woff2 []byte

// Das offizielle Stammtisch-Emblem, kreisrund freigestellt (256px, PNG mit
// Alpha). Jedes Design führt es — mal als Wappen neben dem Titel, mal als
// Dienstsiegel oder Stempel —, damit die Karte immer als Stammtisch-Karte
// erkennbar ist.
//
//go:embed assets/logo.png
var logoPNG []byte

// Der Zumba-Bot selbst (der Raspberry Pi, auf dem alles läuft), als
// 4:3-Ausschnitt (320×240 JPEG). Jedes Design zeigt ihn mit leicht
// gerundeten Ecken neben der Bot-Zeile im Fuß – als "Absender" der Karte.
//
//go:embed assets/zumba-bot.jpg
var botJPG []byte

// CardWidth ist die Viewport-Breite, mit der die Karte gerendert werden muss.
const CardWidth = 720

// DefaultCardStyle ist das Live-Design – es kommt, wenn keine Rotation
// gepflegt ist und für unbekannte IDs.
const DefaultCardStyle = cards.Default

// CardStyle ist ein auswählbares Design der Bild-Karte.
type CardStyle struct {
	ID    string
	Label string

	tmpl  *template.Template
	fonts func(*cardFonts)
	skin  string // Farbwelt innerhalb eines Templates (leer = Standard)
}

// CardStyles listet alle Bild-Designs in Katalog-Reihenfolge (IDs und Labels
// wie shared/cards). Welche davon im Umlauf sind, pflegt das Admin-UI.
func CardStyles() []CardStyle {
	return []CardStyle{
		{ID: "wrapped", Label: "Wrapped (live)", tmpl: parseCard("wrapped", cardTmplSrc), fonts: withAnton},
		{ID: "bierdeckel", Label: "Bierdeckel hell", tmpl: parseCard("bierdeckel", cardBierdeckelSrc), fonts: withCaveat},
		{ID: "bierdeckel-dunkel", Label: "Bierdeckel dunkel", tmpl: parseCard("bierdeckel", cardBierdeckelSrc), fonts: withCaveat, skin: "dunkel"},
		{ID: "tafel", Label: "Kreidetafel", tmpl: parseCard("tafel", cardTafelSrc), fonts: withCaveat},
		{ID: "masskrug", Label: "Maßkrug", tmpl: parseCard("masskrug", cardMasskrugSrc), fonts: withCaveat},
		{ID: "zeitung", Label: "Zeitung", tmpl: parseCard("zeitung", cardZeitungSrc), fonts: withPlayfair},
		{ID: "arena", Label: "Arena", tmpl: parseCard("arena", cardArenaSrc), fonts: withAnton},
		{ID: "stempelkarte", Label: "Treuekarte", tmpl: parseCard("stempelkarte", cardStempelkarteSrc), fonts: withCaveat},
		{ID: "sammelkarten", Label: "Sammelalbum", tmpl: parseCard("sammelkarten", cardSammelkartenSrc), fonts: withAnton},
		{ID: "formular", Label: "Amtsformular", tmpl: parseCard("formular", cardFormularSrc), fonts: withAnton},
		{ID: "abfahrtstafel", Label: "Abfahrtstafel", tmpl: parseCard("abfahrtstafel", cardAbfahrtstafelSrc), fonts: fontMix(withAnton, withBarlow, withSpaceMono)},
		{ID: "kassenbon", Label: "Kassenbon", tmpl: parseCard("kassenbon", cardKassenbonSrc), fonts: withSpaceMono},
		{ID: "gipfelbuch", Label: "Gipfelbuch", tmpl: parseCard("gipfelbuch", cardGipfelbuchSrc), fonts: withCaveat},
		{ID: "wetterbericht", Label: "Wetterbericht", tmpl: parseCard("wetterbericht", cardWetterberichtSrc), fonts: fontMix(withAnton, withBarlow, withSpaceMono)},
		{ID: "hochrechnung", Label: "Hochrechnung", tmpl: parseCard("hochrechnung", cardHochrechnungSrc), fonts: fontMix(withAnton, withBarlow, withSpaceMono)},
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

	// Schriften mit mehreren statischen Schnitten: das Template legt je
	// Eintrag ein @font-face an.
	Barlow    []fontFace // Barlow Condensed
	SpaceMono []fontFace
}

// fontFace ist ein einzelner Schnitt einer Schrift.
type fontFace struct {
	Weight int
	URL    template.URL
}

func fontURL(b []byte) template.URL {
	return template.URL("data:font/woff2;base64," + base64.StdEncoding.EncodeToString(b))
}

// logoURL ist das eingebettete Emblem als Data-URL (der Renderer-Container
// hat keinen Netzzugriff). Designs, die das Emblem vielfach zeigen
// (Treuekarte, Sammelalbum), legen es einmal in eine CSS-Variable statt die
// Data-URL je Element zu wiederholen.
func logoURL() template.URL {
	return template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(logoPNG))
}

// botURL ist das eingebettete Bot-Bild als Data-URL (Signet im Kartenfuß).
func botURL() template.URL {
	return template.URL("data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(botJPG))
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

func withBarlow(f *cardFonts) {
	f.Barlow = []fontFace{
		{500, fontURL(barlow500Woff2)}, {600, fontURL(barlow600Woff2)},
		{700, fontURL(barlow700Woff2)}, {800, fontURL(barlow800Woff2)},
	}
}

func withSpaceMono(f *cardFonts) {
	f.SpaceMono = []fontFace{{400, fontURL(spaceMono400Woff2)}, {700, fontURL(spaceMono700Woff2)}}
}

// fontMix kombiniert mehrere Font-Helfer für Designs mit mehreren Schriften.
func fontMix(fs ...func(*cardFonts)) func(*cardFonts) {
	return func(f *cardFonts) {
		for _, with := range fs {
			with(f)
		}
	}
}

type cardUser struct {
	Medal      string // 🥇/🥈/🥉, sonst leer (dann zählt Rank)
	Rank       int
	Name       string
	Attendance int
	Away       int
	Percent    string  // formatiert, ohne %-Zeichen
	PercentVal float64 // für die Balkenbreite
	Quote      int     // Percent gerundet (Designs mit ganzzahliger Quote)
	Streak     int
	StreakAbs  int    // Betrag der Serie (Designs setzen das Vorzeichen selbst)
	StreakTag  string // "🔥+4" / "❄️-2" / leer
	Top3       bool
	NameLang   bool       // mehr als 10 Zeichen: schmale Kacheln setzen ihn kleiner
	Wetter     wetterLage // Serie als Wetter ("wetterbericht")

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

	// Bergprofil ("gipfelbuch"): RidgeX ist die Fahnenposition in Prozent
	// der Breite (Rang gleichmäßig verteilt), Letzter bekommt wie die Top 3
	// den Namen statt der Platzziffer. FahnePole (Stangenlänge in px) und
	// FahneSeite (Schild mittig/"rechts"/"links") legt gipfelFahnen so fest,
	// dass sich die Schilder nicht überdecken.
	RidgeX     float64
	Letzter    bool
	FahnePole  int
	FahneSeite string
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
	Warnstufe int // 1–4 nach Betrag (DWD-Farben im "wetterbericht")
}

type cardData struct {
	WeeklyNote bool
	Datum      string // "6.8.2026"
	DatumLang  string // "6. August 2026"
	Year       string // Stammtischjahr ("2026")
	Total      int

	AvgPercent  int // Ø-Quote aller Mitglieder, gerundet
	TotalAttend int // Summe aller Anwesenheiten
	TotalAway   int // Summe aller Absagen
	SitzGrad    int // Anteil der Anwesenheiten als Winkel eines Halbkreises (0–180)
	OffenSum    int // Summe der offenen Strafen in €
	Warnstufe   int // höchste Warnstufe der offenen Strafen (0 = keine offen)

	// Ridge ist das clip-path-Polygon des Bergprofils ("gipfelbuch"): ein
	// Punkt je Mitglied, Höhe = Quote. RidgeKopf ist der Abstand des Profils
	// zum Kopf in px – genug Himmel für die höchste Fahne.
	Ridge     template.CSS
	RidgeKopf int

	GoatName    string
	GoatPercent string
	GoatQuote   int // GoatPercent gerundet

	MaxStreak      int
	MaxStreakNames string
	MaxFlame       string
	MinStreak      int // als positive Zahl (Pause-Länge)
	MinStreakNames string
	MinIce         string

	Users   []cardUser
	Strafen []cardStrafe

	Skin  string // Farbwelt-Variante des gewählten Designs
	Fonts cardFonts
	Logo  template.URL // Stammtisch-Emblem als Data-URL
	Bot   template.URL // Bild des Zumba-Bots als Data-URL
}

// BuildCardHTML baut die Karte im Live-Design (siehe DefaultCardStyle).
func BuildCardHTML(rows []store.Stat, entries []penalty.Entry, asOf time.Time, weekly bool) (string, error) {
	return BuildCardHTMLByStyle(DefaultCardStyle, rows, entries, asOf, "", weekly)
}

// BuildCardHTMLByStyle baut das self-contained HTML der Statistik-Karte im
// gewählten Design (unbekannt/leer → Live-Design). entries dürfen leer sein
// (dann erscheint die "Keine offenen Strafen"-Zeile); season ist das Label
// des Stammtischjahres ("2026", leer → Kalenderjahr von asOf); weekly stellt
// den Wochenreport-Hinweis voran.
func BuildCardHTMLByStyle(style string, rows []store.Stat, entries []penalty.Entry, asOf time.Time, season string, weekly bool) (string, error) {
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
		Year:       season,
		Skin:       sel.skin,
		Logo:       logoURL(),
		Bot:        botURL(),
	}
	if data.Year == "" {
		data.Year = fmt.Sprint(asOf.Year())
	}
	sel.fonts(&data.Fonts)

	if len(rows) > 0 {
		a := analyze(rows)
		data.Total = a.total
		data.AvgPercent = a.avgPercent
		data.GoatName = a.mvp.Name
		data.GoatPercent = fmtNum(a.mvp.Percent)
		data.GoatQuote = int(math.Round(a.mvp.Percent))

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

		n := len(a.users)
		for i, u := range a.users {
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
				Percent: fmtNum(u.Percent), PercentVal: u.Percent, Quote: int(math.Round(u.Percent)),
				Streak: u.Streak, StreakAbs: abs(u.Streak), StreakTag: tag, Top3: u.rank <= 3,
				NameLang: utf8.RuneCountInString(u.Name) > 10,
				Wetter:   wetterFuer(u.Streak),
				Bundles:  bundles,
				Rest:     rest,
				Pausen:   make([]int, pausen),
				Lit:      make([]int, u.Attendance),
				Dark:     make([]int, u.Away),
				RidgeX:   ridgeX(i, n),
				Letzter:  i == n-1,
			})
			data.TotalAttend += u.Attendance
			data.TotalAway += u.Away
		}
		if sum := data.TotalAttend + data.TotalAway; sum > 0 {
			data.SitzGrad = int(math.Round(float64(data.TotalAttend) / float64(sum) * 180))
		}
		data.Ridge = ridgePolygon(data.Users)
		data.RidgeKopf = gipfelFahnen(data.Users)
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
		s := cardStrafe{Name: e.Name, Grund: grund, Betrag: e.Betrag, Warnstufe: warnstufe(e.Betrag)}
		if e.Status == penalty.StatusBeglichen {
			s.Icon, s.Beglichen = "✅", true
		} else {
			s.Icon = "⚠️"
			data.OffenSum += e.Betrag
			data.Warnstufe = max(data.Warnstufe, s.Warnstufe)
		}
		data.Strafen = append(data.Strafen, s)
	}

	var buf bytes.Buffer
	if err := sel.tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("card template %q: %w", sel.ID, err)
	}
	return buf.String(), nil
}

// ridgeX verteilt die Ränge gleichmäßig über die Breite des Bergprofils
// (0–100 %); ein einzelnes Mitglied steht in der Mitte.
func ridgeX(i, n int) float64 {
	if n < 2 {
		return 50
	}
	return float64(i) / float64(n-1) * 100
}

// ridgePolygon baut den Bergkörper des "gipfelbuch": ein Punkt je Mitglied
// bei (RidgeX, Quote), links und rechts etwas abfallend zur Grundlinie.
func ridgePolygon(users []cardUser) template.CSS {
	if len(users) == 0 {
		return ""
	}
	hoehe := func(u cardUser, abfall float64) float64 {
		return math.Min(100, 100-u.PercentVal+abfall)
	}
	first, last := users[0], users[len(users)-1]
	pts := []string{"0% 100%", fmt.Sprintf("0%% %.1f%%", hoehe(first, 6))}
	for _, u := range users {
		pts = append(pts, fmt.Sprintf("%.1f%% %.1f%%", u.RidgeX, hoehe(u, 0)))
	}
	pts = append(pts, fmt.Sprintf("100%% %.1f%%", hoehe(last, 8)), "100% 100%")
	return template.CSS("polygon(" + strings.Join(pts, ", ") + ")")
}
