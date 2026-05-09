"""AĒR web-crawler — Phase 122 / single configurable binary.

CLI entry point. Reads ``probes/<probe-id>/sources.yaml`` (path is
configurable so the same image can serve every probe — Probe 0, Probe 1,
…), orchestrates per-source discovery + crawl, and POSTs Bronze
documents to the ingestion API.

Usage::

    python main.py --probe probe0
"""

from __future__ import annotations

import argparse
import logging
import os
import sys
from pathlib import Path
from typing import Any, Iterable

import structlog
import yaml

from internal.discovery.rss_hint import discover as discover_rss
from internal.discovery.sitemap import DiscoveredUrl, discover as discover_sitemap
from internal.fetch.scrapy_spider import run_crawl_for_source
from internal.ingestion.client import IngestionClient
from internal.state.dedup import CrawlerState

DEFAULT_USER_AGENT = (
    "AerWebCrawler/0.1 (+https://aer.example/about; mailto:contact@example)"
)


def _configure_logging() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s %(message)s",
    )
    structlog.configure(
        processors=[
            structlog.processors.add_log_level,
            structlog.processors.TimeStamper(fmt="iso"),
            structlog.processors.JSONRenderer(),
        ]
    )


def _build_pg_dsn() -> str:
    dsn = os.getenv("CRAWLER_PG_DSN", "").strip()
    if dsn:
        return dsn
    user = os.getenv("POSTGRES_USER", "aer")
    password = os.getenv("POSTGRES_PASSWORD", "")
    host = os.getenv("POSTGRES_HOST", "postgres")
    port = os.getenv("POSTGRES_PORT", "5432")
    db = os.getenv("POSTGRES_DB", "aer")
    return f"postgresql://{user}:{password}@{host}:{port}/{db}"


def _load_sources(probe: str, config_dir: Path) -> list[dict[str, Any]]:
    path = config_dir / probe / "sources.yaml"
    if not path.exists():
        raise FileNotFoundError(f"probe configuration not found: {path}")
    with path.open("r", encoding="utf-8") as fh:
        config = yaml.safe_load(fh) or {}
    sources = config.get("sources") or []
    if not sources:
        raise ValueError(f"probe {probe!r} has no sources configured at {path}")
    return sources


def _discover_for_source(source: dict[str, Any]) -> list[DiscoveredUrl]:
    """Surface every URL for one source: sitemap entries first, then any
    RSS-only newcomers as DiscoveredUrl with empty sitemap context.
    """
    seen: dict[str, DiscoveredUrl] = {}
    sitemap_urls: Iterable[str] = source.get("sitemap_urls") or []
    for entry in discover_sitemap(list(sitemap_urls)):
        if entry.url not in seen:
            seen[entry.url] = entry
    rss_url: str = source.get("rss_hint_url") or ""
    if rss_url:
        for url in discover_rss(rss_url):
            if url and url not in seen:
                seen[url] = DiscoveredUrl(
                    url=url, sitemap_lastmod=None, sitemap_section=None
                )
    return list(seen.values())


def cli(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        prog="aer-web-crawler",
        description="AĒR Phase-122 web crawler — one binary, every news-website probe.",
    )
    parser.add_argument(
        "--probe",
        required=True,
        help="probe identifier (matches a directory under --config-dir)",
    )
    parser.add_argument(
        "--config-dir",
        default=os.getenv("PROBES_DIR", "probes"),
        help="directory holding <probe>/sources.yaml configs (default: probes/)",
    )
    parser.add_argument(
        "--api-url",
        default=os.getenv("INGESTION_URL", "http://localhost:8081/api/v1/ingest"),
    )
    parser.add_argument(
        "--sources-url",
        default=os.getenv("SOURCES_URL", "http://localhost:8081/api/v1/sources"),
    )
    parser.add_argument(
        "--api-key",
        default=os.getenv("INGESTION_API_KEY", ""),
    )
    parser.add_argument(
        "--user-agent",
        default=os.getenv("WEB_CRAWLER_USER_AGENT", DEFAULT_USER_AGENT),
    )
    args = parser.parse_args(argv)

    _configure_logging()
    log = structlog.get_logger()

    if not args.api_key:
        log.error("INGESTION_API_KEY (or --api-key) is required")
        return 2

    config_dir = Path(args.config_dir).resolve()
    try:
        sources = _load_sources(args.probe, config_dir)
    except (FileNotFoundError, ValueError) as exc:
        log.error("probe configuration invalid", error=str(exc))
        return 2

    ingestion = IngestionClient(
        ingest_url=args.api_url,
        sources_url=args.sources_url,
        api_key=args.api_key,
    )
    state = CrawlerState(_build_pg_dsn())

    try:
        for source in sources:
            name = source.get("name", "")
            if not name:
                log.warning("skipping anonymous source entry")
                continue
            log.info("processing source", probe=args.probe, source=name)
            try:
                source_id = ingestion.resolve_source_id(name)
            except Exception as exc:
                log.error("source id resolution failed", source=name, error=str(exc))
                continue

            urls = _discover_for_source(source)
            log.info(
                "discovery complete",
                source=name,
                discovered=len(urls),
                sitemap_count=len(source.get("sitemap_urls") or []),
                rss_hint=bool(source.get("rss_hint_url")),
            )
            if not urls:
                continue

            run_crawl_for_source(
                source_id=source_id,
                source_name=name,
                urls=urls,
                politeness=source.get("politeness", {}) or {},
                url_filter=source.get("url_filter", {}) or {},
                content_filter=source.get("content_filter", {}) or {},
                custom_extractors=source.get("custom_extractors", {}) or {},
                state=state,
                ingestion_client=ingestion,
                user_agent=args.user_agent,
            )
    finally:
        state.close()
        ingestion.close()

    return 0


if __name__ == "__main__":
    sys.exit(cli())
