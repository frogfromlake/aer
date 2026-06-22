"""Corpus-level revision-diff sweep (Phase 122d.3 / ADR-032): compares
consecutive Wayback captures of the same article — and each chain head against
the current Silver body — to mark which re-archivals are genuine editorial edits
vs. byte-identical re-fetches, writing the editorial-revision counts (and, when
enabled, the discourse deltas) to Gold. Runs as a background tick alongside the
other corpus sweeps (see main.py)."""

from __future__ import annotations

import asyncio
import time
from contextlib import nullcontext

import structlog

from internal.corpus_revision_io import (
    ARTICLE_REVISIONS_COLUMNS_FULL,
    RevisionDiffConfig,
    _silver_text_to_html,
    fetch_silver_body_for_article,
    fetch_undiffed_pairs,
    write_editorial_revision_counts,
)

logger = structlog.get_logger()


def run_revision_diff_sweep(
    ch_pool,
    snapshot_fetcher,
    minio_client,
    bucket: str,
    max_pairs: int,
    delta_tools=None,
) -> int:
    """One revision-diff sweep tick. Returns rows written.

    Handles two pair kinds:

    * **mid_chain** (revision_index > 0): fetches BOTH `prev` and
      `curr` Wayback HTMLs; diff is paragraph-aligned between them.

    * **chain_head** (the per-article MIN revision_index — Phase 133):
      fetches the chain's NEWEST snapshot's Wayback HTML AND the current
      Silver body; diff is "newest snapshot → current article", stored on
      the head row (which keeps its own archive_url). Combined with the
      mid-chain pairs this tessellates S0→…→Sn→current with no overlap, so
      `countIf(is_edit)` is the exact editorial-edit count. Makes every
      article with ≥ 1 Wayback snapshot diffable, not only chainLength ≥ 2.

    Fail-silent per pair: any single pair that fails (snapshot fetch
    error, trafilatura empty result, Silver miss) is logged and
    skipped; the sweep continues with the next pair. Empty diffs are
    written with the SENTINEL_IDENTICAL_OP marker (BUG-B) so the
    next sweep does not re-process them.

    Phase 122d.3 — when ``delta_tools`` (a ``RevisionDeltaTools`` bundle)
    is supplied, the sweep ALSO re-extracts the discourse shift for every
    pair that is a real edit, in the SAME pass (the two snapshot texts are
    already in hand — no second Wayback fetch). Deltas are written as 5
    extra columns. Identical re-archivals (the sentinel diff) and rows
    where the language is unknown get default deltas with
    ``deltas_computed=False`` so the BFF never aggregates them. The model
    pass is the per-tick cost lever: lower ``REVISION_DIFF_MAX_PAIRS_PER_TICK``
    if a tick approaches the (hourly) interval.
    """
    # Imported here rather than at module top so the corpus module
    # boots without the wayback dependency (legacy compatibility for
    # operators running the worker with WAYBACK_CDX_ENABLED=false).
    import json

    from internal.article_revisions_diff import (
        SENTINEL_IDENTICAL_OP,
        compute_diff,
        extract_paragraphs,
    )
    from internal.extractors.revision_deltas import (
        DeltaResult,
        compute_deltas,
        orient_pair,
    )
    from internal.wayback.snapshot_fetcher import FETCH_OK

    _sentinel_json = json.dumps(SENTINEL_IDENTICAL_OP, ensure_ascii=False)

    pairs = fetch_undiffed_pairs(ch_pool, max_pairs)
    if not pairs:
        return 0

    rows_to_write: list[list[object]] = []
    for pair in pairs:
        # Resolve the (prev_html, curr_html) the diff compares. The row that
        # gets WRITTEN always keeps the pair's own identity (incl. its own
        # `curr_archive_url`); only the FETCH target differs per kind.
        if pair["kind"] == "mid_chain":
            # prev = previous snapshot, curr = this snapshot.
            curr_result = snapshot_fetcher.fetch(pair["curr_archive_url"])
            if curr_result.status != FETCH_OK:
                continue
            prev_result = snapshot_fetcher.fetch(pair["prev_archive_url"])
            if prev_result.status != FETCH_OK:
                continue
            prev_html = prev_result.html
            curr_html = curr_result.html
        elif pair["kind"] == "chain_head":
            # Phase 133 — diff the current Silver body against the chain's
            # NEWEST snapshot (`compare_archive_url`), NOT the head row's own
            # (oldest) archive. The result is the `newest-snapshot → current`
            # transition, stored on the head row (which keeps its own
            # `curr_archive_url` for identity/link). prev = Silver-now, curr =
            # newest Wayback HTML; the L5 frontend labels it "latest snapshot
            # → current article".
            curr_result = snapshot_fetcher.fetch(pair["compare_archive_url"])
            if curr_result.status != FETCH_OK:
                continue
            silver_text = fetch_silver_body_for_article(
                ch_pool, minio_client, bucket, pair["article_id"]
            )
            if not silver_text:
                # Article has no Silver body (archived-only past the
                # analytical window, MinIO miss, etc.). Skip this tick;
                # next tick re-attempts. Do NOT write the sentinel — the
                # row is genuinely undiffed, not diffed-but-empty.
                continue
            prev_html = _silver_text_to_html(silver_text)
            curr_html = curr_result.html
        else:
            continue

        diff = compute_diff(prev_html, curr_html)

        # Phase 122d.3 — re-extract the discourse shift for real edits only.
        # `is_edit` mirrors the revision_count reconcile predicate: a non-
        # sentinel paragraph diff OR a headline change. Identical re-archivals
        # (~55% of pairs) keep default zero deltas with deltas_computed=False.
        deltas = DeltaResult()
        is_edit = bool(diff.headline_changed) or (
            bool(diff.diff_paragraphs) and diff.diff_paragraphs != [_sentinel_json]
        )
        if delta_tools is not None and is_edit:
            # Orient earliest→latest so every delta is later-minus-earlier.
            # The chain_head reversal lives in `orient_pair` (unit-pinned).
            older_html, newer_html = orient_pair(pair["kind"], prev_html, curr_html)
            try:
                older_text = "\n\n".join(extract_paragraphs(older_html))
                newer_text = "\n\n".join(extract_paragraphs(newer_html))
                deltas = compute_deltas(
                    older_text, newer_text, pair.get("language"), delta_tools
                )
            except Exception as exc:
                # Fail-silent: a model error must never lose the diff. The
                # row is still written with default (uncomputed) deltas; a
                # later re-extraction can fill them after a re-diff.
                logger.warning(
                    "revision_diff.deltas.failed",
                    article_id=pair["article_id"],
                    error=str(exc),
                    error_type=type(exc).__name__,
                )
                deltas = DeltaResult()

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
                deltas.sentiment_delta,
                deltas.entities_added,
                deltas.entities_removed,
                deltas.topic_shift_score,
                deltas.deltas_computed,
            ]
        )

    if not rows_to_write:
        return 0

    ch_pool.insert(
        "aer_gold.article_revisions",
        rows_to_write,
        column_names=ARTICLE_REVISIONS_COLUMNS_FULL,
    )

    # Phase 133 — recompute the editorial `revision_count` metric for every
    # article whose diffs we just (re)classified. The sweep is the SOLE
    # writer of revision_count (= editorial edits); the per-article capture
    # count is no longer written (article_revisions.upload_article_revisions).
    # Fail-silent: the diffs are already persisted, so a metric-write error
    # only delays the count by one tick.
    touched_ids = list({str(r[0]) for r in rows_to_write})
    try:
        metric_rows = write_editorial_revision_counts(ch_pool, touched_ids)
        logger.info(
            "revision_diff.revision_count.reconciled",
            articles=len(touched_ids),
            metric_rows=metric_rows,
        )
    except Exception as e:
        logger.error(
            "revision_diff.revision_count.failed",
            error=str(e),
            error_type=type(e).__name__,
        )
    return len(rows_to_write)


async def revision_diff_extraction_loop(
    ch_pool,
    snapshot_fetcher,
    minio_client,
    bucket: str,
    config: RevisionDiffConfig,
    stop_event: asyncio.Event,
    delta_tools=None,
    *,
    extraction_lock: asyncio.Lock | None = None,
) -> None:
    """Background task: every ``interval_seconds`` invoke a diff sweep.

    The minio client + bucket are forwarded to the sweep so the
    chain-head pair (BUG-11, revision_index=0) can pull the current
    Silver body for diffing against Wayback[0]. Mid-chain pairs
    ignore them.

    ``delta_tools`` (Phase 122d.3) is the load-once ``RevisionDeltaTools``
    bundle (sentiment + NER + E5 embedder) used to re-extract the
    discourse shift in the same pass as the diff. ``None`` disables the
    delta path (diffs still flow).

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
            async with (extraction_lock or nullcontext()):
                rows_written = await asyncio.to_thread(
                    run_revision_diff_sweep,
                    ch_pool,
                    snapshot_fetcher,
                    minio_client,
                    bucket,
                    config.max_pairs_per_tick,
                    delta_tools,
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
