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
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any, Iterable, Iterator, Optional

import structlog
import yaml

from internal.discovery import ChannelStats, DiscoveryConfigurationError
from internal.discovery.archive_index import discover as discover_archive_index
from internal.discovery.html_sitemap import discover as discover_html_sitemap
from internal.discovery.rss_hint import discover as discover_rss
from internal.discovery.sitemap import DiscoveredUrl, discover as discover_sitemap
from internal.fetch.scrapy_spider import build_crawler_process, queue_source_crawl
from internal.ingestion.client import IngestionClient
from internal.state.dedup import CrawlerState
from internal.state.discovery_runs import (
    DiscoveryRunRecord,
    DiscoveryRunsWriter,
)

DEFAULT_USER_AGENT = (
    "AerWebCrawler/0.1 (+https://aer.example/about; mailto:contact@example)"
)

# Phase 122b — fallback for `probe.time_window_days` when the probe YAML
# omits the `probe:` block. Emits a structured warning at startup so the
# default is visible without breaking existing probe configs that
# pre-date the cutoff field.
DEFAULT_TIME_WINDOW_DAYS = 365

# SEC-075 — freshness safety net for dateless discovery channels (HTML
# sitemaps, archive indexes, undated RSS) whose `lastmod` never changes, so
# the `sitemap_lastmod`-newer re-fetch trigger can never re-fire. Past this
# many days since the last fetch, the URL is re-discovered and re-requested
# with conditional-GET headers, so an unchanged article short-circuits at a
# 304 (no re-submit). Tune relative to the crawl cadence and the 90-day
# Bronze TTL: small enough that a corrected article reaches a fresh Bronze
# object well within the original's lifetime, large enough to keep dated
# channels from re-issuing a conditional GET every run. `0` disables it.
DEFAULT_REFETCH_STALE_AFTER_DAYS = 14


def _format_red_banner(message: str) -> str:
    """Render a high-visibility ANSI-red error banner. Used to surface
    the Phase-122g configuration hard-stop so the operator cannot
    miss it in scrollback. Falls back to plain text on non-TTY stderr."""
    RED = "\033[1;31m"
    RESET = "\033[0m"
    bar = "═" * 78
    return (
        f"\n{RED}{bar}{RESET}\n"
        f"{RED}  CRAWLER REFUSING TO START — UNRESOLVED `discovery:` CONFIGURATION  {RESET}\n"
        f"{RED}{bar}{RESET}\n\n"
        f"{message}\n\n"
        f"{RED}{bar}{RESET}\n"
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


def _load_probe_config(probe: str, config_dir: Path) -> dict[str, Any]:
    """Read ``probes/<probe>/sources.yaml`` and return a normalised
    config: ``{"sources": [...], "time_window_days": int}``.

    Phase 122b — the YAML's optional top-level ``probe:`` block carries
    probe-scoped settings that apply uniformly across every source:
    ``time_window_days`` (temporal cutoff for sitemap discovery; falls back
    to :data:`DEFAULT_TIME_WINDOW_DAYS` with a structured warning when
    absent), ``sitemap_strict_lastmod`` (drop undated sitemap entries), and
    ``refetch_stale_after_days`` (SEC-075 re-fetch staleness window; falls
    back to :data:`DEFAULT_REFETCH_STALE_AFTER_DAYS`, ``0`` disables).
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

    # SEC-075 — re-fetch staleness window (days) for dateless discovery
    # channels; defaults to DEFAULT_REFETCH_STALE_AFTER_DAYS, `0` disables.
    raw_stale = probe_block.get("refetch_stale_after_days")
    if raw_stale is None:
        refetch_stale_after_days = DEFAULT_REFETCH_STALE_AFTER_DAYS
    else:
        try:
            refetch_stale_after_days = int(raw_stale)
        except (TypeError, ValueError) as exc:
            raise ValueError(
                "probe.refetch_stale_after_days must be an integer "
                f"(got {raw_stale!r})"
            ) from exc
        if refetch_stale_after_days < 0:
            raise ValueError(
                "probe.refetch_stale_after_days must be non-negative "
                f"(got {refetch_stale_after_days})"
            )

    return {
        "sources": sources,
        "time_window_days": time_window_days,
        "sitemap_strict_lastmod": sitemap_strict_lastmod,
        "refetch_stale_after_days": refetch_stale_after_days,
    }


def _normalise_source_discovery(source: dict[str, Any]) -> dict[str, Any]:
    """Normalise per-source discovery config into the Phase-122g shape.

    Accepts both the new ``discovery:`` block (Phase 122g) and the legacy
    flat keys at source root (``sitemap_urls``, ``rss_hint_url`` singular,
    ``archive_index``). Returns a dict with the normalised channel set:
    ``{sitemap_urls, rss_hint_urls, html_sitemap_urls, archive_index,
    expected_floor_per_run}``.

    The legacy flat keys are forwarded with a one-shot structured warning
    so operators see the migration prompt without breaking existing
    configs. The aliasing is retired in Phase 127.
    """
    # SEC-094 — memoise on the source dict so the normalisation (and its
    # legacy-key warning) runs exactly once per source, no matter how many of
    # the three call sites (discover, validate, telemetry) invoke it. Honours
    # the docstring's "one-shot warning" promise and drops the duplicated work.
    cached = source.get("_normalised_discovery")
    if cached is not None:
        return cached

    log = structlog.get_logger()
    discovery = source.get("discovery")
    legacy_used: list[str] = []

    if discovery is None:
        # Pure legacy shape — flat keys at source root.
        sitemap_urls = list(source.get("sitemap_urls") or [])
        if "sitemap_urls" in source:
            legacy_used.append("sitemap_urls")
        rss_hint_url = source.get("rss_hint_url") or ""
        rss_hint_urls = [rss_hint_url] if rss_hint_url else []
        if "rss_hint_url" in source:
            legacy_used.append("rss_hint_url")
        archive_index = source.get("archive_index") or None
        if "archive_index" in source:
            legacy_used.append("archive_index")
        html_sitemap_urls: list[Any] = []
        expected_floor = None
    else:
        sitemap_urls = list(discovery.get("sitemap_urls") or [])
        rss_hint_urls = list(discovery.get("rss_hint_urls") or [])
        # Forgive a singular `rss_hint_url` inside the discovery block.
        single = discovery.get("rss_hint_url")
        if single and single not in rss_hint_urls:
            rss_hint_urls.append(single)
        html_sitemap_urls = list(discovery.get("html_sitemap_urls") or [])
        archive_index = discovery.get("archive_index") or None
        expected_floor = discovery.get("expected_floor_per_run")

    if legacy_used:
        log.warning(
            "legacy_discovery_keys_at_source_root",
            source=source.get("name"),
            legacy_keys=legacy_used,
            migration="wrap into `discovery:` block per Phase 122g — flat keys retire in Phase 127",
        )

    normalised = {
        "sitemap_urls": sitemap_urls,
        "rss_hint_urls": rss_hint_urls,
        "html_sitemap_urls": html_sitemap_urls,
        "archive_index": archive_index,
        "expected_floor_per_run": expected_floor,
    }
    source["_normalised_discovery"] = normalised
    return normalised


@dataclass(frozen=True)
class ChannelCount:
    """One channel's per-run telemetry — raw discovered + after-dedup.

    Phase 148d (WP-007) adds the **declared denominator**: the
    publisher-advertised, in-window item count measured at the channel's
    parse boundary, before AĒR's cross-channel dedup/filters
    (``completeness = collected / declared``). ``declared_indeterminate``
    is True when that count is only a lower bound (a fetch/parse error, a
    walk/fetch cap, or advertised-but-undatable content) — in which case
    completeness is reported as *indeterminate*, never a clean ratio.
    """

    channel: str
    urls_discovered: int
    urls_after_dedup: int
    declared: Optional[int] = None
    declared_indeterminate: bool = False


@dataclass(frozen=True)
class DiscoveryResult:
    """Structured result from :func:`_discover_for_source` (Phase 122g).

    Iterable + indexable + has ``len()`` so existing call sites that
    treat the result as a ``list[DiscoveredUrl]`` keep working.
    """

    urls: list[DiscoveredUrl]
    channel_counts: list[ChannelCount]
    run_started_at: datetime
    run_completed_at: datetime

    def __iter__(self) -> Iterator[DiscoveredUrl]:
        return iter(self.urls)

    def __len__(self) -> int:
        return len(self.urls)

    def __getitem__(self, idx) -> DiscoveredUrl:
        return self.urls[idx]


def _discover_for_source(
    source: dict[str, Any],
    since: Optional[datetime] = None,
    sitemap_strict_lastmod: bool = True,
) -> DiscoveryResult:
    """Surface every URL for one source, newest-first.

    Phase 122g — discovery runs across the channels declared in the
    source's ``discovery:`` block (or the legacy flat keys, forwarded
    via :func:`_normalise_source_discovery`): ``sitemap_urls``,
    ``rss_hint_urls`` (plural — multi-feed publishers like
    bundesregierung's four-feed catalogue), ``html_sitemap_urls``
    (publisher-built HTML navigation pages — e.g. tagesschau's
    ``/infoservices/startseite-sitemap-*.html``), and ``archive_index``
    (date-indexed walkers — e.g. tagesschau's ``/archiv?datum=...``).
    Every channel honours ``since`` (Phase 122b temporal symmetry).

    Returned list is sorted by ``sitemap_lastmod`` descending so partial
    crawls (Ctrl+C, overnight stop) yield the most-recent slice of the
    cutoff window first. Entries with no ``sitemap_lastmod`` sink to
    the end — they would otherwise dominate the head ordering
    arbitrarily.

    Channel-collision rule: the first channel to surface a URL wins
    (the sitemap entry, when present, carries the canonical lastmod
    and the ``sitemap_section`` context — both load-bearing for the
    Bronze handoff and the newest-first sort).
    """
    discovery = _normalise_source_discovery(source)
    seen: dict[str, DiscoveredUrl] = {}
    channel_counts: list[ChannelCount] = []
    run_started_at = datetime.now(tz=timezone.utc)

    def _add_channel(
        channel: str,
        items: Iterable[tuple[str, Optional[datetime]]],
        stats: Optional[ChannelStats] = None,
    ) -> None:
        """Run one channel's discovery, attributing first-seen URLs to it.

        ``urls_discovered`` is the raw count from the channel (pre-
        dedup); ``urls_after_dedup`` counts only URLs that were unique
        when this channel surfaced them (i.e. the channel's net
        contribution to the merged set). This makes per-channel
        telemetry attributable: when sitemap + html_sitemap both yield
        URL X, sitemap (first) gets +1 ``after_dedup``, html_sitemap
        (second) gets +1 ``discovered`` but +0 ``after_dedup``.

        Phase 148d — ``stats`` is the per-channel declared-denominator
        sink the underlying ``discover_*`` populated while streaming. When
        the channel left it at the sentinel (a mocked test double, or a
        legacy path), ``declared`` falls back to ``discovered`` so the
        denominator never regresses to a spurious zero.
        """
        discovered = 0
        after_dedup = 0
        for url, lastmod in items:
            if not url:
                continue
            discovered += 1
            if url not in seen:
                seen[url] = DiscoveredUrl(
                    url=url, sitemap_lastmod=lastmod, sitemap_section=None
                )
                after_dedup += 1
        declared = (
            stats.declared if stats is not None and stats.declared is not None
            else discovered
        )
        channel_counts.append(
            ChannelCount(
                channel=channel,
                urls_discovered=discovered,
                urls_after_dedup=after_dedup,
                declared=declared,
                declared_indeterminate=bool(stats.indeterminate) if stats else False,
            )
        )

    # Channel 1: XML sitemaps (primary structured channel where the
    # publisher exposes one). Sitemap entries carry the richest context
    # so they win on URL collision — they go FIRST in the channel
    # sequence so the section + lastmod fields are preserved through
    # subsequent collisions.
    sitemap_discovered = 0
    sitemap_after_dedup = 0
    sitemap_stats = ChannelStats()
    for entry in discover_sitemap(
        discovery["sitemap_urls"],
        since=since,
        strict_lastmod=sitemap_strict_lastmod,
        stats=sitemap_stats,
    ):
        sitemap_discovered += 1
        if entry.url and entry.url not in seen:
            seen[entry.url] = entry  # preserve the full DiscoveredUrl (incl. section)
            sitemap_after_dedup += 1
    channel_counts.append(
        ChannelCount(
            channel="sitemap",
            urls_discovered=sitemap_discovered,
            urls_after_dedup=sitemap_after_dedup,
            declared=(
                sitemap_stats.declared
                if sitemap_stats.declared is not None
                else sitemap_discovered
            ),
            declared_indeterminate=sitemap_stats.indeterminate,
        )
    )

    # Channel 2: RSS / Atom feeds (peer-equal to sitemap since Phase
    # 122e F-A1; plural since Phase 122g — a publisher's RSS catalogue
    # may expose multiple official feeds, e.g. bundesregierung's four).
    # All configured feeds are flattened into one telemetry row tagged
    # `rss` so cross-source comparison stays uniform.
    rss_stats = ChannelStats()

    def _rss_items() -> Iterator[tuple[str, Optional[datetime]]]:
        for rss_url in discovery["rss_hint_urls"]:
            if not rss_url:
                continue
            yield from discover_rss(rss_url, since=since, stats=rss_stats)

    _add_channel("rss", _rss_items(), stats=rss_stats)

    # Channel 3: HTML sitemap pages (Phase 122g — for publishers who
    # don't ship a sitemap.xml but DO publish a navigation page that
    # surfaces the current article set, e.g. tagesschau's
    # `/infoservices/startseite-sitemap-*.html`). Entries carry no
    # per-article timestamp; they flow in with `sitemap_lastmod=None`
    # and sink to the end of the newest-first sort.
    html_sitemap_stats = ChannelStats()
    _add_channel(
        "html_sitemap",
        discover_html_sitemap(
            discovery["html_sitemap_urls"], since=since, stats=html_sitemap_stats
        ),
        stats=html_sitemap_stats,
    )

    # Channel 4: date-indexed archive walker (Phase 122e A20 — for
    # publishers exposing `/<archive-path>?datum=YYYY-MM-DD` or
    # equivalent). Code already shipped in Phase 122e; Phase 122g
    # activates per-source configuration on tagesschau.
    archive_index_cfg = discovery["archive_index"]
    if archive_index_cfg:
        archive_stats = ChannelStats()
        _add_channel(
            "archive_index",
            discover_archive_index(archive_index_cfg, since=since, stats=archive_stats),
            stats=archive_stats,
        )
    else:
        # Channel not configured for this source — declared is a true 0
        # (nothing advertised here), not an indeterminate measurement.
        channel_counts.append(
            ChannelCount(
                channel="archive_index",
                urls_discovered=0,
                urls_after_dedup=0,
                declared=0,
                declared_indeterminate=False,
            )
        )

    def _sort_key(entry: DiscoveredUrl) -> tuple[int, float]:
        # Tuple ordering: (lastmod-is-None → 1, then negative timestamp).
        # `False` < `True`, so entries with a real lastmod sort before
        # None entries; within the real-lastmod group, larger timestamps
        # sort first (newest-first).
        if entry.sitemap_lastmod is None:
            return (1, 0.0)
        return (0, -entry.sitemap_lastmod.timestamp())

    urls = sorted(seen.values(), key=_sort_key)
    run_completed_at = datetime.now(tz=timezone.utc)
    return DiscoveryResult(
        urls=urls,
        channel_counts=channel_counts,
        run_started_at=run_started_at,
        run_completed_at=run_completed_at,
    )


def cli(argv: list[str] | None = None) -> int:
    """CLI entrypoint for the web crawler: run one probe's configured sources
    end-to-end against the ingestion API. Returns a process exit code."""
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

    # Phase 122g HARD STOP — refuse to start when any source carries an
    # `article_url_pattern` that is still the audit-CLI placeholder
    # (`EDIT-ME-...`). Without this check, a forgotten placeholder would
    # silently match zero article URLs at runtime — the channel would
    # appear configured but contribute nothing, and the gap would only
    # show up after the two-consecutive-runs underflow alert. We turn
    # silent failure into loud startup failure.
    from internal.discovery import assert_pattern_usable
    for source in sources:
        name = source.get("name", "<anonymous>")
        discovery = _normalise_source_discovery(source)
        for entry in discovery.get("html_sitemap_urls") or []:
            page_url = (entry or {}).get("url") or "<missing url>"
            pattern = (entry or {}).get("article_url_pattern") or ""
            try:
                assert_pattern_usable(
                    pattern, channel="html_sitemap", where=f"{name}: {page_url}"
                )
            except DiscoveryConfigurationError as exc:
                log.error(
                    "discovery_configuration_invalid",
                    source=name,
                    channel="html_sitemap",
                    message=str(exc),
                )
                print(_format_red_banner(str(exc)), file=sys.stderr)
                return 2
        archive_cfg = discovery.get("archive_index")
        if archive_cfg:
            pattern = (archive_cfg or {}).get("article_url_pattern") or ""
            template = (archive_cfg or {}).get("url_template") or "<missing template>"
            try:
                assert_pattern_usable(
                    pattern, channel="archive_index", where=f"{name}: {template}"
                )
            except DiscoveryConfigurationError as exc:
                log.error(
                    "discovery_configuration_invalid",
                    source=name,
                    channel="archive_index",
                    message=str(exc),
                )
                print(_format_red_banner(str(exc)), file=sys.stderr)
                return 2
    time_window_days = probe_config["time_window_days"]
    sitemap_strict_lastmod = probe_config["sitemap_strict_lastmod"]
    refetch_stale_after_days = probe_config["refetch_stale_after_days"]
    refetch_stale_after = (
        timedelta(days=refetch_stale_after_days)
        if refetch_stale_after_days > 0
        else None
    )
    since = datetime.now(tz=timezone.utc) - timedelta(days=time_window_days)
    log.info(
        "crawl_window_configured",
        probe=args.probe,
        time_window_days=time_window_days,
        refetch_stale_after_days=refetch_stale_after_days,
        since=since.isoformat(),
    )

    ingestion = IngestionClient(
        ingest_url=args.api_url,
        sources_url=args.sources_url,
        api_key=args.api_key,
    )
    state = CrawlerState(_build_pg_dsn())
    # Phase 122g — per-channel discovery telemetry uses the same pg
    # connection pool the dedup state already owns. Sibling tables.
    discovery_writer = DiscoveryRunsWriter(state._pool)  # noqa: SLF001 — internal-by-design

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

            result = _discover_for_source(
                source,
                since=since,
                sitemap_strict_lastmod=sitemap_strict_lastmod,
            )
            urls = result.urls
            discovery = _normalise_source_discovery(source)
            log.info(
                "discovery complete",
                source=name,
                discovered=len(urls),
                per_channel={
                    cc.channel: {
                        "declared": cc.declared,
                        "discovered": cc.urls_discovered,
                        "after_dedup": cc.urls_after_dedup,
                        "indeterminate": cc.declared_indeterminate,
                    }
                    for cc in result.channel_counts
                },
                sitemap_count=len(discovery["sitemap_urls"]),
                rss_feed_count=len(discovery["rss_hint_urls"]),
                html_sitemap_count=len(discovery["html_sitemap_urls"]),
                archive_index_configured=bool(discovery["archive_index"]),
                expected_floor_per_run=discovery["expected_floor_per_run"],
                since=since.isoformat(),
            )

            # Phase 122g — record per-channel telemetry + evaluate the
            # two-consecutive-underflow alert. Failures are non-fatal:
            # the crawl proceeds even if Postgres is unreachable so a
            # telemetry outage never silently degrades coverage.
            try:
                discovery_writer.record_run_batch(
                    DiscoveryRunRecord(
                        source_id=source_id,
                        channel=cc.channel,
                        urls_discovered=cc.urls_discovered,
                        urls_after_dedup=cc.urls_after_dedup,
                        declared=cc.declared,
                        declared_indeterminate=cc.declared_indeterminate,
                        run_started_at=result.run_started_at,
                        run_completed_at=result.run_completed_at,
                    )
                    for cc in result.channel_counts
                )
                alert_event = discovery_writer.evaluate_alerts(
                    source_id=source_id,
                    expected_floor=discovery["expected_floor_per_run"],
                    urls_after_dedup_this_run=len(urls),
                    run_started_at=result.run_started_at,
                )
                if alert_event == "alerted":
                    log.warning(
                        "discovery_underflow",
                        source=name,
                        expected_floor=discovery["expected_floor_per_run"],
                        urls_after_dedup=len(urls),
                        consecutive_runs="≥2",
                    )
                elif alert_event == "pending":
                    log.info(
                        "discovery_underflow_pending",
                        source=name,
                        expected_floor=discovery["expected_floor_per_run"],
                        urls_after_dedup=len(urls),
                    )
                elif alert_event == "recovered":
                    log.info(
                        "discovery_underflow_recovered",
                        source=name,
                        urls_after_dedup=len(urls),
                    )
            except Exception as exc:
                log.warning(
                    "discovery_telemetry_write_failed",
                    source=name,
                    error=str(exc),
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
                refetch_stale_after=refetch_stale_after,
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
