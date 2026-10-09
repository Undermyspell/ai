package report

// wetter.go übersetzt Serien und Strafen in die Bildsprache des
// "wetterbericht": je Serienstufe ein eigenes Wetter (Symbol, Text,
// Kachelfarbe) und je Strafe eine Warnstufe in den Farben des Deutschen
// Wetterdienstes.

// wetterLage ist das Wetter einer Kachel. Art ist die CSS-Klasse der
// Kachelfarbe.
type wetterLage struct {
	Symbol string
	Text   string
	Art    string
}

// wetterFuer ordnet die laufende Serie einer Wetterlage zu: je länger die
// Anwesenheits-Serie, desto heißer; je länger die Pause, desto kälter. Ab fünf
// Wochen Pause – dort setzt die Fehltage-Strafe ein – herrscht Frost.
func wetterFuer(streak int) wetterLage {
	switch {
	case streak > 12:
		return wetterLage{"🌋", "Vulkan", "vulkan"}
	case streak >= 8:
		return wetterLage{"🔥", "Rekordhitze", "hitze"}
	case streak >= 5:
		return wetterLage{"🥵", "Hitzewelle", "hitze"}
	case streak >= 3:
		return wetterLage{"☀️", "sonnig", "sonne"}
	case streak == 2:
		return wetterLage{"🌤️", "heiter", "sonne"}
	case streak == 1:
		return wetterLage{"🌈", "Regenbogen", "regenbogen"}
	case streak == 0:
		return wetterLage{"⛅", "wechselhaft", "wechsel"}
	case streak == -1:
		return wetterLage{"🌦️", "Schauer", "regen"}
	case streak == -2:
		return wetterLage{"🌧️", "Regen", "regen"}
	case streak == -3:
		return wetterLage{"⛈️", "Gewitter", "gewitter"}
	case streak == -4:
		return wetterLage{"🌨️", "Schnee", "schnee"}
	case streak > -10:
		return wetterLage{"☃️", "Dauerfrost", "frost"}
	}
	return wetterLage{"🧊", "Eiszeit", "frost"}
}

// warnstufe ordnet einen Strafbetrag den Warnstufen des DWD zu: 1 gelb
// (Wetterwarnung), 2 orange (markantes Wetter), 3 rot (Unwetter), 4 violett
// (extremes Unwetter). Fehltage-Strafen starten bei 25 € (Stufe 1), ab zehn
// Fehltagen bzw. bei einem No-Show (50 €) ist es ein Unwetter.
func warnstufe(betrag int) int {
	switch {
	case betrag >= 100:
		return 4
	case betrag >= 50:
		return 3
	case betrag >= 30:
		return 2
	}
	return 1
}
