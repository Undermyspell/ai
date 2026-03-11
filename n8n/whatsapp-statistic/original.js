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

// --- Prozent-Emoji-System ---
function emojiForPercent(percent) {
    if (percent > 85) return "🟩";   // Top Tier
    if (percent > 70) return "🟦";   // High
    if (percent > 50) return "🟨";   // Mid
    if (percent > 30) return "🟧";   // Low
    return "🟥";                      // Bottom
}

// --- Dynamische Streak-Emojis & deutsche Texte ---
function streakText(streak) {
    if (streak > 0) return `${"🔥".repeat(Math.min(streak,3))} *${streak}-er Serie*`;
    if (streak < 0) return `${"❄️".repeat(Math.min(Math.abs(streak),3))} *-${Math.abs(streak)}-er Serie*`;
    return "";
}

// --- Medaillen vergeben mit Gleichstand ---
const medals = ["🥇", "🥈", "🥉"];
let lastAttendance = null;
let lastPercent = null;
let rank = 0;

const userMedals = users.map(u => {
    let medal = "";

    // Neuer Rang nur wenn kein Gleichstand
    if (lastAttendance !== u.attendance || lastPercent !== u.percent) {
        rank++;
    }

    if (rank <= medals.length) {
        medal = medals[rank - 1];
    } else {
        medal = emojiForPercent(u.percent);
    }

    lastAttendance = u.attendance;
    lastPercent = u.percent;

    return { ...u, medal };
});

// --- Build Report ---
let text = `🍻 *Zumba Jahreswertung*\n\n`;
text += "\nZeitraum:\n*Weihnachtsfeier bis Weihnachtsfeier*\n";
text += `\n_Stammtische insgesamt_: \n${total}\n\n`;

text += userMedals.map(u => {
    let startDateText = "";
    if (u.start_date) {
        const d = new Date(u.start_date);
        startDateText = `(ab ${d.getDate()}.${d.getMonth()+1}.)`;
    }
    const streakLabel = streakText(u.streak);
    return `${u.medal} *${u.name}*: ${u.attendance}-${u.away} (${u.percent}%) ${startDateText} ${streakLabel}`;
}).join('\n');

text += `\n\n🤖🍺 *Automatisch erstellt vom Zumba-Bot*`;

return {
    message: text,
    receiver: $('Webhook').first().json.body.data.key.remoteJid
};
