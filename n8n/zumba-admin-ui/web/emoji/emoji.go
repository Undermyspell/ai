// Package emoji maps user names to a stable display emoji.
// The DB doesn't store emojis; we derive them deterministically so the same
// person always gets the same glyph across the app.
package emoji

import "hash/fnv"

var pool = []string{
	"🍺", "🎸", "⚽", "🎮", "📚", "🏔️", "🚴", "🎬",
	"💻", "🎯", "🍕", "🏋️", "🎵", "🎨", "🏀", "🥨",
	"🧀", "🦌", "🪕", "🎿",
}

// Curated overrides for the known Stammtisch crew (mirrors wrapped/data/mock.go).
var overrides = map[string]string{
	"Max":       "🍺",
	"Thomas":    "🎸",
	"Stefan":    "⚽",
	"Andreas":   "🎮",
	"Michael":   "📚",
	"Christian": "🏔️",
	"Markus":    "🚴",
	"Daniel":    "🎬",
	"Sebastian": "💻",
	"Patrick":   "🎯",
	"Florian":   "🍕",
	"Tobias":    "🏋️",
	"Martin":    "🎵",
	"Philipp":   "🎨",
	"Jan":       "🏀",
}

func For(name string) string {
	if e, ok := overrides[name]; ok {
		return e
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return pool[h.Sum32()%uint32(len(pool))]
}
