package report

import "strings"

// BuildWirtReminder baut die wöchentliche Bitte an den Wirt, den
// Stammtisch-Tisch freizuhalten. Sie geht in die Stammtisch-Gruppe, der Wirt
// wird darin markiert – donnerstags um 16 Uhr, also am Stammtisch-Tag selbst:
// deshalb "heute Abend" und keine Ankündigung, dass wir kommen. mention ist
// die Nummer (nur Ziffern), die markiert wird – WhatsApp zeigt "@<nummer>"
// als Kontaktnamen, wenn dieselbe Nummer in der mentioned-Liste des Versands
// steht. Ohne mention bleibt die Anrede neutral.
func BuildWirtReminder(mention string) string {
	anrede := "Servus"
	if mention != "" {
		anrede += " @" + mention
	}

	var b strings.Builder
	b.WriteString("🍻 *Stammtisch heute Abend*\n\n")
	b.WriteString(anrede + " 👋\n\n")
	b.WriteString("bitte halt uns heute Abend unseren Tisch frei 🙏\n\n")
	b.WriteString("Danke dir – bis später! 😉\n")
	b.WriteString("_Automatische Erinnerung vom Zumba-Stammtisch_")
	return b.String()
}
