"""Postgres writer for the Phase 148d (WP-007 §5) per-source crawl funnel.

Records one row per source per crawl run in ``crawler_funnel_runs``,
capturing the drop at each spider stage so an unexplained per-source
conversion gap (e.g. franceinfo's discovered→Gold ratio versus
tagesschau's) is attributable stage-by-stage instead of left as a silent
artifact (WP-007 §2).

The funnel's channel-attributable head (declared → discovered →
after_dedup) lives in ``crawler_discovery_runs`` (per channel); once URLs
are merged into one crawl list the channel a URL came from is gone, so the
spider stages here are PER SOURCE (WP-007 Decision A). The tail
(extracted → Gold) is reconciled at BFF read-time against ClickHouse.

The row-construction logic (:func:`build_funnel_record`) is a pure
function so it is unit-testable without Scrapy installed; the Scrapy
spider's ``closed`` handler is a thin shim that calls it.

Test seam: pass an alternative pool to inject a fake connection.
"""

from __future__ import annotations

import logging
import uuid
from dataclasses import dataclass
from datetime import datetime
from typing import Any, Mapping

logger = logging.getLogger(__name__)


@dataclass(frozen=True)
class FunnelRunRecord:
    """One source's per-run collection funnel (WP-007 §5).

    ``discovered`` is the after-dedup input the discovery layer handed the
    spider; the remaining fields are the attributable drops/outcomes. They
    are not a strict arithmetic identity (Scrapy retries/redirects blur the
    edges) but are attributable enough to localise *where* a source loses
    documents — which is the whole point of the funnel.
    """

    source_id: int
    discovered: int
    url_filtered: int
    already_collected: int
    fetched: int
    not_modified: int
    content_dropped: int
    thin_content_dropped: int
    submitted: int
    errored: int
    run_started_at: datetime
    run_completed_at: datetime


_FUNNEL_COUNTER_FIELDS = (
    "discovered",
    "url_filtered",
    "already_collected",
    "fetched",
    "not_modified",
    "content_dropped",
    "thin_content_dropped",
    "submitted",
    "errored",
)


def funnel_counters_from(obj: Any) -> dict[str, int]:
    """Extract the funnel counters from a spider-like object (duck-typed).

    Scrapy-free so the spider's ``closed`` handler stays a thin shim while
    the attribute mapping is covered here. A missing attribute reads as 0.
    """
    return {field: int(getattr(obj, field, 0) or 0) for field in _FUNNEL_COUNTER_FIELDS}


def build_funnel_record(
    *,
    source_id: int,
    counters: Mapping[str, Any],
    run_started_at: datetime,
    run_completed_at: datetime,
) -> FunnelRunRecord:
    """Build a :class:`FunnelRunRecord` from a spider's counter mapping.

    Pure + Scrapy-free so the spider's ``closed`` handler stays a thin,
    untested shim while the field mapping is covered here. Missing keys
    default to 0 so a spider that never reached a stage records a true
    zero, not a crash.
    """

    def _c(key: str) -> int:
        return int(counters.get(key, 0) or 0)

    return FunnelRunRecord(
        source_id=source_id,
        discovered=_c("discovered"),
        url_filtered=_c("url_filtered"),
        already_collected=_c("already_collected"),
        fetched=_c("fetched"),
        not_modified=_c("not_modified"),
        content_dropped=_c("content_dropped"),
        thin_content_dropped=_c("thin_content_dropped"),
        submitted=_c("submitted"),
        errored=_c("errored"),
        run_started_at=run_started_at,
        run_completed_at=run_completed_at,
    )


class FunnelRunsWriter:
    """Postgres writer for ``crawler_funnel_runs``.

    Wraps the connection pool already held by
    :class:`internal.state.dedup.CrawlerState`. One INSERT per source per
    crawl run, issued from the spider's ``closed`` handler.
    """

    def __init__(self, pool):
        self._pool = pool

    def record_run(self, record: FunnelRunRecord) -> uuid.UUID:
        """Insert one funnel row; return the generated run_id."""
        run_id = uuid.uuid4()
        conn = self._pool.getconn()
        try:
            with conn:
                with conn.cursor() as cur:
                    cur.execute(
                        """
                        INSERT INTO crawler_funnel_runs
                            (run_id, source_id, discovered, url_filtered,
                             already_collected, fetched, not_modified,
                             content_dropped, thin_content_dropped, submitted,
                             errored, run_started_at, run_completed_at)
                        VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                        """,
                        (
                            str(run_id),
                            record.source_id,
                            record.discovered,
                            record.url_filtered,
                            record.already_collected,
                            record.fetched,
                            record.not_modified,
                            record.content_dropped,
                            record.thin_content_dropped,
                            record.submitted,
                            record.errored,
                            record.run_started_at,
                            record.run_completed_at,
                        ),
                    )
        finally:
            self._pool.putconn(conn)
        return run_id
