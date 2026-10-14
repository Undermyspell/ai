#!/usr/bin/env bash
# Importiert generierte Synthetics in ml_synthetic_messages (Staging-DB).
#
#   scripts/import_synthetic.sh data/generated/<datei>.jsonl [...]     # nur Vorschau
#   APPLY=1 scripts/import_synthetic.sh data/generated/<datei>.jsonl   # schreibt
#
# Holt die Referenztexte (echte Nachrichten + vorhandene Synthetics) lesend
# über den Postgres-Pod, filtert per import_synthetic.py und spielt das SQL
# nur mit APPLY=1 ein – in einer Transaktion (BEGIN/COMMIT im SQL).
set -euo pipefail
cd "$(dirname "$0")/.."

PI="${PI:-pi@192.168.178.46}"
NS="${NS:-zumba-staging}"
# SQL kommt über stdin – kein Quoting durch ssh und kubectl hindurch.
psql_pod() {
  ssh "$PI" "sudo k3s kubectl exec -i -n $NS zumba-postgres-0 -- psql -q -X -v ON_ERROR_STOP=1 -U n8n -d zumba $*"
}

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

cat > "$tmp/ref.sql" <<'SQL'
SET default_transaction_read_only = on;
SELECT json_build_object('kind', 'real', 'text', message) FROM ml_messages
UNION ALL SELECT json_build_object('kind', 'real', 'text', message) FROM ml_test_messages
UNION ALL SELECT json_build_object('kind', 'real', 'text', message) FROM (
  SELECT DISTINCT message FROM stammtisch_abwesenheit
  WHERE message IS NOT NULL AND btrim(message) <> '') a
UNION ALL SELECT json_build_object('kind', 'synthetic', 'text', message) FROM ml_synthetic_messages;
SQL
psql_pod -At < "$tmp/ref.sql" > "$tmp/ref.jsonl"

uv run scripts/import_synthetic.py --ref "$tmp/ref.jsonl" "$@" > "$tmp/insert.sql"

n=$(grep -c '^INSERT' "$tmp/insert.sql" || true)
if [[ "${APPLY:-}" != "1" ]]; then
  echo "Vorschau: $n Inserts. Mit APPLY=1 einspielen." >&2
  exit 0
fi
psql_pod < "$tmp/insert.sql"
echo "$n Inserts eingespielt." >&2
