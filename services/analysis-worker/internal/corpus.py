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
    DocumentRecord,
    EntityCoOccurrenceExtractor,
    EntityRecord,
    MetricBaselineExtractor,
    TimeWindow,
    TopicAssignmentRow,
    TopicModelingExtractor,
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


TOPIC_ASSIGNMENT_COLUMNS = [
    "window_start",
    "window_end",
    "source",
    "article_id",
    "language",
    "topic_id",
    "topic_label",
    "topic_confidence",
    "model_hash",
    "ingestion_version",
]


SILVER_BUCKET_DEFAULT = "silver"


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


@dataclass
class TopicConfig:
    """Tuneables for the periodic ``TopicModelingExtractor`` loop (Phase 120).

    BERTopic is expensive (E5 embeddings + UMAP + HDBSCAN over an entire
    corpus window), so the default cadence is weekly rather than the
    co-occurrence loop's hourly cadence. The 30-day default window matches
    the BERTopic methodology recommendation in WP-001 §3.3 — long enough
    to surface stable topics, short enough that topic drift is detectable
    by re-running the extractor.
    """

    enabled: bool = field(
        default_factory=lambda: os.getenv("TOPIC_EXTRACTION_ENABLED", "false").lower() == "true"
    )
    interval_seconds: float = field(
        default_factory=lambda: float(os.getenv("TOPIC_EXTRACTION_INTERVAL_SECONDS", str(7 * 86400)))
    )
    window_seconds: float = field(
        default_factory=lambda: float(os.getenv("TOPIC_EXTRACTION_WINDOW_SECONDS", str(30 * 86400)))
    )
    initial_delay_seconds: float = field(
        default_factory=lambda: float(os.getenv("TOPIC_EXTRACTION_INITIAL_DELAY_SECONDS", "600"))
    )
    silver_bucket: str = field(
        default_factory=lambda: os.getenv("WORKER_SILVER_BUCKET", SILVER_BUCKET_DEFAULT)
    )


@dataclass
class BaselineConfig:
    """Tuneables for the periodic ``MetricBaselineExtractor`` loop (Phase 115)."""

    enabled: bool = field(
        default_factory=lambda: os.getenv("BASELINE_EXTRACTION_ENABLED", "true").lower() == "true"
    )
    # Default cadence is daily — baselines move slowly relative to per-document
    # metrics, so a tighter interval would be wasted compute.
    interval_seconds: float = field(
        default_factory=lambda: float(os.getenv("BASELINE_EXTRACTION_INTERVAL_SECONDS", "86400"))
    )
    # Default rolling window is 90 days, matching the Operations Playbook's
    # documented manual-script default and the Bronze ILM TTL.
    window_seconds: float = field(
        default_factory=lambda: float(os.getenv("BASELINE_EXTRACTION_WINDOW_SECONDS", str(90 * 86400)))
    )
    initial_delay_seconds: float = field(
        default_factory=lambda: float(os.getenv("BASELINE_EXTRACTION_INITIAL_DELAY_SECONDS", "300"))
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


async def baseline_extraction_loop(
    ch_pool,
    extractor: MetricBaselineExtractor,
    config: BaselineConfig,
    stop_event: asyncio.Event,
) -> None:
    """
    Phase 115: periodic baseline-maintenance loop.

    Every ``interval_seconds`` (default 86400) computes metric baselines for
    the previous ``window_seconds`` (default 7_776_000 ≈ 90 days) and inserts
    them into ``aer_gold.metric_baselines``. Idempotent via
    ReplacingMergeTree(``compute_date``); a re-run within the same second
    produces an identical row that the ReplacingMergeTree collapses on merge.

    The standalone ``scripts/compute_baselines.py`` retained for ad-hoc
    operations shares the same computation function — see
    ``internal.extractors.metric_baseline.compute_baseline_rows`` and
    Phase-115 byte-identical regression test.
    """
    if not config.enabled:
        logger.info("baseline.loop.disabled")
        return

    logger.info(
        "baseline.loop.started",
        extractor=extractor.name,
        interval_seconds=config.interval_seconds,
        window_seconds=config.window_seconds,
    )

    try:
        await asyncio.wait_for(stop_event.wait(), timeout=config.initial_delay_seconds)
        return  # stop fired during initial delay
    except asyncio.TimeoutError:
        pass

    while not stop_event.is_set():
        now = datetime.now(timezone.utc).replace(tzinfo=None)
        window = TimeWindow(
            start=now - timedelta(seconds=config.window_seconds),
            end=now,
        )

        try:
            with corpus_extraction_duration_seconds.labels(extractor=extractor.name).time():
                result = await asyncio.to_thread(extractor.run, ch_pool, window)
            corpus_extraction_runs_total.labels(
                extractor=extractor.name, outcome="ok"
            ).inc()
            if result.rows_written:
                corpus_extraction_rows_written_total.labels(
                    extractor=extractor.name,
                    table="aer_gold.metric_baselines",
                ).inc(result.rows_written)
            logger.info(
                "baseline.loop.tick_complete",
                extractor=extractor.name,
                window_start=str(window.start),
                window_end=str(window.end),
                rows_written=result.rows_written,
            )
        except Exception as e:  # pragma: no cover — defensive top-level guard
            corpus_extraction_runs_total.labels(
                extractor=extractor.name, outcome="error"
            ).inc()
            logger.error(
                "baseline.loop.failed",
                extractor=extractor.name,
                error=str(e),
                error_type=type(e).__name__,
            )

        try:
            await asyncio.wait_for(stop_event.wait(), timeout=config.interval_seconds)
        except asyncio.TimeoutError:
            continue

    logger.info("baseline.loop.stopped", extractor=extractor.name)


# --- Phase 120: Topic Modeling corpus loop ---


def fetch_silver_documents_for_window(
    ch_pool,
    minio_client,
    bucket: str,
    window: TimeWindow,
) -> list[DocumentRecord]:
    """Read the article list from ``aer_silver.documents`` and pull the
    cleaned text for each one from MinIO.

    The Silver projection table only carries ``cleaned_text_length`` —
    the actual prose lives in the MinIO Silver envelope. We index the
    article list via ClickHouse (cheap) and resolve text via MinIO
    (one GET per article). For a typical 30-day Probe-0 window this
    is on the order of 10² requests, which the corpus loop's weekly
    cadence absorbs comfortably.

    Documents whose Silver fetch fails are skipped with a warning;
    a single missing object never aborts the sweep.
    """
    import json

    client = ch_pool.getconn()
    try:
        result = client.query(
            """
            SELECT article_id, source, language, bronze_object_key
            FROM aer_silver.documents FINAL
            WHERE timestamp >= %(start)s
              AND timestamp < %(end)s
              AND bronze_object_key != ''
              AND article_id != ''
            """,
            parameters={"start": window.start, "end": window.end},
        )
        index_rows = list(result.result_rows)
    finally:
        ch_pool.putconn(client)

    documents: list[DocumentRecord] = []
    for article_id, source, language, object_key in index_rows:
        try:
            response = minio_client.get_object(bucket, object_key)
            try:
                envelope = json.loads(response.read().decode("utf-8"))
            finally:
                response.close()
                response.release_conn()
        except Exception as e:
            logger.warning(
                "topic.sweep.silver_fetch_failed",
                article_id=article_id,
                object_key=object_key,
                error=str(e),
            )
            continue

        cleaned_text = (envelope.get("core") or {}).get("cleaned_text", "")
        if not cleaned_text:
            continue
        documents.append(
            DocumentRecord(
                article_id=article_id,
                source=source,
                language=language or "und",
                cleaned_text=cleaned_text,
            )
        )
    return documents


def insert_topic_assignment_rows(
    ch_pool,
    rows: list[TopicAssignmentRow],
    ingestion_version: int,
) -> None:
    """Bulk-insert topic-assignment rows; ingestion_version stamped per sweep."""
    if not rows:
        return
    payload = [
        [
            r.window_start,
            r.window_end,
            r.source,
            r.article_id,
            r.language,
            r.topic_id,
            r.topic_label,
            r.topic_confidence,
            r.model_hash,
            ingestion_version,
        ]
        for r in rows
    ]
    ch_pool.insert(
        "aer_gold.topic_assignments",
        payload,
        column_names=TOPIC_ASSIGNMENT_COLUMNS,
    )


def run_topic_sweep(
    ch_pool,
    minio_client,
    extractor: TopicModelingExtractor,
    window: TimeWindow,
    bucket: str,
) -> int:
    """Run one BERTopic sweep across the (window) corpus. Returns rows written.

    Unlike the co-occurrence sweep (which iterates per-source), the topic
    sweep is global per-language: BERTopic's value is in cross-source
    structure within one cultural-linguistic frame (WP-004 §3.4).
    Per-language partitioning happens inside the extractor.
    """
    documents = fetch_silver_documents_for_window(ch_pool, minio_client, bucket, window)
    if not documents:
        logger.info("topic.sweep.no_documents", window_start=str(window.start))
        return 0

    rows = extractor.extract_topics(documents, window)
    if not rows:
        logger.info(
            "topic.sweep.empty",
            window_start=str(window.start),
            input_documents=len(documents),
        )
        return 0

    ingestion_version = time.time_ns()
    try:
        insert_topic_assignment_rows(ch_pool, rows, ingestion_version)
    except Exception as e:
        logger.error(
            "topic.sweep.insert_failed",
            window_start=str(window.start),
            rows=len(rows),
            error=str(e),
        )
        corpus_extraction_runs_total.labels(
            extractor=extractor.name, outcome="insert_failed"
        ).inc()
        return 0

    corpus_extraction_rows_written_total.labels(
        extractor=extractor.name, table="aer_gold.topic_assignments"
    ).inc(len(rows))
    logger.info(
        "topic.sweep.complete",
        window_start=str(window.start),
        window_end=str(window.end),
        input_documents=len(documents),
        rows_written=len(rows),
    )
    return len(rows)


async def topic_extraction_loop(
    ch_pool,
    minio_client,
    extractor: TopicModelingExtractor,
    config: TopicConfig,
    stop_event: asyncio.Event,
) -> None:
    """Background task: every ``interval_seconds`` invoke ``run_topic_sweep``
    over the previous ``window_seconds``. Exits cleanly when ``stop_event`` is set.

    The loop is opt-in (``TOPIC_EXTRACTION_ENABLED`` defaults to false)
    because the model bake-in lands the E5 weights into the worker image
    (~2.2 GB). Operators flip the flag once the heavier image is
    deployed; until then the worker behaves exactly as before Phase 120.
    """
    if not config.enabled:
        logger.info("topic.loop.disabled")
        return

    logger.info(
        "topic.loop.started",
        extractor=extractor.name,
        interval_seconds=config.interval_seconds,
        window_seconds=config.window_seconds,
    )

    try:
        await asyncio.wait_for(stop_event.wait(), timeout=config.initial_delay_seconds)
        return
    except asyncio.TimeoutError:
        pass

    while not stop_event.is_set():
        now = datetime.now(timezone.utc).replace(tzinfo=None)
        window = TimeWindow(
            start=now - timedelta(seconds=config.window_seconds),
            end=now,
        )

        try:
            with corpus_extraction_duration_seconds.labels(extractor=extractor.name).time():
                rows_written = await asyncio.to_thread(
                    run_topic_sweep,
                    ch_pool,
                    minio_client,
                    extractor,
                    window,
                    config.silver_bucket,
                )
            corpus_extraction_runs_total.labels(
                extractor=extractor.name, outcome="ok"
            ).inc()
            logger.info(
                "topic.loop.tick_complete",
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
                "topic.loop.failed",
                extractor=extractor.name,
                error=str(e),
                error_type=type(e).__name__,
            )

        try:
            await asyncio.wait_for(stop_event.wait(), timeout=config.interval_seconds)
        except asyncio.TimeoutError:
            continue

    logger.info("topic.loop.stopped", extractor=extractor.name)
