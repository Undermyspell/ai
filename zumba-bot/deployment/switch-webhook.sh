#!/usr/bin/env bash
# Schaltet das Ziel des Evolution-API-Webhooks (Instanz "whatsapp") um.
#
#   ./switch-webhook.sh bot     # Cutover: Events -> whatsapp-bot
#   ./switch-webhook.sh n8n     # Rollback: Events -> n8n-Workflow
#   ./switch-webhook.sh status  # nur aktuellen Stand zeigen
#
# Env-Overrides: CONTEXT (default rpi5), NAMESPACE (default zumba-staging),
#                INSTANCE (default whatsapp).
set -euo pipefail

CONTEXT="${CONTEXT:-rpi5}"
NAMESPACE="${NAMESPACE:-zumba-staging}"
INSTANCE="${INSTANCE:-whatsapp}"

BOT_URL="http://zumba-whatsapp-bot:8080/webhook/whatsapp"
N8N_URL="http://zumba-n8n:5678/webhook/34eb3798-c645-45e5-9a0d-4054c806bdcf"

target="${1:-status}"

k() { kubectl --context "$CONTEXT" -n "$NAMESPACE" "$@"; }

EVO=$(k get pods -l app.kubernetes.io/component=evolution-api -o jsonpath='{.items[0].metadata.name}')
[ -n "$EVO" ] || { echo "❌ evolution-api Pod nicht gefunden"; exit 1; }

APIKEY=$(k get secret evolution-api-secrets -o jsonpath='{.data.AUTHENTICATION_API_KEY}' | base64 -d)

show() {
  echo "Aktueller Webhook (Instanz $INSTANCE):"
  k exec "$EVO" -c evolution-api -- wget -qO- --header="apikey: $APIKEY" \
    "http://localhost:8080/webhook/find/$INSTANCE" 2>/dev/null \
    | grep -oE '"url":"[^"]*"|"enabled":[a-z]*' | sed 's/^/  /'
}

case "$target" in
  bot) URL="$BOT_URL" ;;
  n8n) URL="$N8N_URL" ;;
  status) show; exit 0 ;;
  *) echo "Usage: $0 {bot|n8n|status}"; exit 2 ;;
esac

echo "→ Setze Webhook auf: $URL"
k exec "$EVO" -c evolution-api -- wget -qO- \
  --header="Content-Type: application/json" --header="apikey: $APIKEY" \
  --post-data="{\"webhook\":{\"enabled\":true,\"url\":\"$URL\",\"webhookByEvents\":false,\"webhookBase64\":false,\"events\":[\"MESSAGES_UPSERT\"]}}" \
  "http://localhost:8080/webhook/set/$INSTANCE" >/dev/null 2>&1

echo "✅ Gesetzt."
show

if [ "$target" = "n8n" ]; then
  echo
  echo "ℹ️  Rollback: stelle sicher, dass der n8n-Workflow aktiv ist:"
  echo "   kubectl --context $CONTEXT -n $NAMESPACE exec <n8n-pod> -c n8n -- n8n update:workflow --id=HG0zPlWsmPI3Mt7z --active=true"
fi
