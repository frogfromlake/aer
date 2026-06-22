"""Date-indexed HTML archive page as a *discovery channel* (Phase 122e A20 / F-A20).

Many publishers expose a per-day archive page parameterised by date —
for example ``tagesschau.de/archiv?datum=YYYY-MM-DD`` returns ≈ 140 article
URLs per day going back ≥ 4 years, exposing a deep historical archive that
the publisher's RSS top-stories feed (a sliding 70-item window) does not.

When the publisher exposes neither an XML sitemap (no ``Sitemap:``
directive in robots.txt and no ``/sitemap.xml``) nor a comprehensive RSS
catalogue, this channel is the methodologically-clean way to reach
historical content: the publisher built the date-indexed page deliberately,
parameterises it by date, and we ingest every article-shaped link verbatim.
No editorial filter on sections, topics, or article types beyond the
universal probe-level URL filter (which the article-fetch stage already
applies). This satisfies WP-006 §3 — *publisher* curation, never
researcher curation.

Politeness: one HTTP GET per day in the configured window. At a 1 s delay
a 5-year window costs ≈ 30 minutes for discovery alone; the resulting
article queue is fetched at the per-source politeness budget. Failures on
any single date (timeout, 4xx / 5xx, parse error) are logged and skipped —
a single bad day must not abort the entire backfill.

Cross-source uniformity: this module mirrors :mod:`internal.discovery.sitemap`
and :mod:`internal.discovery.rss_hint` — same ``since`` cutoff for temporal
symmetry, same ``DiscoveredUrl``-compatible ``(url, lastmod)`` return
shape. The caller in ``main.py`` treats all three channels symmetrically,
populating ``DiscoveredUrl.sitemap_lastmod`` from the date-indexed page's
date so URLs from this channel compete fairly in the newest-first sort.
"""

from __future__ import annotations

import logging
import re
import time
from datetime import datetime, timedelta, timezone
from typing import Any, Iterator, Optional, Tuple
from urllib.parse import urlparse

import requests

from . import ChannelStats, assert_pattern_usable


logger = logging.getLogger(__name__)

DEFAULT_DATE_FORMAT = "%Y-%m-%d"
DEFAULT_DELAY_SECONDS = 1.0
DEFAULT_TIMEOUT_SECONDS = 15.0
DEFAULT_USER_AGENT = "AerWebCrawler/0.1"

# Pre-compiled href extractor. The crawler avoids a heavy HTML parser
# dependency in this module — discovery only needs a flat sweep of href
# attributes from anchor tags. Edge cases (script-injected links, JS-
# rendered SPAs) are out of scope: the publisher chose to expose the
# date-indexed page as static HTML.
_HREF_RE = re.compile(r'href=["\']([^"\']+)["\']', re.IGNORECASE)


def discover(
    config: dict[str, Any],
    since: Optional[datetime] = None,
    *,
    user_agent: str = DEFAULT_USER_AGENT,
    delay_seconds: float = DEFAULT_DELAY_SECONDS,
    timeout_seconds: float = DEFAULT_TIMEOUT_SECONDS,
    now: Optional[datetime] = None,
    http_get=None,
    sleep=None,
    stats: Optional[ChannelStats] = None,
) -> Iterator[Tuple[str, Optional[datetime]]]:
    """Yield ``(url, published_date)`` pairs from a date-indexed archive page.

    Parameters
    ----------
    config:
        ``url_template`` (required) — a URL with a ``{date}`` placeholder,
        e.g. ``https://www.tagesschau.de/archiv?datum={date}``.
        ``date_format`` (optional, default ``%Y-%m-%d``) — strftime format
        the publisher accepts.
        ``granularity`` (optional, default ``"daily"``) — either
        ``"daily"`` (one HTTP fetch per day in the window) or
        ``"monthly"`` (one HTTP fetch per calendar month, with the cursor
        landing on the first of the month). Use ``"monthly"`` when the
        publisher's archive page returns the same content for every date
        within a calendar month — verify by sampling two distant dates
        in the same month and confirming identical URL lists. Probe-0's
        tagesschau is the canonical monthly case (page title:
        ``"Archiv - Inhalte vom Juni 2024"``).
        ``article_url_pattern`` (required) — a Python regex that matches
        article URLs. Links that don't match are dropped (filters out
        section landing pages, navigation, and asset URLs).
        ``base_url`` (optional) — base for resolving relative links; if
        omitted, derived from ``url_template``'s scheme + netloc.
    since:
        Discovery cutoff — dates older than ``since`` are not walked. When
        ``None``, walks back 5 years (matches Probe 0's
        ``time_window_days``).
    now:
        Test seam — defaults to ``datetime.now(tz=UTC)``.
    http_get / sleep:
        Test seams for the HTTP fetcher and the politeness sleep. The
        production defaults are ``requests.get`` and ``time.sleep``.

    Yields
    ------
    Each unique article URL the archive surfaced, paired with a UTC
    midnight datetime corresponding to the archive page's ``date``
    parameter. The caller uses this datetime to populate
    ``DiscoveredUrl.sitemap_lastmod`` so this channel's URLs sort by
    freshness alongside sitemap-discovered and RSS-discovered URLs.
    """
    template = (config or {}).get("url_template") or ""
    if not template or "{date}" not in template:
        return

    pattern = (config or {}).get("article_url_pattern") or ""
    # Phase 122g — same safety contract as html_sitemap: refuse to
    # silently skip the channel. The operator must see a loud error
    # rather than a zero-ingestion mystery.
    assert_pattern_usable(pattern, channel="archive_index", where=template)
    try:
        article_re = re.compile(pattern)
    except re.error as exc:
        from . import DiscoveryConfigurationError
        raise DiscoveryConfigurationError(
            f"archive_index `article_url_pattern` regex is invalid ({exc}). "
            f"url_template was {template!r}. Fix the regex in sources.yaml "
            "before re-running the crawler."
        ) from exc

    date_format = (config.get("date_format") or DEFAULT_DATE_FORMAT)
    granularity = (config.get("granularity") or "daily").lower()
    if granularity not in ("daily", "monthly"):
        logger.warning(
            "archive_index granularity must be 'daily' or 'monthly'; got %r",
            granularity,
        )
        return
    base_url = config.get("base_url") or _derive_base_url(template)

    end = now or datetime.now(tz=timezone.utc)
    if since is None:
        since = end - timedelta(days=1825)

    fetch = http_get or requests.get
    polite_sleep = sleep or time.sleep

    headers = {"User-Agent": user_agent}

    yielded: set[str] = set()

    if granularity == "monthly":
        # Cursor lands on the FIRST of each month and steps backward one
        # calendar month per iteration. The publisher's monthly archive
        # endpoint returns the same content for every date within a
        # calendar month, so daily walking is wasteful (~ 30× redundant
        # fetches) AND would stamp every URL in the month with the
        # most-recent day-walked date — distorting the newest-first sort.
        cursor = end.date().replace(day=1)
        earliest = since.date().replace(day=1)
    else:
        cursor = end.date()
        earliest = since.date()

    while cursor >= earliest:
        date_str = cursor.strftime(date_format)
        url = template.replace("{date}", date_str)
        try:
            resp = fetch(url, headers=headers, timeout=timeout_seconds)
        except Exception as exc:
            logger.warning("archive_index fetch failed: %s — %s", url, exc)
            # Phase 148d — a transport failure loses a date page's worth of
            # in-window listings: the declared count becomes a lower bound
            # (WP-007 §5). A 4xx (below) is the publisher saying "no archive
            # page for this date" — a legitimate empty, not lost data.
            if stats is not None:
                stats.mark_indeterminate()
            cursor = _step_back(cursor, granularity)
            polite_sleep(delay_seconds)
            continue

        status = getattr(resp, "status_code", 0)
        if status != 200:
            logger.info(
                "archive_index non-200; skipping %s (url=%s, status=%s)",
                cursor,
                url,
                status,
            )
            # 5xx is a server-side failure (lost listings → lower bound);
            # 4xx is a legitimate empty date and does not taint the count.
            if status >= 500 and stats is not None:
                stats.mark_indeterminate()
            cursor = _step_back(cursor, granularity)
            polite_sleep(delay_seconds)
            continue

        # The article's `published_date` is set by the worker's WebAdapter
        # from JSON-LD / meta tags. The archive-page date is a *hint* — it
        # populates `DiscoveredUrl.sitemap_lastmod` so the newest-first
        # sort works, identical to the role `sitemap.last_modified` plays
        # for sitemap discovery. For monthly granularity we use the
        # first-of-month date as the bucket — articles published mid-
        # month sort alongside the rest of the month, which is the
        # publisher's own granularity choice.
        entry_dt = datetime(cursor.year, cursor.month, cursor.day, tzinfo=timezone.utc)

        html = getattr(resp, "text", "") or ""
        for match in _HREF_RE.finditer(html):
            href = match.group(1).strip()
            absolute = _absolute_url(href, base_url)
            if not absolute:
                continue
            if not article_re.search(absolute):
                continue
            if absolute in yielded:
                continue
            yielded.add(absolute)
            # Phase 148d — declared denominator (WP-007 §4.1, Decision B):
            # count the article-pattern links listed on the date pages we
            # walk anyway, before AĒR's cross-channel dedup/filters. Zero
            # extra fetches — the polite, measured floor for a paginated
            # archive that has no single advertised total.
            if stats is not None:
                stats.count()
            yield absolute, entry_dt

        cursor = _step_back(cursor, granularity)
        polite_sleep(delay_seconds)


def _step_back(cursor, granularity: str):
    """Return the previous cursor date according to the configured granularity."""
    if granularity == "monthly":
        # Step back one calendar month, keeping cursor on the first of the month.
        if cursor.month == 1:
            return cursor.replace(year=cursor.year - 1, month=12, day=1)
        return cursor.replace(month=cursor.month - 1, day=1)
    return cursor - timedelta(days=1)


def _derive_base_url(template: str) -> str:
    parsed = urlparse(template)
    if not parsed.scheme or not parsed.netloc:
        return ""
    return f"{parsed.scheme}://{parsed.netloc}"


def _absolute_url(href: str, base_url: str) -> str:
    if not href:
        return ""
    if href.startswith("http://") or href.startswith("https://"):
        return href
    if href.startswith("//"):
        scheme = "https" if base_url.startswith("https") else "http"
        return f"{scheme}:{href}"
    if href.startswith("/"):
        if not base_url:
            return ""
        return base_url + href
    return ""  # purely relative paths — skip
