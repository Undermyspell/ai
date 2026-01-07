// Stammtisch Wrapped 2025 - App Logic
// =====================================

class StammtischWrapped {
	constructor() {
		this.currentSlide = 0;
		this.slides = [];
		this.timer = null;
		this.progressInterval = null;
		this.isPaused = false;
		this.slideDuration = 5000; // Default: 5 Sekunden

		this.init();
	}

	init() {
		// Slides sammeln
		this.slides = document.querySelectorAll(".slide");
		this.totalSlides = this.slides.length;

		// Navigation Dots generieren
		this.generateNavDots();

		// Event Listeners
		this.setupEventListeners();

		// Erste Slide anzeigen
		this.showSlide(0);

		// Auto-Navigation starten
		this.startAutoNavigation();

		console.log(
			`🍺 Stammtisch Wrapped gestartet mit ${this.totalSlides} Slides`,
		);
	}

	generateNavDots() {
		const container = document.getElementById("nav-dots");
		container.innerHTML = "";

		for (let i = 0; i < this.totalSlides; i++) {
			const dot = document.createElement("button");
			dot.className = `w-2 h-2 rounded-full transition-all duration-300 ${
				i === 0 ? "bg-biergold w-6" : "bg-schaum/30"
			}`;
			dot.setAttribute("data-slide", i);
			dot.addEventListener("click", () => this.goToSlide(i));
			container.appendChild(dot);
		}
	}

	updateNavDots() {
		const dots = document.querySelectorAll("#nav-dots button");
		dots.forEach((dot, index) => {
			if (index === this.currentSlide) {
				dot.className =
					"w-6 h-2 rounded-full bg-biergold transition-all duration-300";
			} else if (index < this.currentSlide) {
				dot.className =
					"w-2 h-2 rounded-full bg-biergold/50 transition-all duration-300";
			} else {
				dot.className =
					"w-2 h-2 rounded-full bg-schaum/30 transition-all duration-300";
			}
		});
	}

	setupEventListeners() {
		// Tap/Click für nächste Slide
		document.addEventListener("click", (e) => {
			// Ignoriere Klicks auf Navigation
			if (e.target.closest("#nav-dots")) return;
			this.nextSlide();
		});

		// Keyboard Navigation
		document.addEventListener("keydown", (e) => {
			switch (e.key) {
				case "ArrowRight":
				case " ":
					this.nextSlide();
					break;
				case "ArrowLeft":
					this.prevSlide();
					break;
				case "Escape":
					this.togglePause();
					break;
			}
		});

		// Touch Swipe
		let touchStartX = 0;
		let touchEndX = 0;

		document.addEventListener(
			"touchstart",
			(e) => {
				touchStartX = e.changedTouches[0].screenX;
			},
			{ passive: true },
		);

		document.addEventListener(
			"touchend",
			(e) => {
				touchEndX = e.changedTouches[0].screenX;
				this.handleSwipe(touchStartX, touchEndX);
			},
			{ passive: true },
		);
	}

	handleSwipe(startX, endX) {
		const threshold = 50;
		const diff = startX - endX;

		if (Math.abs(diff) > threshold) {
			if (diff > 0) {
				this.nextSlide();
			} else {
				this.prevSlide();
			}
		}
	}

	showSlide(index) {
		// Bounds check
		if (index < 0 || index >= this.totalSlides) return;

		// Alte Slide ausblenden
		this.slides.forEach((slide) => {
			slide.classList.remove("active");
		});

		// Neue Slide einblenden
		this.currentSlide = index;
		const currentSlideEl = this.slides[index];
		currentSlideEl.classList.add("active");

		// Animationen triggern
		this.triggerAnimations(currentSlideEl);

		// Nav Dots updaten
		this.updateNavDots();

		// Slide-spezifische Duration
		this.slideDuration = parseInt(currentSlideEl.dataset.duration, 10) || 5000;
	}

	triggerAnimations(slideEl) {
		// Alle animierten Elemente zurücksetzen und neu triggern
		const animatedElements = slideEl.querySelectorAll(".animate-on-enter");

		animatedElements.forEach((el) => {
			// Animation zurücksetzen
			el.style.animation = "none";
			el.offsetHeight; // Reflow trigger
			el.style.animation = "";

			// Opacity auf 1 setzen nach Animation
			setTimeout(() => {
				el.style.opacity = "1";
			}, 2000);
		});

		// Counter-Animationen starten wenn vorhanden
		const counters = slideEl.querySelectorAll("[data-count-to]");
		counters.forEach((counter) => {
			this.animateCounter(counter);
		});
	}

	animateCounter(element) {
		const target = parseInt(element.dataset.countTo, 10);
		const duration = parseInt(element.dataset.countDuration, 10) || 2000;
		const suffix = element.dataset.countSuffix || "";
		const start = 0;
		const startTime = performance.now();

		const updateCounter = (currentTime) => {
			const elapsed = currentTime - startTime;
			const progress = Math.min(elapsed / duration, 1);

			// Easing
			const easeOutQuart = 1 - (1 - progress) ** 4;
			const current = Math.floor(start + (target - start) * easeOutQuart);

			element.textContent = current + suffix;

			if (progress < 1) {
				requestAnimationFrame(updateCounter);
			} else {
				element.textContent = target + suffix;
			}
		};

		requestAnimationFrame(updateCounter);
	}

	nextSlide() {
		this.resetTimer();

		if (this.currentSlide < this.totalSlides - 1) {
			this.showSlide(this.currentSlide + 1);
			this.startAutoNavigation();
		} else {
			// Ende erreicht - optional: Loop oder Stop
			console.log("🎉 Wrapped fertig!");
			this.stopAutoNavigation();
		}
	}

	prevSlide() {
		this.resetTimer();

		if (this.currentSlide > 0) {
			this.showSlide(this.currentSlide - 1);
			this.startAutoNavigation();
		}
	}

	goToSlide(index) {
		this.resetTimer();
		this.showSlide(index);
		this.startAutoNavigation();
	}

	startAutoNavigation() {
		if (this.isPaused) return;

		this.resetTimer();

		let elapsed = 0;
		const progressBar = document.getElementById("progress-bar");

		// Progress Bar Animation
		this.progressInterval = setInterval(() => {
			elapsed += 100;
			const progress = (elapsed / this.slideDuration) * 100;
			progressBar.style.width = `${progress}%`;
		}, 100);

		// Auto-Navigation Timer
		this.timer = setTimeout(() => {
			this.nextSlide();
		}, this.slideDuration);
	}

	stopAutoNavigation() {
		this.resetTimer();
		const progressBar = document.getElementById("progress-bar");
		progressBar.style.width = "100%";
	}

	resetTimer() {
		if (this.timer) {
			clearTimeout(this.timer);
			this.timer = null;
		}
		if (this.progressInterval) {
			clearInterval(this.progressInterval);
			this.progressInterval = null;
		}

		const progressBar = document.getElementById("progress-bar");
		progressBar.style.width = "0%";
	}

	togglePause() {
		this.isPaused = !this.isPaused;

		if (this.isPaused) {
			this.resetTimer();
			console.log("⏸️ Pausiert");
		} else {
			this.startAutoNavigation();
			console.log("▶️ Fortgesetzt");
		}
	}
}

// =====================================
// DYNAMISCHE CONTENT GENERIERUNG
// =====================================

function populateDynamicContent() {
	// Globale Statistiken einfügen
	const totalAttendances = document.getElementById("total-attendances");
	const totalCancellations = document.getElementById("total-cancellations");
	const avgRate = document.getElementById("avg-rate");

	if (totalAttendances)
		totalAttendances.dataset.countTo = GLOBAL_STATS.totalAttendances;
	if (totalCancellations)
		totalCancellations.dataset.countTo = GLOBAL_STATS.totalCancellations;
	if (avgRate) avgRate.dataset.countTo = GLOBAL_STATS.averageAttendanceRate;

	// Rankings generieren
	generateRankings();

	// Streaks generieren
	generateStreaks();

	// Ausreden-Kategorien
	generateExcuseCategories();

	// Beste Ausreden
	generateBestExcuses();

	// Heatmap
	generateHeatmap();
}

function generateRankings() {
	// Top 5
	const top5Container = document.getElementById("top5-ranking");
	if (top5Container) {
		top5Container.innerHTML = USER_RANKING.slice(0, 5)
			.map((user, i) => createRankingCard(user, i, "top"))
			.join("");
	}

	// Plätze 6-10
	const midContainer = document.getElementById("mid-ranking");
	if (midContainer) {
		midContainer.innerHTML = USER_RANKING.slice(5, 10)
			.map((user, i) => createRankingCard(user, i + 5, "mid"))
			.join("");
	}

	// Plätze 11-15
	const bottomContainer = document.getElementById("bottom-ranking");
	if (bottomContainer) {
		bottomContainer.innerHTML = USER_RANKING.slice(10, 15)
			.map((user, i) => createRankingCard(user, i + 10, "bottom"))
			.join("");
	}
}

function createRankingCard(user, index, tier) {
	const medals = ["🥇", "🥈", "🥉"];
	const rankDisplay =
		index < 3
			? medals[index]
			: `<span class="text-schaum/50">#${user.rank}</span>`;

	const bgColor =
		tier === "top"
			? "bg-gradient-to-r from-biergold/30 to-holz-light/50"
			: tier === "mid"
				? "bg-holz-light/40"
				: "bg-holz-light/20";

	const barColor =
		user.attendanceRate >= 80
			? "bg-green-500"
			: user.attendanceRate >= 60
				? "bg-biergold"
				: user.attendanceRate >= 40
					? "bg-orange-500"
					: "bg-red-400";

	const delay = (index % 5) * 100 + 200;

	return `
        <div class="animate-on-enter animate-slide-left delay-${delay} ${bgColor} rounded-xl p-4 flex items-center gap-4">
            <div class="text-2xl w-10 text-center">${rankDisplay}</div>
            <div class="flex-1">
                <div class="flex items-center gap-2 mb-1">
                    <span class="text-xl">${user.emoji}</span>
                    <span class="font-bold text-schaum">${user.name}</span>
                </div>
                <div class="text-xs text-schaum/60 mb-2">${user.titleEmoji} ${user.title}</div>
                <div class="w-full bg-holz rounded-full h-2">
                    <div class="${barColor} h-2 rounded-full transition-all duration-1000" style="width: ${user.attendanceRate}%"></div>
                </div>
            </div>
            <div class="text-right">
                <span class="text-2xl font-bold text-biergold">${user.attendanceRate}%</span>
            </div>
        </div>
    `;
}

function generateStreaks() {
	// Beste Anwesenheits-Streaks
	const attendanceStreaks = [...USER_STATS]
		.sort((a, b) => b.maxAttendanceStreak - a.maxAttendanceStreak)
		.slice(0, 3);

	const attendanceContainer = document.getElementById("attendance-streaks");
	if (attendanceContainer) {
		attendanceContainer.innerHTML = attendanceStreaks
			.map(
				(user, i) => `
            <div class="animate-on-enter animate-scale-in delay-${i * 200 + 200} bg-gradient-to-r from-orange-500/30 to-red-500/20 rounded-xl p-4 flex items-center gap-4">
                <div class="text-3xl animate-fire">🔥</div>
                <div class="flex-1">
                    <div class="font-bold text-schaum">${user.emoji} ${user.name}</div>
                    <div class="text-sm text-schaum/60">${user.maxAttendanceStreak} Wochen am Stück da!</div>
                </div>
                <div class="text-3xl font-bold text-orange-400">${user.maxAttendanceStreak}</div>
            </div>
        `,
			)
			.join("");
	}

	// Längste Absage-Streaks
	const cancellationStreaks = [...USER_STATS]
		.filter((u) => u.maxCancellationStreak > 0)
		.sort((a, b) => b.maxCancellationStreak - a.maxCancellationStreak)
		.slice(0, 3);

	const cancellationContainer = document.getElementById("cancellation-streaks");
	if (cancellationContainer) {
		cancellationContainer.innerHTML = cancellationStreaks
			.map(
				(user, i) => `
            <div class="animate-on-enter animate-scale-in delay-${i * 200 + 700} bg-gradient-to-r from-blue-500/30 to-cyan-500/20 rounded-xl p-4 flex items-center gap-4">
                <div class="text-3xl">🧊</div>
                <div class="flex-1">
                    <div class="font-bold text-schaum">${user.emoji} ${user.name}</div>
                    <div class="text-sm text-schaum/60">${user.maxCancellationStreak} Wochen gefehlt</div>
                </div>
                <div class="text-3xl font-bold text-blue-400">${user.maxCancellationStreak}</div>
            </div>
        `,
			)
			.join("");
	}
}

function generateExcuseCategories() {
	const container = document.getElementById("excuse-categories");
	if (!container) return;

	const sortedCategories = Object.entries(CATEGORY_STATS)
		.sort((a, b) => b[1] - a[1])
		.filter(([_, count]) => count > 0);

	const maxCount = sortedCategories[0]?.[1] || 1;

	container.innerHTML = sortedCategories
		.map(([category, count], i) => {
			const info = CATEGORY_LABELS[category];
			const percentage = Math.round((count / maxCount) * 100);

			return `
            <div class="animate-on-enter animate-slide-left delay-${i * 100 + 200} bg-holz-light/40 rounded-xl p-4">
                <div class="flex items-center justify-between mb-2">
                    <div class="flex items-center gap-2">
                        <span class="text-2xl">${info.emoji}</span>
                        <span class="font-bold text-schaum">${info.label}</span>
                    </div>
                    <span class="text-biergold font-bold">${count}x</span>
                </div>
                <div class="w-full bg-holz rounded-full h-3">
                    <div class="bg-biergold h-3 rounded-full transition-all duration-1000" style="width: ${percentage}%"></div>
                </div>
            </div>
        `;
		})
		.join("");
}

function generateBestExcuses() {
	const container = document.getElementById("best-excuses");
	if (!container) return;

	// Finde kreative Ausreden
	const creativeExcuses = CANCELLATIONS.filter(
		(c) => c.category === "kreativ",
	).slice(0, 5);

	container.innerHTML = creativeExcuses
		.map(
			(excuse, i) => `
        <div class="animate-on-enter animate-fade-in-up delay-${i * 200 + 200} bg-gradient-to-r from-purple-500/20 to-pink-500/20 rounded-xl p-4">
            <div class="flex items-start gap-3">
                <span class="text-2xl">💬</span>
                <div class="flex-1">
                    <p class="text-schaum italic">"${excuse.message}"</p>
                    <p class="text-schaum/50 text-sm mt-2">— ${excuse.userName}</p>
                </div>
            </div>
        </div>
    `,
		)
		.join("");
}

function generateHeatmap() {
	const container = document.getElementById("heatmap");
	const insightContainer = document.getElementById("heatmap-insight");
	if (!container) return;

	const months = [
		"Jan",
		"Feb",
		"Mär",
		"Apr",
		"Mai",
		"Jun",
		"Jul",
		"Aug",
		"Sep",
		"Okt",
		"Nov",
		"Dez",
	];

	const monthData = months.map((month, i) => {
		const monthKey = `2025-${String(i + 1).padStart(2, "0")}`;
		return {
			month,
			count: CANCELLATIONS_BY_MONTH[monthKey] || 0,
		};
	});

	const maxCount = Math.max(...monthData.map((m) => m.count), 1);

	container.innerHTML = `
        <div class="grid grid-cols-4 gap-2">
            ${monthData
							.map((data, i) => {
								const intensity = data.count / maxCount;
								const bgColor =
									intensity > 0.75
										? "bg-red-500"
										: intensity > 0.5
											? "bg-orange-500"
											: intensity > 0.25
												? "bg-yellow-500"
												: intensity > 0
													? "bg-green-500/50"
													: "bg-holz-light/30";

								return `
                    <div class="animate-on-enter animate-scale-in delay-${i * 50 + 200} ${bgColor} rounded-lg p-3 text-center">
                        <div class="text-xs text-schaum/70">${data.month}</div>
                        <div class="text-lg font-bold text-schaum">${data.count}</div>
                    </div>
                `;
							})
							.join("")}
        </div>
        <div class="flex justify-center gap-2 mt-4 text-xs text-schaum/50">
            <span class="flex items-center gap-1"><span class="w-3 h-3 bg-green-500/50 rounded"></span> Wenig</span>
            <span class="flex items-center gap-1"><span class="w-3 h-3 bg-yellow-500 rounded"></span> Mittel</span>
            <span class="flex items-center gap-1"><span class="w-3 h-3 bg-red-500 rounded"></span> Viel</span>
        </div>
    `;

	// Insight generieren
	const worstMonth = monthData.reduce((prev, curr) =>
		curr.count > prev.count ? curr : prev,
	);
	const bestMonth = monthData
		.filter((m) => m.count > 0)
		.reduce(
			(prev, curr) => (curr.count < prev.count ? curr : prev),
			monthData[0],
		);

	if (insightContainer) {
		insightContainer.innerHTML = `
            <p class="text-schaum/70">😰 Schlechtester Monat: <span class="text-red-400 font-bold">${worstMonth.month}</span> mit ${worstMonth.count} Absagen</p>
            <p class="text-schaum/70 mt-1">🎉 Bester Monat: <span class="text-green-400 font-bold">${bestMonth.month}</span> mit nur ${bestMonth.count} Absagen</p>
        `;
	}
}

// =====================================
// PHASE 3: NEUE FEATURES
// =====================================

function generateAISummary() {
	const container = document.getElementById("ai-summary");
	if (!container) return;

	// Finde interessante Statistiken
	const topUser = USER_RANKING[0];
	const bottomUser = USER_RANKING[USER_RANKING.length - 1];
	const avgRate = GLOBAL_STATS.averageAttendanceRate;

	// Finde den beliebtesten Monat
	const monthNames = [
		"Januar",
		"Februar",
		"März",
		"April",
		"Mai",
		"Juni",
		"Juli",
		"August",
		"September",
		"Oktober",
		"November",
		"Dezember",
	];
	const monthData = monthNames.map((_, i) => {
		const monthKey = `2025-${String(i + 1).padStart(2, "0")}`;
		return {
			month: monthNames[i],
			count: CANCELLATIONS_BY_MONTH[monthKey] || 0,
		};
	});
	const worstMonth = monthData.reduce((prev, curr) =>
		curr.count > prev.count ? curr : prev,
	);
	const bestMonth = monthData
		.filter((m) => m.count > 0)
		.reduce(
			(prev, curr) => (curr.count < prev.count ? curr : prev),
			monthData[0],
		);

	// Generiere AI-Style Text
	const summaries = [
		`2025 war ein Jahr der Hingabe – mit einer durchschnittlichen Teilnahme von <span class="text-biergold font-bold">${avgRate}%</span>. ${topUser.name} führte das Feld an, während ${bottomUser.name} noch Potenzial nach oben hat. Im ${worstMonth.month} war die Motivation am niedrigsten, aber im ${bestMonth.month} zeigte sich wahre Stammtisch-Treue!`,
		`Der Stammtisch 2025: Eine Geschichte von Bier, Freundschaft und... kreativen Ausreden. <span class="text-biergold font-bold">${topUser.name}</span> war der unerschütterliche Fels, während <span class="text-biergold font-bold">${bottomUser.name}</span> eher spirituell dabei war. Der ${worstMonth.month} forderte uns heraus – aber wir haben durchgehalten!`,
		`Was für ein Jahr! <span class="text-biergold font-bold">${GLOBAL_STATS.totalAttendances}</span> mal wurde am Stammtisch angestoßen. ${topUser.name} verpasste kaum einen Donnerstag, während ${bottomUser.name} den Begriff "Stammtisch" eher flexibel interpretierte. Der Sommer war stark, der ${worstMonth.month} war eine Herausforderung.`,
	];

	container.innerHTML = summaries[Math.floor(Math.random() * summaries.length)];
}

function generatePersonalSlides() {
	// Kompakte persönliche Übersicht in 3 Gruppen

	// Persönliche Nachrichten basierend auf Stats
	const getPersonalMessage = (user) => {
		if (user.attendanceRate >= 90) return "Legende! 🏆";
		if (user.attendanceRate >= 80) return "Stark dabei! 💪";
		if (user.attendanceRate >= 70) return "Solide! 📈";
		if (user.attendanceRate >= 60) return "Wenn's passt 🤷";
		if (user.attendanceRate >= 50) return "Spontan 🎲";
		return "Mysteriös 👻";
	};

	// Fun Facts pro User (kurz)
	const getFunFact = (user) => {
		if (user.neverCancelled) return "👑 Nie abgesagt!";
		if (user.maxAttendanceStreak >= 10)
			return `🔥 ${user.maxAttendanceStreak}er Serie`;
		if (user.maxCancellationStreak >= 4)
			return `🧊 ${user.maxCancellationStreak} Wochen Pause`;
		if (user.favoriteExcuseCategory === "kreativ") return "🎨 Kreativ-Ausreder";
		if (user.favoriteExcuseCategory === "arbeit") return "💼 Workaholic";
		return `${user.cancellationCount}x gefehlt`;
	};

	// Generiere kompakte User-Karte
	const createUserCard = (user, index, tierColor) => {
		const barColor =
			user.attendanceRate >= 80
				? "bg-green-500"
				: user.attendanceRate >= 60
					? "bg-biergold"
					: user.attendanceRate >= 40
						? "bg-orange-500"
						: "bg-red-400";

		return `
			<div class="animate-on-enter animate-slide-left delay-${index * 100 + 100} ${tierColor} rounded-xl p-3 flex items-center gap-3">
				<span class="text-3xl">${user.emoji}</span>
				<div class="flex-1 min-w-0">
					<div class="flex items-center gap-2">
						<span class="font-bold text-schaum truncate">${user.name}</span>
						<span class="text-xs text-schaum/50">${user.titleEmoji}</span>
					</div>
					<div class="flex items-center gap-2 mt-1">
						<div class="flex-1 bg-holz rounded-full h-1.5">
							<div class="${barColor} h-1.5 rounded-full" style="width: ${user.attendanceRate}%"></div>
						</div>
						<span class="text-xs text-schaum/60">${getFunFact(user)}</span>
					</div>
				</div>
				<div class="text-right">
					<span class="text-xl font-bold text-biergold">${user.attendanceRate}%</span>
					<p class="text-xs text-schaum/50">${getPersonalMessage(user)}</p>
				</div>
			</div>
		`;
	};

	// Top 5
	const top5Container = document.getElementById("personal-top5");
	if (top5Container) {
		top5Container.innerHTML = USER_RANKING.slice(0, 5)
			.map((user, i) =>
				createUserCard(
					user,
					i,
					"bg-gradient-to-r from-yellow-500/20 to-amber-600/10",
				),
			)
			.join("");
	}

	// Mitte 5
	const mid5Container = document.getElementById("personal-mid5");
	if (mid5Container) {
		mid5Container.innerHTML = USER_RANKING.slice(5, 10)
			.map((user, i) => createUserCard(user, i, "bg-holz-light/40"))
			.join("");
	}

	// Bottom 5
	const bottom5Container = document.getElementById("personal-bottom5");
	if (bottom5Container) {
		bottom5Container.innerHTML = USER_RANKING.slice(10, 15)
			.map((user, i) => createUserCard(user, i, "bg-holz-light/20"))
			.join("");
	}
}

function generatePersonalityTypes() {
	const container = document.getElementById("personality-types");
	if (!container) return;

	// Definiere Persönlichkeitstypen
	const types = [
		{
			emoji: "🪨",
			name: "Der Fels",
			description: "Immer da, immer zuverlässig",
			users: USER_RANKING.filter((u) => u.attendanceRate >= 85),
		},
		{
			emoji: "🎲",
			name: "Der Spontane",
			description: "Kommt wenn der Vibe stimmt",
			users: USER_RANKING.filter(
				(u) => u.attendanceRate >= 50 && u.attendanceRate < 70,
			),
		},
		{
			emoji: "🎨",
			name: "Der Kreative",
			description: "Hat die besten Ausreden",
			users: USER_STATS.filter((u) => u.favoriteExcuseCategory === "kreativ"),
		},
		{
			emoji: "👻",
			name: "Das Phantom",
			description: "Selten gesichtet, aber legendär",
			users: USER_RANKING.filter((u) => u.attendanceRate < 50),
		},
	];

	container.innerHTML = types
		.filter((t) => t.users.length > 0)
		.map(
			(type, i) => `
			<div class="animate-on-enter animate-slide-left delay-${i * 150 + 200} bg-holz-light/40 rounded-xl p-4">
				<div class="flex items-center gap-3 mb-2">
					<span class="text-3xl">${type.emoji}</span>
					<div>
						<h3 class="font-bold text-biergold">${type.name}</h3>
						<p class="text-xs text-schaum/50">${type.description}</p>
					</div>
				</div>
				<div class="flex flex-wrap gap-2 mt-2">
					${type.users
						.slice(0, 5)
						.map(
							(u) =>
								`<span class="bg-holz/50 rounded-full px-2 py-1 text-xs text-schaum">${u.emoji} ${u.name}</span>`,
						)
						.join("")}
					${type.users.length > 5 ? `<span class="text-schaum/50 text-xs">+${type.users.length - 5}</span>` : ""}
				</div>
			</div>
		`,
		)
		.join("");
}

function generateAwards() {
	const container = document.getElementById("awards-container");
	if (!container) return;

	const awardsList = [
		{
			emoji: "👑",
			title: "Stammtisch-König",
			subtitle: "Höchste Teilnahme 2025",
			winner: AWARDS.koenig,
			color: "from-yellow-500/30 to-amber-600/20",
		},
		{
			emoji: "🥈",
			title: "Fast immer da",
			subtitle: "Zweithöchste Teilnahme",
			winner: AWARDS.fastImmerDa,
			color: "from-gray-400/30 to-gray-500/20",
		},
		{
			emoji: "🔥",
			title: "Streak-Master",
			subtitle: "Längste Anwesenheits-Serie",
			winner: [...USER_STATS].sort(
				(a, b) => b.maxAttendanceStreak - a.maxAttendanceStreak,
			)[0],
			color: "from-orange-500/30 to-red-500/20",
		},
		{
			emoji: "🎨",
			title: "Ausreden-Künstler",
			subtitle: "Kreativste Entschuldigungen",
			winner: AWARDS.ausredenLegende,
			color: "from-purple-500/30 to-pink-500/20",
		},
		{
			emoji: "🚀",
			title: "Potenzial 2026",
			subtitle: "Raum nach oben",
			winner: AWARDS.potenzial,
			color: "from-blue-500/30 to-cyan-500/20",
		},
	];

	container.innerHTML = awardsList
		.map(
			(award, i) => `
		<div class="animate-on-enter animate-scale-in delay-${i * 200 + 200} bg-gradient-to-r ${award.color} rounded-xl p-4">
			<div class="flex items-center gap-4">
				<span class="text-4xl">${award.emoji}</span>
				<div class="flex-1">
					<h3 class="font-bold text-biergold text-lg">${award.title}</h3>
					<p class="text-xs text-schaum/50">${award.subtitle}</p>
				</div>
				<div class="text-right">
					<span class="text-2xl">${award.winner.emoji}</span>
					<p class="font-bold text-schaum">${award.winner.name}</p>
				</div>
			</div>
		</div>
	`,
		)
		.join("");
}

function generateConfetti() {
	const container = document.getElementById("confetti-container");
	if (!container) return;

	const colors = ["#F59E0B", "#FEF3C7", "#D97706", "#92400E", "#ffffff"];
	const confettiCount = 50;

	let confettiHTML = "";
	for (let i = 0; i < confettiCount; i++) {
		const color = colors[Math.floor(Math.random() * colors.length)];
		const left = Math.random() * 100;
		const delay = Math.random() * 3;
		const duration = 3 + Math.random() * 2;
		const size = 5 + Math.random() * 10;

		confettiHTML += `
			<div style="
				position: absolute;
				left: ${left}%;
				top: -20px;
				width: ${size}px;
				height: ${size}px;
				background: ${color};
				border-radius: ${Math.random() > 0.5 ? "50%" : "0"};
				animation: confetti ${duration}s ease-in ${delay}s infinite;
			"></div>
		`;
	}

	container.innerHTML = confettiHTML;
}

// App starten wenn DOM ready
document.addEventListener("DOMContentLoaded", () => {
	// Dynamische Inhalte zuerst generieren
	populateDynamicContent();

	// Phase 3 Features generieren
	generateAISummary();
	generatePersonalSlides();
	generatePersonalityTypes();
	generateAwards();
	generateConfetti();

	// Dann App starten
	window.app = new StammtischWrapped();
});
