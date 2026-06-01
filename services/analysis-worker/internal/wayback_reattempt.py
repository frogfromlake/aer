"""
Wayback CDX re-attempt task (ADR-036) — the first registered `ReAttemptTask`.

Re-calls the Internet Archive CDX API for articles whose ingest-time lookup did
NOT complete (`aer_gold.wayback_lookups` status ∉ {ok, no_snapshots}) and, on
success, writes the `cdx_snapshot` revisions + refreshes the lookup status. A
transient IA outage therefore self-heals on a later tick instead of leaving a
permanent, silent gap — no re-crawl required.

This is the **re-call-external** flavour: it needs only the stored
`canonical_url`, no Bronze read. `discourse_function` is left '' on the revision
rows — the BFF resolves DF by source→probe at query time (migration 023) — and
the sitemap-based `republication_trigger` rows are written at ingest,
independent of the Archive, so they are not this task's concern.
"""

from __future__ import annotations

import time
from collections import Counter
from datetime import datetime, timezone
from types import SimpleNamespace

import structlog

from internal.adapters.web_meta import WebMeta
from internal.article_revisions import upload_article_revisions
from internal.wayback.client import WaybackCDXClient
from internal.wayback_lookups import COMPLETED_STATUSES, upload_wayback_lookup

logger = structlog.get_logger()


class WaybackReAttemptTask:
    """ADR-036 re-attempt task for the per-article Wayback CDX lookup."""

    name = "wayback"

    def __init__(self, ch_pool, wayback_client: WaybackCDXClient) -> None:
        self._pool = ch_pool
        self._client = wayback_client

    def run(self, limit: int) -> Counter:
        """Re-attempt up to `limit` incomplete lookups; return outcome counts.

        The raw pool connection is held only for the (fast) discovery query.
        The per-item writes then go through ``self._pool.insert`` — the pool
        WRAPPER, which is the only insert path that accepts
        ``deduplication_token`` (it forwards it as the
        ``insert_deduplication_token`` setting). The raw client from
        ``getconn`` rejects that kwarg, so it is used for the SELECT only. The
        wrapper does its own getconn/putconn per insert, so the slow CDX HTTP
        calls in the loop never pin a pool slot."""
        client = self._pool.getconn()
        try:
            items = self._find_incomplete(client, limit)
        finally:
            self._pool.putconn(client)
        outcomes: Counter = Counter()
        for item in items:
            outcomes[self._reattempt_one(item)] += 1
        return outcomes

    def _find_incomplete(self, client, limit: int) -> list[dict]:
        # FINAL collapses ReplacingMergeTree to the latest row per
        # (source, article_id); only the non-completed ones with a usable
        # canonical_url can be re-attempted.
        result = client.query(
            """
            SELECT source, article_id, canonical_url, status
            FROM aer_gold.wayback_lookups FINAL
            WHERE status NOT IN ('ok', 'no_snapshots') AND canonical_url != ''
            LIMIT %(limit)s
            """,
            parameters={"limit": int(limit)},
        )
        return [
            {"source": r[0], "article_id": r[1], "canonical_url": r[2], "prev_status": r[3]}
            for r in result.result_rows
        ]

    def _reattempt_one(self, item: dict) -> str:
        url = item["canonical_url"]
        res = self._client.lookup(url)
        # time_ns() at re-attempt time is reliably greater than the ingest-time
        # version (a past event timestamp) and any prior re-attempt → the
        # ReplacingMergeTree rows replace correctly.
        version = time.time_ns()
        now = datetime.now(timezone.utc)
        core = SimpleNamespace(source=item["source"], document_id=item["article_id"])
        meta = WebMeta(
            source_type="web",
            canonical_url=url,
            wayback_revisions=[rev.to_dict() for rev in res.revisions],
            wayback_lookup_status=res.status,
        )
        # Writes go through the pool WRAPPER (self._pool), never the raw
        # getconn'd client: only the wrapper's insert() accepts
        # deduplication_token. Always refresh the completeness status — records
        # THIS attempt's outcome (a still-failing lookup keeps a current
        # timestamp and is re-attempted next tick; a recovered one flips to
        # ok/no_snapshots and drops out of `_find_incomplete`).
        upload_wayback_lookup(self._pool, core, meta, version, now)
        if res.status in COMPLETED_STATUSES:
            # `no_snapshots` → build yields no rows (empty revisions); `ok` →
            # writes the cdx_snapshot chain. Both are now durably recorded.
            upload_article_revisions(
                self._pool, core, meta, discourse_function="", ingestion_version=version
            )
        return res.status
