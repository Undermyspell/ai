"""Exportiert echte Nachrichten aus der zumba-DB nach data/real.jsonl.

Quellen:
- stammtisch_abwesenheit.message  -> Label "false" (jede Zeile ist eine Absage)
- bot_trace (classification in true/false/invalid) -> Gemini-Label

Dedup über normalisierten Text; bot_trace-Label gewinnt bei Konflikt nie gegen
stammtisch_abwesenheit (eine persistierte Absage ist verlässlicher als ein Trace).

Env: DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME (Defaults wie wrapped/).
"""

import json
import os
import re
import sys
import unicodedata
from pathlib import Path

import psycopg2

OUT = Path(__file__).resolve().parent.parent / "data" / "real.jsonl"


def normalize(text: str) -> str:
    """Dedup-Schlüssel: casefold, Emojis/Satzzeichen raus, Whitespace kollabiert."""
    t = unicodedata.normalize("NFKC", text).casefold()
    t = "".join(c for c in t if c.isalnum() or c.isspace())
    return re.sub(r"\s+", " ", t).strip()


def main() -> None:
    conn = psycopg2.connect(
        host=os.getenv("DB_HOST", "192.168.178.46"),
        port=os.getenv("DB_PORT", "5433"),
        user=os.getenv("DB_USER", "n8n"),
        password=os.getenv("DB_PASSWORD", "n8n_password"),
        dbname=os.getenv("DB_NAME", "zumba"),
    )
    rows: dict[str, dict] = {}  # norm-key -> record

    with conn, conn.cursor() as cur:
        # Absagen zuerst: verlässlichstes Label, gewinnt bei Dedup-Konflikt.
        cur.execute(
            """SELECT DISTINCT message FROM stammtisch_abwesenheit
               WHERE message IS NOT NULL AND btrim(message) <> ''"""
        )
        for (msg,) in cur.fetchall():
            key = normalize(msg)
            if key:
                rows[key] = {"text": msg.strip(), "label": "false", "source": "real",
                             "origin": "stammtisch_abwesenheit", "verified": False}

        cur.execute(
            """SELECT message, classification FROM bot_trace
               WHERE classification IN ('true', 'false', 'invalid')
                 AND message IS NOT NULL AND btrim(message) <> ''"""
        )
        for msg, label in cur.fetchall():
            key = normalize(msg)
            if key and key not in rows:
                rows[key] = {"text": msg.strip(), "label": label, "source": "real",
                             "origin": "bot_trace", "verified": False}

    OUT.parent.mkdir(parents=True, exist_ok=True)
    with OUT.open("w", encoding="utf-8") as f:
        for rec in rows.values():
            f.write(json.dumps(rec, ensure_ascii=False) + "\n")

    counts: dict[str, int] = {}
    for rec in rows.values():
        counts[rec["label"]] = counts.get(rec["label"], 0) + 1
    print(f"{len(rows)} Nachrichten -> {OUT}", file=sys.stderr)
    print(f"Labels: {counts}", file=sys.stderr)


if __name__ == "__main__":
    main()
