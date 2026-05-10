"""RSS feed parsing as a *discovery channel* (Phase 122 / Phase 122e).

The crawler never uses the RSS body. The feed's ``<link>`` URLs surface
freshly-published articles, and for some publishers (Probe 0's
bundesregierung.de in particular) the RSS feed is the **only** channel
that exposes actual news content — the public sitemap exposes only
service/archive pages. Phase 122e elevates RSS from "hint only" to a
peer-equal discovery channel by returning each entry's ``published_parsed``
timestamp alongside its URL, so the caller can populate the
``DiscoveredUrl.sitemap_lastmod`` field and let RSS-discovered URLs
compete fairly in the newest-first sort. Items already visible in the
sitemap are deduplicated by the caller; if the same URL is in both
channels, the sitemap entry wins (it carries the canonical lastmod and
the sitemap_section context).

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
from typing import Iterator, Optional, Tuple


logger = logging.getLogger(__name__)


def discover(
    rss_url: str, since: Optional[datetime] = None
) -> Iterator[Tuple[str, Optional[datetime]]]:
    """Yield ``(url, published_at)`` pairs surfaced by the RSS feed.

    ``published_at`` is the UTC datetime parsed from the entry's
    ``published_parsed`` (or ``updated_parsed`` fallback) when feedparser
    could resolve it; ``None`` otherwise. The caller uses this to
    populate ``DiscoveredUrl.sitemap_lastmod`` so RSS-discovered URLs
    sort by freshness alongside sitemap-discovered URLs (Phase 122e).

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
        entry_dt = _entry_datetime(entry)
        if since is not None and entry_dt is not None and entry_dt < since:
            continue
        yield url, entry_dt


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
