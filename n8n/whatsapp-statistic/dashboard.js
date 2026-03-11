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

// --- Balken mit ▰▱ ---
function barChart(percent, length = 6) {
    const filled = Math.round((percent / 100) * length);
    return "▰".repeat(filled) + "▱".repeat(length - filled);
}

// --- Medaillen mit Gleichstand ---
const medals = ["🥇", "🥈", "🥉"];
let lastAttendance = null;
let lastPercent = null;
let rank = 0;

const ranked = users.map(u => {
    if (lastAttendance !== u.attendance || lastPercent !== u.percent) {
        rank++;
    }
    const medal = rank <= medals.length ? medals[rank - 1] : `${rank} `;
    lastAttendance = u.attendance;
    lastPercent = u.percent;
    return { ...u, medal, rank };
});

// --- Highlights ---
const mvp = ranked[0];
const avgPercent = Math.round(users.reduce((s, u) => s + u.percent, 0) / users.length);
const hottest = ranked.reduce((a, b) => a.streak > b.streak ? a : b);
const coldest = ranked.reduce((a, b) => a.streak < b.streak ? a : b);

// --- Build Report ---
let text = `🍻 *ZUMBA STATS*\n`;
text += `_Weihnachtsfeier → Weihnachtsfeier_\n\n`;

text += `📊 *${total}* Stammtische\n\n`;
text += `👑 *MVP:* ${mvp.name} (${mvp.percent}%)\n`;
if (hottest.streak > 0) text += `🔥 *Heißeste Serie:* ${hottest.name} (${hottest.streak}x)\n`;
if (coldest.streak < 0) text += `❄️ *Längste Pause:* ${coldest.name} (${Math.abs(coldest.streak)}x)\n`;
text += `📈 *Durchschnitt:* ${avgPercent}%\n\n`;

text += `── *RANGLISTE* ──\n\n`;

text += ranked.map(u => {
    let startDateText = "";
    if (u.start_date) {
        const d = new Date(u.start_date);
        startDateText = ` _(${d.getDate()}.${d.getMonth() + 1}.)_`;
    }
    const bar = barChart(u.percent);
    let streakLabel = "";
    if (u.streak > 0) streakLabel = ` 🔥+${u.streak}`;
    if (u.streak < 0) streakLabel = ` ❄️${u.streak}`;
    return `${u.medal} *${u.name}* ${bar} ${u.attendance}-${u.away} (${u.percent}%)${streakLabel}${startDateText}`;
}).join('\n');

text += `\n\n🤖🍺 *Automatisch erstellt vom Zumba-Bot*`;

return {
    message: text,
    receiver: $('Webhook').first().json.body.data.key.remoteJid
};
