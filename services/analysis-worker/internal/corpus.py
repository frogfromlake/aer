"""
Corpus-extraction loop (Phase 102).

Periodic asyncio task that drives the EntityCoOccurrenceExtractor (and any
future CorpusExtractor) over the previous time window for every active
source. Reads ``aer_gold.entities`` from ClickHouse, computes pair rows in
process, and bulk-inserts into ``aer_gold.entity_cooccurrences``.

The loop intentionally lives inside the analysis-worker process rather than
behind a separate scheduler: one place to deploy, one place to drain on
SIGTERM, and idempotency via ReplacingMergeTree(ingestion_version) makes
overlapping or repeated sweeps harmless.
"""

from __future__ import annotations

import asyncio
import os
import time
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone

import structlog

from internal.extractors import (
    CoOccurrenceRow,
    EntityCoOccurrenceExtractor,
    EntityRecord,
    TimeWindow,
)
from internal.metrics import (
    corpus_extraction_duration_seconds,
    corpus_extraction_rows_written_total,
    corpus_extraction_runs_total,
)

logger = structlog.get_logger()


COOCCURRENCE_COLUMNS = [
    "window_start",
    "window_end",
    "source",
    "article_id",
    "entity_a_text",
    "entity_a_label",
    "entity_b_text",
    "entity_b_label",
    "cooccurrence_count",
    "ingestion_version",
]


@dataclass
class CorpusConfig:
    """Tuneables for the corpus-extraction loop, injectable for tests."""

    enabled: bool = field(
        default_factory=lambda: os.getenv("CORPUS_EXTRACTION_ENABLED", "true").lower() == "true"
    )
    interval_seconds: float = field(
        default_factory=lambda: float(os.getenv("CORPUS_EXTRACTION_INTERVAL_SECONDS", "3600"))
    )
    window_seconds: float = field(
        default_factory=lambda: float(os.getenv("CORPUS_EXTRACTION_WINDOW_SECONDS", "3600"))
    )
    initial_delay_seconds: float = field(
        default_factory=lambda: float(os.getenv("CORPUS_EXTRACTION_INITIAL_DELAY_SECONDS", "60"))
    )


def list_active_sources(pg_pool) -> list[str]:
    """Return the set of source names the worker should sweep this tick."""
    conn = pg_pool.getconn()
    try:
        with conn.cursor() as cur:
            cur.execute("SELECT name FROM sources ORDER BY name;")
            return [row[0] for row in cur.fetchall()]
    finally:
        pg_pool.putconn(conn)


def fetch_entities_for_window(ch_pool, source: str, window: TimeWindow) -> list[EntityRecord]:
    """Read entity rows from aer_gold.entities for one (source, window)."""
    client = ch_pool.getconn()
    try:
        # FINAL collapses ReplacingMergeTree duplicates so a re-run reads
        # the same logical rows on every sweep, keeping the output stable.
        result = client.query(
            """
            SELECT article_id, entity_text, entity_label
            FROM aer_gold.entities FINAL
            WHERE source = %(source)s
              AND timestamp >= %(start)s
              AND timestamp < %(end)s
              AND article_id IS NOT NULL
              AND entity_text != ''
            """,
            parameters={
                "source": source,
                "start": window.start,
                "end": window.end,
            },
        )
        return [
            EntityRecord(article_id=row[0], entity_text=row[1], entity_label=row[2])
            for row in result.result_rows
        ]
    finally:
        ch_pool.putconn(client)


def insert_cooccurrence_rows(
    ch_pool,
    rows: list[CoOccurrenceRow],
    ingestion_version: int,
) -> None:
    """Bulk-insert co-occurrence rows; ingestion_version stamped per sweep."""
    if not rows:
        return
    payload = [
        [
            r.window_start,
            r.window_end,
            r.source,
            r.article_id,
            r.entity_a_text,
            r.entity_a_label,
            r.entity_b_text,
            r.entity_b_label,
            r.cooccurrence_count,
            ingestion_version,
        ]
        for r in rows
    ]
    ch_pool.insert(
        "aer_gold.entity_cooccurrences",
        payload,
        column_names=COOCCURRENCE_COLUMNS,
    )


def run_sweep(
    ch_pool,
    pg_pool,
    extractor: EntityCoOccurrenceExtractor,
    window: TimeWindow,
) -> int:
    """Run one corpus sweep across all active sources. Returns rows written."""
    sources = list_active_sources(pg_pool)
    if not sources:
        logger.info("corpus.sweep.no_sources", window_start=str(window.start))
        return 0

    ingestion_version = time.time_ns()
    total_rows = 0
    for source in sources:
        try:
            records = fetch_entities_for_window(ch_pool, source, window)
        except Exception as e:  # pragma: no cover — defensive
            logger.error(
                "corpus.sweep.fetch_failed",
                source=source,
                window_start=str(window.start),
                error=str(e),
            )
            corpus_extraction_runs_total.labels(
                extractor=extractor.name, outcome="fetch_failed"
            ).inc()
            continue

        rows = extractor.extract_pairs(records, window, source)
        if not rows:
            logger.info(
                "corpus.sweep.empty",
                source=source,
                window_start=str(window.start),
                input_records=len(records),
            )
            continue

        try:
            insert_cooccurrence_rows(ch_pool, rows, ingestion_version)
        except Exception as e:
            logger.error(
                "corpus.sweep.insert_failed",
                source=source,
                window_start=str(window.start),
                rows=len(rows),
                error=str(e),
            )
            corpus_extraction_runs_total.labels(
                extractor=extractor.name, outcome="insert_failed"
            ).inc()
            continue

        corpus_extraction_rows_written_total.labels(
            extractor=extractor.name, table="aer_gold.entity_cooccurrences"
        ).inc(len(rows))
        total_rows += len(rows)
        logger.info(
            "corpus.sweep.source_complete",
            source=source,
            window_start=str(window.start),
            rows=len(rows),
            input_records=len(records),
        )

    return total_rows


async def corpus_extraction_loop(
    ch_pool,
    pg_pool,
    extractor: EntityCoOccurrenceExtractor,
    config: CorpusConfig,
    stop_event: asyncio.Event,
) -> None:
    """
    Background task: every ``interval_seconds`` invoke ``run_sweep`` for the
    previous ``window_seconds``. Exits cleanly when ``stop_event`` is set.
    """
    if not config.enabled:
        logger.info("corpus.loop.disabled")
        return

    logger.info(
        "corpus.loop.started",
        extractor=extractor.name,
        interval_seconds=config.interval_seconds,
        window_seconds=config.window_seconds,
    )

    try:
        await asyncio.wait_for(stop_event.wait(), timeout=config.initial_delay_seconds)
        return  # stop fired during initial delay
    except asyncio.TimeoutError:
        pass  # initial delay elapsed; begin sweeping

    while not stop_event.is_set():
        now = datetime.now(timezone.utc).replace(tzinfo=None)
        window = TimeWindow(
            start=now - timedelta(seconds=config.window_seconds),
            end=now,
        )

        try:
            with corpus_extraction_duration_seconds.labels(extractor=extractor.name).time():
                rows_written = await asyncio.to_thread(
                    run_sweep, ch_pool, pg_pool, extractor, window
                )
            corpus_extraction_runs_total.labels(
                extractor=extractor.name, outcome="ok"
            ).inc()
            logger.info(
                "corpus.sweep.complete",
                extractor=extractor.name,
                window_start=str(window.start),
                window_end=str(window.end),
                rows_written=rows_written,
            )
        except Exception as e:  # pragma: no cover — defensive top-level guard
            corpus_extraction_runs_total.labels(
                extractor=extractor.name, outcome="error"
            ).inc()
            logger.error(
                "corpus.sweep.failed",
                extractor=extractor.name,
                error=str(e),
                error_type=type(e).__name__,
            )

        try:
            await asyncio.wait_for(stop_event.wait(), timeout=config.interval_seconds)
        except asyncio.TimeoutError:
            continue  # interval elapsed without shutdown — next sweep

    logger.info("corpus.loop.stopped", extractor=extractor.name)
