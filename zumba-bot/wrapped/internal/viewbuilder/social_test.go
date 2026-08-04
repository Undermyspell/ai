package viewbuilder

import (
	"testing"
	"time"

	"github.com/michael/stammtisch-wrapped/pkg/models"
)

func thursdaysFrom(start time.Time, n int) []models.ThursdayStat {
	out := make([]models.ThursdayStat, 0, n)
	d := start
	for range n {
		out = append(out, models.ThursdayStat{Date: d, Attendees: 2, Total: 3})
		d = d.AddDate(0, 0, 7)
	}
	return out
}

func cancel(name string, date time.Time, msg string) models.Cancellation {
	return models.Cancellation{UserName: name, Date: date, Message: msg}
}

func testUsers() []models.UserStats {
	return []models.UserStats{
		{User: models.User{Name: "Anna", Emoji: "🍺"}},
		{User: models.User{Name: "Ben", Emoji: "⚽"}},
		{User: models.User{Name: "Carl", Emoji: "🎸"}},
	}
}

func TestBuildDuoCardsZwillinge(t *testing.T) {
	start := time.Date(2025, 12, 4, 0, 0, 0, 0, time.UTC)
	ts := thursdaysFrom(start, 6)

	cancels := []models.Cancellation{
		cancel("Anna", ts[0].Date, ""),
		cancel("Ben", ts[0].Date, ""),
		cancel("Anna", ts[1].Date, ""),
		cancel("Ben", ts[1].Date, ""),
	}

	cards := buildDuoCards(cancels, testUsers(), ts)

	var found bool
	for _, c := range cards {
		if c.Title == "Die Absage-Zwillinge" {
			found = true
			if c.Headline != "🍺 Anna & ⚽ Ben" {
				t.Errorf("unexpected headline: %q", c.Headline)
			}
		}
	}
	if !found {
		t.Fatal("expected Absage-Zwillinge card")
	}
}

func TestCopyPasteCardCrossUserOnly(t *testing.T) {
	start := time.Date(2025, 12, 4, 0, 0, 0, 0, time.UTC)
	ts := thursdaysFrom(start, 4)

	// Same text twice from the SAME user must not count
	sameUser := []models.Cancellation{
		cancel("Anna", ts[0].Date, "Bin krank"),
		cancel("Anna", ts[1].Date, "Bin krank"),
	}
	p := buildPresence(sameUser, testUsers(), ts)
	if _, ok := copyPasteCard(sameUser, p); ok {
		t.Error("same-user duplicate must not produce a copy-paste card")
	}

	// Same text from two users counts (case-insensitive)
	crossUser := []models.Cancellation{
		cancel("Anna", ts[0].Date, "Bin krank"),
		cancel("Ben", ts[1].Date, "bin krank"),
	}
	p = buildPresence(crossUser, testUsers(), ts)
	card, ok := copyPasteCard(crossUser, p)
	if !ok {
		t.Fatal("expected copy-paste card")
	}
	if card.Headline != "🍺 Anna & ⚽ Ben" {
		t.Errorf("unexpected headline: %q", card.Headline)
	}
}

func TestBuildForensikCards(t *testing.T) {
	start := time.Date(2025, 12, 4, 0, 0, 0, 0, time.UTC)
	long := "Es tut mir wahnsinnig leid, aber heute wird es wirklich absolut gar nichts, weil einfach alles zusammenkommt 😭😭😭"
	cancels := []models.Cancellation{
		cancel("Anna", start, long),
		cancel("Ben", start.AddDate(0, 0, 7), "Nö."),
		cancel("Anna", start.AddDate(0, 0, 14), "Wieder nix 😭"),
	}

	cards := buildForensikCards(cancels, testUsers())

	titles := make(map[string]string)
	for _, c := range cards {
		titles[c.Title] = c.Headline
	}
	if titles["Der Romanautor"] != "🍺 Anna" {
		t.Errorf("Romanautor: expected Anna, got %q", titles["Der Romanautor"])
	}
	if titles["Der Minimalist"] != "⚽ Ben" {
		t.Errorf("Minimalist: expected Ben, got %q", titles["Der Minimalist"])
	}
	// 4 × 😭 from Anna → Emoji-König (threshold 3)
	if titles["Der Emoji-König"] != "🍺 Anna" {
		t.Errorf("Emoji-König: expected Anna, got %q", titles["Der Emoji-König"])
	}
}

func TestBuildMuffelCards(t *testing.T) {
	users := testUsers()
	cancels := []models.Cancellation{
		cancel("Anna", time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC), ""),
		cancel("Anna", time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC), ""),
		cancel("Anna", time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC), ""),
		cancel("Ben", time.Date(2025, 12, 11, 0, 0, 0, 0, time.UTC), ""),
		cancel("Ben", time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC), ""),
		cancel("Ben", time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC), ""),
	}

	cards := buildMuffelCards(cancels, users)
	if len(cards) != 2 {
		t.Fatalf("expected 2 muffel cards, got %d", len(cards))
	}
	if cards[0].Title != "Der Sommermuffel" || cards[0].Headline != "🍺 Anna" {
		t.Errorf("Sommermuffel: got %q / %q", cards[0].Title, cards[0].Headline)
	}
	if cards[1].Title != "Der Wintermuffel" || cards[1].Headline != "⚽ Ben" {
		t.Errorf("Wintermuffel: got %q / %q", cards[1].Title, cards[1].Headline)
	}
}
