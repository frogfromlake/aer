#!/usr/bin/env python3
"""One-shot backfill: populate documents.article_id for rows processed before Phase 101.

The analysis worker derives article_id deterministically as
    SHA-256(source_name || bronze_object_key)
with no separator between operands (see
services/analysis-worker/internal/models/__init__.py generate_document_id).

Phase 101 added the column nullable so existing rows stay valid; this
script computes the hash for every already-processed row whose article_id
is NULL, in batches, and leaves in-flight (pending/failed/quarantined)
rows untouched — the worker still writes the canonical value when it
commits those.

Idempotent: re-running after a complete pass is a no-op (the WHERE clause
filters on `article_id IS NULL`).

Usage:
    python scripts/backfill_article_id.py            # uses .env / env vars
    python scripts/backfill_article_id.py --dry-run  # prints counts only
"""

from __future__ import annotations

import argparse
import hashlib
import os
import sys
from urllib.parse import urlparse

import psycopg2
from dotenv import load_dotenv


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dry-run", action="store_true", help="Count rows; do not update.")
    parser.add_argument("--batch-size", type=int, default=500)
    parser.add_argument("--db-url", default=None, help="Postgres DSN. Defaults to $DB_URL.")
    args = parser.parse_args()

    load_dotenv()
    dsn = args.db_url or os.environ.get("DB_URL")
    if not dsn:
        print("error: DB_URL not set (and --db-url not supplied)", file=sys.stderr)
        return 2

    parsed = urlparse(dsn)
    if parsed.scheme not in {"postgres", "postgresql"}:
        print(f"error: DB_URL has unexpected scheme: {parsed.scheme}", file=sys.stderr)
        return 2

    conn = psycopg2.connect(dsn)
    conn.autocommit = False
    try:
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT count(*)
                  FROM documents d
                  JOIN ingestion_jobs j ON j.id = d.job_id
                 WHERE d.status = 'processed'
                   AND d.article_id IS NULL
                """
            )
            todo = cur.fetchone()[0]
        print(f"rows needing backfill: {todo}")
        if todo == 0 or args.dry_run:
            return 0

        updated = 0
        while True:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    SELECT d.id, s.name, d.bronze_object_key
                      FROM documents d
                      JOIN ingestion_jobs j ON j.id = d.job_id
                      JOIN sources s ON s.id = j.source_id
                     WHERE d.status = 'processed'
                       AND d.article_id IS NULL
                     LIMIT %s
                     FOR UPDATE OF d SKIP LOCKED
                    """,
                    (args.batch_size,),
                )
                batch = cur.fetchall()
                if not batch:
                    break

                payload = [
                    (
                        hashlib.sha256(f"{name}{key}".encode("utf-8")).hexdigest(),
                        doc_id,
                    )
                    for doc_id, name, key in batch
                ]
                cur.executemany(
                    "UPDATE documents SET article_id = %s WHERE id = %s",
                    payload,
                )
            conn.commit()
            updated += len(batch)
            print(f"  backfilled {updated}/{todo}")
        print(f"done: {updated} rows updated")
    finally:
        conn.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
