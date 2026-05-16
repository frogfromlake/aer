"""Publisher-built HTML sitemap page as a *discovery channel* (Phase 122g).

Some publishers expose a navigation page that lists the current article
set in HTML (not XML) at a non-standard URL. Example: tagesschau.de
publishes a daily-refreshed sitemap at
``/infoservices/startseite-sitemap-102.html`` (≈ 60 article links per
page) even though ``/sitemap.xml`` returns HTML 404 and the publisher's
robots.txt carries no ``Sitemap:`` directive — standard auto-discovery
(``<link rel="alternate">``, ``robots.txt``-based sitemap detection)
finds none of this.

This channel is operator-discoverable, not library-discoverable: the
``audit-source-discovery`` CLI probes common HTML-sitemap paths
(``/sitemap``, ``/sitemap.html``, ``/index/sitemap``, etc.) and reports
what each publisher exposes; the operator records the result in
``sources.yaml`` per source. The pattern is publisher-curated by
construction (the publisher built the page, we ingest the links
verbatim) — methodologically clean per WP-006 §3.

Implementation choices:

* Regex-based ``<a href>`` extraction over a heavy HTML-parser
  dependency. The crawler is intentionally lean (no lxml / bs4 in its
  pyproject) and discovery only needs a flat sweep of href attributes.
  Edge cases (script-injected links, JS-rendered SPAs) are out of scope
  — the publisher chose to expose the page as static HTML.
* Per-source ``article_url_pattern`` regex filters non-article hrefs
  (navigation, asset URLs, the page's own self-link). Same convention
  as :mod:`internal.discovery.archive_index`.
* ``DiscoveredUrl.sitemap_lastmod = None`` for URLs from this channel —
  the HTML sitemap surfaces the *current* article set without per-
  article timestamps; the worker's WebAdapter populates
  ``published_date`` from JSON-LD at the Silver boundary. These URLs
  consequently sink to the end of the newest-first sort, which is
  correct given the channel has no per-article date signal.
* ``since`` is accepted for symmetry with the other discovery channels
  but is a no-op here — the HTML sitemap is by construction current
  (the page is daily-refreshed by the publisher); URLs surfaced are
  always within the active publication window.

Cross-source uniformity: this module mirrors
:mod:`internal.discovery.sitemap`, :mod:`internal.discovery.rss_hint`,
and :mod:`internal.discovery.archive_index` — same
``DiscoveredUrl``-compatible ``(url, lastmod)`` return shape. The
caller in ``main.py`` treats all four channels symmetrically.
"""

from __future__ import annotations

import logging
import re
from datetime import datetime
from typing import Any, Iterator, Optional, Tuple
from urllib.parse import urlparse

import requests

from . import assert_pattern_usable


logger = logging.getLogger(__name__)

DEFAULT_TIMEOUT_SECONDS = 15.0
DEFAULT_USER_AGENT = "AerWebCrawler/0.1"

# Pre-compiled href extractor — same flat-sweep approach as
# archive_index.py. Discovery never needs deeper parsing than the href
# attribute of an anchor tag.
_HREF_RE = re.compile(r'href=["\']([^"\']+)["\']', re.IGNORECASE)


def discover(
    configs: list[dict[str, Any]],
    since: Optional[datetime] = None,  # accepted for symmetry; no-op here
    *,
    user_agent: str = DEFAULT_USER_AGENT,
    timeout_seconds: float = DEFAULT_TIMEOUT_SECONDS,
    http_get=None,
) -> Iterator[Tuple[str, Optional[datetime]]]:
    """Yield ``(url, None)`` pairs from publisher-built HTML sitemap pages.

    Parameters
    ----------
    configs:
        List of per-page configs. Each entry is a dict with:

        * ``url`` (required) — the HTML sitemap page to fetch.
        * ``article_url_pattern`` (required) — Python regex matching
          article URLs. Non-matching hrefs are dropped (filters out
          navigation, asset URLs, and the page's own self-link).
        * ``base_url`` (optional) — base for resolving relative ``href``
          values. If omitted, derived from ``url`` (scheme + netloc).
    since:
        Accepted for symmetry with the other discovery channels. The
        HTML sitemap is by construction current (publisher-refreshed),
        so this parameter is currently unused; declared in the signature
        so the caller can thread it through uniformly.
    user_agent / timeout_seconds / http_get:
        Politeness + test seams. Production defaults are ``requests.get``
        with a 15 s timeout.

    Yields
    ------
    ``(url, None)`` tuples. ``None`` for the timestamp because the HTML
    sitemap channel exposes no per-article date; the worker's
    WebAdapter populates ``published_date`` from JSON-LD at the Silver
    boundary as for any other discovery channel.
    """
    if not configs:
        return

    fetch = http_get or requests.get
    headers = {"User-Agent": user_agent}
    yielded: set[str] = set()
    del since  # accepted for symmetry; no-op

    for entry in configs:
        page_url = (entry or {}).get("url") or ""
        if not page_url:
            continue
        pattern = (entry or {}).get("article_url_pattern") or ""
        # Phase 122g — refuse to silently skip a channel with a missing
        # or placeholder pattern. Either the operator sees a loud error
        # at crawler startup, or the channel ingests zero articles
        # invisibly. We pick loud-error.
        assert_pattern_usable(pattern, channel="html_sitemap", where=page_url)
        try:
            article_re = re.compile(pattern)
        except re.error as exc:
            from . import DiscoveryConfigurationError
            raise DiscoveryConfigurationError(
                f"html_sitemap entry at {page_url!r} has an invalid "
                f"`article_url_pattern` regex ({exc}). Fix the regex in "
                "sources.yaml before re-running the crawler."
            ) from exc
        base_url = entry.get("base_url") or _derive_base_url(page_url)

        try:
            resp = fetch(page_url, headers=headers, timeout=timeout_seconds)
        except Exception as exc:
            logger.warning("html_sitemap fetch failed: %s — %s", page_url, exc)
            continue
        if getattr(resp, "status_code", 0) != 200:
            logger.warning(
                "html_sitemap non-200 (url=%s, status=%s)",
                page_url,
                getattr(resp, "status_code", 0),
            )
            continue

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
            yield absolute, None


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
