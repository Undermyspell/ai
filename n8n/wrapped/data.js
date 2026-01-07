// Stammtisch Wrapped 2025 - Dummy Data
// =====================================

// Die 15 Stammtisch-Teilnehmer
const USERS = [
	{ id: 1, name: "Max", emoji: "🍺" },
	{ id: 2, name: "Thomas", emoji: "🎸" },
	{ id: 3, name: "Stefan", emoji: "⚽" },
	{ id: 4, name: "Andreas", emoji: "🎮" },
	{ id: 5, name: "Michael", emoji: "📚" },
	{ id: 6, name: "Christian", emoji: "🏔️" },
	{ id: 7, name: "Markus", emoji: "🚴" },
	{ id: 8, name: "Daniel", emoji: "🎬" },
	{ id: 9, name: "Sebastian", emoji: "💻" },
	{ id: 10, name: "Patrick", emoji: "🎯" },
	{ id: 11, name: "Florian", emoji: "🍕" },
	{ id: 12, name: "Tobias", emoji: "🏋️" },
	{ id: 13, name: "Martin", emoji: "🎵" },
	{ id: 14, name: "Philipp", emoji: "🎨" },
	{ id: 15, name: "Jan", emoji: "🏀" },
];

// Alle Donnerstage in 2025 (bis heute)
const THURSDAYS_2025 = [
	"2025-01-02",
	"2025-01-09",
	"2025-01-16",
	"2025-01-23",
	"2025-01-30",
	"2025-02-06",
	"2025-02-13",
	"2025-02-20",
	"2025-02-27",
	"2025-03-06",
	"2025-03-13",
	"2025-03-20",
	"2025-03-27",
	"2025-04-03",
	"2025-04-10",
	"2025-04-17",
	"2025-04-24",
	"2025-05-01",
	"2025-05-08",
	"2025-05-15",
	"2025-05-22",
	"2025-05-29",
	"2025-06-05",
	"2025-06-12",
	"2025-06-19",
	"2025-06-26",
	"2025-07-03",
	"2025-07-10",
	"2025-07-17",
	"2025-07-24",
	"2025-07-31",
	"2025-08-07",
	"2025-08-14",
	"2025-08-21",
	"2025-08-28",
	"2025-09-04",
	"2025-09-11",
	"2025-09-18",
	"2025-09-25",
	"2025-10-02",
	"2025-10-09",
	"2025-10-16",
	"2025-10-23",
	"2025-10-30",
	"2025-11-06",
	"2025-11-13",
	"2025-11-20",
	"2025-11-27",
	"2025-12-04",
	"2025-12-11",
	"2025-12-18",
];

const TOTAL_THURSDAYS = THURSDAYS_2025.length; // 51

// Ausreden-Kategorien und Beispiele
const EXCUSE_CATEGORIES = {
	arbeit: [
		"Muss länger arbeiten, sorry Jungs 😔",
		"Meeting bis 20 Uhr, das wird nix heute",
		"Deadline morgen, sitze noch im Büro",
		"Chef hat spontan was reingedrückt...",
		"Überstunden ohne Ende, nächste Woche wieder!",
		"Projekt-Crunch, ihr kennt das 💼",
		"Kundenbesuch, muss leider absagen",
	],
	familie: [
		"Familienfeier, muss zur Schwiegermutter 😅",
		"Kind ist krank, bleibe daheim",
		"Hochzeitstag vergessen... muss was gutmachen",
		"Eltern kommen zu Besuch",
		"Kindergeburtstag, nächste Woche!",
		"Frau hat was geplant, sorry!",
		"Familiending, kann nicht weg",
	],
	gesundheit: [
		"Bin flach, Erkältung hat mich erwischt 🤧",
		"Rücken macht nicht mit heute",
		"Migräne, liege im Dunkeln",
		"Magen-Darm, sag ich nur...",
		"Arzttermin morgen früh, muss fit sein",
		"Bin angeschlagen, will euch nicht anstecken",
	],
	muede: [
		"Komplett platt, sorry Leute 😴",
		"Null Energie heute, wird ne Couch-Session",
		"Die Woche war brutal, brauch Schlaf",
		"Bin durch, nächste Woche wieder fit!",
		"Einfach zu müde für alles",
	],
	wetter: [
		"Bei dem Wetter geh ich nicht raus 🌧️",
		"Schnee ohne Ende, Auto eingefroren",
		"Sturm angesagt, bleib lieber daheim",
		"40 Grad? Ich bleib in der Klimaanlage",
	],
	freizeit: [
		"Champions League heute, sorry nicht sorry ⚽",
		"Konzert-Tickets seit Monaten, muss hin 🎸",
		"Kumpel von früher ist in der Stadt",
		"Geburtstag von nem Kollegen",
		"Andere Verabredung, war zuerst geplant",
	],
	kreativ: [
		"Mein Goldfisch hat Geburtstag 🐟",
		"Muss meine Pflanzen gießen, die sehen traurig aus",
		"Hab mir vorgenommen heute mal früh ins Bett zu gehen (lol)",
		"Sitze in der Badewanne, kein Bock rauszugehen",
		"Mars steht ungünstig, Astrologe sagt nein 🔮",
		"Netflix hat neue Staffel released, ihr versteht",
		"Bin in einem Wikipedia-Rabbit-Hole gefangen",
		"Muss meinen Kühlschrank sortieren, dringend",
		"Hab mich ausgesperrt und warte auf den Schlüsseldienst (Spoiler: Lüge)",
		"Meine Katze braucht emotionale Unterstützung heute 🐱",
	],
	keine_lust: [
		"Hab heute einfach keinen Bock, sorry 😬",
		"Brauch mal ne Pause, nächste Woche!",
		"Heute nicht, Jungs",
		"Chill-Abend geplant, ohne Menschen",
	],
};

// Generiere Absagen für jeden User
function generateCancellations() {
	const cancellations = [];

	// Verschiedene Absage-Raten für jeden User (um Vielfalt zu erzeugen)
	const userCancellationRates = {
		1: 0.08, // Max - sehr zuverlässig
		2: 0.12, // Thomas
		3: 0.18, // Stefan
		4: 0.35, // Andreas - oft weg
		5: 0.15, // Michael
		6: 0.25, // Christian
		7: 0.1, // Markus - sehr zuverlässig
		8: 0.45, // Daniel - selten da
		9: 0.22, // Sebastian
		10: 0.3, // Patrick
		11: 0.14, // Florian
		12: 0.2, // Tobias
		13: 0.55, // Martin - sehr selten
		14: 0.28, // Philipp
		15: 0.38, // Jan
	};

	// Lieblings-Ausreden-Kategorien pro User
	const userFavoriteExcuses = {
		1: ["arbeit", "gesundheit"],
		2: ["freizeit", "arbeit"],
		3: ["freizeit", "familie"],
		4: ["muede", "keine_lust", "kreativ"],
		5: ["arbeit", "familie"],
		6: ["wetter", "freizeit"],
		7: ["arbeit", "gesundheit"],
		8: ["keine_lust", "kreativ", "muede"],
		9: ["arbeit", "muede"],
		10: ["familie", "freizeit"],
		11: ["gesundheit", "arbeit"],
		12: ["gesundheit", "muede"],
		13: ["keine_lust", "kreativ", "muede"],
		14: ["kreativ", "freizeit"],
		15: ["freizeit", "keine_lust"],
	};

	THURSDAYS_2025.forEach((date) => {
		USERS.forEach((user) => {
			const rate = userCancellationRates[user.id];
			if (Math.random() < rate) {
				// User sagt ab
				const favoriteCategories = userFavoriteExcuses[user.id];
				const category =
					favoriteCategories[
						Math.floor(Math.random() * favoriteCategories.length)
					];
				const excuses = EXCUSE_CATEGORIES[category];
				const message = excuses[Math.floor(Math.random() * excuses.length)];

				cancellations.push({
					date: date,
					userId: user.id,
					userName: user.name,
					message: message,
					category: category,
				});
			}
		});
	});

	return cancellations;
}

// Alle Absagen
const CANCELLATIONS = generateCancellations();

// =====================================
// BERECHNETE STATISTIKEN
// =====================================

// Statistiken pro User berechnen
function calculateUserStats() {
	return USERS.map((user) => {
		const userCancellations = CANCELLATIONS.filter((c) => c.userId === user.id);
		const cancellationCount = userCancellations.length;
		const attendanceCount = TOTAL_THURSDAYS - cancellationCount;
		const attendanceRate = Math.round(
			(attendanceCount / TOTAL_THURSDAYS) * 100,
		);

		// Streaks berechnen
		let currentAttendanceStreak = 0;
		let maxAttendanceStreak = 0;
		let currentCancellationStreak = 0;
		let maxCancellationStreak = 0;

		THURSDAYS_2025.forEach((date) => {
			const cancelled = userCancellations.some((c) => c.date === date);
			if (cancelled) {
				currentCancellationStreak++;
				maxCancellationStreak = Math.max(
					maxCancellationStreak,
					currentCancellationStreak,
				);
				currentAttendanceStreak = 0;
			} else {
				currentAttendanceStreak++;
				maxAttendanceStreak = Math.max(
					maxAttendanceStreak,
					currentAttendanceStreak,
				);
				currentCancellationStreak = 0;
			}
		});

		// Lieblings-Ausrede-Kategorie
		const categoryCount = {};
		userCancellations.forEach((c) => {
			categoryCount[c.category] = (categoryCount[c.category] || 0) + 1;
		});
		const favoriteExcuseCategory =
			Object.entries(categoryCount).sort((a, b) => b[1] - a[1])[0]?.[0] || null;

		return {
			...user,
			cancellationCount,
			attendanceCount,
			attendanceRate,
			maxAttendanceStreak,
			maxCancellationStreak,
			neverCancelled: cancellationCount === 0,
			favoriteExcuseCategory,
			cancellations: userCancellations,
		};
	});
}

const USER_STATS = calculateUserStats();

// Nach Teilnahmequote sortiert (für Ranking)
const USER_RANKING = [...USER_STATS].sort(
	(a, b) => b.attendanceRate - a.attendanceRate,
);

// Titel basierend auf Verhalten
function getTitle(stats, _rank) {
	if (stats.neverCancelled) return { title: "Die Legende", emoji: "👑" };
	if (stats.attendanceRate >= 90)
		return { title: "Fels in der Brandung", emoji: "🪨" };
	if (stats.attendanceRate >= 80)
		return { title: "Der Wirtshaus-Veteran", emoji: "🍺" };
	if (stats.attendanceRate >= 70)
		return { title: "Stammgast mit Ausnahmen", emoji: "✅" };
	if (stats.attendanceRate >= 60)
		return { title: "Kommt wenn's passt", emoji: "🤷" };
	if (stats.attendanceRate >= 50) return { title: "Der Spontane", emoji: "🎲" };
	if (stats.attendanceRate >= 40)
		return { title: "Mystische Erscheinung", emoji: "👻" };
	if (stats.favoriteExcuseCategory === "kreativ")
		return { title: "Ausreden-Künstler", emoji: "🎨" };
	if (stats.favoriteExcuseCategory === "keine_lust")
		return { title: "Der Ehrliche", emoji: "😬" };
	return { title: "Der Unsichtbare", emoji: "🫥" };
}

// Titel für alle User hinzufügen
USER_RANKING.forEach((user, index) => {
	const titleInfo = getTitle(user, index + 1);
	user.title = titleInfo.title;
	user.titleEmoji = titleInfo.emoji;
	user.rank = index + 1;
});

// Globale Statistiken
const GLOBAL_STATS = {
	totalThursdays: TOTAL_THURSDAYS,
	totalUsers: USERS.length,
	totalCancellations: CANCELLATIONS.length,
	totalAttendances: TOTAL_THURSDAYS * USERS.length - CANCELLATIONS.length,
	averageAttendanceRate: Math.round(
		USER_STATS.reduce((sum, u) => sum + u.attendanceRate, 0) /
			USER_STATS.length,
	),
};

// Absagen pro Monat (für Heatmap)
const CANCELLATIONS_BY_MONTH = {};
CANCELLATIONS.forEach((c) => {
	const month = c.date.substring(0, 7); // "2025-01"
	CANCELLATIONS_BY_MONTH[month] = (CANCELLATIONS_BY_MONTH[month] || 0) + 1;
});

// Beste Ausreden
const _BEST_EXCUSES = CANCELLATIONS.filter(
	(c) => c.category === "kreativ",
).slice(0, 10);

// Kategorie-Statistiken
const CATEGORY_STATS = {};
Object.keys(EXCUSE_CATEGORIES).forEach((cat) => {
	CATEGORY_STATS[cat] = CANCELLATIONS.filter((c) => c.category === cat).length;
});

// eslint-disable-next-line no-unused-vars
const CATEGORY_LABELS = {
	arbeit: { label: "Arbeit", emoji: "💼" },
	familie: { label: "Familie", emoji: "👨‍👩‍👧" },
	gesundheit: { label: "Gesundheit", emoji: "🤒" },
	muede: { label: "Müdigkeit", emoji: "😴" },
	wetter: { label: "Wetter", emoji: "🌧️" },
	freizeit: { label: "Andere Pläne", emoji: "🎉" },
	kreativ: { label: "Kreativ", emoji: "🎨" },
	keine_lust: { label: "Keine Lust", emoji: "😬" },
};

// Stammtisch-König (höchste Teilnahme)
// eslint-disable-next-line no-unused-vars
const STAMMTISCH_KOENIG = USER_RANKING[0];

// Awards
// eslint-disable-next-line no-unused-vars
const AWARDS = {
	koenig: USER_RANKING[0],
	fastImmerDa: USER_RANKING[1],
	potenzial: USER_RANKING[USER_RANKING.length - 1],
	ausredenLegende: USER_STATS.reduce((prev, curr) =>
		(curr.cancellations?.filter((c) => c.category === "kreativ").length || 0) >
		(prev.cancellations?.filter((c) => c.category === "kreativ").length || 0)
			? curr
			: prev,
	),
	derUnsichtbare: USER_RANKING[USER_RANKING.length - 1],
};

// Debug: Log Statistiken
console.log("📊 Stammtisch Wrapped 2025 - Daten geladen");
console.log("Globale Stats:", GLOBAL_STATS);
console.log(
	"User Ranking:",
	USER_RANKING.map((u) => `${u.rank}. ${u.name}: ${u.attendanceRate}%`),
);
