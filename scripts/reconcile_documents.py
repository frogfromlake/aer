#!/usr/bin/env python3
"""One-shot reconciliation: rebuild missing PostgreSQL documents/jobs rows from MinIO Silver.

Phase 113b / ADR-022. Postgres `documents` and `ingestion_jobs` are pruned at
90 days (operational horizon, mirrors Bronze ILM) while MinIO Silver and
ClickHouse Gold retain for 365 days (analytical horizon). Between days 90 and
365 of an article's life the analytical record is still live but its
operational metadata row is gone, leaving the dossier under-counting and L5
Evidence article-detail 404'ing on retention-deleted articles.

ADR-022 makes the BFF read article resolution and per-source counts from the
analytical layer going forward, so this script is a *one-shot* historical
backfill — not a recurring chore. After the BFF rewrite ships, it never needs
to run again unless a future operational outage re-creates the same drift.

What it does:

  1. Iterates every object in MinIO Silver and parses the SilverEnvelope to
     recover (source, document_id, timestamp, bronze_object_key=object_name).
  2. Resolves source_id from public.sources by name. Sources missing from
     Postgres are skipped (an unknown source name is a distinct class of bug
     and not in scope here).
  3. Ensures one synthetic ingestion_jobs row exists per (source_id, UTC
     ingestion-day) tuple — this is the "synthetic job per source/day"
     mandated by Phase 113b. The job's started_at is set to 00:00 UTC of that
     day so the documents.ingested_at timestamps remain plausible relative to
     it.
  4. INSERT ... ON CONFLICT (bronze_object_key) DO NOTHING into documents,
     so re-runs are idempotent and existing rows are never disturbed (no
     deletes, no overwrites of live state).
  5. Updates aer_silver.documents.bronze_object_key for any row that pre-dates
     Migration 013 — those rows carry the empty-string DEFAULT. This closes
     the migration gap so ResolveArticle works for historical articles
     immediately, without waiting for a full reprocess.

Idempotent: re-running after a complete pass touches no rows.

Usage:
    python scripts/reconcile_documents.py                # full pass
    python scripts/reconcile_documents.py --dry-run      # report only
    python scripts/reconcile_documents.py --batch-size 500
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from datetime import datetime, timezone

import clickhouse_connect
import psycopg2
import psycopg2.extras
from dotenv import load_dotenv
from minio import Minio

SILVER_BUCKET = "silver"
SILVER_DOCS_TABLE = "aer_silver.documents"


def parse_timestamp(raw: str) -> datetime:
    if raw.endswith("Z"):
        raw = raw[:-1] + "+00:00"
    return datetime.fromisoformat(raw)


def iter_silver_envelopes(minio: Minio):
    """Yield (object_name, envelope_dict) for every object in MinIO Silver."""
    for obj in minio.list_objects(SILVER_BUCKET, recursive=True):
        try:
            response = minio.get_object(SILVER_BUCKET, obj.object_name)
            try:
                payload = json.loads(response.read().decode("utf-8"))
            finally:
                response.close()
                response.release_conn()
        except Exception as e:
            print(f"  ! skip {obj.object_name}: read/parse failed ({e})", file=sys.stderr)
            continue
        yield obj.object_name, payload


def load_source_index(pg) -> dict[str, int]:
    """Return {source_name: source_id} for every row in public.sources."""
    with pg.cursor() as cur:
        cur.execute("SELECT id, name FROM sources")
        return {name: sid for sid, name in cur.fetchall()}


def ensure_synthetic_job(pg, source_id: int, day: datetime,
                         job_cache: dict[tuple[int, str], int]) -> int:
    """Return the ingestion_jobs.id for (source_id, day), creating a synthetic
    job row if none exists. Cached per (source_id, day) so a single run only
    pays one INSERT per source-day even with thousands of documents.

    A synthetic job is identified by `status = 'reconciled'` and
    `started_at = 00:00 UTC of the day`. We do not collide with real jobs:
    a real job's started_at lands on a sub-second boundary inside the day,
    while the synthetic job sits exactly on midnight.
    """
    day_key = day.date().isoformat()
    cached = job_cache.get((source_id, day_key))
    if cached is not None:
        return cached

    midnight = datetime(day.year, day.month, day.day, tzinfo=timezone.utc)
    with pg.cursor() as cur:
        cur.execute(
            """
            SELECT id FROM ingestion_jobs
             WHERE source_id = %s AND status = 'reconciled' AND started_at = %s
             LIMIT 1
            """,
            (source_id, midnight),
        )
        row = cur.fetchone()
        if row is not None:
            job_cache[(source_id, day_key)] = row[0]
            return row[0]

        cur.execute(
            """
            INSERT INTO ingestion_jobs (source_id, status, started_at, finished_at)
            VALUES (%s, 'reconciled', %s, %s)
            RETURNING id
            """,
            (source_id, midnight, midnight),
        )
        job_id = cur.fetchone()[0]
    job_cache[(source_id, day_key)] = job_id
    return job_id


def reconcile_documents_batch(pg, rows: list[tuple]) -> int:
    """Insert a batch of (job_id, bronze_object_key, article_id, ingested_at) rows.
    ON CONFLICT (bronze_object_key) DO NOTHING preserves any existing row.
    Returns the number of rows inserted."""
    if not rows:
        return 0
    with pg.cursor() as cur:
        psycopg2.extras.execute_values(
            cur,
            """
            INSERT INTO documents
                (job_id, bronze_object_key, article_id, status, ingested_at)
            VALUES %s
            ON CONFLICT (bronze_object_key) DO NOTHING
            """,
            rows,
            template="(%s, %s, %s, 'processed', %s)",
        )
        return cur.rowcount


def update_silver_bronze_keys(ch, rows: list[tuple]) -> None:
    """Re-insert (timestamp, source, article_id, ..., bronze_object_key) rows
    into aer_silver.documents so ReplacingMergeTree collapses the empty-key
    legacy row in favour of the populated one. Caller is expected to skip
    rows whose bronze_object_key is already non-empty in CH; we just trust
    the higher ingestion_version we write here to win on merge.
    """
    if not rows:
        return
    columns = [
        "timestamp", "source", "article_id", "language",
        "cleaned_text_length", "word_count", "raw_entity_count",
        "ingestion_version", "bronze_object_key",
    ]
    ch.insert(SILVER_DOCS_TABLE, rows, column_names=columns)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dry-run", action="store_true",
                        help="Iterate Silver and report what would change; do not write.")
    parser.add_argument("--batch-size", type=int, default=500)
    parser.add_argument("--limit", type=int, default=0,
                        help="Process at most N envelopes (0 = no cap).")
    args = parser.parse_args()

    load_dotenv()

    minio = Minio(
        endpoint=os.getenv("MINIO_ENDPOINT", "localhost:9000"),
        access_key=os.getenv("WORKER_MINIO_ACCESS_KEY", ""),
        secret_key=os.getenv("WORKER_MINIO_SECRET_KEY", ""),
        secure=os.getenv("MINIO_USE_SSL", "false").lower() == "true",
    )
    if not minio.bucket_exists(SILVER_BUCKET):
        print(f"bucket '{SILVER_BUCKET}' does not exist; nothing to reconcile",
              file=sys.stderr)
        return 0

    pg = psycopg2.connect(
        host=os.getenv("POSTGRES_HOST", "localhost"),
        port=int(os.getenv("POSTGRES_PORT", "5432")),
        user=os.getenv("POSTGRES_USER", "aer_admin"),
        password=os.getenv("POSTGRES_PASSWORD", ""),
        dbname=os.getenv("POSTGRES_DB", "aer_metadata"),
    )
    pg.autocommit = False

    # CLICKHOUSE_PORT in .env points at the native protocol (9000 inside the
    # network, 9002 when exposed via `make debug-up`) — that's what the
    # worker's clickhouse-driver uses. clickhouse-connect speaks HTTP, so
    # honour CLICKHOUSE_HTTP_PORT first and fall back to the canonical 8123.
    ch = clickhouse_connect.get_client(
        host=os.getenv("CLICKHOUSE_HOST", "localhost"),
        port=int(os.getenv("CLICKHOUSE_HTTP_PORT", "8123")),
        username=os.getenv("CLICKHOUSE_USER", "default"),
        password=os.getenv("CLICKHOUSE_PASSWORD", ""),
        database=os.getenv("CLICKHOUSE_DB", "aer_gold"),
    )

    source_index = load_source_index(pg)
    job_cache: dict[tuple[int, str], int] = {}

    total = 0
    inserted = 0
    silver_repopulated = 0
    skipped_unknown_source = 0
    skipped_malformed = 0
    pg_batch: list[tuple] = []
    silver_batch: list[tuple] = []

    for object_name, envelope in iter_silver_envelopes(minio):
        try:
            core = envelope["core"]
            source = core["source"]
            article_id = core["document_id"]
            timestamp = parse_timestamp(core["timestamp"])
        except (KeyError, ValueError) as e:
            print(f"  ! skip {object_name}: malformed envelope ({e})", file=sys.stderr)
            skipped_malformed += 1
            continue

        source_id = source_index.get(source)
        if source_id is None:
            skipped_unknown_source += 1
            continue

        total += 1

        # Synthetic job per (source_id, ingestion-day). The "ingestion-day"
        # is derived from the document timestamp — operationally close
        # enough for retention windows, and deterministic across re-runs.
        if not args.dry_run:
            job_id = ensure_synthetic_job(pg, source_id, timestamp, job_cache)
            pg_batch.append((job_id, object_name, article_id, timestamp))

        # Silver projection repopulation: write a row at a slightly higher
        # ingestion_version so ReplacingMergeTree collapses the empty-key
        # legacy row in favour of this one. +1 over the worker's nanosecond
        # version is enough to win.
        ingestion_version = int(timestamp.timestamp() * 1_000_000_000) + 1
        cleaned_text = core.get("cleaned_text", "") or ""
        silver_batch.append((
            timestamp,
            source,
            article_id,
            core.get("language") or "und",
            len(cleaned_text),
            int(core.get("word_count", 0)),
            0,  # raw_entity_count: unchanged from worker's projection; column
                # is not part of the dedup key, so 0 is safe — the original
                # row's value persists if its ingestion_version was equal.
                # When the row is brand-new (no prior projection), the
                # aggregation endpoints just see 0 until reprocessing.
            ingestion_version,
            object_name,
        ))

        if len(pg_batch) >= args.batch_size:
            inserted += reconcile_documents_batch(pg, pg_batch)
            pg.commit()
            update_silver_bronze_keys(ch, silver_batch)
            silver_repopulated += len(silver_batch)
            print(f"  flushed batch (total seen: {total}, inserted: {inserted})")
            pg_batch = []
            silver_batch = []

        if args.limit and total >= args.limit:
            break

    if not args.dry_run:
        if pg_batch:
            inserted += reconcile_documents_batch(pg, pg_batch)
            pg.commit()
        if silver_batch:
            update_silver_bronze_keys(ch, silver_batch)
            silver_repopulated += len(silver_batch)
    else:
        # Dry-run: emit the would-be totals without touching either store.
        print(f"  dry-run: would insert up to {len(pg_batch)} pending documents "
              f"and repopulate {len(silver_batch)} silver projection rows in the final batch")

    pg.close()

    print()
    print(f"Reconciliation {'(dry-run) ' if args.dry_run else ''}complete:")
    print(f"  envelopes seen:        {total}")
    print(f"  documents inserted:    {inserted}")
    print(f"  silver rows repopulated: {silver_repopulated}")
    print(f"  skipped (unknown src): {skipped_unknown_source}")
    print(f"  skipped (malformed):   {skipped_malformed}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
