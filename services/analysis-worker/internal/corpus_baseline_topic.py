from __future__ import annotations

import asyncio
import time
from contextlib import nullcontext
from datetime import datetime, timedelta, timezone

import structlog

from internal.extractors import (
    DocumentRecord,
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
from internal.corpus import (
    BaselineConfig,
    TopicConfig,
    TOPIC_ASSIGNMENT_COLUMNS,
)

logger = structlog.get_logger()


def _run_baseline_sweep(
    ch_pool, extractor: MetricBaselineExtractor, window: TimeWindow
):
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
    *,
    extraction_lock: asyncio.Lock | None = None,
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
            async with (extraction_lock or nullcontext()):
                with corpus_extraction_duration_seconds.labels(
                    extractor=extractor.name
                ).time():
                    result = await asyncio.to_thread(
                        _run_baseline_sweep, ch_pool, extractor, window
                    )
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
    *,
    extraction_lock: asyncio.Lock | None = None,
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
            async with (extraction_lock or nullcontext()):
                with corpus_extraction_duration_seconds.labels(
                    extractor=extractor.name
                ).time():
                    # Phase 148c — bound the sweep so a runaway fit can't hold the
                    # corpus mutex forever (it would starve co-occurrence /
                    # baseline / revision-diff). On timeout this raises into the
                    # except below, the lock releases, and the loop retries next
                    # tick. The orphaned to_thread finishes in the background;
                    # Gold writes are idempotent (ReplacingMergeTree).
                    rows_written = await asyncio.wait_for(
                        asyncio.to_thread(
                            run_topic_sweep,
                            ch_pool,
                            minio_client,
                            extractor,
                            window,
                            config.silver_bucket,
                        ),
                        timeout=config.sweep_timeout_seconds,
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
