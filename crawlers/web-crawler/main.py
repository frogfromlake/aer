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

from internal.discovery.archive_index import discover as discover_archive_index
from internal.discovery.rss_hint import discover as discover_rss
from internal.discovery.sitemap import DiscoveredUrl, discover as discover_sitemap
from internal.fetch.scrapy_spider import build_crawler_process, queue_source_crawl
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

    # Phase 122e A21 / F-A21 — `sitemap_strict_lastmod` controls whether
    # sitemap entries lacking a `<lastmod>` field bypass the
    # `time_window_days` cutoff. Default `true` for continuous-monitoring
    # safety; explicit `false` re-enables the Phase-122b "fall through to
    # preserve coverage on sparse sitemaps" behaviour for backfill mode.
    raw_strict = probe_block.get("sitemap_strict_lastmod")
    if raw_strict is None:
        sitemap_strict_lastmod = True
    elif isinstance(raw_strict, bool):
        sitemap_strict_lastmod = raw_strict
    else:
        raise ValueError(
            f"probe.sitemap_strict_lastmod must be a boolean (got {raw_strict!r})"
        )

    return {
        "sources": sources,
        "time_window_days": time_window_days,
        "sitemap_strict_lastmod": sitemap_strict_lastmod,
    }


def _discover_for_source(
    source: dict[str, Any],
    since: Optional[datetime] = None,
    sitemap_strict_lastmod: bool = True,
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
    for entry in discover_sitemap(
        list(sitemap_urls), since=since, strict_lastmod=sitemap_strict_lastmod
    ):
        if entry.url not in seen:
            seen[entry.url] = entry
    # Phase 122e — promote the RSS feed from "hint only" to a peer-equal
    # discovery channel: populate `sitemap_lastmod` from the RSS entry's
    # published_parsed so RSS URLs compete fairly in the newest-first
    # sort. For Probe 0's bundesregierung.de the RSS feed is the ONLY
    # channel that surfaces actual /aktuelles/ news content (the public
    # sitemap exposes only service/archive pages), so without this fix
    # bounded-time crawls never reach real news. The sitemap entry wins
    # on URL collision (carries the canonical lastmod and the
    # sitemap_section context).
    rss_url: str = source.get("rss_hint_url") or ""
    if rss_url:
        for url, entry_dt in discover_rss(rss_url, since=since):
            if url and url not in seen:
                seen[url] = DiscoveredUrl(
                    url=url, sitemap_lastmod=entry_dt, sitemap_section=None
                )

    # Phase 122e A20 — date-indexed HTML archive page. Used by sources
    # whose RSS exposes only a sliding 70-item top-stories window and
    # whose XML sitemap is absent (e.g. tagesschau.de's
    # `/archiv?datum=YYYY-MM-DD` exposes ≈ 140 articles/day going back
    # ≥ 4 years). Methodologically equivalent to sitemap discovery —
    # the publisher built and parameterised the page; we ingest every
    # article-shaped link verbatim. Sitemap / RSS entries win on URL
    # collision (canonical lastmod + sitemap_section context).
    archive_index_cfg = source.get("archive_index") or {}
    if archive_index_cfg:
        for url, entry_dt in discover_archive_index(archive_index_cfg, since=since):
            if url and url not in seen:
                seen[url] = DiscoveredUrl(
                    url=url, sitemap_lastmod=entry_dt, sitemap_section=None
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
    sitemap_strict_lastmod = probe_config["sitemap_strict_lastmod"]
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

    # Single CrawlerProcess for the entire run — Twisted's reactor is a
    # process-wide singleton (cannot be restarted via `process.start()`),
    # so all sources are queued onto one process and the reactor is
    # started exactly once after every source has been queued. The
    # politeness settings come from the FIRST source's `politeness:`
    # block; per-source overrides are not honoured today (Probe 0's two
    # sources share identical politeness, so this is moot in practice).
    # Per-domain throttling (Scrapy's `CONCURRENT_REQUESTS_PER_DOMAIN`
    # and per-domain `DOWNLOAD_DELAY`) keeps each source's host on its
    # own budget regardless.
    crawler_process = None
    sources_queued = 0

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

            urls = _discover_for_source(
                source,
                since=since,
                sitemap_strict_lastmod=sitemap_strict_lastmod,
            )
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

            if crawler_process is None:
                crawler_process = build_crawler_process(
                    politeness=source.get("politeness", {}) or {},
                    user_agent=args.user_agent,
                )

            queue_source_crawl(
                crawler_process,
                source_id=source_id,
                source_name=name,
                urls=urls,
                url_filter=source.get("url_filter", {}) or {},
                content_filter=source.get("content_filter", {}) or {},
                custom_extractors=source.get("custom_extractors", {}) or {},
                state=state,
                ingestion_client=ingestion,
            )
            sources_queued += 1

        # Start the Twisted reactor exactly once. Blocks until every
        # queued spider has finished or a SIGINT is received.
        if crawler_process is not None and sources_queued > 0:
            log.info("starting crawler reactor", sources_queued=sources_queued)
            crawler_process.start()
        else:
            log.warning(
                "no sources had crawlable URLs — nothing to start",
                probe=args.probe,
            )
    finally:
        state.close()
        ingestion.close()

    return 0


if __name__ == "__main__":
    sys.exit(cli())
