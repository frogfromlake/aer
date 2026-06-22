"""Silent-Edit Observability Gold-row writer — Phase 122d.0 (ADR-032).

Sibling of `internal.metadata_coverage` and `internal.silver_projection`:
the processor calls this module after Silver is uploaded, and it
projects the per-article Wayback CDX result PLUS the publisher-side
republication-trigger signal into one row per detected revision in
`aer_gold.article_revisions`.

Two revision-trigger families
-----------------------------

* ``cdx_snapshot`` — one row per Wayback CDX snapshot. The
  `WaybackCDXClient` deduplicates consecutive identical digests via
  the CDX `collapse=digest` parameter, so each row is a real content
  transition. `prev_content_hash` + `time_since_prev_hours` describe
  the chain.

* ``republication_trigger`` — Phase 131a artefact reconciled in 122d.0.
  When the publisher's sitemap-lastmod is significantly newer than the
  article's `published_date`, the publisher re-listed an old article.
  That mechanism is itself a silent-edit signal: the dashboard surface
  should treat the re-list as a first-class revision event, not as a
  "stale article slipping past the time window" bug. We emit one
  synthetic row whose content-hash is the SHA-1 of the current
  cleaned_text — future CDX snapshots can compare against it without a
  special case.

The two families coexist for one article: a publisher may both bump
sitemap-lastmod AND have Wayback snapshots. Both rows belong to the
same `article_id` key; the dashboard renders them together as the
revision chain.
"""

from __future__ import annotations

import os
from datetime import datetime, timedelta, timezone
from typing import Iterable, Optional

import structlog

from internal.adapters.web_meta import WebMeta
from internal.wayback.client import synthesise_republication_hash

logger = structlog.get_logger()

# Phase 148c — cap the per-article revision chain. A live-ticker / continuously
# updated page (stable URL, ever-changing content — weather tickers, live-blogs)
# accumulates dozens-to-hundreds of distinct Wayback CDX content-versions;
# backfilling and then diffing/enriching every one costs CPU for zero analytical
# value (it is not discourse). Past this cap we keep the chronologically-first N
# and stop — the article then surfaces as the `live_ticker` Negative-Space class
# in the UI (revision count >= the matching frontend floor) and is excluded from
# the analytical reading rather than silently dropped (DISCLOSE-NEVER-COERCE,
# WP-007 §4.3). Provisional engineering default; real articles in the 2026-06-21
# validation topped out at 8 revisions, so 20 leaves ample headroom.
MAX_CDX_REVISIONS_PER_ARTICLE = int(os.getenv("MAX_CDX_REVISIONS_PER_ARTICLE", "20"))

ARTICLE_REVISIONS_TABLE = "aer_gold.article_revisions"

# When the publisher's sitemap-lastmod is at least this far ahead of the
# article's `published_date`, the worker emits a republication-trigger
# row. 7 days is conservative — a small sitemap-lastmod jitter (cache
# warm-up, sitemap rewrite) does not fire; only meaningful re-list
# events do. Operator-tunable via env so probe-specific calibration
# (newsroom A re-lists hourly; newsroom B never) does not require a
# code change.
REPUBLICATION_TRIGGER_MIN_DELTA_DAYS = 7


def _republication_min_delta() -> timedelta:
    raw = os.getenv("REPUBLICATION_TRIGGER_MIN_DELTA_DAYS", "").strip()
    if not raw:
        return timedelta(days=REPUBLICATION_TRIGGER_MIN_DELTA_DAYS)
    try:
        days = int(raw)
    except (TypeError, ValueError):
        return timedelta(days=REPUBLICATION_TRIGGER_MIN_DELTA_DAYS)
    if days < 1:
        return timedelta(days=REPUBLICATION_TRIGGER_MIN_DELTA_DAYS)
    return timedelta(days=days)


def _normalise_dt(value: Optional[datetime]) -> Optional[datetime]:
    if value is None:
        return None
    if value.tzinfo is None:
        return value.replace(tzinfo=timezone.utc)
    return value.astimezone(timezone.utc)


def _detect_republication(meta: WebMeta, article_id: str) -> Optional[dict]:
    """Return a synthetic republication-trigger row when one is warranted.

    The detection logic is intentionally narrow:

      * `published_date` and `sitemap_lastmod` must both be set —
        without both, no delta is computable.
      * `sitemap_lastmod` - `published_date` ≥ the configured floor.

    A publisher-side re-list with no `published_date` (the publisher
    only emits sitemap-lastmod and we never resolved an authoritative
    publication date) does NOT fire this trigger — that case is already
    visible to operators via `timestamp_source = 'fetch_at_fallback'`
    and surfaces in the metadata-coverage panel.

    The synthetic content_hash is a deterministic trigger-identity
    digest (see `synthesise_republication_hash`); it stays stable
    across Bronze→Silver replays so re-processing the same article
    after a parser upgrade collapses on the ReplacingMergeTree key
    instead of producing a second row.
    """
    pub = _normalise_dt(meta.published_date)
    lastmod = _normalise_dt(meta.sitemap_lastmod)
    if pub is None or lastmod is None:
        return None
    if lastmod - pub < _republication_min_delta():
        return None
    return {
        "snapshot_at": lastmod,
        "content_hash": synthesise_republication_hash(article_id, lastmod.isoformat()),
        "trigger": "republication_trigger",
    }


def _build_chain(
    raw_revisions: Iterable[dict],
    republication: Optional[dict],
) -> list[dict]:
    """Compose the ordered revision chain from raw CDX entries + the optional
    republication-trigger pseudo-revision.

    The CDX client emits `wayback_revisions[]` as a list of plain dicts
    on the WebMeta envelope (we serialise via `WaybackRevision.to_dict`).
    We deduplicate by `content_hash` while preserving chronological
    order — the CDX `collapse=digest` projection already does this on
    the network side, but a robust local guard is cheap and keeps the
    invariant readable.
    """
    chain: list[dict] = []
    seen_hashes: set[str] = set()

    if republication is not None:
        chain.append(republication)
        seen_hashes.add(republication["content_hash"])

    for entry in raw_revisions:
        if not isinstance(entry, dict):
            continue
        snapshot_raw = entry.get("snapshot_at")
        content_hash = entry.get("content_hash") or ""
        if not content_hash or content_hash in seen_hashes:
            continue
        snapshot_at = _parse_iso(snapshot_raw)
        if snapshot_at is None:
            continue
        chain.append(
            {
                "snapshot_at": snapshot_at,
                "content_hash": str(content_hash),
                # Phase 122d.1: carry the archive_url forward so the Gold
                # row has everything the Phase-122d.1 sweep loop needs to
                # fetch the snapshot HTML without a second round-trip to
                # CDX or to Silver MinIO. Empty string for the
                # republication-trigger pseudo-revisions — they have no
                # IA archive page (it is a publisher-side re-list event,
                # not a third-party witness).
                "archive_url": str(entry.get("archive_url") or ""),
                "trigger": "cdx_snapshot",
            }
        )
        seen_hashes.add(content_hash)

    chain.sort(key=lambda r: r["snapshot_at"])
    # Phase 148c — bound a live-ticker's chain (see MAX_CDX_REVISIONS_PER_ARTICLE).
    if len(chain) > MAX_CDX_REVISIONS_PER_ARTICLE:
        logger.info(
            "article_revisions.chain_capped",
            kept=MAX_CDX_REVISIONS_PER_ARTICLE,
            dropped=len(chain) - MAX_CDX_REVISIONS_PER_ARTICLE,
        )
        chain = chain[:MAX_CDX_REVISIONS_PER_ARTICLE]
    return chain


def _parse_iso(value: object) -> Optional[datetime]:
    if not isinstance(value, str):
        return None
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None
    if parsed.tzinfo is None:
        return parsed.replace(tzinfo=timezone.utc)
    return parsed.astimezone(timezone.utc)


def build_revision_rows(
    *,
    article_id: str,
    source: str,
    discourse_function: str,
    meta: WebMeta,
    ingestion_version: int,
) -> list[list[object]]:
    """Construct the (article_id, source, …) rows for `article_revisions`.

    Returns an empty list when the article has no detectable revisions
    — the processor skips the insert in that case, so an article with
    `wayback_lookup_status='no_snapshots'` and no republication trigger
    produces ZERO rows (not a sentinel row).
    """
    republication = _detect_republication(meta, article_id)
    chain = _build_chain(meta.wayback_revisions or [], republication)
    if not chain:
        return []

    rows: list[list[object]] = []
    prev_hash = ""
    prev_at: Optional[datetime] = None
    for idx, rev in enumerate(chain):
        snapshot_at = rev["snapshot_at"]
        time_since_prev = 0.0
        if prev_at is not None:
            time_since_prev = (snapshot_at - prev_at).total_seconds() / 3600.0
        rows.append(
            [
                article_id,
                source,
                discourse_function or "",
                snapshot_at,
                rev["content_hash"],
                prev_hash,
                idx,
                time_since_prev,
                rev["trigger"],
                ingestion_version,
                # Phase 122d.1 — archive_url column (migration 000024).
                # Empty string for republication-trigger rows.
                rev.get("archive_url", ""),
            ]
        )
        prev_hash = rev["content_hash"]
        prev_at = snapshot_at
    return rows


def upload_article_revisions(
    ch_client,
    core,
    meta,
    discourse_function: str,
    ingestion_version: int,
) -> None:
    """Insert revision rows for one Silver write.

    Only `WebMeta` envelopes carry the silent-edit provenance. RSS /
    legacy `SilverMeta` subclasses have no `wayback_revisions` field,
    so the function silently no-ops there — Phase 122d.0 is web-only
    by design (the Wayback signal is meaningful for HTML archive
    captures, not for RSS snippets).

    Failure mode mirrors `metadata_coverage`: a missing row only
    impacts the silent-edit panel; the canonical Silver record is the
    MinIO envelope and the rest of the Gold pipeline must not be
    affected.
    """
    if not isinstance(meta, WebMeta):
        return

    try:
        rows = build_revision_rows(
            article_id=core.document_id,
            source=core.source,
            discourse_function=discourse_function,
            meta=meta,
            ingestion_version=ingestion_version,
        )
        if not rows:
            return
        ch_client.insert(
            ARTICLE_REVISIONS_TABLE,
            rows,
            column_names=[
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
            ],
            deduplication_token=f"{ARTICLE_REVISIONS_TABLE}:{core.document_id}:{ingestion_version}",
        )
        logger.info(
            "Article revisions captured",
            source=core.source,
            article_id=core.document_id,
            capture_count=len(rows),
            wayback_status=getattr(meta, "wayback_lookup_status", ""),
        )
        # Phase 133: the `revision_count` Gold metric is NO LONGER the chain
        # length (captures). It is now the count of EDITORIAL edits, written
        # by the revision-diff sweep (`corpus.run_revision_diff_sweep` →
        # `write_editorial_revision_counts`) once it has classified each
        # pair's diff. Writing the capture count here would re-introduce the
        # Wayback-archival-frequency artefact the metric was redefined to
        # exclude. This function now ONLY persists the raw capture chain to
        # `aer_gold.article_revisions` (the observation/evidence record); the
        # editorial metric is derived from it downstream.
    except Exception as exc:
        logger.error(
            "Article revisions insert failed; silent-edit panel will be missing this article.",
            source=core.source,
            article_id=core.document_id,
            error=str(exc),
        )
