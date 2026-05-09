"""Scrapy spider that fetches discovered URLs and pushes Bronze documents
to the ingestion API.

The spider is parameterised per source — politeness defaults, conditional
GET headers, and the URL/content technical filters are passed in by the
caller. Robots.txt is honoured via Scrapy's built-in
:class:`RobotsTxtMiddleware` (``ROBOTSTXT_OBEY = True``). Article body
extraction does not happen here: the fetched HTML is forwarded verbatim
through :mod:`internal.translate.contract` to Bronze.
"""

from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import Any, Iterator, Optional
from urllib.parse import urlsplit

import scrapy  # type: ignore
from scrapy.crawler import CrawlerProcess  # type: ignore
from scrapy.exceptions import IgnoreRequest  # type: ignore
from scrapy.http import Response  # type: ignore

from internal.discovery.sitemap import DiscoveredUrl
from internal.ingestion.client import IngestionClient
from internal.state.dedup import CrawlerState, content_hash
from internal.translate.contract import (
    FetchEnvelope,
    build_payload,
    canonical_url_or,
)

logger = logging.getLogger(__name__)


def _passes_url_filter(url: str, url_filter: dict[str, Any]) -> bool:
    """Apply technical-only URL filters. Section-level editorial filtering
    is rejected per WP-006 §3 and is therefore intentionally absent.
    """
    if not url_filter:
        return True
    parsed = urlsplit(url)
    path = (parsed.path or "").lower()

    for ext in url_filter.get("exclude_extensions", []) or []:
        if path.endswith(f".{ext.lower()}"):
            return False
    for prefix in url_filter.get("exclude_path_prefixes", []) or []:
        if path.startswith(prefix.lower()):
            return False
    return True


def _passes_content_type(headers: dict[str, str], require_html: bool) -> bool:
    if not require_html:
        return True
    content_type = (headers.get("content-type", "") or "").lower()
    return "text/html" in content_type or "application/xhtml" in content_type


class WebSpider(scrapy.Spider):
    """Scrapy spider for one source. Multiple sources mean multiple spider
    runs sequenced by :func:`run_crawl_for_source`.
    """

    name = "aer-web"

    def __init__(
        self,
        *,
        source_id: int,
        source_name: str,
        urls: list[DiscoveredUrl],
        url_filter: dict[str, Any],
        content_filter: dict[str, Any],
        custom_extractors: Optional[dict[str, Any]],
        state: CrawlerState,
        ingestion_client: IngestionClient,
        **kwargs: Any,
    ) -> None:
        super().__init__(**kwargs)
        self.source_id = source_id
        self.source_name = source_name
        self._urls = urls
        self._url_filter = url_filter or {}
        self._content_filter = content_filter or {}
        self._custom_extractors = custom_extractors
        self._state = state
        self._ingestion = ingestion_client
        self.submitted = 0
        self.skipped = 0
        self.errored = 0

    def start_requests(self) -> Iterator[scrapy.Request]:
        for entry in self._urls:
            url = entry.url
            canonical = canonical_url_or(url)

            if not _passes_url_filter(url, self._url_filter):
                self.skipped += 1
                continue

            if self._state.has_seen(self.source_id, canonical, entry.sitemap_lastmod):
                self.skipped += 1
                continue

            headers = self._state.conditional_headers(self.source_id, canonical)
            yield scrapy.Request(
                url=url,
                callback=self.parse_article,
                errback=self.errback,
                headers=headers,
                meta={
                    "canonical_url": canonical,
                    "sitemap_lastmod": entry.sitemap_lastmod,
                    "sitemap_section": entry.sitemap_section,
                },
                dont_filter=False,
            )

    def parse_article(self, response: Response) -> None:
        meta = response.meta
        canonical_url: str = meta["canonical_url"]
        sitemap_lastmod = meta.get("sitemap_lastmod")
        sitemap_section = meta.get("sitemap_section")

        # Conditional-GET path: 304 Not Modified → just refresh the
        # last_fetched stamp so future runs respect the new lastmod.
        if response.status == 304:
            self._state.record(
                source_id=self.source_id,
                canonical_url=canonical_url,
                etag=response.headers.get("ETag", b"").decode(errors="ignore") or None,
                http_last_modified=_parse_http_date(
                    response.headers.get("Last-Modified", b"").decode(errors="ignore")
                ),
                content_sha256=None,
                sitemap_lastmod=sitemap_lastmod,
            )
            self.skipped += 1
            return

        if response.status != 200:
            self.errored += 1
            logger.warning("non-200 status %s for %s", response.status, response.url)
            return

        response_headers = {
            k.decode(errors="ignore").lower(): (v[0].decode(errors="ignore") if v else "")
            for k, v in response.headers.items()
        }

        if not _passes_content_type(
            response_headers,
            self._url_filter.get("require_html_content_type", True),
        ):
            self.skipped += 1
            return

        html = response.text or ""
        if not html.strip():
            self.skipped += 1
            return

        # Cheap technical heuristic: word-count gate. Final decision is at
        # the worker (`require_extraction_success`), but a 200-byte 404
        # template is not worth Bronze storage.
        min_word_count = int(self._content_filter.get("min_word_count", 0) or 0)
        if min_word_count and len(html.split()) < min_word_count:
            self.skipped += 1
            return

        envelope = FetchEnvelope(
            source=self.source_name,
            original_url=response.url,
            canonical_url=canonical_url,
            fetch_at=datetime.now(tz=timezone.utc),
            http_status=response.status,
            response_headers=response_headers,
            sitemap_lastmod=sitemap_lastmod,
            sitemap_section=sitemap_section,
            custom_extractors=self._custom_extractors,
        )

        try:
            object_key, payload = build_payload(html, envelope)
        except ValueError as exc:
            logger.warning("build_payload skipped %s: %s", response.url, exc)
            self.errored += 1
            return

        body_hash = content_hash(html)
        try:
            self._ingestion.submit(self.source_id, object_key, payload)
        except Exception as exc:
            logger.error("ingestion submit failed for %s: %s", response.url, exc)
            self.errored += 1
            return

        self._state.record(
            source_id=self.source_id,
            canonical_url=canonical_url,
            etag=response_headers.get("etag") or None,
            http_last_modified=_parse_http_date(response_headers.get("last-modified")),
            content_sha256=body_hash,
            sitemap_lastmod=sitemap_lastmod,
        )
        self.submitted += 1

    def errback(self, failure: Any) -> None:
        # Most transport errors get auto-retried by Scrapy's retry middleware;
        # IgnoreRequest is raised for filtered-out URLs and is benign.
        if failure.check(IgnoreRequest):
            self.skipped += 1
            return
        logger.warning("request error: %s", failure.getErrorMessage())
        self.errored += 1


def _parse_http_date(value: Optional[str]) -> Optional[datetime]:
    if not value:
        return None
    from email.utils import parsedate_to_datetime

    try:
        parsed = parsedate_to_datetime(value)
    except (TypeError, ValueError):
        return None
    if parsed is None:
        return None
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=timezone.utc)
    return parsed


def run_crawl_for_source(
    *,
    source_id: int,
    source_name: str,
    urls: list[DiscoveredUrl],
    politeness: dict[str, Any],
    url_filter: dict[str, Any],
    content_filter: dict[str, Any],
    custom_extractors: Optional[dict[str, Any]],
    state: CrawlerState,
    ingestion_client: IngestionClient,
    user_agent: str,
) -> None:
    """Synchronously crawl ``urls`` for one source. Blocks until the Twisted
    reactor has finished. Multiple sources are crawled by sequential calls
    (the reactor is single-shot per process by Scrapy convention).
    """
    settings = {
        "ROBOTSTXT_OBEY": True,
        "USER_AGENT": user_agent,
        "AUTOTHROTTLE_ENABLED": bool(politeness.get("autothrottle", True)),
        "AUTOTHROTTLE_TARGET_CONCURRENCY": 1.0,
        "DOWNLOAD_DELAY": float(politeness.get("delay_seconds", 1.0)),
        "CONCURRENT_REQUESTS_PER_DOMAIN": int(
            politeness.get("max_concurrent_per_domain", 2)
        ),
        "COOKIES_ENABLED": False,
        "REDIRECT_ENABLED": True,
        "RETRY_ENABLED": True,
        "RETRY_TIMES": 3,
        "DOWNLOAD_TIMEOUT": 30,
        "LOG_LEVEL": "INFO",
        "TELNETCONSOLE_ENABLED": False,
        "HTTPCACHE_ENABLED": False,
    }

    process = CrawlerProcess(settings=settings)
    process.crawl(
        WebSpider,
        source_id=source_id,
        source_name=source_name,
        urls=urls,
        url_filter=url_filter,
        content_filter=content_filter,
        custom_extractors=custom_extractors,
        state=state,
        ingestion_client=ingestion_client,
    )
    process.start()
