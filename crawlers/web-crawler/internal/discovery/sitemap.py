"""Sitemap discovery via ultimate-sitemap-parser.

Yields the URL discovery surface for a single source. As of Phase 122e
(F-A1) sitemaps and RSS feeds are peer-equal discovery channels — see
:mod:`internal.discovery.rss_hint`. For sources without a public XML
sitemap (Probe 0's tagesschau, whose ``sitemap.xml`` returns HTML 404),
RSS is the sole channel; the empty ``sitemap_urls: []`` in
``sources.yaml`` is the explicit configuration of that fact. The
``last_modified`` value is preserved from the sitemap entry so the
WebAdapter can use it as the ``timestamp_source = "sitemap_lastmod"``
fallback when a JSON-LD ``datePublished`` is unavailable.

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
    strict_lastmod: bool = True,
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
    never queued for fetch.

    Phase 122e A21 / F-A21 — `strict_lastmod` controls how entries
    *without* a ``<lastmod>`` are treated when ``since`` is supplied:

    * ``strict_lastmod=True`` (default, continuous-monitoring mode) —
      entries with no lastmod are *dropped*. This is the only safe
      behaviour when the publisher's sitemap is fully undated, as in
      Probe 0's bundesregierung.de (638-of-638 entries lack lastmod in
      one leaf). Without strict mode, every undated entry bypasses
      `time_window_days` and the temporal filter is a no-op.
    * ``strict_lastmod=False`` (backfill mode) — entries with no lastmod
      *fall through*. The worker's ``timestamp_source = "fetch_at_fallback"``
      classifies them as Negative Space (Brief §7.7) — methodologically
      defensible only when the operator has explicitly opted into
      ingesting unbounded historical content.

    When ``since`` is ``None`` (no temporal cutoff), ``strict_lastmod``
    has no effect — all entries are yielded regardless of lastmod.
    """
    # Phase 123 — parse ONLY the operator-configured sitemap URL(s). The
    # previous implementation called usp `sitemap_tree_for_homepage(root)`,
    # which treats `root` as a homepage: it reads robots.txt, auto-discovers
    # EVERY sitemap the publisher declares, and recurses the entire tree. For a
    # publisher like franceinfo that exposes a monthly archive index back to
    # 2012 across article/audio/video/image sitemaps, a single crawl fanned out
    # into thousands of sitemap fetches before the temporal filter discarded
    # them — minutes of churn per source and a hard scalability wall at
    # multi-probe scale. We now fetch and parse exactly the configured URL; a
    # configured sitemap-INDEX recurses only into the children it explicitly
    # lists (the operator's choice), never robots.txt, and is bounded by
    # `_MAX_SITEMAP_FETCHES`.
    seen_urls: set[str] = set()
    for root in sitemap_urls:
        for url, lastmod in _iter_sitemap_entries(root):
            if not url or url in seen_urls:
                continue
            if since is not None:
                if lastmod is None:
                    if strict_lastmod:
                        continue  # F-A21: drop undated entries in continuous mode
                    # else: fall through (backfill mode)
                elif lastmod < since:
                    continue
            seen_urls.add(url)
            section = _section_from_url(url)
            yield DiscoveredUrl(url=url, sitemap_lastmod=lastmod, sitemap_section=section)


# Hard cap on how many sitemap documents a single configured root may fan out
# into when it is a sitemap-index. Bounds the blast radius if an operator points
# at a large index; a flat news sitemap (the common case) costs exactly one
# fetch.
_MAX_SITEMAP_FETCHES = 50


def _iter_sitemap_entries(root: str) -> Iterator[tuple[Optional[str], "datetime | None"]]:
    """Yield ``(loc, lastmod)`` from the configured sitemap URL.

    Handles both ``<urlset>`` (flat list of page URLs) and ``<sitemapindex>``
    (recurse only into the child sitemaps it lists). Never consults robots.txt
    and never auto-discovers sitemaps the operator did not configure. Fully
    fail-silent: any fetch/parse error logs a warning and yields nothing for
    that document, so discovery degrades to the other channels (RSS, etc.)
    rather than aborting the crawl.
    """
    import gzip
    import xml.etree.ElementTree as ET

    import requests

    queue: list[str] = [root]
    fetched = 0
    visited: set[str] = set()
    while queue and fetched < _MAX_SITEMAP_FETCHES:
        sm_url = queue.pop(0)
        if sm_url in visited:
            continue
        visited.add(sm_url)
        fetched += 1
        try:
            resp = requests.get(sm_url, timeout=30)
            resp.raise_for_status()
            body = resp.content
            # Some publishers gzip sitemaps (.xml.gz) or send gzip transparently.
            if sm_url.endswith(".gz") or body[:2] == b"\x1f\x8b":
                body = gzip.decompress(body)
            root_el = ET.fromstring(body)
        except Exception as exc:
            logger.warning("Failed to fetch/parse sitemap %s: %s", sm_url, exc)
            continue

        tag = _localname(root_el.tag)
        if tag == "sitemapindex":
            # Child sitemaps explicitly listed by THIS index only.
            for sm in root_el:
                loc = _child_text(sm, "loc")
                if loc:
                    queue.append(loc.strip())
        else:
            # Treat anything else as a urlset.
            for url_el in root_el:
                loc = _child_text(url_el, "loc")
                if not loc:
                    continue
                yield loc.strip(), _parse_lastmod(_child_text(url_el, "lastmod"))


def _localname(tag: str) -> str:
    """Strip the XML namespace from an element tag (`{ns}urlset` → `urlset`)."""
    return tag.rsplit("}", 1)[-1].lower()


def _child_text(parent, name: str) -> Optional[str]:
    """Return the text of the first direct child whose local name matches."""
    for child in parent:
        if _localname(child.tag) == name:
            return child.text
    return None


def _parse_lastmod(raw: Optional[str]) -> "datetime | None":
    """Parse a sitemap ``<lastmod>`` into a UTC-aware datetime, or None.

    Accepts full ISO-8601 timestamps (with `Z` or an offset) and date-only
    values (`YYYY-MM-DD`). A naive result is assumed to be UTC so it compares
    correctly against the tz-aware `since` watermark.
    """
    if not raw:
        return None
    from datetime import datetime, timezone

    text = raw.strip()
    if not text:
        return None
    try:
        dt = datetime.fromisoformat(text.replace("Z", "+00:00"))
    except ValueError:
        try:
            dt = datetime.strptime(text[:10], "%Y-%m-%d")
        except ValueError:
            return None
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt


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
