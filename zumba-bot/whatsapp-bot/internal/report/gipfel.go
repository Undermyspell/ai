package report

import (
	"math"
	"strconv"
	"unicode/utf8"
)

// gipfel.go legt die Fahnen im Bergprofil des "gipfelbuch" aus. Jede Fahne
// steht auf dem Gipfelpunkt ihres Mitglieds (x = Rang, Höhe = Quote). Bei
// vielen Mitgliedern liegen die Gipfel nur gut 30px auseinander – feste
// Stangenlängen reichen dann nicht, die Schilder überdecken sich. Deshalb
// sucht gipfelFahnen je Fahne die kürzeste Stange und die Schild-Seite, bei
// der Schild und Stange nichts Belegtes treffen.
//
// Gerechnet wird in Pixeln des Profils, y nach oben ab der Grundlinie. Die
// Maße entsprechen card-gipfelbuch.tmpl.
const (
	profilBreite = 640.0 // 720px Karte − 2×40px Rand
	profilHoehe  = 250.0
	punktHoehe   = 8.0
	stangeAb     = 6.0 // die Stange beginnt 2px im 8px-Punkt
	stangeMax    = 160.0
	profilKopf   = 56   // Mindestabstand zwischen Kopf und Profil (Entwurf)
	kopfLuft     = 8.0  // Luft über dem höchsten Schild
	schildLuft   = 2.0  // Mindestabstand zwischen Schildern
	schildRand   = 36.0 // so weit darf ein Schild über das Profil hinaus (Kartenrand 40px)
)

// Schild-Seiten: mittig über der Stange oder wie eine Fahne zu einer Seite.
const (
	seiteMitte  = ""
	seiteRechts = "rechts" // Schild beginnt an der Stange und weht nach rechts
	seiteLinks  = "links"
)

type rechteck struct{ x0, x1, y0, y1 float64 }

func (a rechteck) trifft(b rechteck, luft float64) bool {
	return a.x0 < b.x1+luft && b.x0 < a.x1+luft && a.y0 < b.y1+luft && b.y0 < a.y1+luft
}

type belegt struct {
	r      rechteck
	von    int  // Index der Fahne (-1 = Höhenlinien-Beschriftung)
	stang  bool // dünne Stange statt Fläche
	schild bool // gesetztes Schild – nur das lässt sich durch eine längere Stange verschieben
}

// schildBreite schätzt die Breite eines Caveat-Schilds (≈0,42em je Zeichen
// plus 2×5px Innenabstand) – lieber etwas zu breit als zu knapp.
func schildBreite(text string, fs float64) float64 {
	return float64(utf8.RuneCountInString(text))*0.42*fs + 10
}

// gipfelFahnen setzt FahnePole/FahneSeite je Mitglied und liefert den
// Abstand, den das Profil nach oben braucht, damit auch das höchste Schild
// unter den Kopf passt. users muss RidgeX und Letzter schon tragen.
func gipfelFahnen(users []cardUser) int {
	kopf, _ := gipfelLayout(users)
	return kopf
}

// gipfelLayout ist gipfelFahnen samt den Schild-Flächen (für Tests).
//
// Gesetzt wird der Reihe nach (erst Namen, dann Ziffern), je Fahne die
// kürzeste freie Stange. Findet eine Fahne keinen freien Platz, liegt das an
// schon gesetzten Schildern – typisch: das Schild des Nachbarn hängt über
// der eigenen Stange. Dann bekommen genau diese Nachbarn eine längere
// Mindeststange und die Runde beginnt neu, bis alles frei ist.
func gipfelLayout(users []cardUser) (int, []rechteck) {
	n := len(users)
	f := make([]fahne, n)
	minPole := make([]float64, n)
	for i, u := range users {
		fl := fahne{x: u.RidgeX / 100 * profilBreite, y: u.PercentVal / 100 * profilHoehe, h: 17, name: u.Top3 || u.Letzter}
		if fl.name {
			fl.w, fl.h = schildBreite(u.Name, 20), 22
		} else {
			fl.w = schildBreite(strconv.Itoa(u.Rank), 15)
		}
		f[i] = fl
		minPole[i] = minStange(fl.name)
	}

	var p plan
	for runde := 0; runde < 120; runde++ {
		p = fahnenPlan(f, minPole)
		if len(p.stoerer) == 0 {
			break
		}
		gehoben := false
		for j := range p.stoerer {
			if minPole[j]+6 <= stangeMax {
				minPole[j] += 6
				gehoben = true
			}
		}
		if !gehoben {
			break // nichts mehr zu holen – mit den wenigsten Überdeckungen leben
		}
	}

	oben := profilHoehe
	for i := range users {
		users[i].FahnePole = int(p.pole[i])
		users[i].FahneSeite = p.seite[i]
		oben = math.Max(oben, p.schild[i].y1)
	}
	return max(profilKopf, int(math.Ceil(oben-profilHoehe+kopfLuft))), p.schild
}

type fahne struct {
	x, y, w, h float64
	name       bool
}

type plan struct {
	pole    []float64
	seite   []string
	schild  []rechteck
	stoerer map[int]bool // Fahnen, deren Schild/Stange eine andere blockiert
}

// fahnenPlan setzt alle Fahnen einmal mit den gegebenen Mindeststangen.
func fahnenPlan(f []fahne, minPole []float64) plan {
	n := len(f)
	p := plan{pole: make([]float64, n), seite: make([]string, n), schild: make([]rechteck, n), stoerer: map[int]bool{}}

	var frei []belegt
	for i, fl := range f {
		// Gipfelpunkt und kürzeste Stange jeder Fahne sind von Anfang an belegt.
		frei = append(frei,
			belegt{r: rechteck{fl.x - 4, fl.x + 4, fl.y, fl.y + punktHoehe}, von: i},
			belegt{r: rechteck{fl.x - 1, fl.x + 1, fl.y + punktHoehe, fl.y + stangeAb + minPole[i]}, von: i, stang: true},
		)
	}
	// Beschriftungen der Höhenlinien (100/75/50/25 %) am rechten Rand; sie
	// enden 2px vor der Kante, die Stange der letzten Fahne läuft daneben.
	for _, h := range []float64{1, .75, .5, .25} {
		frei = append(frei, belegt{r: rechteck{profilBreite - 42, profilBreite - 3, h*profilHoehe - 13, h * profilHoehe}, von: -1})
	}

	// Erst die Namensschilder (Top 3, Letzter), dann die Platzziffern.
	var reihe []int
	for i := range f {
		if f[i].name {
			reihe = append(reihe, i)
		}
	}
	for i := range f {
		if !f[i].name {
			reihe = append(reihe, i)
		}
	}

	for _, i := range reihe {
		fl := f[i]
		seiten := []string{seiteMitte, seiteRechts, seiteLinks}
		switch {
		case n > 1 && i == 0:
			seiten = []string{seiteRechts} // am linken Rand weht es nach innen
		case n > 1 && i == n-1:
			seiten = []string{seiteLinks}
		}

		var (
			best       = math.MaxInt
			bestPole   float64
			bestSeite  string
			bestSchild rechteck
			bestStange rechteck
			bestVon    []int
		)
	suche:
		for pole := minPole[i]; pole <= stangeMax; pole += 4 {
			for _, seite := range seiten {
				schild := schildRechteck(fl.x, fl.w, fl.h, fl.y+stangeAb+pole, seite)
				if schild.x0 < -schildRand || schild.x1 > profilBreite+schildRand {
					continue
				}
				stange := rechteck{fl.x - 1, fl.x + 1, fl.y + punktHoehe, fl.y + stangeAb + pole}
				treffer := 0
				var von []int
				for _, b := range frei {
					if b.von == i {
						continue
					}
					if schild.trifft(b.r, schildLuft) || (!b.stang && stange.trifft(b.r, 1)) {
						treffer++
						if b.schild {
							von = append(von, b.von)
						}
					}
				}
				if treffer < best {
					best, bestPole, bestSeite, bestSchild, bestStange, bestVon = treffer, pole, seite, schild, stange, von
				}
				if treffer == 0 {
					break suche
				}
			}
		}

		for _, j := range bestVon {
			p.stoerer[j] = true
		}
		p.pole[i], p.seite[i], p.schild[i] = bestPole, bestSeite, bestSchild
		frei = append(frei, belegt{r: bestSchild, von: i, schild: true}, belegt{r: bestStange, von: i, stang: true})
	}
	return p
}

func minStange(name bool) float64 {
	if name {
		return 24 // Namensschilder stehen sichtbar höher als die Ziffern
	}
	return 14
}

func schildRechteck(x, w, h, unten float64, seite string) rechteck {
	switch seite {
	case seiteRechts:
		return rechteck{x - 6, x - 6 + w, unten, unten + h}
	case seiteLinks:
		return rechteck{x + 6 - w, x + 6, unten, unten + h}
	}
	return rechteck{x - w/2, x + w/2, unten, unten + h}
}
