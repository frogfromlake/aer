#!/usr/bin/env python3
"""One-shot backfill: populate aer_gold.entity_links for entity rows that
already exist in aer_gold.entities.

Use case. The Phase 118 entity-linking step (`NamedEntityExtractor` +
`WikidataAliasIndex`) writes entity_links rows at ingestion time. Documents
ingested *before* the alias index was deployed (or before the worker image
shipped with the linking code) have entity-spans in `aer_gold.entities` but
no rows in `aer_gold.entity_links`. This script closes that gap by reading
the canonical entity spans, running each through the alias index, and
writing the resulting links — without re-running NER, without touching
Bronze or Silver, and without re-publishing NATS events.

Architecture. Pure ClickHouse-to-ClickHouse: SELECT entities → lookup →
INSERT entity_links. Per-document language is taken from the rank=1 row
in `aer_gold.language_detections`; if none exists, the `--default-language`
fallback applies (default `de` for Probe 0). The script reuses the
worker's `WikidataAliasIndex` directly, so confidence weights and
disambiguation behaviour match the live extractor exactly.

Idempotency. `aer_gold.entity_links` is `ReplacingMergeTree(ingestion_version)`.
ingestion_version is derived per row from the entity's timestamp
(nanoseconds), matching the backfill_silver_projection.py convention.
Re-running is safe: existing rows for the same `(article_id, entity_text)`
collapse on merge.

Usage.

    # Run inside the analysis-worker container (has venv + index mount):
    docker compose exec analysis-worker python /scripts/backfill_entity_links.py
    # Or from host if /tmp/wd_venv has the deps:
    docker cp scripts/backfill_entity_links.py aer_analysis_worker:/tmp/
    docker compose exec analysis-worker python /tmp/backfill_entity_links.py

    # Filtered window:
    python backfill_entity_links.py --start 2026-04-01 --end 2026-05-04 \
        --source bundesregierung --batch-size 5000

    # Inspect without writing:
    python backfill_entity_links.py --dry-run

The script honours WIKIDATA_INDEX_PATH and WIKIDATA_INDEX_SHA256 from the
worker's environment for fail-fast hash verification.
"""

from __future__ import annotations

import argparse
import os
import sys
from datetime import datetime, timezone
from pathlib import Path

# Reuse the worker's entity-linking implementation so disambiguation
# semantics match the live extractor exactly. Backfill correctness is
# defined as "produce the same rows the live extractor would have
# produced for these spans".
#
# Two layouts supported:
#   * Repo-root execution from the host: services/analysis-worker/internal/...
#   * In-container execution: /app/internal/... (worker WORKDIR=/app).
# The first existing path wins.
for candidate in (
    Path(__file__).resolve().parent.parent / "services" / "analysis-worker",
    Path("/app"),
):
    if (candidate / "internal" / "extractors" / "entity_linking.py").exists():
        sys.path.insert(0, str(candidate))
        break

import clickhouse_connect  # noqa: E402
from dotenv import load_dotenv  # noqa: E402

from internal.extractors.entity_linking import WikidataAliasIndex  # noqa: E402


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--start", default=None,
                        help="Window lower bound (ISO date or datetime, UTC). Default: no lower bound.")
    parser.add_argument("--end", default=None,
                        help="Window upper bound (ISO date or datetime, UTC). Default: no upper bound.")
    parser.add_argument("--source", action="append", default=[],
                        help="Restrict to one or more source names (repeatable).")
    parser.add_argument("--default-language", default="de",
                        help="Fallback language when language_detections has no rank=1 row. Default: de.")
    parser.add_argument("--batch-size", type=int, default=2000,
                        help="Bulk INSERT batch size. Default: 2000.")
    parser.add_argument("--dry-run", action="store_true",
                        help="Read + lookup only; do not INSERT.")
    parser.add_argument("--limit", type=int, default=0,
                        help="Cap rows scanned. 0 = no cap.")
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


def build_select(args: argparse.Namespace) -> tuple[str, dict]:
    """Build the entities + language_detections JOIN query."""
    clauses = []
    params: dict = {}
    if args.start:
        clauses.append("e.timestamp >= %(start)s")
        params["start"] = parse_iso(args.start)
    if args.end:
        clauses.append("e.timestamp < %(end)s")
        params["end"] = parse_iso(args.end)
    if args.source:
        clauses.append("e.source IN %(sources)s")
        params["sources"] = tuple(args.source)
    where = ("WHERE " + " AND ".join(clauses)) if clauses else ""
    limit = f"LIMIT {int(args.limit)}" if args.limit > 0 else ""
    # LEFT JOIN onto rank=1 detections gives one row per entity span with
    # the document's detected language. Fallback handled in Python because
    # SQL `coalesce` with parameterised default would force a string-array
    # round-trip we don't need here.
    return (
        f"""
        SELECT
            e.timestamp     AS timestamp,
            e.article_id    AS article_id,
            e.entity_text   AS entity_text,
            e.entity_label  AS entity_label,
            l.detected_language AS detected_language
        FROM aer_gold.entities AS e
        LEFT JOIN (
            SELECT article_id, argMax(detected_language, ingestion_version) AS detected_language
            FROM aer_gold.language_detections
            WHERE rank = 1
            GROUP BY article_id
        ) AS l USING (article_id)
        {where}
        ORDER BY e.timestamp ASC
        {limit}
        """,
        params,
    )


def main() -> int:
    args = parse_args()
    load_dotenv()

    index_path = os.getenv("WIKIDATA_INDEX_PATH", "/data/wikidata/wikidata_aliases.db")
    expected_sha256 = os.getenv("WIKIDATA_INDEX_SHA256", "").strip() or None
    idx = WikidataAliasIndex(index_path, expected_sha256=expected_sha256)

    ch = clickhouse_connect.get_client(
        host=os.getenv("CLICKHOUSE_HOST", "localhost"),
        port=int(os.getenv("CLICKHOUSE_PORT", "8123")),
        username=os.getenv("CLICKHOUSE_USER", "default"),
        password=os.getenv("CLICKHOUSE_PASSWORD", ""),
        database=os.getenv("CLICKHOUSE_DB", "aer_gold"),
    )

    sql, params = build_select(args)
    print(f"[backfill] scanning aer_gold.entities (default-language={args.default_language!r}) ...")

    total = matched = unmatched = 0
    insert_columns = [
        "timestamp", "article_id", "entity_text", "entity_label",
        "wikidata_qid", "link_confidence", "link_method", "ingestion_version",
    ]
    batch: list[list] = []

    with ch.query_row_block_stream(sql, parameters=params) as stream:
        for block in stream:
            for ts, article_id, entity_text, entity_label, detected_language in block:
                total += 1
                lang = (detected_language or args.default_language or "").lower()
                if not lang:
                    unmatched += 1
                    continue
                cand = idx.lookup(entity_text, lang)
                if cand is None:
                    unmatched += 1
                    continue
                matched += 1
                # ingestion_version mirrors the convention used by the live
                # extractor and the existing backfill_silver_projection
                # script: the document's own timestamp in nanoseconds, so a
                # future live event for the same span collapses cleanly.
                ingestion_version = int(ts.timestamp() * 1_000_000_000)
                batch.append([
                    ts, article_id, entity_text, entity_label,
                    cand.wikidata_qid, cand.confidence, cand.method,
                    ingestion_version,
                ])
                if len(batch) >= args.batch_size and not args.dry_run:
                    ch.insert("aer_gold.entity_links", batch, column_names=insert_columns)
                    batch.clear()
            if total % 10000 == 0 and total > 0:
                print(f"  scanned={total:>8d} matched={matched:>8d} unmatched={unmatched:>8d}")

    if batch and not args.dry_run:
        ch.insert("aer_gold.entity_links", batch, column_names=insert_columns)

    rate = (100.0 * matched / total) if total else 0.0
    mode = "(dry-run, no rows written)" if args.dry_run else "(rows inserted)"
    print()
    print(f"[backfill] complete {mode}")
    print(f"  scanned   {total:>8d} entity spans")
    print(f"  linked    {matched:>8d} ({rate:.1f}%)")
    print(f"  unlinked  {unmatched:>8d}  (no candidate above 0.7 confidence threshold)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
