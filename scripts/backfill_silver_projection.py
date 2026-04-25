#!/usr/bin/env python3
"""One-shot backfill: populate aer_silver.documents from existing MinIO Silver envelopes.

Phase 103b adds the projection table `aer_silver.documents` and wires the
analysis worker to write one row per document at processing time. Existing
Silver envelopes (uploaded before this phase shipped) have no corresponding
projection row, so the aggregation endpoints would return empty results
for historic windows until reprocessing — this script closes that gap by
reading the canonical Silver records from MinIO and writing the derived
projection rows directly.

Idempotent via `ingestion_version` (microseconds of the document timestamp):
re-runs collapse to the same row under ReplacingMergeTree semantics, so
this is safe to interrupt and resume.

Inputs come from the same env vars the analysis worker uses
(MINIO_ENDPOINT, WORKER_MINIO_ACCESS_KEY/SECRET_KEY, CLICKHOUSE_HOST/PORT/
USER/PASSWORD).

Usage:
    python scripts/backfill_silver_projection.py                # full pass
    python scripts/backfill_silver_projection.py --dry-run      # list only
    python scripts/backfill_silver_projection.py --batch-size 500
"""

from __future__ import annotations

import argparse
import io
import json
import os
import re
import sys
from datetime import datetime, timezone
from pathlib import Path

# Allow running from repo root without PYTHONPATH gymnastics.
WORKER_INTERNAL = Path(__file__).resolve().parent.parent / "services" / "analysis-worker"
sys.path.insert(0, str(WORKER_INTERNAL))

import clickhouse_connect  # noqa: E402
from dotenv import load_dotenv  # noqa: E402
from minio import Minio  # noqa: E402

SILVER_BUCKET = "silver"
SILVER_DOCS_TABLE = "aer_silver.documents"

# Same regex as services/analysis-worker/internal/silver_projection.py — the
# pre-NER capitalised-token heuristic. Kept inline so this script has zero
# runtime dependency on the worker package layout (which lives behind a
# `internal.` import root that requires PYTHONPATH gymnastics).
_CAPITALIZED_TOKEN = re.compile(r"\b[A-ZÄÖÜ][A-Za-zÄÖÜäöüß]+\b")


def raw_entity_count(cleaned_text: str) -> int:
    if not cleaned_text:
        return 0
    return len(_CAPITALIZED_TOKEN.findall(cleaned_text))


def parse_timestamp(raw: str) -> datetime:
    # Silver envelopes serialise timestamps with the trailing Z; Python 3.11+
    # accepts +00:00 but not Z directly via fromisoformat on some builds.
    if raw.endswith("Z"):
        raw = raw[:-1] + "+00:00"
    return datetime.fromisoformat(raw)


def iter_silver_envelopes(minio: Minio):
    """Stream all Silver objects, yielding (object_name, envelope_dict)."""
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


def envelope_to_row(envelope: dict):
    core = envelope.get("core") or {}
    cleaned_text = core.get("cleaned_text", "") or ""
    timestamp = parse_timestamp(core["timestamp"])
    # ingestion_version mirrors the worker (nanoseconds of the document
    # timestamp). A backfill run uses the document's own timestamp rather
    # than wall-clock so that a future live event for the same document
    # (same timestamp) collapses cleanly under ReplacingMergeTree.
    ingestion_version = int(timestamp.timestamp() * 1_000_000_000)
    return [
        timestamp,
        core["source"],
        core["document_id"],
        core.get("language") or "und",
        len(cleaned_text),
        int(core.get("word_count", 0)),
        raw_entity_count(cleaned_text),
        ingestion_version,
    ]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dry-run", action="store_true",
                        help="Iterate Silver and print counts; do not insert.")
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
        print(f"bucket '{SILVER_BUCKET}' does not exist; nothing to backfill",
              file=sys.stderr)
        return 0

    ch = None
    if not args.dry_run:
        ch = clickhouse_connect.get_client(
            host=os.getenv("CLICKHOUSE_HOST", "localhost"),
            port=int(os.getenv("CLICKHOUSE_PORT", "8123")),
            username=os.getenv("CLICKHOUSE_USER", "default"),
            password=os.getenv("CLICKHOUSE_PASSWORD", ""),
            # The projection table lives in aer_silver, not aer_gold. Use
            # whichever default the env declares; queries are fully
            # qualified anyway.
            database=os.getenv("CLICKHOUSE_DB", "aer_gold"),
        )

    columns = [
        "timestamp", "source", "article_id", "language",
        "cleaned_text_length", "word_count", "raw_entity_count", "ingestion_version",
    ]

    total = 0
    skipped = 0
    batch: list[list] = []

    for object_name, envelope in iter_silver_envelopes(minio):
        try:
            row = envelope_to_row(envelope)
        except (KeyError, ValueError) as e:
            print(f"  ! skip {object_name}: malformed envelope ({e})", file=sys.stderr)
            skipped += 1
            continue

        batch.append(row)
        total += 1

        if len(batch) >= args.batch_size:
            if ch is not None:
                ch.insert(SILVER_DOCS_TABLE, batch, column_names=columns)
            print(f"  flushed batch (total so far: {total})")
            batch = []

        if args.limit and total >= args.limit:
            break

    if batch and ch is not None:
        ch.insert(SILVER_DOCS_TABLE, batch, column_names=columns)
        print(f"  flushed final batch (total: {total})")
    elif batch:
        print(f"  dry-run: would flush final batch of {len(batch)}")

    print(f"\nProjection backfill {'(dry-run) ' if args.dry_run else ''}complete: "
          f"{total} envelopes processed, {skipped} skipped.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
