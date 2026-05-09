"""Sitemap discovery via ultimate-sitemap-parser.

Yields the URL discovery surface for a single source. The crawler treats
sitemaps as the *primary* discovery channel; RSS feeds are a hint only
(see :mod:`internal.discovery.rss_hint`). The ``last_modified`` value is
preserved from the sitemap entry so the WebAdapter can use it as the
``timestamp_source = "sitemap_lastmod"`` fallback when a JSON-LD
``datePublished`` is unavailable.

The function is robust to nested sitemap indexes — ultimate-sitemap-parser
recursively expands ``<sitemap>`` entries into the leaf URL set.
"""

from __future__ import annotations

import logging
from dataclasses import dataclass
from datetime import datetime
from typing import Iterator, Optional


@dataclass(frozen=True)
class DiscoveredUrl:
    url: str
    sitemap_lastmod: Optional[datetime]
    sitemap_section: Optional[str]


logger = logging.getLogger(__name__)


def discover(
    sitemap_urls: list[str],
    since: Optional[datetime] = None,
) -> Iterator[DiscoveredUrl]:
    """Yield every leaf URL surfaced by the supplied sitemap roots.

    Imports ``usp.tree`` lazily so the discovery module remains importable
    in the test suite even when ``ultimate-sitemap-parser`` is not yet
    installed (the test path is `internal.discovery.sitemap.discover` →
    monkey-patched). Each leaf URL produces exactly one
    :class:`DiscoveredUrl`; entries with the same URL across multiple
    sitemap roots collapse on the consumer side.

    Phase 122b — temporal symmetry. When ``since`` is supplied, entries
    with ``sitemap_lastmod < since`` are dropped at discovery time and
    never queued for fetch. Entries whose ``last_modified`` is ``None``
    fall through (the worker's ``timestamp_source = "fetch_at_fallback"``
    already classifies them as Negative Space per Brief §7.7 — we do not
    silently drop coverage on publishers with sparse sitemaps).
    """
    try:
        from usp.tree import sitemap_tree_for_homepage  # type: ignore
    except Exception as exc:  # pragma: no cover - import-shim
        logger.warning("ultimate-sitemap-parser not installed: %s", exc)
        return

    seen_urls: set[str] = set()
    for root in sitemap_urls:
        try:
            tree = sitemap_tree_for_homepage(root)
        except Exception as exc:
            logger.warning("Failed to parse sitemap root %s: %s", root, exc)
            continue
        for page in tree.all_pages():
            url = getattr(page, "url", None)
            if not url or url in seen_urls:
                continue
            lastmod = getattr(page, "last_modified", None)
            if since is not None and lastmod is not None and lastmod < since:
                continue
            seen_urls.add(url)
            section = _section_from_url(url)
            yield DiscoveredUrl(url=url, sitemap_lastmod=lastmod, sitemap_section=section)


def _section_from_url(url: str) -> Optional[str]:
    """Best-effort guess of the URL's first path segment, used purely as a
    contextual tag (`sitemap_section`). Not a topical filter — section-
    level filtering is explicitly rejected per WP-006 §3.
    """
    try:
        from urllib.parse import urlparse

        path = urlparse(url).path or ""
        parts = [p for p in path.split("/") if p]
        if not parts:
            return None
        return parts[0]
    except Exception:
        return None
