"""AĒR audit-source-discovery CLI — Phase 122g.

Operator-facing tool that probes a candidate source's homepage and
reports the discovery channels the publisher exposes. Output is YAML-
shaped so the operator can paste the result directly into
``probes/<probe-id>/sources.yaml`` under a new source's ``discovery:``
block.

This is the source-onboarding equivalent of the silently-installed
``ldap-search`` tool an operator uses to discover what an LDAP server
exposes before configuring an LDAP-bound app. The audit happens
*once* per new source; runtime crawls then use the operator-curated
configuration, never the auto-discovery. That separation is the
load-bearing decision recorded in ADR-031: auto-discovery as a
config-authoring helper, not a runtime fallback.

Channels probed:
  * RSS / Atom feed auto-discovery via ``trafilatura.feeds.find_feed_urls``
    (``<link rel="alternate">`` + standard CMS paths). Trafilatura is
    optional — the audit gracefully degrades if it isn't installed
    (it lives in the worker venv, not the crawler venv).
  * XML sitemap auto-discovery via ``trafilatura.sitemaps.sitemap_search``
    (robots.txt + standard locations). Same optional-dep degradation.
  * HTML sitemap probes — common publisher paths that surface a
    navigation index in HTML (tagesschau's
    ``/infoservices/startseite-sitemap-*.html`` is the canonical
    example).
  * Date-indexed archive probes — common patterns publishers expose
    for date-walking (``?datum=YYYY-MM-DD``, ``?date=...``, ``/archiv``,
    ``/archive/...``).

Usage::

    python audit_source.py <homepage_url>
    python audit_source.py https://www.tagesschau.de
    python audit_source.py https://www.bundesregierung.de --verbose

Output is YAML printed to stdout. Operator reviews the suggested
``discovery:`` block, edits as needed (in particular: the
``article_url_pattern`` regex which is publisher-specific), and pastes
into ``sources.yaml``.

Cross-link: Mediacloud (https://search.mediacloud.org) maintains a
public registry of ~ 60,000 news sources with curated feed lists. If
the source already exists there, the CLI suggests importing.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys
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
    "/index/sitemap",
    "/site-map",
    "/sitemap-index",
    "/infoservices/startseite-sitemap-102.html",  # tagesschau pattern
    "/infoservices/sitemap-102.html",
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
]


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

    # Trafilatura feed + sitemap auto-discovery.
    feed_urls = _try_trafilatura_feeds(homepage)
    sitemap_urls = _try_trafilatura_sitemaps(homepage)
    report["trafilatura_feeds_found"] = (
        feed_urls if feed_urls is not None else "skipped — trafilatura not installed"
    )
    report["trafilatura_sitemaps_found"] = (
        sitemap_urls if sitemap_urls is not None else "skipped — trafilatura not installed"
    )

    # HTML sitemap probes.
    html_sitemap_hits: list[dict[str, Any]] = []
    for path in HTML_SITEMAP_CANDIDATES:
        probe_url = urljoin(origin + "/", path.lstrip("/"))
        result = _probe_http(probe_url, http_get, timeout)
        result["probed_url"] = probe_url
        # 200 + HTML body + size > 1 KB → operator-discoverable hit.
        if (
            result.get("status") == 200
            and result.get("has_html_tag")
            and result.get("body_size", 0) > 1024
        ):
            html_sitemap_hits.append({
                "url": probe_url,
                "body_size": result["body_size"],
                "content_type": result.get("content_type", ""),
            })
        elif verbose:
            html_sitemap_hits.append({
                "url": probe_url,
                "status": result.get("status"),
                "skipped": True,
            })
    report["html_sitemap_candidates"] = html_sitemap_hits

    # Archive-walker probes — try today's date in common patterns.
    from datetime import datetime, timezone
    today = datetime.now(tz=timezone.utc).strftime("%Y-%m-%d")
    archive_index_hits: list[dict[str, Any]] = []
    for pattern in ARCHIVE_INDEX_CANDIDATES:
        probe_path = pattern.replace("{date}", today)
        probe_url = urljoin(origin + "/", probe_path.lstrip("/"))
        result = _probe_http(probe_url, http_get, timeout)
        result["probed_url"] = probe_url
        if (
            result.get("status") == 200
            and result.get("has_html_tag")
            and result.get("body_size", 0) > 1024
        ):
            archive_index_hits.append({
                "url_template": pattern.replace("{date}", "{date}"),
                "sample_url": probe_url,
                "body_size": result["body_size"],
                # The operator must verify granularity (daily vs monthly)
                # by sampling two distant dates — see ADR-031.
                "granularity_note": "verify daily vs monthly by sampling two dates",
            })
        elif verbose:
            archive_index_hits.append({
                "url_template": pattern,
                "status": result.get("status"),
                "skipped": True,
            })
    report["archive_index_candidates"] = archive_index_hits

    return report


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
    lines.append("# Next steps:")
    lines.append("# 1. Edit the `article_url_pattern` regex(es) above to match the publisher's")
    lines.append("#    article URL convention (sample a few real article URLs to derive it).")
    lines.append("# 2. Verify the archive_index granularity by curling two distant dates.")
    lines.append("# 3. Cross-check Mediacloud (https://search.mediacloud.org) — if the source")
    lines.append("#    exists there, compare their curated feed list against this audit.")
    lines.append("# 4. Run `make crawl-<probe-id>` and observe the per-channel telemetry to set")
    lines.append("#    `expected_floor_per_run` from the empirical baseline.")
    return "\n".join(lines)


def cli(argv: Optional[list[str]] = None) -> int:
    parser = argparse.ArgumentParser(
        prog="audit-source-discovery",
        description="Probe a candidate news source's discovery surfaces (Phase 122g).",
    )
    parser.add_argument(
        "homepage",
        help="The candidate source's homepage URL (e.g. https://www.tagesschau.de)",
    )
    parser.add_argument(
        "--verbose",
        action="store_true",
        help="Include non-200 probe results in the output (debugging only).",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Emit the raw audit report as JSON instead of the YAML suggestion.",
    )
    parser.add_argument(
        "--timeout",
        type=float,
        default=DEFAULT_TIMEOUT,
        help=f"Per-probe HTTP timeout in seconds (default: {DEFAULT_TIMEOUT})",
    )
    args = parser.parse_args(argv)

    logging.basicConfig(level=logging.INFO, format="%(levelname)s %(name)s: %(message)s")

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
