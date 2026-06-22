"""audit_source() orchestrator + discovered-URL extraction — extracted from audit_source.py (Phase 141)."""

import logging
from typing import Any
from urllib.parse import urljoin, urlparse

import requests

from audit_probe import (
    ARCHIVE_INDEX_CANDIDATES,
    DEFAULT_TIMEOUT,
    _extract_feed_links_from_catalogue,
    _fetch_body,
    _fetch_homepage,
    HTML_SITEMAP_CANDIDATES,
    _probe_http,
    _probe_rss_catalogues,
    _probe_rss_paths,
    _try_trafilatura_feeds,
    _try_trafilatura_sitemaps,
)
from audit_pattern import _detect_cms, _extract_non_article_links, _validate_article_listing_page
from audit_datewalk import _verify_date_walker

logger = logging.getLogger(__name__)


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
    homepage_inline_feeds = _extract_feed_links_from_catalogue(homepage_body, homepage) if homepage_body else []
    report["homepage_inline_feeds"] = homepage_inline_feeds

    # Trafilatura feed + sitemap auto-discovery.
    feed_urls = _try_trafilatura_feeds(homepage)
    sitemap_urls = _try_trafilatura_sitemaps(homepage)
    report["trafilatura_feeds_found"] = feed_urls if feed_urls is not None else "skipped — trafilatura not installed"
    report["trafilatura_sitemaps_found"] = (
        sitemap_urls if sitemap_urls is not None else "skipped — trafilatura not installed"
    )

    # Direct RSS-path probes — convention-based fallback when neither
    # trafilatura nor `<link rel="alternate">` surfaces the feed.
    report["rss_path_hits"] = _probe_rss_paths(origin, http_get, timeout, verbose=verbose)

    # Catalogue-page parsing — publishers like bundesregierung publish
    # a /service/newsletter-und-abos/... page that enumerates several
    # official feeds, none of which is advertised via `<link rel="alternate">`.
    report["rss_catalogue_hits"] = _probe_rss_catalogues(origin, http_get, timeout, verbose=verbose)

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
                html_sitemap_hits.append(
                    {
                        "url": probe_url,
                        "status": status,
                        "skipped": True,
                    }
                )
            continue
        sanity = _validate_article_listing_page(body, probe_url)
        if not sanity["is_listing"]:
            if verbose:
                html_sitemap_hits.append(
                    {
                        "url": probe_url,
                        "status": status,
                        "rejected_reason": sanity["reason"],
                        "skipped": True,
                    }
                )
            continue
        non_articles = _extract_non_article_links(body, probe_url)
        html_sitemap_hits.append(
            {
                "url": probe_url,
                "body_size": len(body),
                "article_url_sample": sanity["article_urls"][:30],
                "non_article_url_sample": non_articles[:30],
                "article_count": len(sanity["article_urls"]),
                "sanity_note": sanity["reason"],
            }
        )
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
            pattern.replace("{date}", today)
            .replace("{year}", now.strftime("%Y"))
            .replace("{month}", now.strftime("%m"))
            .replace("{day}", now.strftime("%d"))
        )
        probe_url = urljoin(origin + "/", probe_path.lstrip("/"))
        result = _probe_http(probe_url, http_get, timeout)
        if result.get("status") != 200 or not result.get("has_html_tag") or result.get("body_size", 0) <= 1024:
            if verbose:
                archive_index_hits.append(
                    {
                        "url_template": pattern,
                        "status": result.get("status"),
                        "skipped": True,
                    }
                )
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
                archive_index_hits.append(
                    {
                        "url_template": pattern,
                        "rejected_reason": verification["reason"],
                        "today_url": verification["today_url"],
                        "old_url": verification["old_url"],
                        "overlap_ratio": verification["overlap_ratio"],
                        "skipped": True,
                    }
                )
            continue

        archive_index_hits.append(
            {
                "url_template": pattern,
                "sample_url": probe_url,
                "body_size": result["body_size"],
                "article_url_sample": verification["today_articles"][:30],
                "non_article_url_sample": (verification.get("today_non_articles") or [])[:30],
                "article_count": len(verification["today_articles"]),
                "date_walker_overlap_ratio": verification["overlap_ratio"],
                "today_url": verification["today_url"],
                "old_url": verification["old_url"],
                "sanity_note": verification["reason"],
            }
        )
    report["archive_index_candidates"] = archive_index_hits

    # Phase 148d (WP-007 §6) — the onboarding completeness contract: record the
    # publisher-declared inventory this audit observed per channel, so the
    # source's `expected_floor_per_run` is a MEASURED starting point rather than
    # a hand-typed guess.
    report["completeness_baseline"] = _compute_completeness_baseline(report)

    return report


def _compute_completeness_baseline(report: dict[str, Any]) -> dict[str, Any]:
    """Phase 148d (WP-007 §6) — measure the publisher-declared inventory the
    audit observed, per channel, to seed a measured underflow floor.

    Each count is the number of article URLs the channel surfaced during this
    one-shot audit (trafilatura's sitemap/feed discovery returns article URLs;
    the HTML-sitemap / archive probes count article-shaped links). It is a
    point-in-time sample, NOT a guarantee — the runtime declared-denominator
    telemetry (the per-run `declared` column, Phase 148d) is the authoritative
    completeness signal; this baseline only seeds the floor so the
    two-consecutive-runs underflow alert is sane from the very first run.
    """
    per_channel: dict[str, int] = {}

    sm = report.get("trafilatura_sitemaps_found")
    if isinstance(sm, list):
        per_channel["sitemap"] = len(sm)
    feeds = report.get("trafilatura_feeds_found")
    if isinstance(feeds, list):
        per_channel["rss"] = len(feeds)

    def _max_article_count(key: str) -> "int | None":
        hits = report.get(key) or []
        counts = [int(h.get("article_count", 0) or 0) for h in hits if not h.get("skipped")]
        return max(counts) if counts else None

    html = _max_article_count("html_sitemap_candidates")
    if html is not None:
        per_channel["html_sitemap"] = html
    arch = _max_article_count("archive_index_candidates")
    if arch is not None:
        per_channel["archive_index"] = arch

    observed_total = sum(per_channel.values())
    # Conservative floor — half the observed inventory — so a genuine quiet day
    # does not false-fire the underflow alert. The runtime drift detector is the
    # precise signal; this is only a sane measured seed (WP-007 §5, §6).
    suggested_floor = observed_total // 2 if observed_total else 0
    return {
        "per_channel": per_channel,
        "observed_total": observed_total,
        "suggested_floor_per_run": suggested_floor,
    }


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
