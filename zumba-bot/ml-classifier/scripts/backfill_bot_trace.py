"""Migriert bot_trace-Nachrichten nach ml_messages, damit sie im Admin-UI
(/ml-shadow) bewertet werden koennen.

Hintergrund: bot_trace ist das Trace-Protokoll des Bots von vor dem Shadow-Modus
und enthaelt dieselbe Population wie ml_messages -- nur ohne Verify-Pfad im UI.
Nach der Migration tauchen die Eintraege auf der bestehenden Shadow-Seite auf und
koennen dort bewertet werden; ihr Label faellt danach unter "von mir verifiziert"
und darf ins Training (siehe export_real.py).

Auswahl:
- nur Zeilen mit Nachrichtentext (leere sind Statistik-Aufrufe/Fehlerpfade)
- Dedup ueber normalisierten Text (gleiche Normalisierung wie export_real.py)
- uebersprungen wird, was schon in ml_messages steht (Idempotenz) oder im
  ML-Test bereits von Hand bewertet wurde

gemini_label = bot_trace.classification. model_label/model_confidence werden per
classifier-service nachgezogen, falls CLASSIFIER_URL erreichbar ist -- dann zeigt
die Shadow-Seite beim Bewerten direkt den Vergleich. Sonst bleiben sie NULL, was
das Schema und die UI ausdruecklich zulassen.

Dry-Run per Default. Schreiben nur mit --apply.

Env: DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME (Defaults wie export_real.py),
     CLASSIFIER_URL (optional, z.B. http://localhost:8080)
"""

import json
import os
import sys
import urllib.error
import urllib.request

import psycopg2

from common import DATA
from export_real import normalize

LABELS = ("true", "false", "invalid")


def db_connect():
    return psycopg2.connect(
        host=os.getenv("DB_HOST", "192.168.178.46"),
        port=os.getenv("DB_PORT", "5433"),
        user=os.getenv("DB_USER", "n8n"),
        password=os.getenv("DB_PASSWORD", "n8n_password"),
        dbname=os.getenv("DB_NAME", "zumba"),
    )


def classify(url: str, message: str) -> tuple[str | None, float | None]:
    """Fragt den classifier-service. Fehler sind nicht fatal -> (None, None)."""
    req = urllib.request.Request(
        url.rstrip("/") + "/classify",
        data=json.dumps({"text": message}).encode(),
        headers={"Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            body = json.load(resp)
    except (urllib.error.URLError, OSError, ValueError) as e:
        print(f"  classify fehlgeschlagen: {e}", file=sys.stderr)
        return None, None
    label = body.get("label")
    conf = body.get("confidence")
    return (label if label in LABELS else None,
            float(conf) if isinstance(conf, (int, float)) else None)


def collect(cur) -> list[dict]:
    """Kandidaten aus bot_trace, dedupliziert und gegen Bestand abgeglichen."""
    cur.execute(
        """SELECT message, COALESCE(classification, ''), COALESCE(user_id, ''),
                  COALESCE(user_name, ''), created_at
           FROM bot_trace
           WHERE message IS NOT NULL AND btrim(message) <> ''
           ORDER BY created_at"""
    )
    candidates: dict[str, dict] = {}
    dropped_dup = 0
    for msg, label, uid, uname, created in cur.fetchall():
        key = normalize(msg)
        if not key:
            continue
        if key in candidates:
            dropped_dup += 1
            continue
        candidates[key] = {
            "message": msg.strip(),
            "gemini_label": label if label in LABELS else None,
            "user_id": uid or None,
            "user_name": uname or None,
            "created_at": created,
        }

    # Schon migriert? (Idempotenz -- Skript darf mehrfach laufen.)
    cur.execute("SELECT message FROM ml_messages")
    existing = {normalize(m) for (m,) in cur.fetchall()}
    # Schon im ML-Test von Hand bewertet -> dort zaehlt das Urteil bereits.
    cur.execute("SELECT message FROM ml_test_messages WHERE expected_label IS NOT NULL")
    judged = {normalize(m) for (m,) in cur.fetchall()}

    out, skipped_existing, skipped_judged = [], 0, 0
    for key, rec in candidates.items():
        if key in existing:
            skipped_existing += 1
            continue
        if key in judged:
            skipped_judged += 1
            continue
        out.append(rec)

    print(f"bot_trace mit Text        : {len(candidates) + dropped_dup}", file=sys.stderr)
    print(f"  exakte Duplikate        : -{dropped_dup}", file=sys.stderr)
    print(f"  schon in ml_messages    : -{skipped_existing}", file=sys.stderr)
    print(f"  schon im ML-Test bewertet: -{skipped_judged}", file=sys.stderr)
    print(f"  ==> zu migrieren        : {len(out)}", file=sys.stderr)
    return out


def main() -> None:
    apply = "--apply" in sys.argv
    classifier_url = os.getenv("CLASSIFIER_URL", "").strip()

    conn = db_connect()
    with conn, conn.cursor() as cur:
        rows = collect(cur)
        if not rows:
            print("Nichts zu tun.", file=sys.stderr)
            return

        if classifier_url:
            for rec in rows:
                rec["model_label"], rec["model_confidence"] = classify(
                    classifier_url, rec["message"])
        else:
            print("CLASSIFIER_URL nicht gesetzt -> model_label bleibt NULL.",
                  file=sys.stderr)
            for rec in rows:
                rec["model_label"] = rec["model_confidence"] = None

        print(f"\n{'gemini':8s} {'modell':8s} {'konf':>5s}  nachricht", file=sys.stderr)
        print("-" * 72, file=sys.stderr)
        for rec in rows:
            conf = f"{rec['model_confidence']:.2f}" if rec["model_confidence"] else "  - "
            print(f"{rec['gemini_label'] or '-':8s} {rec['model_label'] or '-':8s} "
                  f"{conf:>5s}  {rec['message'][:44]}", file=sys.stderr)

        if not apply:
            print(f"\nDRY-RUN -- nichts geschrieben. Mit --apply ausfuehren.",
                  file=sys.stderr)
            return

        # Einzeln mit RETURNING, damit die IDs exakt protokolliert werden --
        # ml_messages fuellt sich durch den laufenden Shadow-Modus parallel
        # weiter, ein pauschales DELETE waere zum Zurueckrollen nicht sicher.
        inserted: list[int] = []
        for r in rows:
            agree = (r["model_label"] == r["gemini_label"]
                     if r["model_label"] and r["gemini_label"] else None)
            cur.execute(
                """INSERT INTO ml_messages
                     (created_at, user_id, user_name, message, gemini_label,
                      model_label, model_confidence, agree, verified)
                   VALUES (%(created_at)s, %(user_id)s, %(user_name)s, %(message)s,
                           %(gemini_label)s, %(model_label)s, %(model_confidence)s,
                           %(agree)s, false)
                   RETURNING id""",
                {**r, "agree": agree},
            )
            inserted.append(cur.fetchone()[0])

        undo = DATA / "backfill_bot_trace_ids.json"
        undo.write_text(json.dumps(inserted), encoding="utf-8")
        print(f"\n{len(inserted)} Zeilen nach ml_messages geschrieben. "
              f"Jetzt auf /ml-shadow bewerten.", file=sys.stderr)
        print(f"Rollback-IDs: {undo}", file=sys.stderr)
        print(f"  DELETE FROM ml_messages WHERE id IN "
              f"({','.join(map(str, inserted))});", file=sys.stderr)


if __name__ == "__main__":
    main()
