package report

import "testing"

// Dichtes Feld wie im Live-Betrieb: 19 Mitglieder, Gleichstände, lange und
// kurze Namen – kein Schild darf ein anderes überdecken.
func TestGipfelFahnenUeberdeckenSichNicht(t *testing.T) {
	quoten := []float64{85, 80, 78, 70, 55, 55, 48, 48, 48, 48, 38, 38, 35, 30, 28, 27, 25, 22, 20}
	namen := []string{"Anna", "Bartholomäus", "Christine", "Didi", "Emil", "Fritz", "Gabi", "Hans", "Ida", "Jo",
		"Karl", "Lena", "Max", "Nora", "Otto", "Paula", "Quirin", "Rosi", "Sepperla"}
	n := len(quoten)
	users := make([]cardUser, n)
	rang := 0
	for i, q := range quoten {
		if i == 0 || q != quoten[i-1] {
			rang++
		}
		users[i] = cardUser{Name: namen[i], Rank: rang, PercentVal: q, Top3: rang <= 3, RidgeX: ridgeX(i, n), Letzter: i == n-1}
	}

	kopf, schilder := gipfelLayout(users)
	for i := range schilder {
		for j := i + 1; j < len(schilder); j++ {
			if schilder[i].trifft(schilder[j], 0) {
				t.Errorf("Schild %d (%s) überdeckt Schild %d (%s)", i, namen[i], j, namen[j])
			}
		}
		if schilder[i].x0 < -schildRand || schilder[i].x1 > profilBreite+schildRand {
			t.Errorf("Schild %d (%s) ragt aus der Karte: %+v", i, namen[i], schilder[i])
		}
	}
	if kopf < profilKopf {
		t.Errorf("Kopfabstand %d < Minimum %d", kopf, profilKopf)
	}
	if users[0].FahneSeite != seiteRechts || users[n-1].FahneSeite != seiteLinks {
		t.Errorf("Randfahnen müssen nach innen wehen: %q / %q", users[0].FahneSeite, users[n-1].FahneSeite)
	}
}

func TestGipfelFahnenEinzeln(t *testing.T) {
	users := []cardUser{{Name: "Anna", Rank: 1, PercentVal: 100, Top3: true, RidgeX: ridgeX(0, 1), Letzter: true}}
	kopf, _ := gipfelLayout(users)
	if users[0].FahneSeite != seiteMitte || users[0].FahnePole < 24 {
		t.Errorf("einzelne Fahne: Seite %q, Stange %d", users[0].FahneSeite, users[0].FahnePole)
	}
	if kopf < profilKopf {
		t.Errorf("Kopfabstand %d", kopf)
	}
}
