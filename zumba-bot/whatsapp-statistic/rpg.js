// --- Map DB Input ---
const users = $input.all().map(item => ({
    name: item.json.user_name,
    attendance: Number(item.json.attendance_count),
    away: Number(item.json.away_count),
    start_date: item.json.start_date,
    percent: Number(item.json.attend_percentage),
    streak: Number(item.json.streak)
}));

const total = users[0].attendance + users[0].away;

// --- Sortiere User nach Attendance & Prozent ---
users.sort((a, b) => {
    if (b.attendance !== a.attendance) return b.attendance - a.attendance;
    return b.percent - a.percent;
});

// --- Stammtisch-Rang & Icon nach Prozent ---
function stammtischRang(percent) {
    if (percent > 85) return { icon: "🍺", rang: "PLATZHIRSCH", skill: "Eiserner Hintern" };
    if (percent > 70) return { icon: "🍻", rang: "STAMMGAST", skill: "Immer dabei" };
    if (percent > 55) return { icon: "🪑", rang: "GELEGENHEITSTRINKER", skill: "Mal so, mal so" };
    if (percent > 40) return { icon: "🚪", rang: "ZAUNGAST", skill: "Einen Fuß in der Tür" };
    if (percent > 25) return { icon: "👻", rang: "KARTEILEICHE", skill: "Unsichtbar" };
    return { icon: "📭", rang: "VERMISST", skill: "Verschollen" };
}

// --- Promille-Balken ---
function promilleBar(percent, length = 10) {
    const filled = Math.round((percent / 100) * length);
    return "🟢".repeat(filled) + "⚫".repeat(length - filled);
}

// --- Streak als Runde/Fehlschicht ---
function streakText(streak) {
    if (streak > 0) return `🔥 +${streak} Runden in Folge`;
    if (streak < 0) return `❄️ ${Math.abs(streak)}× geschwänzt`;
    return "";
}

// --- Build Report ---
let text = `🍺 *STAMMTISCH CHRONICLES*\n`;
text += `_Saison ${total} · Die Tafelrunde_\n`;

text += users.map(u => {
    let startDateText = "";
    if (u.start_date) {
        const d = new Date(u.start_date);
        startDateText = ` _(ab ${d.getDate()}.${d.getMonth() + 1}.)_`;
    }
    const rang = stammtischRang(u.percent);
    const bar = promilleBar(u.percent);
    const streak = streakText(u.streak);

    let line = `\n${rang.icon} *${u.name}* – ${rang.rang}  Runde ${u.attendance}${startDateText}`;
    line += `\n  ${bar} ${u.percent}%`;
    const parts = [];
    if (streak) parts.push(streak);
    parts.push(`Spezial: ${rang.skill}`);
    line += `\n  ${parts.join(" · ")}`;
    return line;
}).join('\n');

text += `\n\n_Nächste Runde = nächster Aufstieg!_`;
text += `\n🤖🍺 *Automatisch erstellt vom Zumba-Bot*`;

return {
    message: text,
    receiver: $('Webhook').first().json.body.data.key.remoteJid
};
