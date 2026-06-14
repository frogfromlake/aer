from __future__ import annotations

import os
import time
from dataclasses import dataclass, field

import structlog


logger = structlog.get_logger()

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
    # Phase 122d.3 — Silent-Edit Discourse Shift deltas.
    "sentiment_delta",
    "entities_added",
    "entities_removed",
    "topic_shift_score",
    "deltas_computed",
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
        default_factory=lambda: (
            os.getenv("REVISION_DIFF_EXTRACTION_ENABLED", "false").lower() == "true"
        )
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
        default_factory=lambda: int(
            os.getenv("REVISION_DIFF_MAX_PAIRS_PER_TICK", "200")
        )
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
                prev.archive_url           AS prev_archive_url,
                lang.language              AS language
            FROM aer_gold.article_revisions AS curr FINAL
            INNER JOIN aer_gold.article_revisions AS prev FINAL
                ON prev.article_id     = curr.article_id
               AND prev.revision_index = curr.revision_index - 1
            LEFT JOIN (
                SELECT article_id, argMax(language, ingestion_version) AS language
                FROM aer_silver.documents
                GROUP BY article_id
            ) AS lang
                ON lang.article_id = curr.article_id
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
                "language": row[12],
            }
            for row in mid_result.result_rows
        ]

        remaining = max(0, int(limit) - len(pairs))
        if remaining <= 0:
            return pairs

        # Chain-head pair (Phase 133 — anchored on the chain's NEWEST
        # snapshot, not the oldest). The head ROW is the per-article MINIMUM
        # revision_index — the only row without a mid-chain predecessor, so
        # its diff slot is free — robust to offset chains (e.g. 4..17 from
        # ADR-036 rebuilds), never a literal revision_index = 0. The
        # COMPARISON is the current Silver body vs the NEWEST snapshot
        # (max snapshot_at). The head row thus carries the
        # `newest-snapshot → current` transition; together with the
        # mid-chain pairs (S0→S1 … S_{n-1}→Sn) it tessellates the full
        # timeline S0→…→Sn→current with NO overlap, so `countIf(is_edit)`
        # over the rows is the EXACT editorial-edit count — no double-count
        # of a cumulative diff, and 1-snapshot articles still get their
        # single current-vs-snapshot edit. The head row keeps its OWN
        # archive_url (identity / playback link); `compare_archive_url` is
        # the newest snapshot we actually fetch + diff against.
        head_result = client.query(
            """
            SELECT
                r.article_id, r.source, r.discourse_function, r.snapshot_at,
                r.content_hash, r.prev_content_hash, r.revision_index,
                r.time_since_prev_hours, r.revision_trigger,
                r.ingestion_version, r.archive_url,
                n.newest_archive AS compare_archive_url,
                lang.language AS language
            FROM aer_gold.article_revisions AS r FINAL
            INNER JOIN (
                SELECT article_id, min(revision_index) AS head_index
                FROM aer_gold.article_revisions FINAL
                GROUP BY article_id
            ) AS h
                ON h.article_id = r.article_id
               AND h.head_index = r.revision_index
            INNER JOIN (
                SELECT article_id, argMax(archive_url, snapshot_at) AS newest_archive
                FROM aer_gold.article_revisions FINAL
                WHERE archive_url != ''
                GROUP BY article_id
            ) AS n
                ON n.article_id = r.article_id
            LEFT JOIN (
                SELECT article_id, argMax(language, ingestion_version) AS language
                FROM aer_silver.documents
                GROUP BY article_id
            ) AS lang
                ON lang.article_id = r.article_id
            WHERE r.revision_trigger = 'cdx_snapshot'
              AND length(r.diff_paragraphs) = 0
              AND r.archive_url != ''
            ORDER BY r.snapshot_at DESC
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
                    # The head row's OWN archive_url — written back unchanged
                    # so the row keeps its identity / playback link.
                    "curr_archive_url": row[10],
                    # The NEWEST snapshot's archive — what we fetch and diff
                    # the current Silver body against (Phase 133 re-anchor).
                    "compare_archive_url": row[11],
                    # No prev_archive_url for chain-head — the "before" side
                    # is the current Silver body, fetched at sweep time.
                    "prev_archive_url": "",
                    "language": row[12],
                }
            )
        return pairs
    finally:
        ch_pool.putconn(client)


def fetch_silver_body_for_article(
    ch_pool, minio_client, bucket: str, article_id: str
) -> str:
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
    "current Silver". `compute_diff` only asserts a headline change
    when BOTH sides carry a real title, so a chain-head pair (whose
    `prev` is this title-less wrapper) never reports a headline
    change even if the title actually drifted. This is the structural
    guarantee behind the headline rule in `article_revisions_diff.compute_diff`.
    """
    paragraphs = cleaned_text.split("\n\n")
    body = "".join(f"<p>{p.strip()}</p>" for p in paragraphs if p.strip())
    return f"<!DOCTYPE html><html><body>{body}</body></html>"


# Phase 133 — `revision_count` is the count of EDITORIAL edits, not Wayback
# captures. `is_edit` = the pair has a computed diff that is NOT the identical
# re-archival sentinel (pending/empty diffs are not edits). The published
# timestamp comes from `aer_silver.documents`; if that row is gone (TTL) we
# fall back to the latest snapshot so the metric is never anchored at the 1970
# epoch (which would TTL-prune it immediately).
_REVISION_COUNT_RECONCILE_QUERY = """
    SELECT
        ar.article_id AS article_id,
        any(ar.source) AS source,
        any(ar.discourse_function) AS discourse_function,
        toFloat64(countIf(
            (length(ar.diff_paragraphs) > 0
             AND NOT arrayExists(x -> JSONExtractString(x, 'op') = 'identical', ar.diff_paragraphs))
            OR ar.headline_changed
        )) AS editorial_edits,
        multiIf(
            any(d.timestamp) > toDateTime('2000-01-01 00:00:00'), any(d.timestamp),
            max(ar.snapshot_at)
        ) AS ts
    FROM aer_gold.article_revisions AS ar FINAL
    LEFT JOIN aer_silver.documents AS d FINAL ON d.article_id = ar.article_id
    WHERE ar.article_id IN {ids:Array(String)}
    GROUP BY ar.article_id
"""


def write_editorial_revision_counts(ch_pool, article_ids: list[str]) -> int:
    """Recompute and upsert `revision_count` = editorial-edit count for the
    given articles (Phase 133). Returns the number of metric rows written.

    The editorial-edit count is derived from `aer_gold.article_revisions`
    (the authoritative revision record) as the number of pairs whose diff is
    a real change — excluding the identical re-archival sentinel and
    not-yet-diffed pending pairs. `aer_gold.metrics` is
    ReplacingMergeTree(ingestion_version) ORDER BY (article_id, metric_name),
    so a fresh (higher) ingestion_version REPLACES the prior value by key —
    the metric is edits-only and converges upward as the sweep classifies
    more pairs. This is the SOLE writer of `revision_count`; no capture count
    is written anywhere (see `article_revisions.upload_article_revisions`).
    """
    if not article_ids:
        return 0
    client = ch_pool.getconn()
    try:
        result = client.query(
            _REVISION_COUNT_RECONCILE_QUERY,
            parameters={"ids": list(article_ids)},
        )
    finally:
        ch_pool.putconn(client)

    version = time.time_ns()
    rows = [
        # column order: timestamp, value, source, metric_name, article_id,
        # discourse_function, ingestion_version, timestamp_source
        [
            row[4],
            float(row[3]),
            row[1],
            "revision_count",
            row[0],
            row[2] or "",
            version,
            "",
        ]
        for row in result.result_rows
    ]
    if not rows:
        return 0
    ch_pool.insert(
        "aer_gold.metrics",
        rows,
        column_names=[
            "timestamp",
            "value",
            "source",
            "metric_name",
            "article_id",
            "discourse_function",
            "ingestion_version",
            "timestamp_source",
        ],
    )
    return len(rows)
