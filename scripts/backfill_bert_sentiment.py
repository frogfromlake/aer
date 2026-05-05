#!/usr/bin/env python3
"""One-shot backfill: populate the Phase-119 BERT sentiment metrics for
documents that pre-date Phase 119.

Use case. The Phase 119 extractors `MultilingualBertSentimentExtractor`
(Tier-2 default per ADR-023) and `GermanNewsBertSentimentExtractor`
(Tier-2.5 refinement) write `sentiment_score_bert_multilingual` and
`sentiment_score_bert_de_news` rows at ingestion time. Documents
ingested *before* the Phase-119 worker image shipped have no rows for
these metric names — the BFF `/metrics/available` endpoint will not
expose them until at least one row exists, and the dashboard's
`MetricSwitcher` will not list them. This script closes that gap.

Architecture. Pure MinIO-Silver-to-ClickHouse-Gold:

  1. List Silver envelopes from the `silver` bucket.
  2. For each envelope, parse the `SilverEnvelope` JSON written by
     `internal/silver.py` to recover `core.cleaned_text`,
     `core.language`, `core.timestamp`, `core.source`, and
     `core.document_id` plus the meta-derived
     `discourse_function` (the only sanctioned point where
     SilverMeta influences Gold — see `processor._derive_discourse_function`).
  3. Run both Phase-119 extractors over `core` and write resulting
     `GoldMetric` rows to `aer_gold.metrics`.

Bronze and the live extractor pipeline are not touched. The
PostgreSQL idempotency table (`documents.status`) is not touched —
this script bypasses the live event flow and writes metrics directly
to ClickHouse, so the live worker's idempotency guarantee is
preserved.

Idempotency. `aer_gold.metrics` is `ReplacingMergeTree(ingestion_version)`.
`ingestion_version` is derived per row from `core.timestamp`
(nanoseconds), matching the live extractor convention. Re-running is
safe: existing rows for the same `(timestamp, source, metric_name,
article_id)` collapse on merge to the most-recent
`ingestion_version`. Because the metric names are *new*, there is
nothing to collapse against on the first run.

Graceful degradation. If `transformers`/`torch` are unavailable in
the execution environment, both extractors disable themselves at
construction time (the same path that runs inside the worker
container) and the script writes nothing — exits with a structured
warning rather than a stack trace.

Usage.

    # Inside the analysis-worker container (has venv + models cached):
    docker compose exec analysis-worker python /scripts/backfill_bert_sentiment.py
    # Or via Make:
    make backfill-bert-sentiment

    # Filtered window:
    python backfill_bert_sentiment.py --start 2026-04-01 --end 2026-05-04 \
        --source bundesregierung

    # Inspect without writing:
    python backfill_bert_sentiment.py --dry-run
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from datetime import datetime, timezone
from pathlib import Path

# Two layouts supported (mirrors backfill_entity_links.py):
#   * Repo-root execution from the host: services/analysis-worker/internal/...
#   * In-container execution: /app/internal/... (worker WORKDIR=/app).
for candidate in (
    Path(__file__).resolve().parent.parent / "services" / "analysis-worker",
    Path("/app"),
):
    if (candidate / "internal" / "extractors" / "sentiment_bert_multilingual.py").exists():
        sys.path.insert(0, str(candidate))
        break

import clickhouse_connect  # noqa: E402
from dotenv import load_dotenv  # noqa: E402
from minio import Minio  # noqa: E402

from internal.extractors.sentiment_bert_de_news import (  # noqa: E402
    GermanNewsBertSentimentExtractor,
)
from internal.extractors.sentiment_bert_multilingual import (  # noqa: E402
    MultilingualBertSentimentExtractor,
)
from internal.models import SilverEnvelope  # noqa: E402

SILVER_BUCKET = "silver"
GOLD_METRICS_TABLE = "aer_gold.metrics"
INSERT_COLUMNS = [
    "timestamp",
    "value",
    "source",
    "metric_name",
    "article_id",
    "discourse_function",
    "ingestion_version",
]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--start",
        default=None,
        help="Window lower bound (ISO date or datetime, UTC). "
        "Default: no lower bound.",
    )
    parser.add_argument(
        "--end",
        default=None,
        help="Window upper bound (ISO date or datetime, UTC). "
        "Default: no upper bound.",
    )
    parser.add_argument(
        "--source",
        action="append",
        default=[],
        help="Restrict to one or more source names (repeatable).",
    )
    parser.add_argument(
        "--batch-size",
        type=int,
        default=500,
        help="Bulk INSERT batch size. Default: 500. BERT inference is "
        "the bottleneck; small batches keep memory predictable.",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Run extractors but do not INSERT into ClickHouse.",
    )
    parser.add_argument(
        "--limit",
        type=int,
        default=0,
        help="Cap envelopes processed. 0 = no cap.",
    )
    return parser.parse_args()


def parse_iso(value: str | None) -> datetime | None:
    if value is None:
        return None
    if value.endswith("Z"):
        value = value[:-1] + "+00:00"
    dt = datetime.fromisoformat(value)
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt


def build_minio_client() -> Minio:
    endpoint = os.getenv("MINIO_ENDPOINT", "localhost:9000")
    return Minio(
        endpoint=endpoint,
        access_key=os.getenv("WORKER_MINIO_ACCESS_KEY", ""),
        secret_key=os.getenv("WORKER_MINIO_SECRET_KEY", ""),
        secure=os.getenv("MINIO_SECURE", "false").lower() == "true",
    )


def derive_discourse_function(meta_dict: dict | None) -> str:
    """Mirrors processor._derive_discourse_function on the JSON payload.

    SilverMeta uses ``SerializeAsAny`` so the Pydantic round-trip from
    raw JSON does not always reconstruct the source-specific subclass.
    Reading the value directly out of the dict avoids the subclass
    re-registration dance and matches the behaviour of the live
    extractor for every adapter that populates ``discourse_context``.
    """
    if not isinstance(meta_dict, dict):
        return ""
    ctx = meta_dict.get("discourse_context")
    if not isinstance(ctx, dict):
        return ""
    val = ctx.get("primary_function")
    return val if isinstance(val, str) else ""


def main() -> int:
    args = parse_args()
    load_dotenv()

    multilingual = MultilingualBertSentimentExtractor()
    de_news = GermanNewsBertSentimentExtractor()

    if not multilingual._enabled and not de_news._enabled:
        print(
            "[backfill] both Phase-119 BERT extractors are disabled "
            "(transformers/torch missing or model not loadable). "
            "Nothing to do.",
            file=sys.stderr,
        )
        return 1

    minio = build_minio_client()

    ch = clickhouse_connect.get_client(
        host=os.getenv("CLICKHOUSE_HOST", "localhost"),
        port=int(os.getenv("CLICKHOUSE_PORT", "8123")),
        username=os.getenv("CLICKHOUSE_USER", "default"),
        password=os.getenv("CLICKHOUSE_PASSWORD", ""),
        database=os.getenv("CLICKHOUSE_DB", "aer_gold"),
    )

    # Phase 116 mirror: the live worker runs LanguageDetectionExtractor first
    # and patches `core.language` with the rank=1 consensus winner before
    # downstream extractors see the document. Pre-Phase-116 Silver envelopes
    # carry `core.language="und"` because language detection ran *after*
    # Silver upload; the live worker fixed this in-flight via `model_copy`,
    # but the on-disk envelopes were never rewritten. Reconstructing the
    # post-detection language here from `aer_gold.language_detections` is
    # what makes the strict Tier-2.5 German-only guard meaningful for the
    # backfill — without it the de_news extractor would emit zero rows.
    language_by_article: dict[str, str] = {}
    rows = ch.query(
        "SELECT article_id, argMax(detected_language, ingestion_version) AS lang "
        "FROM aer_gold.language_detections WHERE rank = 1 GROUP BY article_id"
    ).result_rows
    for article_id, lang in rows:
        if article_id and lang:
            language_by_article[article_id] = lang
    print(
        f"[backfill] loaded {len(language_by_article)} language detections "
        f"from aer_gold.language_detections",
        flush=True,
    )

    start = parse_iso(args.start)
    end = parse_iso(args.end)
    source_filter = set(args.source) if args.source else None

    print(
        f"[backfill] scanning silver/ envelopes "
        f"(start={args.start} end={args.end} sources={args.source or 'all'} "
        f"dry_run={args.dry_run})",
        flush=True,
    )

    scanned = 0
    skipped_filter = 0
    emitted = 0
    batch: list[list] = []

    for obj in minio.list_objects(SILVER_BUCKET, recursive=True):
        if args.limit and scanned >= args.limit:
            break
        if not obj.object_name.endswith(".json"):
            continue

        scanned += 1
        try:
            response = minio.get_object(SILVER_BUCKET, obj.object_name)
            try:
                payload = response.read()
            finally:
                response.close()
                response.release_conn()
            envelope = SilverEnvelope.model_validate_json(payload)
        except Exception as exc:
            print(
                f"[backfill] skipping {obj.object_name!r}: parse failed: {exc}",
                file=sys.stderr,
            )
            continue

        core = envelope.core

        if start is not None and core.timestamp < start:
            skipped_filter += 1
            continue
        if end is not None and core.timestamp >= end:
            skipped_filter += 1
            continue
        if source_filter is not None and core.source not in source_filter:
            skipped_filter += 1
            continue

        # Re-derive `discourse_function` from the raw JSON so the row
        # is byte-equivalent to what the live processor would have
        # produced. Reading the JSON directly side-steps the
        # SerializeAsAny subclass-restoration concern documented above.
        try:
            meta_dict = json.loads(payload).get("meta")
        except Exception:
            meta_dict = None
        discourse_fn = derive_discourse_function(meta_dict)

        ingestion_version = int(core.timestamp.timestamp() * 1_000_000_000)
        article_id = core.document_id

        # Patch core.language from the language_detections table when the
        # envelope's stored language is the legacy `und`/empty marker.
        # Mirrors `processor._handle_message`'s `model_copy(update={...})`
        # behaviour without mutating the original envelope on disk.
        existing = (core.language or "").lower()
        if existing in ("und", "") and article_id in language_by_article:
            core = core.model_copy(update={"language": language_by_article[article_id]})

        results = []
        if multilingual._enabled:
            results.extend(multilingual.extract_all(core, article_id).metrics)
        if de_news._enabled:
            results.extend(de_news.extract_all(core, article_id).metrics)

        for m in results:
            batch.append(
                [
                    m.timestamp,
                    m.value,
                    m.source,
                    m.metric_name,
                    m.article_id,
                    discourse_fn,
                    ingestion_version,
                ]
            )
            emitted += 1

        if not args.dry_run and len(batch) >= args.batch_size:
            ch.insert(GOLD_METRICS_TABLE, batch, column_names=INSERT_COLUMNS)
            batch.clear()

        if scanned % 100 == 0:
            print(
                f"[backfill] scanned={scanned} "
                f"emitted={emitted} skipped(filter)={skipped_filter}",
                flush=True,
            )

    if batch and not args.dry_run:
        ch.insert(GOLD_METRICS_TABLE, batch, column_names=INSERT_COLUMNS)
        batch.clear()

    print(
        f"[backfill] done. scanned={scanned} emitted_rows={emitted} "
        f"skipped(filter)={skipped_filter} "
        f"dry_run={args.dry_run}",
        flush=True,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
