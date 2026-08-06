// Social/forensic fun-card evaluations. These live in the viewbuilder (not in
// evaluations/2026) so the mock path and the DB path share one implementation:
// everything here derives from EvalData only.
package viewbuilder

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/michael/stammtisch-wrapped/pkg/models"
	"github.com/michael/stammtisch-wrapped/web/templates/years/2026/viewmodels"
)

// presenceData is the shared per-user presence/absence matrix over all
// evaluated Thursdays. Users are assumed active for the whole period — true
// for the current roster, where all start dates lie before the period start.
type presenceData struct {
	names       []string // sorted user names
	emoji       map[string]string
	thursdays   []string          // ISO dates ascending
	absent      map[string]map[string]bool
	presentDays map[string][]string // per user: ISO dates present
}

func buildPresence(cancellations []models.Cancellation, users []models.UserStats, thursdayStats []models.ThursdayStat) presenceData {
	p := presenceData{
		emoji:       make(map[string]string, len(users)),
		absent:      make(map[string]map[string]bool),
		presentDays: make(map[string][]string),
	}
	for _, u := range users {
		p.emoji[u.Name] = u.Emoji
		p.names = append(p.names, u.Name)
	}
	sort.Strings(p.names)

	for _, t := range thursdayStats {
		p.thursdays = append(p.thursdays, t.Date.Format("2006-01-02"))
	}
	sort.Strings(p.thursdays)

	for _, c := range cancellations {
		if p.absent[c.UserName] == nil {
			p.absent[c.UserName] = make(map[string]bool)
		}
		p.absent[c.UserName][c.Date.Format("2006-01-02")] = true
	}
	for _, n := range p.names {
		for _, d := range p.thursdays {
			if !p.absent[n][d] {
				p.presentDays[n] = append(p.presentDays[n], d)
			}
		}
	}
	return p
}

func pairHeadline(p presenceData, a, b string) string {
	return fmt.Sprintf("%s %s & %s %s", p.emoji[a], a, p.emoji[b], b)
}

// buildDuoCards computes all pair findings and splits them into the
// "Dynamische Duos" slide and the "Verdächtige Duos" slide
func buildDuoCards(p presenceData, cancellations []models.Cancellation, thursdayStats []models.ThursdayStat) (duos []viewmodels.FunCard, suspects []viewmodels.FunCard) {
	if len(p.thursdays) == 0 || len(p.names) < 2 {
		return nil, nil
	}

	var cards []viewmodels.FunCard

	// Absage-Zwillinge: most shared absence Thursdays
	bestShared, na, nb := 0, "", ""
	// Wachablösung: both absent often, never together
	bestCombined, wa, wb := 0, "", ""
	// Unzertrennliche: most shared presence Thursdays
	bestTogether, ua, ub := 0, "", ""

	for i := 0; i < len(p.names); i++ {
		for j := i + 1; j < len(p.names); j++ {
			a, b := p.names[i], p.names[j]
			sharedAbsent := 0
			for d := range p.absent[a] {
				if p.absent[b][d] {
					sharedAbsent++
				}
			}
			if sharedAbsent >= 2 && sharedAbsent > bestShared {
				bestShared, na, nb = sharedAbsent, a, b
			}
			if sharedAbsent == 0 && len(p.absent[a]) >= 3 && len(p.absent[b]) >= 3 &&
				len(p.absent[a])+len(p.absent[b]) > bestCombined {
				bestCombined, wa, wb = len(p.absent[a])+len(p.absent[b]), a, b
			}
			together := 0
			for _, d := range p.presentDays[a] {
				if !p.absent[b][d] {
					together++
				}
			}
			if together > bestTogether {
				bestTogether, ua, ub = together, a, b
			}
		}
	}

	if bestTogether > 0 {
		cards = append(cards, viewmodels.FunCard{
			Emoji: "🤜🤛", Title: "Die Unzertrennlichen",
			Headline: pairHeadline(p, ua, ub),
			Detail:   fmt.Sprintf("%d× gemeinsam am Tisch – öfter als alle anderen", bestTogether),
			Gradient: "bg-gradient-to-r from-amber-500/25 to-yellow-500/15",
		})
	}
	if bestShared > 0 {
		cards = append(cards, viewmodels.FunCard{
			Emoji: "👯", Title: "Die Absage-Zwillinge",
			Headline: pairHeadline(p, na, nb),
			Detail:   fmt.Sprintf("%d× am selben Donnerstag gefehlt", bestShared),
			Gradient: "bg-gradient-to-r from-purple-500/25 to-pink-500/15",
		})
	}
	if bestCombined > 0 {
		cards = append(cards, viewmodels.FunCard{
			Emoji: "🔄", Title: "Die Wachablösung",
			Headline: pairHeadline(p, wa, wb),
			Detail:   fmt.Sprintf("zusammen %d Absagen – aber nie am selben Tag", bestCombined),
			Gradient: "bg-gradient-to-r from-blue-500/25 to-cyan-500/15",
		})
	}

	// Ping-Pong: longest strictly alternating presence run
	if card, ok := pingPongCard(p); ok {
		cards = append(cards, card)
	}

	// Suspects slide
	var sus []viewmodels.FunCard

	// Magnet & Schreck: B fehlt deutlich öfter, wenn A da ist
	if card, ok := magnetCard(p); ok {
		sus = append(sus, card)
	}

	// Alibi-Duo: same day, same excuse category
	if card, ok := alibiCard(cancellations, p); ok {
		sus = append(sus, card)
	}

	// Copy-Paste-Duo: same verbatim excuse from two different users
	if card, ok := copyPasteCard(cancellations, p); ok {
		sus = append(sus, card)
	}

	// Todesduo: when both are missing, the table stays empty
	if card, ok := todesduoCard(p, thursdayStats); ok {
		sus = append(sus, card)
	}

	for i := range cards {
		cards[i].DelayClass = fmt.Sprintf("delay-%d", i*150+300)
	}
	for i := range sus {
		sus[i].DelayClass = fmt.Sprintf("delay-%d", i*150+300)
	}
	return cards, sus
}

// pingPongCard finds the pair with the longest strictly alternating run:
// week 1 A da / B weg, week 2 B da / A weg, and so on
func pingPongCard(p presenceData) (viewmodels.FunCard, bool) {
	bestRun, ba, bb := 0, "", ""
	var bestStart, bestEnd string

	for i := 0; i < len(p.names); i++ {
		for j := i + 1; j < len(p.names); j++ {
			a, b := p.names[i], p.names[j]
			run, runStart := 0, ""
			prevState := 0 // +1: A da/B weg, -1: B da/A weg, 0: beide/keiner
			for _, d := range p.thursdays {
				state := 0
				if !p.absent[a][d] && p.absent[b][d] {
					state = 1
				} else if p.absent[a][d] && !p.absent[b][d] {
					state = -1
				}
				switch {
				case state == 0:
					run, prevState = 0, 0
					continue
				case state == -prevState:
					run++
				default: // first week of a rally, or same side twice
					run = 1
					runStart = d
				}
				prevState = state
				if run > bestRun {
					bestRun, ba, bb = run, a, b
					bestStart, bestEnd = runStart, d
				}
			}
		}
	}

	if bestRun < 4 {
		return viewmodels.FunCard{}, false
	}
	return viewmodels.FunCard{
		Emoji: "🏓", Title: "Das Ping-Pong-Duo",
		Headline: pairHeadline(p, ba, bb),
		Detail: fmt.Sprintf("%d Wochen striktes Wechselspiel (%s – %s): einer geht, einer kommt",
			bestRun, formatISO(bestStart), formatISO(bestEnd)),
		Gradient: "bg-gradient-to-r from-lime-500/25 to-green-500/15",
	}, true
}

// alibiCard finds the pair most often absent on the same day with the same
// excuse category
func alibiCard(cancellations []models.Cancellation, p presenceData) (viewmodels.FunCard, bool) {
	// date -> user -> category (only categorized cancellations)
	byDate := make(map[string]map[string]string)
	for _, c := range cancellations {
		if c.Category == "" {
			continue
		}
		d := c.Date.Format("2006-01-02")
		if byDate[d] == nil {
			byDate[d] = make(map[string]string)
		}
		byDate[d][c.UserName] = c.Category
	}

	type pairKey struct{ a, b string }
	count := make(map[pairKey]int)
	lastDate := make(map[pairKey]string)
	lastCat := make(map[pairKey]string)

	for d, userCats := range byDate {
		names := make([]string, 0, len(userCats))
		for n := range userCats {
			names = append(names, n)
		}
		sort.Strings(names)
		for i := 0; i < len(names); i++ {
			for j := i + 1; j < len(names); j++ {
				if userCats[names[i]] != userCats[names[j]] {
					continue
				}
				k := pairKey{names[i], names[j]}
				count[k]++
				if d > lastDate[k] {
					lastDate[k] = d
					lastCat[k] = userCats[names[i]]
				}
			}
		}
	}

	best, bestCount := pairKey{}, 0
	for k, n := range count {
		if n > bestCount || (n == bestCount && k.a+k.b < best.a+best.b) {
			best, bestCount = k, n
		}
	}
	if bestCount == 0 {
		return viewmodels.FunCard{}, false
	}

	label := lastCat[best]
	if cat, ok := models.GetAllExcuseCategories()[label]; ok {
		label = cat.Label
	}
	detail := fmt.Sprintf("am %s beide mit '%s' abgesagt – reiner Zufall, sicher 👀", formatISO(lastDate[best]), label)
	if bestCount > 1 {
		detail = fmt.Sprintf("%d× am selben Tag mit derselben Ausrede gefehlt – zuletzt am %s ('%s') 👀",
			bestCount, formatISO(lastDate[best]), label)
	}
	return viewmodels.FunCard{
		Emoji: "🕵️", Title: "Das Alibi-Duo",
		Headline: pairHeadline(p, best.a, best.b),
		Detail:   detail,
		Gradient: "bg-gradient-to-r from-violet-500/25 to-fuchsia-500/15",
	}, true
}

// todesduoCard finds the shared-absence pair whose missing hurts most: on
// their common no-show days the table is emptiest
func todesduoCard(p presenceData, thursdayStats []models.ThursdayStat) (viewmodels.FunCard, bool) {
	attendeesByDay := make(map[string]int, len(thursdayStats))
	totalAttendees := 0
	for _, t := range thursdayStats {
		attendeesByDay[t.Date.Format("2006-01-02")] = t.Attendees
		totalAttendees += t.Attendees
	}
	if len(thursdayStats) == 0 {
		return viewmodels.FunCard{}, false
	}
	overallAvg := float64(totalAttendees) / float64(len(thursdayStats))

	bestAvg := 1e9
	var ba, bb string
	bestShared := 0
	for i := 0; i < len(p.names); i++ {
		for j := i + 1; j < len(p.names); j++ {
			a, b := p.names[i], p.names[j]
			shared, sum := 0, 0
			for d := range p.absent[a] {
				if p.absent[b][d] {
					shared++
					sum += attendeesByDay[d]
				}
			}
			if shared < 3 {
				continue
			}
			avg := float64(sum) / float64(shared)
			if avg < bestAvg {
				bestAvg, ba, bb, bestShared = avg, a, b, shared
			}
		}
	}

	// Must hurt beyond the mechanical -2 of the two of them being gone
	if ba == "" || bestAvg > overallAvg-2.5 {
		return viewmodels.FunCard{}, false
	}
	return viewmodels.FunCard{
		Emoji: "☠️", Title: "Das Todesduo",
		Headline: pairHeadline(p, ba, bb),
		Detail: fmt.Sprintf("fehlen beide, sitzen im Schnitt nur noch %.1f am Tisch (Jahresschnitt: %.1f) – %d× passiert",
			bestAvg, overallAvg, bestShared),
		Gradient: "bg-gradient-to-r from-zinc-600/30 to-red-900/20",
	}, true
}

// formatISO formats an ISO date (2006-01-02) as "2. Jan"
func formatISO(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	germanMonths := []string{
		"Jan", "Feb", "Mär", "Apr", "Mai", "Jun",
		"Jul", "Aug", "Sep", "Okt", "Nov", "Dez",
	}
	return fmt.Sprintf("%d. %s", t.Day(), germanMonths[t.Month()-1])
}

// magnetCard finds the ordered pair (A, B) where B's absence rate is much
// higher on days A is present than B's overall absence rate
func magnetCard(p presenceData) (viewmodels.FunCard, bool) {
	total := len(p.thursdays)
	bestDelta := 0
	var ma, mb string
	var mRate, mBase int

	for _, a := range p.names {
		presentA := p.presentDays[a]
		if len(presentA) < 5 {
			continue
		}
		for _, b := range p.names {
			if a == b || len(p.absent[b]) < 4 {
				continue
			}
			absentBWhenA := 0
			for _, d := range presentA {
				if p.absent[b][d] {
					absentBWhenA++
				}
			}
			rate := (absentBWhenA * 100) / len(presentA)
			base := (len(p.absent[b]) * 100) / total
			if rate-base > bestDelta {
				bestDelta = rate - base
				ma, mb, mRate, mBase = a, b, rate, base
			}
		}
	}

	if bestDelta < 15 {
		return viewmodels.FunCard{}, false
	}
	return viewmodels.FunCard{
		Emoji: "🧲", Title: "Magnet & Schreck",
		Headline: pairHeadline(p, ma, mb),
		Detail: fmt.Sprintf("Wenn %s da ist, fehlt %s in %d%% der Fälle (sonst %d%%) 👀",
			ma, mb, mRate, mBase),
		Gradient: "bg-gradient-to-r from-red-500/25 to-orange-500/15",
	}, true
}

// copyPasteCard finds an identical excuse text used by two different users
func copyPasteCard(cancellations []models.Cancellation, p presenceData) (viewmodels.FunCard, bool) {
	usersByMsg := make(map[string]map[string]bool)
	original := make(map[string]string)
	for _, c := range cancellations {
		msg := strings.TrimSpace(c.Message)
		if msg == "" {
			continue
		}
		key := strings.ToLower(msg)
		if usersByMsg[key] == nil {
			usersByMsg[key] = make(map[string]bool)
		}
		usersByMsg[key][c.UserName] = true
		original[key] = msg
	}

	bestLen, bestKey := 0, ""
	for key, us := range usersByMsg {
		if len(us) >= 2 && len(original[key]) > bestLen {
			bestLen, bestKey = len(original[key]), key
		}
	}
	if bestKey == "" {
		return viewmodels.FunCard{}, false
	}

	names := make([]string, 0, len(usersByMsg[bestKey]))
	for n := range usersByMsg[bestKey] {
		names = append(names, n)
	}
	sort.Strings(names)
	a, b := names[0], names[1]

	return viewmodels.FunCard{
		Emoji: "📋", Title: "Das Copy-Paste-Duo",
		Headline: pairHeadline(p, a, b),
		Detail:   "wortgleiche Ausrede, unabhängig voneinander:",
		Quote:    truncate(original[bestKey], 100),
		Gradient: "bg-gradient-to-r from-teal-500/25 to-emerald-500/15",
	}, true
}

// buildSquadCards computes Dreamteam, Retter in der Not and Mitläufer
func buildSquadCards(p presenceData, thursdayStats []models.ThursdayStat) []viewmodels.FunCard {
	if len(p.thursdays) == 0 || len(p.names) < 3 {
		return nil
	}

	var cards []viewmodels.FunCard

	// Dreamteam: trio most often present together
	bestTrio, t1, t2, t3 := 0, "", "", ""
	for i := 0; i < len(p.names); i++ {
		for j := i + 1; j < len(p.names); j++ {
			for k := j + 1; k < len(p.names); k++ {
				a, b, c := p.names[i], p.names[j], p.names[k]
				together := 0
				for _, d := range p.thursdays {
					if !p.absent[a][d] && !p.absent[b][d] && !p.absent[c][d] {
						together++
					}
				}
				if together > bestTrio {
					bestTrio, t1, t2, t3 = together, a, b, c
				}
			}
		}
	}
	if bestTrio > 0 {
		cards = append(cards, viewmodels.FunCard{
			Emoji: "🏛️", Title: "Das Dreamteam",
			Headline: fmt.Sprintf("%s %s, %s %s & %s %s", p.emoji[t1], t1, p.emoji[t2], t2, p.emoji[t3], t3),
			Detail:   fmt.Sprintf("%d× zu dritt am Tisch – das Fundament des Stammtischs", bestTrio),
			Gradient: "bg-gradient-to-r from-amber-500/25 to-orange-500/15",
		})
	}

	// Attendance per Thursday (for thin/full context)
	attendeesByDay := make(map[string]int, len(thursdayStats))
	for _, t := range thursdayStats {
		attendeesByDay[t.Date.Format("2006-01-02")] = t.Attendees
	}

	// Retter/Mitläufer: average co-attendance on the days a user showed up.
	// Requires enough presence AND enough absences, otherwise the always-there
	// (or never-there) crowd dominates without having made a "choice".
	type avgEntry struct {
		name string
		avg  float64
	}
	var avgs []avgEntry
	for _, n := range p.names {
		present := p.presentDays[n]
		if len(present) < 5 || len(p.absent[n]) < 5 {
			continue
		}
		sum := 0
		for _, d := range present {
			sum += attendeesByDay[d] - 1 // co-attendees, excluding the user
		}
		avgs = append(avgs, avgEntry{name: n, avg: float64(sum) / float64(len(present))})
	}
	if len(avgs) >= 2 {
		sort.Slice(avgs, func(i, j int) bool {
			if avgs[i].avg != avgs[j].avg {
				return avgs[i].avg < avgs[j].avg
			}
			return avgs[i].name < avgs[j].name
		})
		retter, mit := avgs[0], avgs[len(avgs)-1]
		if mit.avg-retter.avg >= 0.5 {
			cards = append(cards, viewmodels.FunCard{
				Emoji: "🦸", Title: "Retter in der Not",
				Headline: fmt.Sprintf("%s %s", p.emoji[retter.name], retter.name),
				Detail:   fmt.Sprintf("kommt auch, wenn sonst kaum einer da ist (Ø %.1f andere am Tisch)", retter.avg),
				Gradient: "bg-gradient-to-r from-green-500/25 to-emerald-500/15",
			})
			cards = append(cards, viewmodels.FunCard{
				Emoji: "🐑", Title: "Der Mitläufer",
				Headline: fmt.Sprintf("%s %s", p.emoji[mit.name], mit.name),
				Detail:   fmt.Sprintf("taucht bevorzugt bei voller Hütte auf (Ø %.1f andere am Tisch)", mit.avg),
				Gradient: "bg-gradient-to-r from-slate-500/25 to-gray-500/15",
			})
		}
	}

	for i := range cards {
		cards[i].DelayClass = fmt.Sprintf("delay-%d", i*150+300)
	}
	return cards
}

// buildForensikCards computes Romanautor, Minimalist and Emoji-König
func buildForensikCards(cancellations []models.Cancellation, users []models.UserStats) []viewmodels.FunCard {
	emojiByName := make(map[string]string, len(users))
	for _, u := range users {
		emojiByName[u.Name] = u.Emoji
	}

	var longest, shortest *models.Cancellation
	emojiCount := make(map[string]int)   // per user
	emojiUsage := make(map[rune]int)     // per emoji rune
	for i := range cancellations {
		c := &cancellations[i]
		msg := strings.TrimSpace(c.Message)
		if msg == "" {
			continue
		}
		length := len([]rune(msg))
		if longest == nil || length > len([]rune(strings.TrimSpace(longest.Message))) {
			longest = c
		}
		if shortest == nil || length < len([]rune(strings.TrimSpace(shortest.Message))) {
			shortest = c
		}
		for _, r := range msg {
			if isEmoji(r) {
				emojiCount[c.UserName]++
				emojiUsage[r]++
			}
		}
	}

	var cards []viewmodels.FunCard
	if longest != nil && shortest != nil && longest != shortest {
		lmsg := strings.TrimSpace(longest.Message)
		cards = append(cards, viewmodels.FunCard{
			Emoji: "📖", Title: "Der Romanautor",
			Headline: fmt.Sprintf("%s %s", emojiByName[longest.UserName], longest.UserName),
			Detail:   fmt.Sprintf("längste Absage des Jahres – %d Zeichen:", len([]rune(lmsg))),
			Quote:    truncate(lmsg, 160),
			Gradient: "bg-gradient-to-r from-indigo-500/25 to-purple-500/15",
		})
		smsg := strings.TrimSpace(shortest.Message)
		cards = append(cards, viewmodels.FunCard{
			Emoji: "🪨", Title: "Der Minimalist",
			Headline: fmt.Sprintf("%s %s", emojiByName[shortest.UserName], shortest.UserName),
			Detail:   fmt.Sprintf("kürzeste Absage des Jahres – %d Zeichen:", len([]rune(smsg))),
			Quote:    smsg,
			Gradient: "bg-gradient-to-r from-stone-500/25 to-zinc-500/15",
		})
	}

	// Emoji-König
	bestUser, bestCount := "", 0
	for n, c := range emojiCount {
		if c > bestCount || (c == bestCount && n < bestUser) {
			bestUser, bestCount = n, c
		}
	}
	if bestCount >= 3 {
		topEmoji, topEmojiCount := ' ', 0
		for r, c := range emojiUsage {
			if c > topEmojiCount {
				topEmoji, topEmojiCount = r, c
			}
		}
		cards = append(cards, viewmodels.FunCard{
			Emoji: "😂", Title: "Der Emoji-König",
			Headline: fmt.Sprintf("%s %s", emojiByName[bestUser], bestUser),
			Detail: fmt.Sprintf("%d Emojis in Absagen verballert – Gruppenliebling: %s (%d×)",
				bestCount, string(topEmoji), topEmojiCount),
			Gradient: "bg-gradient-to-r from-yellow-500/25 to-amber-500/15",
		})
	}

	for i := range cards {
		cards[i].DelayClass = fmt.Sprintf("delay-%d", i*150+300)
	}
	return cards
}

// buildMuffelCards finds who skips summer and who skips winter
func buildMuffelCards(cancellations []models.Cancellation, users []models.UserStats) []viewmodels.FunCard {
	emojiByName := make(map[string]string, len(users))
	for _, u := range users {
		emojiByName[u.Name] = u.Emoji
	}

	summer := map[int]bool{6: true, 7: true, 8: true}
	winter := map[int]bool{12: true, 1: true, 2: true}
	summerCount := make(map[string]int)
	winterCount := make(map[string]int)
	for _, c := range cancellations {
		m := int(c.Date.Month())
		if summer[m] {
			summerCount[c.UserName]++
		}
		if winter[m] {
			winterCount[c.UserName]++
		}
	}

	pick := func(counts map[string]int) (string, int) {
		best, n := "", 0
		for name, c := range counts {
			if c > n || (c == n && name < best) {
				best, n = name, c
			}
		}
		return best, n
	}

	var cards []viewmodels.FunCard
	if name, n := pick(summerCount); n >= 3 {
		cards = append(cards, viewmodels.FunCard{
			Emoji: "☀️", Title: "Der Sommermuffel",
			Headline: fmt.Sprintf("%s %s", emojiByName[name], name),
			Detail:   fmt.Sprintf("%d Absagen im Juni, Juli und August – bei Biergartenwetter!", n),
			Gradient: "bg-gradient-to-r from-orange-500/25 to-yellow-500/15",
		})
	}
	if name, n := pick(winterCount); n >= 3 {
		cards = append(cards, viewmodels.FunCard{
			Emoji: "❄️", Title: "Der Wintermuffel",
			Headline: fmt.Sprintf("%s %s", emojiByName[name], name),
			Detail:   fmt.Sprintf("%d Absagen im Dezember, Januar und Februar – zu kalt draußen?", n),
			Gradient: "bg-gradient-to-r from-sky-500/25 to-blue-500/15",
		})
	}

	for i := range cards {
		cards[i].DelayClass = fmt.Sprintf("delay-%d", i*200+300)
	}
	return cards
}

// isEmoji reports whether the rune falls in the common emoji blocks
func isEmoji(r rune) bool {
	switch {
	case r >= 0x1F300 && r <= 0x1FAFF: // pictographs, emoticons, transport, supplemental
		return true
	case r >= 0x2600 && r <= 0x27BF: // misc symbols + dingbats
		return true
	case r >= 0x2B00 && r <= 0x2BFF: // arrows/stars (⭐)
		return true
	}
	return false
}

// truncate shortens s to max runes, appending an ellipsis
func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}
