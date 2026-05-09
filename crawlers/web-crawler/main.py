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
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any, Iterable, Optional

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

# Phase 122b — fallback for `probe.time_window_days` when the probe YAML
# omits the `probe:` block. Emits a structured warning at startup so the
# default is visible without breaking existing probe configs that
# pre-date the cutoff field.
DEFAULT_TIME_WINDOW_DAYS = 365


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


def _load_probe_config(probe: str, config_dir: Path) -> dict[str, Any]:
    """Read ``probes/<probe>/sources.yaml`` and return a normalised
    config: ``{"sources": [...], "time_window_days": int}``.

    Phase 122b — the YAML's optional top-level ``probe:`` block carries
    probe-scoped settings that apply uniformly across every source. The
    only setting today is ``time_window_days`` (the temporal cutoff for
    sitemap discovery); falls back to :data:`DEFAULT_TIME_WINDOW_DAYS`
    with a structured warning when absent.
    """
    log = structlog.get_logger()
    path = config_dir / probe / "sources.yaml"
    if not path.exists():
        raise FileNotFoundError(f"probe configuration not found: {path}")
    with path.open("r", encoding="utf-8") as fh:
        config = yaml.safe_load(fh) or {}
    sources = config.get("sources") or []
    if not sources:
        raise ValueError(f"probe {probe!r} has no sources configured at {path}")

    probe_block = config.get("probe") or {}
    raw_window = probe_block.get("time_window_days")
    if raw_window is None:
        log.warning(
            "probe.time_window_days unset — defaulting to "
            f"{DEFAULT_TIME_WINDOW_DAYS}; cross-source baselines may be biased",
            probe=probe,
            default=DEFAULT_TIME_WINDOW_DAYS,
        )
        time_window_days = DEFAULT_TIME_WINDOW_DAYS
    else:
        try:
            time_window_days = int(raw_window)
        except (TypeError, ValueError) as exc:
            raise ValueError(
                f"probe.time_window_days must be an integer (got {raw_window!r})"
            ) from exc
        if time_window_days <= 0:
            raise ValueError(
                f"probe.time_window_days must be positive (got {time_window_days})"
            )

    return {"sources": sources, "time_window_days": time_window_days}


def _discover_for_source(
    source: dict[str, Any],
    since: Optional[datetime] = None,
) -> list[DiscoveredUrl]:
    """Surface every URL for one source, newest-first.

    Sitemap entries are the primary channel; RSS-feed entries only
    contribute URLs not already in the sitemap. Both channels honour
    ``since`` if supplied (Phase 122b temporal symmetry).

    Returned list is sorted by ``sitemap_lastmod`` descending so partial
    crawls (Ctrl+C, overnight stop) yield the most-recent slice of the
    cutoff window first. Entries with no ``sitemap_lastmod`` (RSS-only
    discoveries, or sitemaps with no ``lastmod`` field) sink to the end
    — they would otherwise dominate the head ordering arbitrarily.
    """
    seen: dict[str, DiscoveredUrl] = {}
    sitemap_urls: Iterable[str] = source.get("sitemap_urls") or []
    for entry in discover_sitemap(list(sitemap_urls), since=since):
        if entry.url not in seen:
            seen[entry.url] = entry
    rss_url: str = source.get("rss_hint_url") or ""
    if rss_url:
        for url in discover_rss(rss_url, since=since):
            if url and url not in seen:
                seen[url] = DiscoveredUrl(
                    url=url, sitemap_lastmod=None, sitemap_section=None
                )

    def _sort_key(entry: DiscoveredUrl) -> tuple[int, float]:
        # Tuple ordering: (lastmod-is-None → 1, then negative timestamp).
        # `False` < `True`, so entries with a real lastmod sort before
        # None entries; within the real-lastmod group, larger timestamps
        # sort first (newest-first).
        if entry.sitemap_lastmod is None:
            return (1, 0.0)
        return (0, -entry.sitemap_lastmod.timestamp())

    return sorted(seen.values(), key=_sort_key)


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
        probe_config = _load_probe_config(args.probe, config_dir)
    except (FileNotFoundError, ValueError) as exc:
        log.error("probe configuration invalid", error=str(exc))
        return 2

    sources = probe_config["sources"]
    time_window_days = probe_config["time_window_days"]
    since = datetime.now(tz=timezone.utc) - timedelta(days=time_window_days)
    log.info(
        "crawl_window_configured",
        probe=args.probe,
        time_window_days=time_window_days,
        since=since.isoformat(),
    )

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

            urls = _discover_for_source(source, since=since)
            log.info(
                "discovery complete",
                source=name,
                discovered=len(urls),
                sitemap_count=len(source.get("sitemap_urls") or []),
                rss_hint=bool(source.get("rss_hint_url")),
                since=since.isoformat(),
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
