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
        entity_bearing_articles = sum(1 for ents in per_article_unique.values() if len(ents) >= 2)

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
            insert_cooccurrence_rows(ch_pool, rows, ingestion_version, sweep_window=window)
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


def _run_baseline_sweep(ch_pool, extractor: MetricBaselineExtractor, window: TimeWindow):
    """Borrow a client from the pool for a single baseline sweep.

    MetricBaselineExtractor.run expects a clickhouse-connect *client* with
    a .query() method, but the loop only holds the *pool*. Mirrors the
    borrow pattern used by run_sweep / fetch_entities_for_window.
    """
    client = ch_pool.getconn()
    try:
        return extractor.run(client, window)
    finally:
        ch_pool.putconn(client)


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

    The standalone ``scripts/operations/compute_baselines.py`` retained for ad-hoc
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
                result = await asyncio.to_thread(_run_baseline_sweep, ch_pool, extractor, window)
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
    # Phase 122e A19: token granularity = (window, ingestion_version) since
    # a topic sweep is corpus-wide, not per-source.
    window_key = rows[0].window_start.isoformat() if rows else "empty"
    ch_pool.insert(
        "aer_gold.topic_assignments",
        payload,
        column_names=TOPIC_ASSIGNMENT_COLUMNS,
        deduplication_token=f"aer_gold.topic_assignments:{window_key}:{ingestion_version}",
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


# ---------------------------------------------------------------------------
# Phase 122d.1 — Silent-Edit Diff Substance loop.
# ---------------------------------------------------------------------------

ARTICLE_REVISIONS_COLUMNS_FULL = [
    "article_id",
    "source",
    "discourse_function",
    "snapshot_at",
    "content_hash",
    "prev_content_hash",
    "revision_index",
    "time_since_prev_hours",
    "revision_trigger",
    "ingestion_version",
    "archive_url",
    "diff_paragraphs",
    "headline_changed",
    "headline_before",
    "headline_after",
]


@dataclass
class RevisionDiffConfig:
    """Tuneables for the Phase 122d.1 revision-diff sweep loop.

    Operates on `aer_gold.article_revisions` rows whose
    ``revision_trigger='cdx_snapshot'`` and ``revision_index > 0``
    (only consecutive CDX snapshots are diffable; republication-
    trigger rows have no archive_url). Idempotent via
    ``ReplacingMergeTree(ingestion_version)`` — re-runs re-write
    rows with a fresh, higher version and the table collapses.

    Default cadence is hourly, mirroring corpus-extraction. The
    per-tick row budget (``max_pairs_per_tick``) prevents one tick
    from monopolising the worker's CPU + IA-rate-limit budget when
    a fresh crawl produces thousands of new revisions at once.
    """

    enabled: bool = field(
        default_factory=lambda: os.getenv("REVISION_DIFF_EXTRACTION_ENABLED", "false").lower()
        == "true"
    )
    interval_seconds: float = field(
        default_factory=lambda: float(
            os.getenv("REVISION_DIFF_EXTRACTION_INTERVAL_SECONDS", "3600")
        )
    )
    initial_delay_seconds: float = field(
        default_factory=lambda: float(
            os.getenv("REVISION_DIFF_EXTRACTION_INITIAL_DELAY_SECONDS", "180")
        )
    )
    max_pairs_per_tick: int = field(
        default_factory=lambda: int(os.getenv("REVISION_DIFF_MAX_PAIRS_PER_TICK", "200"))
    )


def fetch_undiffed_pairs(ch_pool, limit: int) -> list[dict]:
    """Find consecutive CDX-snapshot pairs that have not yet been diffed.

    Returns two kinds of undiffed pairs:

      1. **Mid-chain pairs** (revision_index > 0): both `curr` and
         `prev` are real Wayback snapshots. `prev_archive_url` is
         non-empty. `compute_diff` runs over the two Wayback HTMLs.

      2. **Chain-head pairs** (revision_index = 0, BUG-11): the
         `curr` row is the oldest Wayback snapshot for the article;
         the "previous" side of the diff is the **current Silver
         body** (the article as crawled now). This answers
         "what has the publisher changed since the last IA capture",
         which is the most direct silent-edit question and makes
         every article with ≥1 Wayback snapshot diffable — including
         the previously-disabled `chainLength=1` case.

    Both kinds return one row per pair with a `kind` field so the
    sweep loop dispatches correctly. The LIMIT bounds the per-tick
    workload; ReplacingMergeTree collapses re-runs cleanly.
    """
    client = ch_pool.getconn()
    try:
        # Mid-chain pairs (existing 122d.1 behaviour).
        mid_result = client.query(
            """
            SELECT
                curr.article_id            AS article_id,
                curr.source                AS source,
                curr.discourse_function    AS discourse_function,
                curr.snapshot_at           AS snapshot_at,
                curr.content_hash          AS content_hash,
                curr.prev_content_hash     AS prev_content_hash,
                curr.revision_index        AS revision_index,
                curr.time_since_prev_hours AS time_since_prev_hours,
                curr.revision_trigger      AS revision_trigger,
                curr.ingestion_version     AS ingestion_version,
                curr.archive_url           AS curr_archive_url,
                prev.archive_url           AS prev_archive_url
            FROM aer_gold.article_revisions AS curr FINAL
            INNER JOIN aer_gold.article_revisions AS prev FINAL
                ON prev.article_id     = curr.article_id
               AND prev.revision_index = curr.revision_index - 1
            WHERE curr.revision_index > 0
              AND curr.revision_trigger = 'cdx_snapshot'
              AND prev.revision_trigger = 'cdx_snapshot'
              AND length(curr.diff_paragraphs) = 0
              AND curr.archive_url != ''
              AND prev.archive_url != ''
            ORDER BY curr.snapshot_at DESC
            LIMIT %(limit)s
            """,
            parameters={"limit": int(limit)},
        )
        pairs: list[dict] = [
            {
                "kind": "mid_chain",
                "article_id": row[0],
                "source": row[1],
                "discourse_function": row[2],
                "snapshot_at": row[3],
                "content_hash": row[4],
                "prev_content_hash": row[5],
                "revision_index": row[6],
                "time_since_prev_hours": row[7],
                "revision_trigger": row[8],
                "ingestion_version": row[9],
                "curr_archive_url": row[10],
                "prev_archive_url": row[11],
            }
            for row in mid_result.result_rows
        ]

        remaining = max(0, int(limit) - len(pairs))
        if remaining <= 0:
            return pairs

        # Chain-head pairs (BUG-11 — Silver-now vs Wayback[0]).
        head_result = client.query(
            """
            SELECT
                article_id, source, discourse_function, snapshot_at,
                content_hash, prev_content_hash, revision_index,
                time_since_prev_hours, revision_trigger,
                ingestion_version, archive_url
            FROM aer_gold.article_revisions FINAL
            WHERE revision_index = 0
              AND revision_trigger = 'cdx_snapshot'
              AND length(diff_paragraphs) = 0
              AND archive_url != ''
            ORDER BY snapshot_at DESC
            LIMIT %(limit)s
            """,
            parameters={"limit": int(remaining)},
        )
        for row in head_result.result_rows:
            pairs.append(
                {
                    "kind": "chain_head",
                    "article_id": row[0],
                    "source": row[1],
                    "discourse_function": row[2],
                    "snapshot_at": row[3],
                    "content_hash": row[4],
                    "prev_content_hash": row[5],
                    "revision_index": row[6],
                    "time_since_prev_hours": row[7],
                    "revision_trigger": row[8],
                    "ingestion_version": row[9],
                    "curr_archive_url": row[10],
                    # No prev_archive_url for chain-head — the
                    # "before" side is the current Silver body,
                    # fetched separately at sweep time.
                    "prev_archive_url": "",
                }
            )
        return pairs
    finally:
        ch_pool.putconn(client)


def fetch_silver_body_for_article(ch_pool, minio_client, bucket: str, article_id: str) -> str:
    """Fetch the current Silver cleaned-text body for one article.

    BUG-11 helper — backs the chain-head diff (current Silver-now vs
    Wayback[0]) so articles with `chainLength=1` become diffable.
    Returns the empty string when:

      * the article has no `aer_silver.documents` row (was archived-
        only past the analytical window, or never harmonised);
      * the MinIO Silver envelope cannot be retrieved;
      * the envelope's `cleaned_text` is empty.

    The sweep loop treats an empty result as "skip this article on
    this tick" and continues.
    """
    import json

    client = ch_pool.getconn()
    try:
        result = client.query(
            """
            SELECT bronze_object_key
            FROM aer_silver.documents FINAL
            WHERE article_id = %(article_id)s
              AND bronze_object_key != ''
            ORDER BY ingestion_version DESC
            LIMIT 1
            """,
            parameters={"article_id": article_id},
        )
        rows = list(result.result_rows)
    finally:
        ch_pool.putconn(client)
    if not rows:
        return ""
    object_key = rows[0][0]
    if not object_key:
        return ""
    try:
        response = minio_client.get_object(bucket, object_key)
        try:
            envelope = json.loads(response.read().decode("utf-8"))
        finally:
            response.close()
            response.release_conn()
    except Exception as exc:
        logger.info(
            "revision_diff.silver_fetch_failed",
            article_id=article_id,
            object_key=object_key,
            error=str(exc),
        )
        return ""
    return (envelope.get("core") or {}).get("cleaned_text", "") or ""


def _silver_text_to_html(cleaned_text: str) -> str:
    """Wrap Silver cleaned-text as a minimal HTML doc so the
    `compute_diff` extractors (which expect HTML) treat it uniformly.

    The Silver `cleaned_text` is plain text with `\\n\\n` paragraph
    breaks (trafilatura's `output_format='txt'`). To diff it against
    a Wayback HTML response without writing two different extraction
    paths, we wrap it as `<html><body>` with paragraphs. The
    headline-extractor returns nothing for this wrapper (no
    `<title>`), which is correct — we have no canonical title for
    "current Silver" so `headline_changed` stays false for
    chain-head pairs even if the title actually drifted.
    """
    paragraphs = cleaned_text.split("\n\n")
    body = "".join(f"<p>{p.strip()}</p>" for p in paragraphs if p.strip())
    return f"<!DOCTYPE html><html><body>{body}</body></html>"


def run_revision_diff_sweep(ch_pool, snapshot_fetcher, minio_client, bucket: str, max_pairs: int) -> int:
    """One revision-diff sweep tick. Returns rows written.

    Handles two pair kinds:

    * **mid_chain** (revision_index > 0): fetches BOTH `prev` and
      `curr` Wayback HTMLs; diff is paragraph-aligned between them.

    * **chain_head** (revision_index = 0, BUG-11): fetches `curr`
      Wayback HTML AND the current Silver body for the same article;
      diff is "current Silver-now → Wayback[0]". Makes every article
      with ≥ 1 Wayback snapshot diffable, not only chainLength ≥ 2.

    Fail-silent per pair: any single pair that fails (snapshot fetch
    error, trafilatura empty result, Silver miss) is logged and
    skipped; the sweep continues with the next pair. Empty diffs are
    written with the SENTINEL_IDENTICAL_OP marker (BUG-B) so the
    next sweep does not re-process them.
    """
    # Imported here rather than at module top so the corpus module
    # boots without the wayback dependency (legacy compatibility for
    # operators running the worker with WAYBACK_CDX_ENABLED=false).
    from internal.article_revisions_diff import compute_diff
    from internal.wayback.snapshot_fetcher import FETCH_OK

    pairs = fetch_undiffed_pairs(ch_pool, max_pairs)
    if not pairs:
        return 0

    rows_to_write: list[list[object]] = []
    for pair in pairs:
        # Fetch the `curr` snapshot — required in both kinds.
        curr_result = snapshot_fetcher.fetch(pair["curr_archive_url"])
        if curr_result.status != FETCH_OK:
            continue

        # Resolve the `prev` content — Wayback HTML for mid-chain,
        # Silver body for chain-head.
        if pair["kind"] == "mid_chain":
            prev_result = snapshot_fetcher.fetch(pair["prev_archive_url"])
            if prev_result.status != FETCH_OK:
                continue
            prev_html = prev_result.html
        elif pair["kind"] == "chain_head":
            silver_text = fetch_silver_body_for_article(
                ch_pool, minio_client, bucket, pair["article_id"]
            )
            if not silver_text:
                # Article has no Silver body (archived-only past the
                # analytical window, MinIO miss, etc.). Skip this
                # tick; next tick re-attempts. We do NOT write the
                # sentinel here — the row is genuinely undiffed, not
                # diffed-but-empty.
                continue
            prev_html = _silver_text_to_html(silver_text)
        else:
            continue

        # Diff direction:
        #   chain-head: prev = Silver-now, curr = Wayback[0]
        #   so the diff says "what was changed FROM the current
        #   Silver TO the older archive" — semantically equivalent
        #   to "what has the publisher changed since archive[0]"
        #   read in reverse. The L5 frontend labels the pair as
        #   "current → archived" for clarity.
        diff = compute_diff(prev_html, curr_result.html)

        new_version = max(int(pair["ingestion_version"]) + 1, time.time_ns())
        rows_to_write.append(
            [
                pair["article_id"],
                pair["source"],
                pair["discourse_function"],
                pair["snapshot_at"],
                pair["content_hash"],
                pair["prev_content_hash"],
                pair["revision_index"],
                pair["time_since_prev_hours"],
                pair["revision_trigger"],
                new_version,
                pair["curr_archive_url"],
                diff.diff_paragraphs,
                diff.headline_changed,
                diff.headline_before,
                diff.headline_after,
            ]
        )

    if not rows_to_write:
        return 0

    ch_pool.insert(
        "aer_gold.article_revisions",
        rows_to_write,
        column_names=ARTICLE_REVISIONS_COLUMNS_FULL,
    )
    return len(rows_to_write)


async def revision_diff_extraction_loop(
    ch_pool,
    snapshot_fetcher,
    minio_client,
    bucket: str,
    config: RevisionDiffConfig,
    stop_event: asyncio.Event,
) -> None:
    """Background task: every ``interval_seconds`` invoke a diff sweep.

    The minio client + bucket are forwarded to the sweep so the
    chain-head pair (BUG-11, revision_index=0) can pull the current
    Silver body for diffing against Wayback[0]. Mid-chain pairs
    ignore them.

    Cleanly stops on ``stop_event``. Errors at the sweep level are
    contained and logged — the fail-silent posture of the Phase-122d
    family extends to this loop.
    """
    if not config.enabled:
        logger.info("revision_diff.loop.disabled")
        return
    if snapshot_fetcher is None:
        logger.info("revision_diff.loop.no_snapshot_fetcher")
        return

    logger.info(
        "revision_diff.loop.started",
        interval_seconds=config.interval_seconds,
        max_pairs_per_tick=config.max_pairs_per_tick,
    )

    try:
        await asyncio.wait_for(stop_event.wait(), timeout=config.initial_delay_seconds)
        return
    except asyncio.TimeoutError:
        pass

    while not stop_event.is_set():
        try:
            rows_written = await asyncio.to_thread(
                run_revision_diff_sweep,
                ch_pool,
                snapshot_fetcher,
                minio_client,
                bucket,
                config.max_pairs_per_tick,
            )
            logger.info("revision_diff.sweep.complete", rows_written=rows_written)
        except Exception as e:
            logger.error(
                "revision_diff.sweep.failed",
                error=str(e),
                error_type=type(e).__name__,
            )

        try:
            await asyncio.wait_for(stop_event.wait(), timeout=config.interval_seconds)
        except asyncio.TimeoutError:
            continue

    logger.info("revision_diff.loop.stopped")
