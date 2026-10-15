#!/usr/bin/env bash
# Exportiert die drei gelabelten Quellen aus der Staging-DB nach data/v2/
# (gitignored – echte Gruppennachrichten gehören nicht ins Repo):
#
#   synthetics.jsonl  gelabelte, nicht verworfene Synthetics   → Training
#   whatsapp.jsonl    in der aktuellen Runde gelabelte echte    → Test
#   manuell.jsonl     gelabelte Prüffälle von /ml-test          → Prüffälle
#
# Nur lesend (default_transaction_read_only) über den Postgres-Pod.
set -euo pipefail
cd "$(dirname "$0")/.."

PI="${PI:-pi@192.168.178.46}"
NS="${NS:-zumba-staging}"
OUT=data/v2
mkdir -p "$OUT"

q() {
  { echo "SET default_transaction_read_only = on;"; echo "$1"; } |
    ssh "$PI" "sudo k3s kubectl exec -i -n $NS zumba-postgres-0 -- psql -q -At -X -v ON_ERROR_STOP=1 -U n8n -d zumba" |
    grep -v '^SET$'
}

q "SELECT json_build_object('id', id, 'text', message, 'label', label, 'art', art,
     'sprache', sprache, 'generated_label', generated_label, 'batch_id', batch_id)
   FROM ml_synthetic_messages
   WHERE labeled_at IS NOT NULL AND NOT verworfen AND label IS NOT NULL ORDER BY id;" > "$OUT/synthetics.jsonl"

q "SELECT json_build_object('id', id, 'created_at', created_at, 'text', message,
     'label', corrected_label, 'gemini', gemini_label,
     'model_012', model_label, 'model_012_conf', model_confidence)
   FROM ml_messages
   WHERE labeled_at IS NOT NULL AND corrected_label IS NOT NULL ORDER BY id;" > "$OUT/whatsapp.jsonl"

q "SELECT json_build_object('id', id, 'text', message, 'label', expected_label,
     'model_012', model_label)
   FROM ml_test_messages
   WHERE labeled_at IS NOT NULL AND expected_label IS NOT NULL ORDER BY id;" > "$OUT/manuell.jsonl"

q "SELECT 'offen: synthetics ' || (SELECT count(*) FROM ml_synthetic_messages WHERE labeled_at IS NULL)
   || ', whatsapp ' || (SELECT count(*) FROM ml_messages WHERE labeled_at IS NULL)
   || ', manuell ' || (SELECT count(*) FROM ml_test_messages WHERE labeled_at IS NULL);" >&2
for f in synthetics whatsapp manuell; do
  echo "$f: $(wc -l < "$OUT/$f.jsonl") Nachrichten" >&2
done
