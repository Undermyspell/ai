#!/usr/bin/env bash
# Startet whatsapp-bot + zumba-admin-ui zusammen mit Air (Hot-Reload).
#   ./dev-local.sh
# Bot läuft auf :8080 (nutzt whatsapp-bot/.env), Admin-UI auf :8090 mit
# BOT_URL auf den Bot. Beide bauen bei Dateiänderungen automatisch neu.
# Strg-C beendet beide. Ports via BOT_PORT/UI_PORT überschreibbar.
set -euo pipefail
cd "$(dirname "$0")"

BOT_PORT="${BOT_PORT:-8080}"
UI_PORT="${UI_PORT:-8090}"

if ! command -v air >/dev/null 2>&1; then
  echo "❌ 'air' nicht gefunden."
  echo "   Installieren: go install github.com/air-verse/air@latest"
  echo "   und sicherstellen, dass \"\$(go env GOPATH)/bin\" im PATH ist."
  exit 1
fi

# kill 0 beendet die ganze Prozessgruppe (beide air-Instanzen + sed-Pipes).
trap 'kill 0' EXIT INT TERM

echo
echo "🤖 whatsapp-bot → http://localhost:${BOT_PORT}   (Air Hot-Reload)"
echo "🖥  admin-ui     → http://localhost:${UI_PORT}   (Air Hot-Reload, Bot-Test: /bot-test)"
echo "   Strg-C beendet beide."
echo

( cd whatsapp-bot   && PORT="${BOT_PORT}" air 2>&1 | sed -u 's/^/[bot] /' ) &
( cd zumba-admin-ui && PORT="${UI_PORT}" BOT_URL="http://localhost:${BOT_PORT}" air 2>&1 | sed -u 's/^/[ui ] /' ) &

wait
