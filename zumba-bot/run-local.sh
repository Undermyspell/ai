#!/usr/bin/env bash
# Startet whatsapp-bot + zumba-admin-ui zusammen zum lokalen Ausprobieren.
#   ./run-local.sh
# Bot läuft auf :8080 (nutzt whatsapp-bot/.env), Admin-UI auf :8090 mit
# BOT_URL auf den Bot. Strg-C beendet beide. Ports via BOT_PORT/UI_PORT überschreibbar.
set -euo pipefail
cd "$(dirname "$0")"

BOT_PORT="${BOT_PORT:-8080}"
UI_PORT="${UI_PORT:-8090}"

echo "🔨 Building bot + admin-ui..."
( cd whatsapp-bot   && go build -o ./tmp/server ./cmd/server )
( cd zumba-admin-ui && go build -o ./tmp/server ./cmd/server )

# kill 0 beendet die ganze Prozessgruppe (beide Server + sed-Pipes) bei Exit/Strg-C.
trap 'kill 0' EXIT INT TERM

echo
echo "🤖 whatsapp-bot → http://localhost:${BOT_PORT}"
echo "🖥  admin-ui     → http://localhost:${UI_PORT}   (Bot-Test: /bot-test)"
echo "   Strg-C beendet beide."
echo

( cd whatsapp-bot   && PORT="${BOT_PORT}" ./tmp/server 2>&1 | sed -u 's/^/[bot] /' ) &
( cd zumba-admin-ui && PORT="${UI_PORT}" BOT_URL="http://localhost:${BOT_PORT}" ./tmp/server 2>&1 | sed -u 's/^/[ui ] /' ) &

wait
