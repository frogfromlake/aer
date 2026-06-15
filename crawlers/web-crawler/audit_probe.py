"""Discovery-probe helpers (HTTP/feed/sitemap/RSS) — extracted from audit_source.py (Phase 141)."""


import logging
import re
from typing import Any, Optional
from urllib.parse import urljoin


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


