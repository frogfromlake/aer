"""AĒR audit-source-discovery CLI — Phase 122g.

Operator-facing tool that probes a candidate source's homepage and
reports the discovery channels the publisher exposes. Two modes:

* **Onboarding mode** (default, no ``--sources-yaml`` argument).
  Prints a YAML-shaped ``discovery:`` block to stdout so the operator
  can paste the result into ``probes/<probe-id>/sources.yaml`` for a
  brand-new source.

* **Re-audit mode** (``--sources-yaml <path> --source <name>``).
  Loads the existing source's ``discovery:`` block from the YAML,
  compares it against the live audit, and prints an additive diff
  (only newly discovered URLs / channels are reported — entries
  present in the YAML but absent from the audit are NEVER flagged
  for removal, because publisher-surface disappearance is a
  methodological event handled by the underflow-alert telemetry,
  not a routine maintenance trigger). If the diff is non-empty the
  CLI prompts ``[y/N]`` and, on confirmation, updates the YAML
  in-place with ``ruamel.yaml`` (preserves comments + formatting)
  and writes a ``.bak`` backup.

This is the source-onboarding equivalent of the silently-installed
``ldap-search`` tool an operator uses to discover what an LDAP server
exposes before configuring an LDAP-bound app. The audit happens
*manually* (onboarding) or *periodically* (re-audit); runtime crawls
ALWAYS use the operator-curated configuration, never auto-discovery.
That separation is the load-bearing decision recorded in ADR-031:
auto-discovery as a config-authoring helper, not a runtime fallback.

Channels probed:
  * RSS / Atom feed auto-discovery via ``trafilatura.feeds.find_feed_urls``
    (``<link rel="alternate">`` + standard CMS paths). Trafilatura is
    optional — the audit gracefully degrades if it isn't installed.
  * XML sitemap auto-discovery via ``trafilatura.sitemaps.sitemap_search``
    (robots.txt + standard locations). Same optional-dep degradation.
  * Direct RSS-path probes — common publisher conventions
    (``/feed``, ``/rss.xml``, ``/atom.xml``, ``/index~rss2.xml`` etc.).
  * RSS-catalogue page parsing — publishers like bundesregierung
    expose a catalogue page at ``/service/newsletter-und-abos/...``
    whose ``<link rel="alternate">`` / ``<a href="*.xml">`` set
    enumerates several official feeds.
  * HTML sitemap probes — common publisher paths that surface a
    navigation index in HTML.
  * Date-indexed archive probes — common patterns publishers expose
    for date-walking.

Usage::

    # Onboarding a brand-new source (prints suggested YAML block)
    python audit_source.py <homepage_url>

    # Re-auditing an existing source (diff against sources.yaml, prompt)
    python audit_source.py <homepage_url> \\
        --sources-yaml probes/probe0/sources.yaml \\
        --source tagesschau

    # Re-auditing every source in a probe (loops, prompts per source)
    python audit_source.py --probe probes/probe0/sources.yaml

Cross-link: Mediacloud (https://search.mediacloud.org) maintains a
public registry of ~ 60,000 news sources with curated feed lists. If
the source already exists there, the CLI suggests importing.
"""

from __future__ import annotations

import argparse
import json
import logging
import re
import shutil
import sys
from pathlib import Path
from typing import Any, Optional
from urllib.parse import urljoin, urlparse

import requests


logger = logging.getLogger(__name__)

DEFAULT_TIMEOUT = 15.0
DEFAULT_USER_AGENT = "AerAuditSourceDiscovery/0.1 (+https://aer.example/about)"

# Common HTML-sitemap paths — operator-curated list of patterns that
# real-world German + international publishers expose. The audit CLI
# probes each in order and reports which return HTTP 200 with HTML.
HTML_SITEMAP_CANDIDATES = [
    "/sitemap",
    "/sitemap.html",
    "/sitemap/",
    "/sitemap/index",
    "/index/sitemap",
    "/site-map",
    "/sitemap-index",
    "/sitemap.xhtml",
    "/infoservices/startseite-sitemap-102.html",  # tagesschau pattern
    "/infoservices/sitemap-102.html",
    "/news-sitemap",
    "/archive/sitemap",
]

# Common date-indexed archive endpoints. The audit probes the latest
# observable date (today) so a 200 + non-trivial body indicates an
# active archive walker the operator can configure with `archive_index`.
ARCHIVE_INDEX_CANDIDATES = [
    "/archiv?datum={date}",
    "/archive?date={date}",
    "/archiv/{date}",
    "/archive/{date}",
    "/archiv/index?datum={date}",
    "/news/archive/{date}",
    "/archiv/{year}/{month}/{day}",
    "/nachrichten/archiv?datum={date}",
]

# Direct RSS / Atom path probes — typical conventions across CMSes
# (WordPress, Drupal, Joomla, CoreMedia, plus publisher-specific
# patterns like tagesschau's ``index~rss2.xml``).
RSS_FEED_CANDIDATES = [
    "/feed",
    "/feed/",
    "/feeds",
    "/feeds/",
    "/rss",
    "/rss/",
    "/rss.xml",
    "/feed.xml",
    "/atom.xml",
    "/index.rss",
    "/index~rss2.xml",  # tagesschau
    "/news/rss",
    "/news/feed",
    "/news.rss",
    "/aktuell/feed",
    "/aktuelles/feed",
    "/aktuell/rss",
    "/?feed=rss2",  # WordPress
    "/rss/news",
]

# Catalogue-page paths — publishers that publish multiple feeds
# typically advertise them on one of these paths rather than via
# `<link rel="alternate">`. The audit fetches each, parses any
# ``<a href="*.xml">`` / ``<link rel="alternate">`` entries it finds,
# and surfaces them as feed candidates.
RSS_CATALOGUE_CANDIDATES = [
    "/service/rss",
    "/service/rss-newsfeed",
    "/service/newsletter-und-abos",
    "/service/newsletter-und-abos/rss-newsfeed",
    "/service/feeds",
    "/feeds-uebersicht",
    "/newsletter",
    "/newsletter-und-abos",
    "/abos",
    "/rss-feeds",
    "/rss-feeds-uebersicht",
    "/hilfe/rss",
]

# Regex to recognise feed-shaped URLs in catalogue HTML.
_FEED_HREF_RE = re.compile(
    r"""href\s*=\s*['"]([^'"]+?\.(?:xml|rss|atom)(?:\?[^'"]*)?)['"]""",
    re.IGNORECASE,
)
_FEED_LINK_REL_RE = re.compile(
    r"""<link[^>]+rel\s*=\s*['"](?:alternate)['"][^>]+type\s*=\s*['"]"""
    r"""application/(?:rss\+xml|atom\+xml)['"][^>]+href\s*=\s*['"]([^'"]+)['"]""",
    re.IGNORECASE,
)
_FEED_LINK_REL_HREF_FIRST_RE = re.compile(
    r"""<link[^>]+href\s*=\s*['"]([^'"]+)['"][^>]+rel\s*=\s*['"](?:alternate)['"]"""
    r"""[^>]+type\s*=\s*['"]application/(?:rss\+xml|atom\+xml)['"]""",
    re.IGNORECASE,
)
_GENERATOR_META_RE = re.compile(
    r"""<meta[^>]+name\s*=\s*['"]generator['"][^>]+content\s*=\s*['"]([^'"]+)['"]""",
    re.IGNORECASE,
)

# Generic <a href="...">-extractor used for article-URL inference on
# HTML-sitemap and archive-index pages. Matches both single and double
# quotes; rejects empty hrefs at the regex level.
_A_HREF_RE = re.compile(
    r"""<a[^>]+href\s*=\s*['"]([^'"#]+)['"]""",
    re.IGNORECASE,
)

# Asset-file extensions excluded when sampling article URLs.
_ASSET_EXTENSIONS = (
    ".jpg", ".jpeg", ".png", ".gif", ".svg", ".webp", ".bmp", ".ico",
    ".mp4", ".mp3", ".wav", ".webm", ".avi", ".mov",
    ".css", ".js", ".json", ".xml",
    ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
    ".woff", ".woff2", ".ttf", ".otf", ".eot",
    ".zip", ".tar", ".gz",
)


def _probe_http(url: str, http_get, timeout: float) -> dict[str, Any]:
    """Return {status, content_type, body_size, has_html_tag} for one URL."""
    try:
        resp = http_get(
            url,
            headers={"User-Agent": DEFAULT_USER_AGENT},
            timeout=timeout,
            allow_redirects=True,
        )
        status = getattr(resp, "status_code", 0)
        body = getattr(resp, "text", "") or ""
        content_type = (resp.headers.get("Content-Type", "")
                        if getattr(resp, "headers", None) else "")
        return {
            "status": status,
            "content_type": content_type,
            "body_size": len(body),
            "has_html_tag": "<html" in body.lower()[:512],
            "final_url": getattr(resp, "url", url),
        }
    except Exception as exc:
        return {
            "status": 0,
            "error": f"{type(exc).__name__}: {exc}",
        }


def _fetch_body(url: str, http_get, timeout: float) -> tuple[int, str]:
    """Return ``(status_code, body)`` for one URL. Empty string on any
    failure. Used by the sanity-check helpers that need the actual
    response body (not just metadata)."""
    try:
        resp = http_get(
            url,
            headers={"User-Agent": DEFAULT_USER_AGENT},
            timeout=timeout,
            allow_redirects=True,
        )
        status = getattr(resp, "status_code", 0)
        body = getattr(resp, "text", "") or ""
        return (status, body)
    except Exception:
        return (0, "")


def _try_trafilatura_feeds(homepage: str) -> Optional[list[str]]:
    """Try `trafilatura.feeds.find_feed_urls(homepage)`. Returns None
    if trafilatura is not installed; otherwise the list of URLs the
    auto-discovery surfaced (note: trafilatura's signature returns
    article URLs from the discovered feeds, not the feed URLs
    themselves — useful as a coverage proxy rather than as a feed list).
    """
    try:
        from trafilatura import feeds  # type: ignore
    except ImportError:
        return None
    try:
        return list(feeds.find_feed_urls(homepage) or [])
    except Exception as exc:
        logger.warning("trafilatura feeds discovery failed: %s", exc)
        return []


def _try_trafilatura_sitemaps(homepage: str) -> Optional[list[str]]:
    """Try `trafilatura.sitemaps.sitemap_search(homepage)`. Returns None
    if trafilatura is not installed; otherwise the URL list."""
    try:
        from trafilatura import sitemaps  # type: ignore
    except ImportError:
        return None
    try:
        return list(sitemaps.sitemap_search(homepage) or [])
    except Exception as exc:
        logger.warning("trafilatura sitemaps discovery failed: %s", exc)
        return []


def _is_feed_like(url: str, body_sample: str, content_type: str) -> bool:
    """Heuristic check whether a probed URL actually returned an
    RSS/Atom feed.

    A 200 response on ``/feed`` doesn't automatically mean a feed — many
    CMSes return a generic HTML page for unknown paths. We require the
    body to start with an XML prolog or contain a ``<rss``/``<feed``
    root element within the first 1 KB.
    """
    if "xml" in content_type.lower() or "rss" in content_type.lower():
        return True
    sample = body_sample[:1024].lstrip().lower()
    return sample.startswith("<?xml") or "<rss" in sample or "<feed" in sample


def _extract_feed_links_from_catalogue(body: str, base_url: str) -> list[str]:
    """Parse a catalogue / newsletter / RSS-overview HTML page and
    return the set of feed-shaped URLs it advertises.

    Two extraction strategies are combined:

    * ``<link rel="alternate" type="application/rss+xml" href="...">``
      (the standard discoverability attribute many CMSes still emit).
    * Any ``<a href="...">`` / ``<link href="...">`` ending in
      ``.xml``, ``.rss``, or ``.atom`` (the publisher pattern when
      the catalogue page is hand-authored rather than CMS-generated).
    """
    found: set[str] = set()
    for match in _FEED_LINK_REL_RE.finditer(body):
        found.add(urljoin(base_url, match.group(1)))
    for match in _FEED_LINK_REL_HREF_FIRST_RE.finditer(body):
        found.add(urljoin(base_url, match.group(1)))
    for match in _FEED_HREF_RE.finditer(body):
        found.add(urljoin(base_url, match.group(1)))
    return sorted(found)


def _probe_rss_paths(
    origin: str,
    http_get,
    timeout: float,
    *,
    verbose: bool = False,
) -> list[dict[str, Any]]:
    """Probe each candidate RSS path; return the subset that returned
    a feed-shaped 200 response."""
    hits: list[dict[str, Any]] = []
    for path in RSS_FEED_CANDIDATES:
        probe_url = urljoin(origin + "/", path.lstrip("/"))
        result = _probe_http(probe_url, http_get, timeout)
        result["probed_url"] = probe_url
        if result.get("status") == 200:
            body_sample = ""
            try:
                # _probe_http does not return body; re-fetch only when we
                # have a positive status, to keep the probe phase cheap.
                resp = http_get(
                    probe_url,
                    headers={"User-Agent": DEFAULT_USER_AGENT},
                    timeout=timeout,
                    allow_redirects=True,
                )
                body_sample = (getattr(resp, "text", "") or "")[:1024]
                content_type = (
                    resp.headers.get("Content-Type", "")
                    if getattr(resp, "headers", None) else ""
                )
            except Exception:
                content_type = result.get("content_type", "")
            if _is_feed_like(probe_url, body_sample, content_type):
                hits.append({
                    "url": probe_url,
                    "content_type": content_type,
                })
                continue
        if verbose:
            hits.append({
                "url": probe_url,
                "status": result.get("status"),
                "skipped": True,
            })
    return hits


def _probe_rss_catalogues(
    origin: str,
    http_get,
    timeout: float,
    *,
    verbose: bool = False,
) -> list[dict[str, Any]]:
    """Probe each catalogue path; for any that returns HTML, parse the
    body for advertised feed URLs."""
    hits: list[dict[str, Any]] = []
    for path in RSS_CATALOGUE_CANDIDATES:
        probe_url = urljoin(origin + "/", path.lstrip("/"))
        try:
            resp = http_get(
                probe_url,
                headers={"User-Agent": DEFAULT_USER_AGENT},
                timeout=timeout,
                allow_redirects=True,
            )
        except Exception as exc:
            if verbose:
                hits.append({
                    "catalogue_url": probe_url,
                    "error": f"{type(exc).__name__}: {exc}",
                    "skipped": True,
                })
            continue
        status = getattr(resp, "status_code", 0)
        body = getattr(resp, "text", "") or ""
        if status == 200 and "<html" in body.lower()[:512] and len(body) > 1024:
            feeds = _extract_feed_links_from_catalogue(body, probe_url)
            if feeds:
                hits.append({
                    "catalogue_url": probe_url,
                    "discovered_feeds": feeds,
                })
                continue
        if verbose:
            hits.append({
                "catalogue_url": probe_url,
                "status": status,
                "skipped": True,
            })
    return hits


def _fetch_homepage(
    homepage: str,
    http_get,
    timeout: float,
) -> tuple[str, str]:
    """Return ``(body, content_type)`` for the homepage; empty strings
    on any failure (the audit degrades gracefully)."""
    try:
        resp = http_get(
            homepage,
            headers={"User-Agent": DEFAULT_USER_AGENT},
            timeout=timeout,
            allow_redirects=True,
        )
        if getattr(resp, "status_code", 0) != 200:
            return ("", "")
        body = getattr(resp, "text", "") or ""
        content_type = (
            resp.headers.get("Content-Type", "")
            if getattr(resp, "headers", None) else ""
        )
        return (body, content_type)
    except Exception as exc:
        logger.warning("homepage fetch failed: %s", exc)
        return ("", "")


def _extract_article_url_candidates(
    body: str,
    base_url: str,
    *,
    self_url: Optional[str] = None,
) -> list[str]:
    """Parse ``<a href>`` links from a publisher page and return the
    subset that plausibly point to articles.

    Filters applied:
      * Same host as ``base_url`` (cross-domain links are navigation
        / partner widgets, not articles).
      * Not the page's own URL (``self_url``) — the HTML sitemap often
        includes its own permalink in the navigation.
      * Path depth ≥ 2 (root + at least one section, articles are
        almost never at root).
      * Path does NOT end in an asset extension.
      * Path is not a query-only or fragment-only link.

    Used by :func:`_infer_article_url_pattern` to derive a publisher-
    specific regex when the operator accepts an html_sitemap or
    archive_index diff.
    """
    if not body:
        return []
    parsed_base = urlparse(base_url)
    if not parsed_base.netloc:
        return []
    base_host = parsed_base.netloc.lower().removeprefix("www.")
    seen: set[str] = set()
    out: list[str] = []
    for match in _A_HREF_RE.finditer(body):
        href = match.group(1).strip()
        if not href or href.startswith(("mailto:", "tel:", "javascript:", "#")):
            continue
        absolute = urljoin(base_url, href)
        parsed = urlparse(absolute)
        if parsed.scheme not in ("http", "https"):
            continue
        host = parsed.netloc.lower().removeprefix("www.")
        if host != base_host:
            continue
        if self_url and absolute.rstrip("/") == self_url.rstrip("/"):
            continue
        path = parsed.path or ""
        if not path or path == "/":
            continue
        if path.lower().endswith(_ASSET_EXTENSIONS):
            continue
        # Path depth ≥ 2 (e.g. /section/slug, not just /about).
        if path.count("/") < 2:
            continue
        # Dedupe by canonical form (drop trailing slash, lowercase host).
        canon = absolute.rstrip("/").lower()
        if canon in seen:
            continue
        seen.add(canon)
        out.append(absolute)
    return out


def _infer_article_url_pattern(
    article_urls: list[str],
    homepage_origin: str,
    *,
    min_sample: int = 5,
) -> Optional[str]:
    """Derive a conservative regex that matches the supplied article
    URLs from a single host. Returns ``None`` when no high-confidence
    pattern can be inferred (the operator will see the ``EDIT-ME``
    placeholder and write one manually).

    Three patterns, tried in order of specificity:

    1. **Slug-with-numeric-id + ``.html``** (e.g. tagesschau:
       ``/inland/.../foo-bar-NNN.html``). Conservative AND specific.
    2. **Common path prefix + ``.html`` extension** when ≥ 80 % of
       URLs share the same first path segment (e.g. bundesregierung:
       ``/breg-de/aktuelles/...``).
    3. **Common path prefix only** when there's a strong shared prefix
       but no consistent extension.

    Pattern format always matches ``http`` and ``https``, with or
    without ``www.`` — matches the conventions used elsewhere in
    ``sources.yaml``.
    """
    if len(article_urls) < min_sample:
        return None

    parsed_homepage = urlparse(homepage_origin)
    host = parsed_homepage.netloc.lower().removeprefix("www.")
    if not host:
        return None
    host_pattern = re.escape(host)
    host_alt = rf"https?://(www\.)?{host_pattern}"

    paths = [urlparse(u).path for u in article_urls]
    paths = [p for p in paths if p]
    if len(paths) < min_sample:
        return None

    # Pattern 1: <prefix>-<digits>.html — the tagesschau / classic
    # CoreMedia convention.
    slug_id_re = re.compile(r"-\d+\.html$")
    if sum(1 for p in paths if slug_id_re.search(p)) / len(paths) >= 0.8:
        return f"^{host_alt}/[^?#]+-\\d+\\.html$"

    # Pattern 2: common first path segment + .html extension.
    first_segments = [p.lstrip("/").split("/", 1)[0] for p in paths]
    seg_counts: dict[str, int] = {}
    for seg in first_segments:
        if seg:
            seg_counts[seg] = seg_counts.get(seg, 0) + 1
    if seg_counts:
        most_common_seg, most_common_n = max(seg_counts.items(), key=lambda kv: kv[1])
        if most_common_n / len(paths) >= 0.8:
            html_share = sum(
                1 for p in paths
                if p.lstrip("/").startswith(most_common_seg + "/")
                and p.lower().endswith(".html")
            ) / most_common_n
            seg_escaped = re.escape(most_common_seg)
            if html_share >= 0.8:
                return f"^{host_alt}/{seg_escaped}/[^?#]+\\.html$"
            return f"^{host_alt}/{seg_escaped}/[^?#]+$"

    # Pattern 3: when no clear segment dominates, accept any HTML page
    # under the host at depth ≥ 2 (very conservative — likely to be
    # imprecise; operator should review).
    if sum(1 for p in paths if p.lower().endswith(".html")) / len(paths) >= 0.8:
        return f"^{host_alt}/[^?#]+\\.html$"

    return None


def _validate_article_listing_page(
    body: str,
    page_url: str,
    *,
    min_article_links: int = 5,
) -> dict[str, Any]:
    """Determine whether a page actually contains a meaningful list of
    article links — used to reject false-positive html_sitemap and
    archive_index candidates that return HTTP 200 + HTML but no real
    article content (the bundesregierung ``?datum=...`` failure mode).

    Returns ``{is_listing: bool, article_urls: list[str], reason: str}``.
    ``article_urls`` carries the extracted candidates (used downstream
    for pattern inference + operator-visible sample).
    """
    article_urls = _extract_article_url_candidates(
        body, page_url, self_url=page_url
    )
    if len(article_urls) < min_article_links:
        return {
            "is_listing": False,
            "article_urls": article_urls,
            "reason": f"only {len(article_urls)} article-shaped link(s) "
                      f"found (threshold {min_article_links}) — likely a "
                      "navigation page, not an article listing.",
        }
    return {
        "is_listing": True,
        "article_urls": article_urls,
        "reason": f"{len(article_urls)} article-shaped links found.",
    }


def _verify_date_walker(
    url_template: str,
    *,
    origin: str,
    http_get,
    timeout: float,
    today: Any,  # datetime
) -> dict[str, Any]:
    """Confirm an archive-index URL template actually behaves like a
    date walker: fetch today's date and a date one year earlier; the
    two pages must surface DIFFERENT article-link sets.

    A publisher whose ``?datum=YYYY-MM-DD`` parameter is silently
    ignored (bundesregierung is the canonical example) returns the
    same generic navigation page regardless of date. This check
    rejects such candidates BEFORE the audit ever proposes them as
    valid archive_index channels.

    Returns ``{is_walker: bool, today_url, old_url, today_articles,
    old_articles, overlap_ratio, reason}``.
    """
    from datetime import timedelta
    past = today - timedelta(days=365)

    def _resolve(when: Any) -> str:
        path = (
            url_template
            .replace("{date}", when.strftime("%Y-%m-%d"))
            .replace("{year}", when.strftime("%Y"))
            .replace("{month}", when.strftime("%m"))
            .replace("{day}", when.strftime("%d"))
        )
        return urljoin(origin + "/", path.lstrip("/"))

    today_url = _resolve(today)
    old_url = _resolve(past)

    today_status, today_body = _fetch_body(today_url, http_get, timeout)
    old_status, old_body = _fetch_body(old_url, http_get, timeout)

    if today_status != 200 or old_status != 200:
        return {
            "is_walker": False,
            "today_url": today_url,
            "old_url": old_url,
            "today_articles": [],
            "today_non_articles": [],
            "old_articles": [],
            "overlap_ratio": 1.0,
            "reason": f"one of the two probe dates returned non-200 "
                      f"(today={today_status}, past={old_status}).",
        }

    today_articles = _extract_article_url_candidates(
        today_body, today_url, self_url=today_url
    )
    today_non_articles = _extract_non_article_links(today_body, today_url)
    old_articles = _extract_article_url_candidates(
        old_body, old_url, self_url=old_url
    )

    if not today_articles and not old_articles:
        return {
            "is_walker": False,
            "today_url": today_url,
            "old_url": old_url,
            "today_non_articles": today_non_articles,
            "today_articles": [],
            "old_articles": [],
            "overlap_ratio": 1.0,
            "reason": "neither probe date surfaced any article-shaped links.",
        }

    today_set = {u.rstrip("/").lower() for u in today_articles}
    old_set = {u.rstrip("/").lower() for u in old_articles}
    if not today_set or not old_set:
        # One side surfaced articles, the other didn't — still suspicious
        # but we'll be lenient and treat as walker since at least one
        # date is yielding content.
        return {
            "is_walker": True,
            "today_url": today_url,
            "old_url": old_url,
            "today_articles": today_articles,
            "today_non_articles": today_non_articles,
            "old_articles": old_articles,
            "overlap_ratio": 0.0,
            "reason": "one date surfaced articles, the other didn't — "
                      "asymmetric but plausible date walker.",
        }

    overlap = today_set & old_set
    # Overlap ratio: how much of the SMALLER set is shared. A genuine
    # date walker shares ~ 0 — articles from a year ago are not in
    # today's archive page. A fake walker (same page regardless of
    # date) shares ~ 1.0.
    overlap_ratio = len(overlap) / min(len(today_set), len(old_set))

    if overlap_ratio > 0.5:
        return {
            "is_walker": False,
            "today_url": today_url,
            "old_url": old_url,
            "today_articles": today_articles,
            "today_non_articles": today_non_articles,
            "old_articles": old_articles,
            "overlap_ratio": overlap_ratio,
            "reason": f"today's and 1y-old's article-link sets overlap by "
                      f"{overlap_ratio:.0%} — the ?date= parameter is being "
                      "ignored by the publisher (not a real date walker).",
        }
    return {
        "is_walker": True,
        "today_url": today_url,
        "old_url": old_url,
        "today_articles": today_articles,
        "old_articles": old_articles,
        "overlap_ratio": overlap_ratio,
        "reason": f"today vs. 1y-old overlap is {overlap_ratio:.0%} — "
                  "distinct content per date, confirmed date walker.",
    }


# CMS-family → conventional article URL pattern. When the strict
# inference rejects its own candidate, we fall back to suggesting one
# of these based on the homepage's `<meta name="generator">` tag.
# Each template uses `{HOST}` as a placeholder for the publisher's
# escaped host string. Patterns are conservative — they target the
# CMS's canonical article-URL form, not every URL that happens to be
# generated by that CMS.
CMS_PATTERN_TEMPLATES: dict[str, list[tuple[str, str]]] = {
    # WordPress permalink default = /YYYY/MM/dd/slug/ (and variants
    # without day or month). Some sites use ?p=N "ugly" permalinks.
    "wordpress": [
        (
            "WordPress /YYYY/MM/DD/slug/ permalink",
            r"^https?://(www\.)?{HOST}/\d{{4}}/\d{{2}}/\d{{2}}/[^/]+/?$",
        ),
        (
            "WordPress /YYYY/MM/slug/ permalink",
            r"^https?://(www\.)?{HOST}/\d{{4}}/\d{{2}}/[^/]+/?$",
        ),
        (
            "WordPress ?p= ugly permalink",
            r"^https?://(www\.)?{HOST}/\?p=\d+$",
        ),
    ],
    # Drupal default content URL = /node/N. Some sites pretty-URL it
    # to /article/<slug> or similar — second pattern covers that.
    "drupal": [
        ("Drupal /node/N", r"^https?://(www\.)?{HOST}/node/\d+$"),
        ("Drupal /article/slug", r"^https?://(www\.)?{HOST}/article/[^/?#]+$"),
    ],
    # Joomla canonical = /index.php?option=com_content&...&id=N or
    # /<category>/<slug>-N.html under SEF.
    "joomla": [
        ("Joomla SEF /category/slug-N.html", r"^https?://(www\.)?{HOST}/[^?#]+-\d+\.html$"),
        ("Joomla com_content", r"^https?://(www\.)?{HOST}/index\.php\?.*id=\d+"),
    ],
    # TYPO3 — canonical varies wildly; common is /<path>/<slug>-N.html
    # or /breg-de/-style locale prefix as bundesregierung does.
    "typo3": [
        ("TYPO3 /<path>/<slug>-N.html", r"^https?://(www\.)?{HOST}/[^?#]+-\d+\.html$"),
        ("TYPO3 /<path>/<slug>-N (no extension)", r"^https?://(www\.)?{HOST}/[^?#]+-\d+/?$"),
    ],
    # CoreMedia (the German publisher backbone — used by tagesschau,
    # ARD, ZDF). Canonical = /<section>/<slug>-NNN.html.
    "coremedia": [
        ("CoreMedia /<section>/<slug>-NNN.html", r"^https?://(www\.)?{HOST}/[^?#]+-\d+\.html$"),
    ],
    # Ghost — canonical /<slug>/.
    "ghost": [
        ("Ghost /slug/", r"^https?://(www\.)?{HOST}/[^/?#]+/?$"),
    ],
    # Hugo / Jekyll / Wagtail — generic /YYYY/MM/DD/slug/ or
    # /YYYY/MM/slug.html static-site convention.
    "hugo": [
        ("Static /YYYY/MM/DD/slug/", r"^https?://(www\.)?{HOST}/\d{{4}}/\d{{2}}/\d{{2}}/[^/]+/?$"),
    ],
    "jekyll": [
        ("Static /YYYY/MM/DD/slug.html", r"^https?://(www\.)?{HOST}/\d{{4}}/\d{{2}}/\d{{2}}/[^/]+\.html$"),
    ],
    "wagtail": [
        ("Wagtail /<section>/<slug>/", r"^https?://(www\.)?{HOST}/[^/]+/[^/]+/?$"),
    ],
}


def cms_pattern_suggestions(
    cms_family: Optional[str],
    homepage_origin: str,
) -> list[tuple[str, str]]:
    """Return ``[(label, regex), ...]`` candidates the operator can
    pick from when the strict auto-inference rejects its own
    candidate. Empty list when no CMS hint is available.
    """
    if not cms_family:
        return []
    family = cms_family.lower()
    templates = CMS_PATTERN_TEMPLATES.get(family)
    if not templates:
        return []
    parsed = urlparse(homepage_origin)
    host = (parsed.netloc or "").lower().removeprefix("www.")
    if not host:
        return []
    escaped_host = re.escape(host)
    return [(label, tmpl.format(HOST=escaped_host)) for label, tmpl in templates]


def _extract_non_article_links(
    body: str,
    base_url: str,
) -> list[str]:
    """Return the links on a page that ARE excluded by the article-URL
    heuristic — i.e. navigation / footer / asset / cross-domain links.
    Used as the anti-match validation set for pattern inference: a
    well-formed `article_url_pattern` must reject ALL of these.
    """
    if not body:
        return []
    parsed_base = urlparse(base_url)
    if not parsed_base.netloc:
        return []
    base_host = parsed_base.netloc.lower().removeprefix("www.")
    self_canon = base_url.rstrip("/").lower()
    seen: set[str] = set()
    out: list[str] = []
    for match in _A_HREF_RE.finditer(body):
        href = match.group(1).strip()
        if not href or href.startswith(("mailto:", "tel:", "javascript:", "#")):
            continue
        absolute = urljoin(base_url, href)
        parsed = urlparse(absolute)
        if parsed.scheme not in ("http", "https"):
            continue
        canon = absolute.rstrip("/").lower()
        if canon in seen:
            continue
        seen.add(canon)
        # An entry is a "non-article link" if it would be filtered out
        # by _extract_article_url_candidates. Mirror those conditions:
        host = parsed.netloc.lower().removeprefix("www.")
        path = parsed.path or ""
        is_non_article = (
            host != base_host
            or canon == self_canon
            or not path
            or path == "/"
            or path.lower().endswith(_ASSET_EXTENSIONS)
            or path.count("/") < 2
        )
        if is_non_article:
            out.append(absolute)
    return out


def validate_inferred_pattern(
    pattern: str,
    *,
    article_urls: list[str],
    non_article_urls: list[str],
) -> dict[str, Any]:
    """Quantitatively validate an inferred ``article_url_pattern``
    regex against two URL sets:

    * ``article_urls`` — URLs the audit identified as article candidates.
      A good pattern matches **all** of these (recall = 100 %).
    * ``non_article_urls`` — navigation / footer / asset links on the
      same page. A good pattern matches **none** of these (false-
      positive rate = 0 %).

    Returns counts + example match / miss URLs so the operator can
    eyeball the validation result. The caller decides whether to write
    the pattern to YAML based on these numbers; the helper itself
    never writes.
    """
    try:
        compiled = re.compile(pattern)
    except re.error as exc:
        return {
            "valid": False,
            "reason": f"pattern does not compile: {exc}",
            "article_matched": 0,
            "article_total": len(article_urls),
            "non_article_matched": 0,
            "non_article_total": len(non_article_urls),
            "matched_articles": [],
            "missed_articles": list(article_urls)[:5],
            "false_positives": [],
        }

    matched_articles = [u for u in article_urls if compiled.match(u)]
    missed_articles = [u for u in article_urls if not compiled.match(u)]
    false_positives = [u for u in non_article_urls if compiled.match(u)]
    return {
        "valid": True,
        "reason": "pattern compiled and evaluated.",
        "article_matched": len(matched_articles),
        "article_total": len(article_urls),
        "non_article_matched": len(false_positives),
        "non_article_total": len(non_article_urls),
        "matched_articles": matched_articles[:5],
        "missed_articles": missed_articles[:5],
        "false_positives": false_positives[:5],
    }


def infer_safe_pattern(
    sample_articles: list[str],
    sample_non_articles: list[str],
    homepage_origin: str,
) -> tuple[Optional[str], dict[str, Any]]:
    """Try to derive an ``article_url_pattern`` regex that is safe to
    write into ``sources.yaml`` without operator review.

    A pattern is considered "safe" only when ALL of these hold:

    1. It's derived from the conservative pattern-1 heuristic (the
       narrow ``-NNN.html`` slug-with-numeric-id convention). Pattern-2
       and pattern-3 are deliberately NOT auto-applied — they're more
       permissive and have produced false positives in informal tests.
    2. It matches 100 % of the article-shaped sample URLs (no silent
       under-matching).
    3. It matches 0 % of the non-article-shaped sample URLs that the
       audit observed on the same page (no silent over-matching).

    Returns ``(pattern_or_None, diagnostic)``. When ``pattern`` is
    ``None`` the diagnostic explains why — used by the YAML applier to
    emit ``EDIT-ME-REGEX-...`` with a helpful comment instead.
    """
    # Run the inference but only accept its narrowest output. We re-run
    # internally so we can inspect what pattern would have been chosen
    # even if the conservative gate rejects it.
    pattern = _infer_article_url_pattern(sample_articles, homepage_origin)
    if not pattern:
        return None, {
            "rejected_reason":
                "no consistent slug-NNN.html pattern across the sample — "
                "publisher's URL convention is non-standard.",
            "sample_article_count": len(sample_articles),
        }

    # Only accept pattern-1 shape (slug-NNN.html). Pattern-2 / pattern-3
    # shapes are detectable by structure: pattern-1 always contains
    # the literal ``-\d+\.html`` token. Anything else → reject for
    # auto-apply.
    if r"-\d+\.html$" not in pattern:
        return None, {
            "rejected_reason":
                "inferred pattern is permissive (no slug-NNN.html "
                "convention detected) — would not auto-apply.",
            "inferred_pattern": pattern,
            "sample_article_count": len(sample_articles),
        }

    val = validate_inferred_pattern(
        pattern,
        article_urls=sample_articles,
        non_article_urls=sample_non_articles,
    )
    if not val["valid"]:
        return None, {
            "rejected_reason": f"pattern compile failed: {val['reason']}",
            "inferred_pattern": pattern,
        }
    if val["article_matched"] != val["article_total"]:
        return None, {
            "rejected_reason":
                f"pattern matches only {val['article_matched']}/"
                f"{val['article_total']} sampled articles (recall < 100 %).",
            "inferred_pattern": pattern,
            "diagnostic": val,
        }
    if val["non_article_matched"] > 0:
        return None, {
            "rejected_reason":
                f"pattern false-positively matches "
                f"{val['non_article_matched']}/{val['non_article_total']} "
                "non-article links on the same page.",
            "inferred_pattern": pattern,
            "diagnostic": val,
        }
    # 100 % recall, 0 % false-positives on the observed sample.
    return pattern, {
        "accepted": True,
        "diagnostic": val,
        "sample_article_count": len(sample_articles),
        "sample_non_article_count": len(sample_non_articles),
    }


def _detect_cms(homepage_html: str) -> Optional[str]:
    """Return the publisher's CMS family if its `<meta name="generator">`
    declares one. Useful as a heuristic hint for the operator."""
    if not homepage_html:
        return None
    match = _GENERATOR_META_RE.search(homepage_html[:8192])
    if not match:
        return None
    raw = match.group(1).strip()
    # Normalise common families.
    lowered = raw.lower()
    for family in ("wordpress", "drupal", "joomla", "typo3", "coremedia",
                   "ghost", "hugo", "jekyll", "wagtail"):
        if family in lowered:
            return family
    return raw[:60]


def audit_source(
    homepage: str,
    *,
    http_get=None,
    timeout: float = DEFAULT_TIMEOUT,
    verbose: bool = False,
) -> dict[str, Any]:
    """Run the audit and return a structured report (no I/O on stdout)."""
    http_get = http_get or requests.get
    parsed = urlparse(homepage)
    if not parsed.scheme or not parsed.netloc:
        raise ValueError(f"invalid homepage URL: {homepage!r}")
    origin = f"{parsed.scheme}://{parsed.netloc}"

    report: dict[str, Any] = {
        "homepage": homepage,
        "origin": origin,
    }

    # Homepage fetch — used for CMS detection AND for parsing
    # `<link rel="alternate">` feed declarations the publisher inlines
    # on their root page (the standard discoverability path many CMSes
    # still emit, distinct from the catalogue-page approach below).
    homepage_body, _ = _fetch_homepage(homepage, http_get, timeout)
    report["cms_detected"] = _detect_cms(homepage_body)
    homepage_inline_feeds = (
        _extract_feed_links_from_catalogue(homepage_body, homepage)
        if homepage_body else []
    )
    report["homepage_inline_feeds"] = homepage_inline_feeds

    # Trafilatura feed + sitemap auto-discovery.
    feed_urls = _try_trafilatura_feeds(homepage)
    sitemap_urls = _try_trafilatura_sitemaps(homepage)
    report["trafilatura_feeds_found"] = (
        feed_urls if feed_urls is not None else "skipped — trafilatura not installed"
    )
    report["trafilatura_sitemaps_found"] = (
        sitemap_urls if sitemap_urls is not None else "skipped — trafilatura not installed"
    )

    # Direct RSS-path probes — convention-based fallback when neither
    # trafilatura nor `<link rel="alternate">` surfaces the feed.
    report["rss_path_hits"] = _probe_rss_paths(
        origin, http_get, timeout, verbose=verbose
    )

    # Catalogue-page parsing — publishers like bundesregierung publish
    # a /service/newsletter-und-abos/... page that enumerates several
    # official feeds, none of which is advertised via `<link rel="alternate">`.
    report["rss_catalogue_hits"] = _probe_rss_catalogues(
        origin, http_get, timeout, verbose=verbose
    )

    # HTML sitemap probes. For each candidate path that returns 200 +
    # HTML, fetch the body and run the article-listing sanity check
    # (≥ 5 article-shaped links). Pages that pass the 200 / size check
    # but fail the content check (typical for landing pages that
    # happen to live at /sitemap) are rejected — they would lead the
    # operator to configure a "channel" that produces zero articles.
    html_sitemap_hits: list[dict[str, Any]] = []
    for path in HTML_SITEMAP_CANDIDATES:
        probe_url = urljoin(origin + "/", path.lstrip("/"))
        status, body = _fetch_body(probe_url, http_get, timeout)
        if status != 200 or "<html" not in body.lower()[:512] or len(body) < 1024:
            if verbose:
                html_sitemap_hits.append({
                    "url": probe_url,
                    "status": status,
                    "skipped": True,
                })
            continue
        sanity = _validate_article_listing_page(body, probe_url)
        if not sanity["is_listing"]:
            if verbose:
                html_sitemap_hits.append({
                    "url": probe_url,
                    "status": status,
                    "rejected_reason": sanity["reason"],
                    "skipped": True,
                })
            continue
        non_articles = _extract_non_article_links(body, probe_url)
        html_sitemap_hits.append({
            "url": probe_url,
            "body_size": len(body),
            "article_url_sample": sanity["article_urls"][:30],
            "non_article_url_sample": non_articles[:30],
            "article_count": len(sanity["article_urls"]),
            "sanity_note": sanity["reason"],
        })
    report["html_sitemap_candidates"] = html_sitemap_hits

    # Archive-walker probes — for each pattern, run the date-walker
    # verification (today vs. 1y-old: must surface distinct article
    # link sets). Only patterns that pass the verification become
    # candidates. This is the load-bearing check that prevents the
    # bundesregierung-style false positive where `?datum=...` is
    # silently ignored and the same generic navigation page is
    # returned for every date.
    from datetime import datetime, timezone
    now = datetime.now(tz=timezone.utc)
    today = now.strftime("%Y-%m-%d")
    archive_index_hits: list[dict[str, Any]] = []
    for pattern in ARCHIVE_INDEX_CANDIDATES:
        probe_path = (
            pattern
            .replace("{date}", today)
            .replace("{year}", now.strftime("%Y"))
            .replace("{month}", now.strftime("%m"))
            .replace("{day}", now.strftime("%d"))
        )
        probe_url = urljoin(origin + "/", probe_path.lstrip("/"))
        result = _probe_http(probe_url, http_get, timeout)
        if (
            result.get("status") != 200
            or not result.get("has_html_tag")
            or result.get("body_size", 0) <= 1024
        ):
            if verbose:
                archive_index_hits.append({
                    "url_template": pattern,
                    "status": result.get("status"),
                    "skipped": True,
                })
            continue

        verification = _verify_date_walker(
            pattern,
            origin=origin,
            http_get=http_get,
            timeout=timeout,
            today=now,
        )
        if not verification["is_walker"]:
            if verbose:
                archive_index_hits.append({
                    "url_template": pattern,
                    "rejected_reason": verification["reason"],
                    "today_url": verification["today_url"],
                    "old_url": verification["old_url"],
                    "overlap_ratio": verification["overlap_ratio"],
                    "skipped": True,
                })
            continue

        archive_index_hits.append({
            "url_template": pattern,
            "sample_url": probe_url,
            "body_size": result["body_size"],
            "article_url_sample": verification["today_articles"][:30],
            "non_article_url_sample":
                (verification.get("today_non_articles") or [])[:30],
            "article_count": len(verification["today_articles"]),
            "date_walker_overlap_ratio": verification["overlap_ratio"],
            "today_url": verification["today_url"],
            "old_url": verification["old_url"],
            "sanity_note": verification["reason"],
        })
    report["archive_index_candidates"] = archive_index_hits

    return report


def extract_discovered_urls(report: dict[str, Any]) -> dict[str, list[str]]:
    """Roll up the audit report into the four ``discovery:`` channel
    URL sets the runtime configuration uses.

    Returns a dict with keys ``sitemap_urls``, ``rss_hint_urls``,
    ``html_sitemap_urls``, ``archive_index_urls`` mapping to sorted URL
    lists. Used by the re-audit diff path to compare audit-discovered
    surfaces against the operator-configured set.

    Note: ``article_url_pattern`` regexes inside ``html_sitemap_urls`` /
    ``archive_index`` are publisher-specific and NEVER generated by the
    audit — they remain operator-authored.
    """
    sitemap_urls: set[str] = set()
    sm = report.get("trafilatura_sitemaps_found")
    if isinstance(sm, list):
        sitemap_urls.update(sm)

    rss_urls: set[str] = set()
    # Direct RSS-path hits — content-sniffed, high-confidence.
    for hit in report.get("rss_path_hits") or []:
        if not hit.get("skipped") and hit.get("url"):
            rss_urls.add(hit["url"])
    # Catalogue-page-discovered feeds — publisher-curated catalogue,
    # high-confidence (the publisher chose to enumerate them).
    for hit in report.get("rss_catalogue_hits") or []:
        if not hit.get("skipped"):
            for feed in hit.get("discovered_feeds") or []:
                rss_urls.add(feed)
    # Homepage `<link rel="alternate">` feeds — same confidence as
    # catalogue, just a different advertising surface.
    for feed in report.get("homepage_inline_feeds") or []:
        rss_urls.add(feed)

    html_sitemap_urls: set[str] = set()
    for hit in report.get("html_sitemap_candidates") or []:
        if not hit.get("skipped") and hit.get("url"):
            html_sitemap_urls.add(hit["url"])

    archive_index_urls: set[str] = set()
    for hit in report.get("archive_index_candidates") or []:
        if not hit.get("skipped"):
            tmpl = hit.get("url_template")
            if tmpl:
                # Normalise to the origin-prefixed form.
                if tmpl.startswith("/"):
                    archive_index_urls.add(report.get("origin", "") + tmpl)
                else:
                    archive_index_urls.add(tmpl)

    return {
        "sitemap_urls": sorted(sitemap_urls),
        "rss_hint_urls": sorted(rss_urls),
        "html_sitemap_urls": sorted(html_sitemap_urls),
        "archive_index_urls": sorted(archive_index_urls),
    }


def _format_yaml_suggestion(report: dict[str, Any]) -> str:
    """Render the report as a `discovery:` block ready for sources.yaml."""
    lines = ["# Suggested sources.yaml `discovery:` block — review before committing."]
    lines.append("# Generated by `audit_source.py` (Phase 122g).")
    lines.append("#")
    lines.append(f"# Audited: {report['homepage']}")
    lines.append("#")
    lines.append("discovery:")

    sm = report.get("trafilatura_sitemaps_found")
    if isinstance(sm, list) and sm:
        lines.append("  sitemap_urls:")
        for url in sm[:8]:
            lines.append(f"    - {url}")
        if len(sm) > 8:
            lines.append(f"    # ... and {len(sm) - 8} more reported by trafilatura.sitemaps.sitemap_search")
    else:
        lines.append("  sitemap_urls: []   # none surfaced by auto-discovery")

    # Trafilatura's `find_feed_urls` returns *article* URLs from the
    # discovered feeds, not feed URLs themselves. We can't directly
    # populate `rss_hint_urls` from it. Operator must visit the
    # publisher's RSS catalogue page manually.
    feeds = report.get("trafilatura_feeds_found")
    if isinstance(feeds, list) and feeds:
        lines.append("  rss_hint_urls:")
        lines.append("    # IMPORTANT: trafilatura returns article URLs, not feed URLs.")
        lines.append("    # Visit the publisher's RSS / newsletter page and enumerate")
        lines.append("    # the actual feed URLs. Many publishers (e.g. bundesregierung)")
        lines.append("    # publish a feed catalogue at /service/newsletter-und-abos or similar.")
        lines.append(f"    # trafilatura surfaced {len(feeds)} article URL(s) from its feed walk.")
        lines.append("    # - https://example.com/rss/feed.xml")
    else:
        lines.append("  rss_hint_urls: []  # operator: enumerate feeds from publisher catalogue page")

    html_hits = report.get("html_sitemap_candidates") or []
    if html_hits:
        lines.append("  html_sitemap_urls:")
        for hit in html_hits:
            if hit.get("skipped"):
                continue
            lines.append(f"    - url: {hit['url']}")
            lines.append("      article_url_pattern: '<edit-me — regex matching article URLs only>'")
    else:
        lines.append("  html_sitemap_urls: []  # no publisher-built HTML sitemap found at common paths")

    archive_hits = report.get("archive_index_candidates") or []
    if archive_hits:
        lines.append("  archive_index:")
        for hit in archive_hits:
            if hit.get("skipped"):
                continue
            lines.append(f"    url_template: {hit['url_template']}")
            lines.append("    date_format: \"%Y-%m-%d\"")
            lines.append("    granularity: daily   # operator: verify daily vs monthly by sampling two dates")
            lines.append("    article_url_pattern: '<edit-me — regex matching article URLs only>'")
            break  # one block; operator picks one if multiple matched
    else:
        lines.append("  # archive_index: not detected at common paths")

    lines.append("  expected_floor_per_run: <edit-me — operator-set after first run>")

    lines.append("")
    lines.append("# ──────────────────────────────────────────────────────────────────────")
    lines.append("# Manual cross-check (~5 min) — no automated tool finds 100 % of a")
    lines.append("# publisher's discovery surfaces. Before committing, spend a few minutes")
    lines.append("# on the four checks below; they catch the surfaces this CLI cannot.")
    lines.append("# ──────────────────────────────────────────────────────────────────────")
    lines.append("#")
    lines.append("# A. Mediacloud registry (the canonical public source list).")
    lines.append("#    Search https://search.mediacloud.org for this publisher. If it is")
    lines.append("#    already catalogued, compare their curated feed list against the")
    lines.append("#    audit output above — any feed they have that we don't is a hint.")
    lines.append("#")
    lines.append("# B. Publisher footer / 'RSS' / 'Feed' / 'Newsletter' links.")
    lines.append("#    Open the homepage in a browser and scroll to the footer. Almost")
    lines.append("#    every news site exposes RSS / feed / newsletter / subscription")
    lines.append("#    links there. Some publishers (e.g. bundesregierung) host a")
    lines.append("#    multi-feed catalogue page at /service/newsletter-und-abos/...")
    lines.append("#    that auto-discovery cannot locate — but a human eye spots it in")
    lines.append("#    seconds.")
    lines.append("#")
    lines.append("# C. robots.txt — inspect by hand.")
    lines.append("#      curl -s <homepage>/robots.txt | grep -iE 'sitemap|feed|rss'")
    lines.append("#    The publisher may declare a Sitemap: directive pointing to a")
    lines.append("#    non-standard location this CLI did not probe (we hit the common")
    lines.append("#    paths, not every possible one).")
    lines.append("#")
    lines.append("# D. Distinguish format-duplicates from real coverage gains.")
    lines.append("#    If the audit found multiple feeds (e.g. .rss + ~atom.xml +")
    lines.append("#    ~rdf.xml all on the same path stem), they are usually format")
    lines.append("#    variants of the SAME article set — adding them all costs HTTP")
    lines.append("#    politeness budget without any coverage gain. Keep one format,")
    lines.append("#    drop the others. The publisher's RSS catalogue page (B) is what")
    lines.append("#    usually surfaces feeds with genuinely different content.")
    lines.append("#")
    lines.append("# Then:")
    lines.append("# 1. Edit the `article_url_pattern` regex(es) above to match the publisher's")
    lines.append("#    article URL convention (sample a few real article URLs to derive it).")
    lines.append("# 2. Verify the archive_index granularity by curling two distant dates.")
    lines.append("# 3. Run `make crawl-<probe-id>` and observe the per-channel telemetry to set")
    lines.append("#    `expected_floor_per_run` from the empirical baseline.")
    lines.append("#")
    lines.append("# Re-audit cadence: run `make audit-probe PROBE=<probe-id>` periodically")
    lines.append("# (e.g. weekly) — the diff workflow surfaces additive publisher-surface")
    lines.append("# drift without requiring this onboarding workflow to be repeated.")
    return "\n".join(lines)


def diff_against_configured(
    discovered: dict[str, list[str]],
    configured: dict[str, Any],
) -> dict[str, list[str]]:
    """Compare audit-discovered URL sets against the operator-configured
    ``discovery:`` block; return only the **additive** delta (URLs the
    audit surfaced that the configuration lacks).

    Removals are NEVER reported — publisher-surface disappearance is a
    methodological event (the underflow-alert telemetry handles it),
    not a routine maintenance trigger. The audit is conservative: an
    empty diff means "no NEW surfaces found", not "configuration is
    correct".

    Excludes URLs that the operator explicitly filtered (e.g.
    bundesregierung's three service-sitemaps are configured but the
    audit also returns them — they're not "new"). Also dedupes by
    canonical equivalence (trailing-slash, scheme).
    """
    def _canon(u: str) -> str:
        return u.rstrip("/").lower()

    diff: dict[str, list[str]] = {
        "sitemap_urls": [],
        "rss_hint_urls": [],
        "html_sitemap_urls": [],
        "archive_index_urls": [],
    }

    configured_sitemaps = {
        _canon(u) for u in (configured.get("sitemap_urls") or [])
    }
    for u in discovered.get("sitemap_urls", []):
        if _canon(u) not in configured_sitemaps:
            diff["sitemap_urls"].append(u)

    configured_rss = {
        _canon(u) for u in (configured.get("rss_hint_urls") or [])
    }
    # Forgive a legacy singular `rss_hint_url`.
    single = configured.get("rss_hint_url")
    if single:
        configured_rss.add(_canon(single))
    for u in discovered.get("rss_hint_urls", []):
        if _canon(u) not in configured_rss:
            diff["rss_hint_urls"].append(u)

    configured_html = {
        _canon(entry["url"] if isinstance(entry, dict) else entry)
        for entry in (configured.get("html_sitemap_urls") or [])
        if (entry.get("url") if isinstance(entry, dict) else entry)
    }
    for u in discovered.get("html_sitemap_urls", []):
        if _canon(u) not in configured_html:
            diff["html_sitemap_urls"].append(u)

    # Archive-index is a single block (url_template) per source, not a
    # list. We report it only if no archive_index block is configured AND
    # the audit found at least one.
    configured_archive = configured.get("archive_index")
    if not configured_archive:
        diff["archive_index_urls"] = list(discovered.get("archive_index_urls", []))

    return diff


def render_diff(diff: dict[str, list[str]], *, color: bool = True) -> str:
    """Render the additive diff as a human-readable string. If ``color``
    is true, additions are shown in ANSI green."""
    GREEN = "\033[32m" if color else ""
    BOLD = "\033[1m" if color else ""
    RESET = "\033[0m" if color else ""

    lines: list[str] = []
    total = 0
    for channel, urls in diff.items():
        if not urls:
            continue
        lines.append(f"{BOLD}{channel}{RESET}: {len(urls)} new")
        for url in urls:
            lines.append(f"  {GREEN}+ {url}{RESET}")
        total += len(urls)
    if total == 0:
        lines.append("(no new surfaces — configuration covers all audit-discovered URLs)")
    return "\n".join(lines)


def _prompt_yes_no(question: str, *, default_no: bool = True) -> bool:
    """Prompt the user with ``[y/N]`` (or ``[Y/n]`` if default_no is
    false). Empty input applies the default. Anything starting with
    ``y`` / ``j`` / ``Y`` / ``J`` is treated as yes. Non-interactive
    stdin (closed) returns the default."""
    suffix = "[y/N]" if default_no else "[Y/n]"
    try:
        answer = input(f"{question} {suffix} ").strip().lower()
    except EOFError:
        return not default_no
    if not answer:
        return not default_no
    return answer[0] in ("y", "j")


def apply_diff_to_yaml(
    yaml_path: Path,
    source_name: str,
    diff: dict[str, list[str]],
    *,
    backup_suffix: str = ".bak",
    pattern_samples: Optional[dict[str, dict[str, list[str]]]] = None,
    homepage_origin: Optional[str] = None,
) -> dict[str, int]:
    """Apply the additive diff to the named source's ``discovery:``
    block in ``yaml_path``. Preserves comments + formatting via
    ``ruamel.yaml``. Writes ``yaml_path.bak`` before mutating.

    Returns a dict ``{channel: count_added}`` for reporting.
    Idempotent: re-applying the same diff is a no-op.

    Pattern-Inferenz: ``pattern_samples`` maps html_sitemap / archive_index
    URLs to ``{"articles": [...], "non_articles": [...]}`` sample lists
    extracted by the audit. When supplied, the function calls
    :func:`infer_safe_pattern` for each new entry. A pattern is written
    to YAML only when it passes 100 % recall + 0 % false-positive on
    the sample (see :func:`infer_safe_pattern` for the safety contract).
    Otherwise the ``EDIT-ME-REGEX-MATCHING-ARTICLE-URLS`` placeholder
    remains — never a silently over-/under-matching pattern.
    """
    try:
        from ruamel.yaml import YAML  # type: ignore
    except ImportError as exc:
        raise RuntimeError(
            "ruamel.yaml is required for in-place YAML editing. "
            "Install with: pip install 'ruamel.yaml==0.18.*'"
        ) from exc

    yaml_rt = YAML(typ="rt")
    yaml_rt.preserve_quotes = True
    yaml_rt.indent(mapping=2, sequence=4, offset=2)

    with yaml_path.open("r", encoding="utf-8") as fh:
        data = yaml_rt.load(fh)

    sources = data.get("sources") or []
    target = None
    for src in sources:
        if src.get("name") == source_name:
            target = src
            break
    if target is None:
        raise ValueError(
            f"source {source_name!r} not found in {yaml_path}"
        )

    if "discovery" not in target or target["discovery"] is None:
        target["discovery"] = {}
    discovery = target["discovery"]

    added: dict[str, int] = {
        "sitemap_urls": 0,
        "rss_hint_urls": 0,
        "html_sitemap_urls": 0,
        "archive_index_urls": 0,
    }

    def _ensure_list(key: str) -> Any:
        if key not in discovery or discovery[key] is None:
            discovery[key] = []
        return discovery[key]

    for url in diff.get("sitemap_urls") or []:
        lst = _ensure_list("sitemap_urls")
        if url not in lst:
            lst.append(url)
            added["sitemap_urls"] += 1
    for url in diff.get("rss_hint_urls") or []:
        lst = _ensure_list("rss_hint_urls")
        if url not in lst:
            lst.append(url)
            added["rss_hint_urls"] += 1
    def _safe_pattern_for(url: str) -> str:
        """Return an inferred article_url_pattern if and only if it
        passes the strict 100 %-recall / 0 %-FP safety gate against the
        observed sample. Otherwise return the EDIT-ME placeholder."""
        if not pattern_samples or not homepage_origin:
            return "EDIT-ME-REGEX-MATCHING-ARTICLE-URLS"
        sample = pattern_samples.get(url)
        if not sample:
            return "EDIT-ME-REGEX-MATCHING-ARTICLE-URLS"
        pattern, _diag = infer_safe_pattern(
            sample.get("articles") or [],
            sample.get("non_articles") or [],
            homepage_origin,
        )
        return pattern or "EDIT-ME-REGEX-MATCHING-ARTICLE-URLS"

    for url in diff.get("html_sitemap_urls") or []:
        lst = _ensure_list("html_sitemap_urls")
        existing_urls = {
            (e.get("url") if isinstance(e, dict) else e)
            for e in lst
        }
        if url not in existing_urls:
            # The pattern is either the safety-gated auto-inferred regex
            # (100 % recall, 0 % false-positive on observed sample) OR
            # the EDIT-ME placeholder. There is no in-between — silent
            # over/under-matching is structurally excluded. The
            # placeholder is a syntactically invalid regex on purpose
            # so the crawler refuses to ingest the channel until a
            # real pattern is provided.
            lst.append({
                "url": url,
                "article_url_pattern": _safe_pattern_for(url),
            })
            added["html_sitemap_urls"] += 1
    # archive_index is a single block, not a list; we never overwrite
    # an existing one, only suggest the first audit-discovered template.
    if diff.get("archive_index_urls") and not discovery.get("archive_index"):
        first = diff["archive_index_urls"][0]
        # Strip the origin so the operator sees a clean url_template.
        parsed = urlparse(first)
        path_template = parsed.path
        if parsed.query:
            path_template += "?" + parsed.query
        discovery["archive_index"] = {
            "url_template": f"{parsed.scheme}://{parsed.netloc}{path_template}",
            "date_format": "%Y-%m-%d",
            "granularity": "daily",
            "article_url_pattern": _safe_pattern_for(first),
        }
        added["archive_index_urls"] = 1

    # Backup, then write.
    backup_path = yaml_path.with_suffix(yaml_path.suffix + backup_suffix)
    shutil.copy2(yaml_path, backup_path)
    with yaml_path.open("w", encoding="utf-8") as fh:
        yaml_rt.dump(data, fh)
    return added


def _load_sources_yaml(yaml_path: Path) -> dict[str, Any]:
    """Read a probe's ``sources.yaml`` and return the parsed dict."""
    import yaml as _yaml
    with yaml_path.open("r", encoding="utf-8") as fh:
        return _yaml.safe_load(fh) or {}


def _find_source_block(data: dict[str, Any], name: str) -> Optional[dict[str, Any]]:
    for src in data.get("sources") or []:
        if src.get("name") == name:
            return src
    return None


def _run_reaudit(
    *,
    yaml_path: Path,
    source_name: str,
    homepage: str,
    timeout: float,
    verbose: bool,
    auto_yes: bool,
    dry_run: bool,
    out=sys.stdout,
) -> int:
    """Re-audit one source: run audit, diff against YAML, optionally apply.

    Returns 0 for any valid operator workflow outcome (no diff,
    dry-run, declined, applied). Returns 2 only for actual errors
    (source missing, audit raised, IO failure). Operator decisions are
    not errors — they're the entire point of the y/N prompt.
    """
    data = _load_sources_yaml(yaml_path)
    source_block = _find_source_block(data, source_name)
    if source_block is None:
        print(
            f"error: source {source_name!r} not found in {yaml_path}",
            file=sys.stderr,
        )
        return 2

    try:
        report = audit_source(homepage, timeout=timeout, verbose=verbose)
    except ValueError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2

    discovered = extract_discovered_urls(report)
    configured = source_block.get("discovery") or {}
    diff = diff_against_configured(discovered, configured)
    has_changes = any(diff[k] for k in diff)

    # Build pattern-inference sample map per URL — pulled from the
    # html_sitemap_candidates / archive_index_candidates the audit
    # already populated. Used by apply_diff_to_yaml's safe-pattern gate.
    pattern_samples: dict[str, dict[str, list[str]]] = {}
    for hit in report.get("html_sitemap_candidates") or []:
        if hit.get("skipped"):
            continue
        pattern_samples[hit["url"]] = {
            "articles": hit.get("article_url_sample") or [],
            "non_articles": hit.get("non_article_url_sample") or [],
        }
    for hit in report.get("archive_index_candidates") or []:
        if hit.get("skipped"):
            continue
        tmpl = hit.get("url_template", "")
        if tmpl.startswith("/"):
            full_template = report.get("origin", "") + tmpl
        else:
            full_template = tmpl
        pattern_samples[full_template] = {
            "articles": hit.get("article_url_sample") or [],
            "non_articles": hit.get("non_article_url_sample") or [],
        }

    print(f"\n=== {source_name} ({homepage}) ===", file=out)
    print(render_diff(diff, color=out.isatty() if hasattr(out, "isatty") else False),
          file=out)

    if not has_changes:
        return 0

    # If html_sitemap / archive_index entries are in the diff, surface
    # the audit's verification context (sample URLs + auto-pattern
    # decision) so the operator sees exactly what will be written
    # before confirming.
    for url in (diff.get("html_sitemap_urls") or []) + (diff.get("archive_index_urls") or []):
        sample = pattern_samples.get(url) or {}
        articles = sample.get("articles") or []
        non_articles = sample.get("non_articles") or []
        if not articles:
            continue
        print(f"\n  for {url}:", file=out)
        print(f"    sample article links found on page ({len(articles)}):",
              file=out)
        for u in articles[:5]:
            print(f"      • {u}", file=out)
        if len(articles) > 5:
            print(f"      ... and {len(articles) - 5} more", file=out)
        if non_articles:
            print(f"    sample non-article links also on page ({len(non_articles)}):",
                  file=out)
            for u in non_articles[:3]:
                print(f"      • {u}", file=out)
        # Show the auto-pattern decision.
        pattern, diag = infer_safe_pattern(
            articles, non_articles, report.get("origin") or homepage,
        )
        if pattern:
            print(f"    auto-pattern: {pattern}", file=out)
            d = diag.get("diagnostic") or {}
            print(
                f"      validation: matched "
                f"{d.get('article_matched', '?')}/{d.get('article_total', '?')} "
                f"sample articles, "
                f"{d.get('non_article_matched', '?')}/{d.get('non_article_total', '?')} "
                f"non-article false-positives — will be written to YAML.",
                file=out,
            )
        else:
            print(
                f"    auto-pattern: NOT inferred — "
                f"{diag.get('rejected_reason', 'unknown')}",
                file=out,
            )
            if diag.get("inferred_pattern"):
                print(
                    f"      (candidate that was rejected: "
                    f"{diag['inferred_pattern']})",
                    file=out,
                )
            # Surface CMS-specific suggestions: when the audit detected
            # a CMS family from `<meta name="generator">`, evaluate the
            # corresponding canonical-URL patterns against the same
            # sample and show the operator their match scores. This is
            # the "you don't need to write the regex yourself" fallback.
            cms_family = report.get("cms_detected")
            suggestions = cms_pattern_suggestions(
                cms_family, report.get("origin") or homepage
            )
            if suggestions:
                print(
                    f"      Suggested patterns based on detected CMS "
                    f"({cms_family}):",
                    file=out,
                )
                for label, candidate in suggestions:
                    val = validate_inferred_pattern(
                        candidate,
                        article_urls=articles,
                        non_article_urls=non_articles,
                    )
                    print(
                        f"        • {label}",
                        file=out,
                    )
                    print(
                        f"            regex: {candidate}",
                        file=out,
                    )
                    if val["valid"]:
                        print(
                            f"            sample match: "
                            f"{val['article_matched']}/{val['article_total']} "
                            f"articles, "
                            f"{val['non_article_matched']}/{val['non_article_total']} "
                            f"non-article false-positives",
                            file=out,
                        )
                    else:
                        print(f"            (won't compile: {val['reason']})",
                              file=out)
                print(
                    "      → if one of these looks right, paste it into the "
                    "YAML manually (or write your own).",
                    file=out,
                )
            print(
                "      → EDIT-ME-REGEX-MATCHING-ARTICLE-URLS placeholder "
                "will be written; the crawler will REFUSE TO START until "
                "you replace it.",
                file=out,
            )
        print(
            "    verify yourself by opening the URL in a browser. If the "
            "shown sample links look like article URLs → accept; if "
            "they look like navigation / footer → decline.",
            file=out,
        )

    # Before the operator answers y/N, remind them of the two judgment
    # calls the tool cannot make: format-variant duplicates vs. genuine
    # coverage gain, and a manual catalogue-page / robots.txt spot-check
    # when something looks surprising. Kept terse — full guidance lives
    # in the onboarding-mode footer + docs/extending/add-a-source.md.
    print(
        "\nBefore accepting, judge each entry:\n"
        "  • Format duplicate? Multiple feeds on the same path stem\n"
        "    (.rss / ~atom.xml / ~rdf.xml) usually carry the SAME\n"
        "    articles in different XML dialects — accepting all costs\n"
        "    HTTP politeness budget for zero coverage gain.\n"
        "  • Genuine new surface? A feed on a distinct path / catalogue\n"
        "    page typically carries different content — worth adding.\n"
        "  • If unsure, check the publisher's footer / robots.txt\n"
        "    (`curl <homepage>/robots.txt`) or cross-reference\n"
        "    https://search.mediacloud.org.",
        file=out,
    )

    if dry_run:
        print("(dry-run — no changes written)", file=out)
        return 0
    if not auto_yes:
        if not _prompt_yes_no(
            f"Apply these additions to {yaml_path}?",
            default_no=True,
        ):
            print("declined — no changes written.", file=out)
            return 0
    added = apply_diff_to_yaml(
        yaml_path,
        source_name,
        diff,
        pattern_samples=pattern_samples,
        homepage_origin=report.get("origin") or homepage,
    )
    summary = ", ".join(f"{k}+{v}" for k, v in added.items() if v)
    print(f"wrote {summary} to {yaml_path} (backup: {yaml_path.with_suffix(yaml_path.suffix + '.bak')})",
          file=out)

    # Phase 122g — emit a HIGH-VISIBILITY red banner if any entry in
    # the just-written YAML still carries the EDIT-ME placeholder
    # (i.e. the strict auto-pattern gate rejected the inferred regex).
    # Combined with the runtime hard-stop in `internal/discovery/` the
    # operator gets two independent safety nets against silent
    # zero-ingestion: a loud CLI banner now, and a refuse-to-start at
    # the next `make crawl-<probe-id>`.
    if _yaml_contains_edit_me(yaml_path):
        print(_format_edit_me_warning(yaml_path), file=out)
    return 0


def _yaml_contains_edit_me(yaml_path: Path) -> bool:
    """Return True if the YAML file contains the audit-CLI placeholder.
    Cheap text scan — we don't need a YAML parser here."""
    try:
        with yaml_path.open("r", encoding="utf-8") as fh:
            return "EDIT-ME-REGEX-MATCHING-ARTICLE-URLS" in fh.read()
    except OSError:
        return False


def _format_edit_me_warning(yaml_path: Path) -> str:
    """High-visibility ANSI-red banner for an unresolved EDIT-ME-placeholder
    in sources.yaml. Mirrors the runtime hard-stop banner the crawler
    emits so the operator's mental model is consistent."""
    RED = "\033[1;31m"
    YEL = "\033[1;33m"
    RESET = "\033[0m"
    bar = "═" * 78
    return (
        f"\n{RED}{bar}{RESET}\n"
        f"{RED}  ⚠  ACTION REQUIRED — UNRESOLVED `article_url_pattern` in YAML  ⚠  {RESET}\n"
        f"{RED}{bar}{RESET}\n"
        f"\n"
        f"  {yaml_path} still contains one or more\n"
        f"  {YEL}article_url_pattern: EDIT-ME-REGEX-MATCHING-ARTICLE-URLS{RESET}\n"
        f"  entries. The audit CLI could not auto-infer a SAFE regex\n"
        f"  (100 % sample-recall + 0 % false-positives) and wrote the\n"
        f"  placeholder instead.\n"
        f"\n"
        f"  The crawler will REFUSE to start with this YAML\n"
        f"  (Phase 122g hard-stop in internal/discovery/__init__.py).\n"
        f"  This is intentional — silent zero-ingestion on a misconfigured\n"
        f"  channel would be far worse than a loud failure now.\n"
        f"\n"
        f"  To resolve, do ONE of the following:\n"
        f"    1. Open the relevant URL in a browser, sample 5–10 article\n"
        f"       URLs, derive a Python regex matching them, and replace\n"
        f"       the EDIT-ME placeholder.\n"
        f"    2. Re-run with `--verbose` to see which candidate pattern\n"
        f"       the audit considered + why it was rejected.\n"
        f"    3. Remove the offending html_sitemap_urls entry /\n"
        f"       archive_index block if you don't need that channel.\n"
        f"\n"
        f"  Backup of the previous YAML is at {yaml_path.with_suffix(yaml_path.suffix + '.bak')}.\n"
        f"{RED}{bar}{RESET}\n"
    )


def cli(argv: Optional[list[str]] = None) -> int:
    parser = argparse.ArgumentParser(
        prog="audit-source-discovery",
        description="Probe a candidate news source's discovery surfaces (Phase 122g).",
    )
    parser.add_argument(
        "homepage",
        nargs="?",
        help="The candidate source's homepage URL (e.g. https://www.tagesschau.de). "
             "Omit when using --probe.",
    )
    parser.add_argument(
        "--sources-yaml",
        type=Path,
        help="Path to the probe's sources.yaml. Activates re-audit / diff mode.",
    )
    parser.add_argument(
        "--source",
        help="Name of the source inside sources.yaml to re-audit (used with --sources-yaml).",
    )
    parser.add_argument(
        "--probe",
        type=Path,
        help="Path to a probe sources.yaml. Re-audits EVERY source in the probe; each source "
             "must declare `homepage_url:`. Prompts per source unless --yes is passed.",
    )
    parser.add_argument(
        "--yes",
        action="store_true",
        help="Auto-confirm all diffs (non-interactive). Use in CI / scripted runs.",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Show the diff but never write to YAML.",
    )
    parser.add_argument(
        "--verbose",
        action="store_true",
        help="Include non-200 probe results in the output (debugging only).",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Emit the raw audit report as JSON instead of the YAML suggestion "
             "(onboarding mode only — incompatible with --sources-yaml/--probe).",
    )
    parser.add_argument(
        "--timeout",
        type=float,
        default=DEFAULT_TIMEOUT,
        help=f"Per-probe HTTP timeout in seconds (default: {DEFAULT_TIMEOUT})",
    )
    args = parser.parse_args(argv)

    logging.basicConfig(level=logging.INFO, format="%(levelname)s %(name)s: %(message)s")

    # Mode 1: batch re-audit (--probe).
    if args.probe:
        if args.json or args.sources_yaml or args.source or args.homepage:
            print(
                "error: --probe is exclusive with --json/--sources-yaml/--source/<homepage>",
                file=sys.stderr,
            )
            return 2
        data = _load_sources_yaml(args.probe)
        exit_code = 0
        for src in data.get("sources") or []:
            name = src.get("name")
            homepage = src.get("homepage_url")
            if not name:
                continue
            if not homepage:
                print(
                    f"\n=== {name}: skipped (no `homepage_url:` declared in source block) ===",
                    file=sys.stderr,
                )
                continue
            rc = _run_reaudit(
                yaml_path=args.probe,
                source_name=name,
                homepage=homepage,
                timeout=args.timeout,
                verbose=args.verbose,
                auto_yes=args.yes,
                dry_run=args.dry_run,
            )
            if rc != 0:
                exit_code = rc  # only real errors (rc==2) propagate
        return exit_code

    # Mode 2: single re-audit (--sources-yaml + --source).
    if args.sources_yaml or args.source:
        if not (args.sources_yaml and args.source and args.homepage):
            print(
                "error: re-audit mode requires --sources-yaml, --source, AND a homepage argument.",
                file=sys.stderr,
            )
            return 2
        return _run_reaudit(
            yaml_path=args.sources_yaml,
            source_name=args.source,
            homepage=args.homepage,
            timeout=args.timeout,
            verbose=args.verbose,
            auto_yes=args.yes,
            dry_run=args.dry_run,
        )

    # Mode 3: onboarding (default — homepage positional argument only).
    if not args.homepage:
        parser.error("either <homepage> or --probe is required")

    try:
        report = audit_source(
            args.homepage,
            timeout=args.timeout,
            verbose=args.verbose,
        )
    except ValueError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2

    if args.json:
        print(json.dumps(report, indent=2, default=str))
    else:
        print(_format_yaml_suggestion(report))

    return 0


if __name__ == "__main__":
    sys.exit(cli())
