#!/usr/bin/env bash
# Startet die komplette lokale Dev-Umgebung — alles außer Postgres +
# Evolution API (laufen im Cluster, siehe whatsapp-bot/.env) und dem
# Renderer (Docker, `make up`):
#   whatsapp-bot   :8080  Air Hot-Reload (nutzt whatsapp-bot/.env)
#   zumba-admin-ui :8090  Air Hot-Reload, BOT_URL auf den Bot
#   wrapped        :8082  Air Hot-Reload (Route /2026)
#   classifier     :8085  go run (Modell eingebettet, ändert sich selten)
# Strg-C beendet alle. Ports via BOT_PORT/UI_PORT/WRAPPED_PORT/CLS_PORT
# überschreibbar.
set -euo pipefail
cd "$(dirname "$0")"

BOT_PORT="${BOT_PORT:-8080}"
UI_PORT="${UI_PORT:-8090}"
WRAPPED_PORT="${WRAPPED_PORT:-8082}"
CLS_PORT="${CLS_PORT:-8085}"

if ! command -v air >/dev/null 2>&1; then
  echo "❌ 'air' nicht gefunden."
  echo "   Installieren: go install github.com/air-verse/air@latest"
  echo "   und sicherstellen, dass \"\$(go env GOPATH)/bin\" im PATH ist."
  exit 1
fi

# kill 0 beendet die ganze Prozessgruppe (alle Server + sed-Pipes).
trap 'kill 0' EXIT INT TERM

echo
echo "🤖 whatsapp-bot → http://localhost:${BOT_PORT}   (Air Hot-Reload)"
echo "🖥  admin-ui     → http://localhost:${UI_PORT}   (Air Hot-Reload, Bot-Test: /bot-test)"
echo "🎁 wrapped      → http://localhost:${WRAPPED_PORT}/2026   (Air Hot-Reload)"
echo "🧠 classifier   → http://localhost:${CLS_PORT}   (go run, ML-Shadow)"
echo "   Renderer (Bild-Karte) kommt aus Docker: make up  → localhost:8091"
echo "   Strg-C beendet alle."
echo

# Bild-Karte: Renderer aus docker-compose (Service "renderer", Port 8091).
# godotenv überschreibt gesetzte Umgebungsvariablen nicht — Werte aus
# whatsapp-bot/.env gewinnen also NICHT gegen diese Defaults.
( cd whatsapp-bot && PORT="${BOT_PORT}" \
    RENDERER_URL="${RENDERER_URL:-http://localhost:8091}" \
    CLASSIFIER_URL="${CLASSIFIER_URL:-http://localhost:${CLS_PORT}}" \
    air 2>&1 | sed -u 's/^/[bot] /' ) &
( cd zumba-admin-ui && PORT="${UI_PORT}" BOT_URL="http://localhost:${BOT_PORT}" air 2>&1 | sed -u 's/^/[ui ] /' ) &
( cd wrapped && PORT="${WRAPPED_PORT}" air 2>&1 | sed -u 's/^/[wrp] /' ) &
( cd classifier-service && PORT="${CLS_PORT}" go run ./cmd/server 2>&1 | sed -u 's/^/[cls] /' ) &

wait
