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
from contextlib import nullcontext
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
        default_factory=lambda: (
            os.getenv("CORPUS_EXTRACTION_ENABLED", "true").lower() == "true"
        )
    )
    interval_seconds: float = field(
        default_factory=lambda: float(
            os.getenv("CORPUS_EXTRACTION_INTERVAL_SECONDS", "3600")
        )
    )
    # Phase 131a (BUG 1.5): the sweep window must cover the entire Gold
    # ``aer_gold.entities`` retention (365 days). A short rolling window
    # (the Phase-102 default of 3600 s) only picked up articles whose
    # ``published_date`` fell in the last hour and silently dropped every
    # entity-bearing article older than that — corpus-wide we observed
    # 771 articles with NER entities but only 11 with co-occurrence rows.
    # Per-article ``window_start = published_date`` (Phase 131a) keeps the
    # row PK stable across overlapping sweeps so re-emission is a no-op
    # under ReplacingMergeTree(ingestion_version).
    window_seconds: float = field(
        default_factory=lambda: float(
            os.getenv("CORPUS_EXTRACTION_WINDOW_SECONDS", str(365 * 86400))
        )
    )
    initial_delay_seconds: float = field(
        default_factory=lambda: float(
            os.getenv("CORPUS_EXTRACTION_INITIAL_DELAY_SECONDS", "60")
        )
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
        default_factory=lambda: (
            os.getenv("TOPIC_EXTRACTION_ENABLED", "false").lower() == "true"
        )
    )
    interval_seconds: float = field(
        default_factory=lambda: float(
            os.getenv("TOPIC_EXTRACTION_INTERVAL_SECONDS", str(7 * 86400))
        )
    )
    window_seconds: float = field(
        default_factory=lambda: float(
            os.getenv("TOPIC_EXTRACTION_WINDOW_SECONDS", str(30 * 86400))
        )
    )
    initial_delay_seconds: float = field(
        default_factory=lambda: float(
            os.getenv("TOPIC_EXTRACTION_INITIAL_DELAY_SECONDS", "600")
        )
    )
    silver_bucket: str = field(
        default_factory=lambda: os.getenv("WORKER_SILVER_BUCKET", SILVER_BUCKET_DEFAULT)
    )
    # Phase 148c — hard ceiling on a single topic sweep. The corpus mutex
    # serialises the heavy sweeps, so a runaway BERTopic fit would otherwise hold
    # the lock indefinitely and starve co-occurrence / baseline / revision-diff
    # ("holds everything else up"). On timeout the loop releases the lock and
    # retries next tick (the orphaned thread finishes in the background; Gold
    # writes are idempotent). Generous: with multi-threaded embedding a full
    # partition is minutes, so this only fires on a genuine pathology.
    sweep_timeout_seconds: float = field(
        default_factory=lambda: float(
            os.getenv("TOPIC_EXTRACTION_SWEEP_TIMEOUT_SECONDS", "1800")
        )
    )


@dataclass
class BaselineConfig:
    """Tuneables for the periodic ``MetricBaselineExtractor`` loop (Phase 115)."""

    enabled: bool = field(
        default_factory=lambda: (
            os.getenv("BASELINE_EXTRACTION_ENABLED", "true").lower() == "true"
        )
    )
    # Default cadence is daily — baselines move slowly relative to per-document
    # metrics, so a tighter interval would be wasted compute.
    interval_seconds: float = field(
        default_factory=lambda: float(
            os.getenv("BASELINE_EXTRACTION_INTERVAL_SECONDS", "86400")
        )
    )
    # Default rolling window is 90 days, matching the Operations Playbook's
    # documented manual-script default and the Bronze ILM TTL.
    window_seconds: float = field(
        default_factory=lambda: float(
            os.getenv("BASELINE_EXTRACTION_WINDOW_SECONDS", str(90 * 86400))
        )
    )
    initial_delay_seconds: float = field(
        default_factory=lambda: float(
            os.getenv("BASELINE_EXTRACTION_INITIAL_DELAY_SECONDS", "300")
        )
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


def fetch_entities_for_window(
    ch_pool, source: str, window: TimeWindow
) -> list[EntityRecord]:
    """Read entity rows from aer_gold.entities for one (source, window).

    Phase 131a: also returns each record's ``timestamp`` (the article's
    ``published_date``). The co-occurrence row uses this per-article
    timestamp as ``window_start`` so re-sweeps collapse on a stable PK.
    """
    client = ch_pool.getconn()
    try:
        # FINAL collapses ReplacingMergeTree duplicates so a re-run reads
        # the same logical rows on every sweep, keeping the output stable.
        result = client.query(
            """
            SELECT article_id, entity_text, entity_label, timestamp
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
            EntityRecord(
                article_id=row[0],
                entity_text=row[1],
                entity_label=row[2],
                timestamp=row[3],
            )
            for row in result.result_rows
        ]
    finally:
        ch_pool.putconn(client)


def insert_cooccurrence_rows(
    ch_pool,
    rows: list[CoOccurrenceRow],
    ingestion_version: int,
    sweep_window: TimeWindow | None = None,
) -> None:
    """Bulk-insert co-occurrence rows; ingestion_version stamped per sweep.

    Phase 131a: each row's ``window_start`` is now the article's
    ``published_date`` (per-article, varying across rows), so the dedup
    token can no longer be derived from ``rows[0].window_start``. We
    instead key the token on the sweep's bounding window — passed in by
    the caller — which is constant for one sweep call. Falls back to
    ``rows[0]`` when the caller does not pass a sweep window so legacy
    test invocations remain valid.
    """
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
    if sweep_window is not None:
        window_key = sweep_window.start.isoformat()
    else:
        window_key = rows[0].window_start.isoformat()
    source_key = rows[0].source
    ch_pool.insert(
        "aer_gold.entity_cooccurrences",
        payload,
        column_names=COOCCURRENCE_COLUMNS,
        deduplication_token=f"aer_gold.entity_cooccurrences:{source_key}:{window_key}:{ingestion_version}",
    )


def run_sweep(
    ch_pool,
    pg_pool,
    extractor: EntityCoOccurrenceExtractor,
    window: TimeWindow,
) -> int:
    """Run one corpus sweep across all active sources. Returns rows written.

    Phase 131a: observability extended so the pipeline-gap failure mode
    (entity-bearing articles producing zero co-occurrence rows) is loud
    in the logs. Each per-source iteration emits
    ``input_articles`` (articles with ≥1 entity) and
    ``entity_bearing_articles`` (articles with ≥2 entities — the actual
    co-occurrence-eligible set). A non-zero
    ``entity_bearing_articles`` with zero output rows now logs a
    structured warning instead of an info line, surfacing the
    regression on every sweep that misbehaves.
    """
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

        # Pipeline-gap visibility (Phase 131a): count articles whose
        # entity set contains ≥2 DISTINCT (text, label) entities so a
        # zero-output sweep can be discriminated from a sparse-corpus
        # sweep at a glance.
        #
        # Counting *rows* would mislead: an article that mentions
        # "Merkel" twice and no one else has two entity rows but only
        # one unique entity → ``_pairs_for_article`` legitimately emits
        # zero pairs. Row-counting would fire a false-positive
        # ``pipeline_gap`` warning on every such article.
        per_article_unique: dict[str, set[tuple[str, str]]] = {}
        for rec in records:
            if rec.article_id and rec.entity_text:
                per_article_unique.setdefault(rec.article_id, set()).add(
                    (rec.entity_text, rec.entity_label)
                )
        input_articles = len(per_article_unique)
        entity_bearing_articles = sum(
            1 for ents in per_article_unique.values() if len(ents) >= 2
        )

        rows = extractor.extract_pairs(records, window, source)
        if not rows:
            if entity_bearing_articles > 0:
                # PIPELINE GAP: the data says rows should exist but the
                # extractor emitted nothing. Loud, structured, sweep-by-sweep.
                logger.warning(
                    "corpus.sweep.pipeline_gap",
                    source=source,
                    window_start=str(window.start),
                    window_end=str(window.end),
                    input_records=len(records),
                    input_articles=input_articles,
                    entity_bearing_articles=entity_bearing_articles,
                    cooccurrence_rows=0,
                )
            else:
                logger.info(
                    "corpus.sweep.empty",
                    source=source,
                    window_start=str(window.start),
                    input_records=len(records),
                    input_articles=input_articles,
                )
            continue

        try:
            insert_cooccurrence_rows(
                ch_pool, rows, ingestion_version, sweep_window=window
            )
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
            window_end=str(window.end),
            rows=len(rows),
            input_records=len(records),
            input_articles=input_articles,
            entity_bearing_articles=entity_bearing_articles,
        )

    return total_rows


async def corpus_extraction_loop(
    ch_pool,
    pg_pool,
    extractor: EntityCoOccurrenceExtractor,
    config: CorpusConfig,
    stop_event: asyncio.Event,
    *,
    extraction_lock: asyncio.Lock | None = None,
) -> None:
    """
    Background task: every ``interval_seconds`` invoke ``run_sweep`` for the
    previous ``window_seconds``. Exits cleanly when ``stop_event`` is set.

    ``extraction_lock`` (Phase 148c) serialises the heavy corpus sweeps
    (co-occurrence / topic / baseline / revision-diff) against each other so
    they never saturate CPU + RAM simultaneously — the contention that starved
    the BERTopic fit and pushed the worker to its memory ceiling. ``None``
    (the default, used by unit tests) means no locking.
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
            async with (extraction_lock or nullcontext()):
                with corpus_extraction_duration_seconds.labels(
                    extractor=extractor.name
                ).time():
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
