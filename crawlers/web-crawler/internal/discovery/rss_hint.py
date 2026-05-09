"""RSS feed parsing as a *discovery hint only* (Phase 122).

The crawler never uses the RSS body. The feed's ``<link>`` URLs surface
freshly-published articles before the next sitemap refresh; the article
body itself is always fetched from the HTML source. Items already
visible in the sitemap are deduplicated by the caller.
"""

from __future__ import annotations

import logging
from typing import Iterator


logger = logging.getLogger(__name__)


def discover(rss_url: str) -> Iterator[str]:
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
        if url:
            yield url
