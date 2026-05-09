"""RSS feed parsing as a *discovery hint only* (Phase 122).

The crawler never uses the RSS body. The feed's ``<link>`` URLs surface
freshly-published articles before the next sitemap refresh; the article
body itself is always fetched from the HTML source. Items already
visible in the sitemap are deduplicated by the caller.

Phase 122b — temporal symmetry. When ``since`` is supplied, entries
older than the cutoff are dropped via the feed entry's
``published_parsed`` (a ``time.struct_time``). Entries without a
parsable date fall through (RSS entries are nearly always recent, so
the filter is largely defensive — it catches archive-only feeds that
expose old items as fresh).
"""

from __future__ import annotations

import calendar
import logging
from datetime import datetime, timezone
from typing import Iterator, Optional


logger = logging.getLogger(__name__)


def discover(rss_url: str, since: Optional[datetime] = None) -> Iterator[str]:
    """Yield article URLs surfaced by the RSS feed.

    ``feedparser`` is imported lazily so the module loads in the test
    suite without the dependency present.
    """
    if not rss_url:
        return
    try:
        import feedparser  # type: ignore
    except Exception as exc:  # pragma: no cover - import-shim
        logger.warning("feedparser not installed: %s", exc)
        return

    try:
        feed = feedparser.parse(rss_url)
    except Exception as exc:
        logger.warning("Failed to parse RSS feed %s: %s", rss_url, exc)
        return

    for entry in feed.entries:
        url = entry.get("link") or ""
        if not url:
            continue
        if since is not None:
            entry_dt = _entry_datetime(entry)
            if entry_dt is not None and entry_dt < since:
                continue
        yield url


def _entry_datetime(entry: object) -> Optional[datetime]:
    """Return the entry's published or updated time as an aware UTC
    datetime, or ``None`` if neither is parsable. ``feedparser`` exposes
    ``published_parsed`` / ``updated_parsed`` as ``time.struct_time`` in
    UTC.
    """
    for attr in ("published_parsed", "updated_parsed"):
        struct = entry.get(attr) if isinstance(entry, dict) else getattr(entry, attr, None)
        if struct is None:
            continue
        try:
            return datetime.fromtimestamp(calendar.timegm(struct), tz=timezone.utc)
        except (TypeError, ValueError, OverflowError):
            continue
    return None
