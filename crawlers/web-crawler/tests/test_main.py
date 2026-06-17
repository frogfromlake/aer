"""Tests for main.py probe-config loading and discovery ordering (Phase 122b).

Covers two load-bearing properties of the temporal-symmetry patch:
  * ``_load_probe_config`` reads ``probe.time_window_days`` from the YAML
    and falls back to the documented default when absent.
  * ``_discover_for_source`` returns URLs in newest-first order
    (``sitemap_lastmod`` desc, with ``None`` lastmod sinking to the end)
    so a partial crawl yields the most-recent slice of the cutoff window
    first.
"""

from __future__ import annotations

from collections.abc import Iterator
from datetime import datetime, timedelta, timezone
from pathlib import Path

import pytest
import yaml

from internal.discovery.sitemap import DiscoveredUrl


# ---------------------------------------------------------------------------
# Shared fixtures / helpers.
# ---------------------------------------------------------------------------

_NOW = datetime(2026, 5, 9, tzinfo=timezone.utc)


def _write_probe_yaml(tmp_path: Path, payload: dict) -> Path:
    """Write a probe `sources.yaml` under <tmp>/probe0 and return <tmp>."""
    probe_dir = tmp_path / "probe0"
    probe_dir.mkdir()
    (probe_dir / "sources.yaml").write_text(yaml.safe_dump(payload), encoding="utf-8")
    return tmp_path


def _make_url(url: str, lastmod: datetime | None, section: str | None = None) -> DiscoveredUrl:
    """Build a sitemap-discovered URL entry."""
    return DiscoveredUrl(url=url, sitemap_lastmod=lastmod, sitemap_section=section)


def _patch_discovery(
    monkeypatch,
    *,
    sitemap=None,
    rss=None,
    html_sitemap=None,
    archive_index=None,
) -> None:
    """Monkeypatch the four `main.discover_*` channels.

    Each argument is either ``None`` (channel yields nothing) or a list of
    items the channel should yield (DiscoveredUrl for sitemap; ``(url,
    pubdate)`` tuples for the others). The ``since``/keyword args every
    real discover_* accepts are absorbed and ignored.
    """
    def _yielder(items):
        def _discover(*_a, since=None, **_kw):
            return iter(list(items or []))
        return _discover

    monkeypatch.setattr("main.discover_sitemap", _yielder(sitemap))
    monkeypatch.setattr("main.discover_rss", _yielder(rss))
    monkeypatch.setattr("main.discover_html_sitemap", _yielder(html_sitemap))
    monkeypatch.setattr("main.discover_archive_index", _yielder(archive_index))


# ---------------------------------------------------------------------------
# _load_probe_config
# ---------------------------------------------------------------------------


def test_load_probe_config_reads_time_window_days(tmp_path: Path) -> None:
    from main import _load_probe_config

    config_dir = _write_probe_yaml(
        tmp_path,
        {
            "probe": {"time_window_days": 1825},
            "sources": [{"name": "x", "sitemap_urls": ["https://x"]}],
        },
    )
    config = _load_probe_config("probe0", config_dir)
    assert config["time_window_days"] == 1825
    assert config["sources"][0]["name"] == "x"


def test_load_probe_config_defaults_to_365_when_absent(tmp_path: Path) -> None:
    from main import _load_probe_config, DEFAULT_TIME_WINDOW_DAYS

    config_dir = _write_probe_yaml(
        tmp_path,
        {"sources": [{"name": "x", "sitemap_urls": ["https://x"]}]},
    )
    config = _load_probe_config("probe0", config_dir)
    assert config["time_window_days"] == DEFAULT_TIME_WINDOW_DAYS == 365


@pytest.mark.parametrize(
    ("payload", "match"),
    [
        pytest.param(
            {
                "probe": {"time_window_days": "ninety"},
                "sources": [{"name": "x", "sitemap_urls": ["https://x"]}],
            },
            "must be an integer",
            id="non-integer-window",
        ),
        pytest.param(
            {
                "probe": {"time_window_days": 0},
                "sources": [{"name": "x", "sitemap_urls": ["https://x"]}],
            },
            "must be positive",
            id="zero-or-negative-window",
        ),
        pytest.param(
            {"sources": []},
            "no sources configured",
            id="empty-sources",
        ),
    ],
)
def test_load_probe_config_rejects_invalid(tmp_path: Path, payload: dict, match: str) -> None:
    from main import _load_probe_config

    config_dir = _write_probe_yaml(tmp_path, payload)
    with pytest.raises(ValueError, match=match):
        _load_probe_config("probe0", config_dir)


def test_load_probe_config_rejects_missing_file(tmp_path: Path) -> None:
    from main import _load_probe_config

    with pytest.raises(FileNotFoundError):
        _load_probe_config("nonexistent", tmp_path)


# ---------------------------------------------------------------------------
# _discover_for_source ordering
# ---------------------------------------------------------------------------


def test_discover_for_source_sorts_newest_first(monkeypatch) -> None:
    """`sitemap_lastmod desc` ordering. `None` lastmods sink to the end."""
    from main import _discover_for_source

    fake_urls = [
        _make_url("https://x/old", _NOW - timedelta(days=10)),
        _make_url("https://x/new", _NOW - timedelta(days=1)),
        _make_url("https://x/none", None),
        _make_url("https://x/medium", _NOW - timedelta(days=5)),
    ]
    _patch_discovery(monkeypatch, sitemap=fake_urls)

    source = {"name": "x", "sitemap_urls": ["https://x"]}
    result = _discover_for_source(source, since=None)

    ordered_urls = [entry.url for entry in result]
    assert ordered_urls == [
        "https://x/new",     # newest
        "https://x/medium",
        "https://x/old",     # oldest with a real lastmod
        "https://x/none",    # None lastmod sinks last
    ]


def test_discover_for_source_threads_since_to_both_channels(monkeypatch) -> None:
    """Both sitemap and RSS-hint discovery receive the `since` cutoff
    so the temporal symmetry holds across both URL-discovery paths."""
    from main import _discover_for_source

    received: dict[str, datetime | None] = {}

    def capture_sitemap(_urls, since=None, **_kw) -> Iterator[DiscoveredUrl]:
        received["sitemap"] = since
        return iter([])

    def capture_rss(_url, since=None, **_kw):
        received["rss"] = since
        return iter([])

    monkeypatch.setattr("main.discover_sitemap", capture_sitemap)
    monkeypatch.setattr("main.discover_rss", capture_rss)

    cutoff = datetime(2026, 4, 1, tzinfo=timezone.utc)
    source = {
        "name": "x",
        "sitemap_urls": ["https://x"],
        "rss_hint_url": "https://x/feed.xml",
    }
    _discover_for_source(source, since=cutoff)

    assert received["sitemap"] == cutoff
    assert received["rss"] == cutoff


def test_discover_for_source_rss_pubdate_promotes_url_to_head(monkeypatch) -> None:
    """Phase 122e — RSS-discovered URLs with a parsed pubDate sort by
    that pubDate alongside sitemap URLs, so a fresh RSS news article
    (no sitemap entry) wins over an older sitemap-discovered service
    page. Without this, RSS news URLs sink to the end of the queue
    behind every sitemap-discovered URL with a real lastmod, and a
    bounded-time crawl never reaches them.
    """
    from main import _discover_for_source

    sitemap_entry = _make_url(
        "https://x/sitemap-old", _NOW - timedelta(days=30), "archive"
    )
    rss_recent = ("https://x/rss-fresh", _NOW - timedelta(hours=2))
    rss_undated = ("https://x/rss-undated", None)
    _patch_discovery(monkeypatch, sitemap=[sitemap_entry], rss=[rss_recent, rss_undated])

    source = {
        "name": "x",
        "sitemap_urls": ["https://x"],
        "rss_hint_url": "https://x/feed.xml",
    }
    result = _discover_for_source(source, since=None)

    ordered_urls = [entry.url for entry in result]
    assert ordered_urls == [
        "https://x/rss-fresh",     # RSS pubDate 2h ago — newest
        "https://x/sitemap-old",   # sitemap lastmod 30d ago — middle
        "https://x/rss-undated",   # RSS without pubDate — sinks to end
    ]
    fresh = next(e for e in result if e.url == "https://x/rss-fresh")
    assert fresh.sitemap_lastmod is not None
    assert fresh.sitemap_section is None  # RSS-only discovery has no section


def test_discover_for_source_dedup_prefers_sitemap_entry(monkeypatch) -> None:
    """When the same URL appears in both sitemap and RSS, the sitemap
    entry (with `sitemap_lastmod` AND `sitemap_section`) wins over the
    RSS entry. Phase 122e adds the RSS pubDate to the lost candidate
    but the sitemap-side winner already carried a canonical lastmod."""
    from main import _discover_for_source

    sitemap_entry = _make_url(
        "https://x/dup", datetime(2026, 5, 1, tzinfo=timezone.utc), "news"
    )
    _patch_discovery(
        monkeypatch,
        sitemap=[sitemap_entry],
        rss=[("https://x/dup", datetime(2026, 5, 8, tzinfo=timezone.utc))],
    )

    source = {
        "name": "x",
        "sitemap_urls": ["https://x"],
        "rss_hint_url": "https://x/feed.xml",
    }
    result = _discover_for_source(source, since=None)

    assert len(result) == 1
    assert result[0].sitemap_lastmod is not None
    assert result[0].sitemap_section == "news"


# ---------------------------------------------------------------------------
# Phase 122g: discovery: block + flat-key aliasing
# ---------------------------------------------------------------------------


def test_normalise_source_discovery_accepts_new_block() -> None:
    """Phase 122g — the new `discovery:` block shape is the source of
    truth for per-source channels. The normaliser returns each channel
    list intact and surfaces `expected_floor_per_run`."""
    from main import _normalise_source_discovery

    source = {
        "name": "x",
        "discovery": {
            "sitemap_urls": ["https://x/sitemap.xml"],
            "rss_hint_urls": [
                "https://x/feed-a.xml",
                "https://x/feed-b.xml",
            ],
            "html_sitemap_urls": [
                {"url": "https://x/sitemap.html", "link_selector": "a"}
            ],
            "archive_index": {
                "url_pattern": "https://x/archiv?datum={YYYY-MM-DD}",
                "granularity": "daily",
            },
            "expected_floor_per_run": 42,
        },
    }
    out = _normalise_source_discovery(source)
    assert out["sitemap_urls"] == ["https://x/sitemap.xml"]
    assert out["rss_hint_urls"] == ["https://x/feed-a.xml", "https://x/feed-b.xml"]
    assert len(out["html_sitemap_urls"]) == 1
    assert out["html_sitemap_urls"][0]["url"] == "https://x/sitemap.html"
    assert out["archive_index"]["granularity"] == "daily"
    assert out["expected_floor_per_run"] == 42


def test_normalise_source_discovery_aliases_legacy_flat_keys() -> None:
    """Phase 122g — the legacy flat keys (`sitemap_urls`,
    `rss_hint_url` singular, `archive_index`) at source root are
    forwarded into the normalised discovery dict with a structured
    warning. Pre-122g sources.yaml continues to parse during the
    migration window (retired in Phase 127).
    """
    from main import _normalise_source_discovery

    source = {
        "name": "legacy",
        "sitemap_urls": ["https://x/sitemap.xml"],
        "rss_hint_url": "https://x/feed.xml",
        "archive_index": {"url_pattern": "https://x/archiv?datum={YYYY-MM-DD}"},
    }
    out = _normalise_source_discovery(source)
    assert out["sitemap_urls"] == ["https://x/sitemap.xml"]
    # Singular legacy key promoted to a single-element list.
    assert out["rss_hint_urls"] == ["https://x/feed.xml"]
    assert out["html_sitemap_urls"] == []
    assert out["archive_index"]["url_pattern"] == "https://x/archiv?datum={YYYY-MM-DD}"
    # No floor declared in legacy shape — None is the explicit signal
    # that the telemetry layer should not emit an underflow alert for
    # this source until the operator sets one.
    assert out["expected_floor_per_run"] is None


def test_normalise_source_discovery_defaults_for_empty_source() -> None:
    """A source with neither a `discovery:` block nor any legacy flat
    keys produces an empty channel set — the crawler will skip it
    (no URLs to fetch) but does not crash."""
    from main import _normalise_source_discovery

    out = _normalise_source_discovery({"name": "empty"})
    assert out == {
        "sitemap_urls": [],
        "rss_hint_urls": [],
        "html_sitemap_urls": [],
        "archive_index": None,
        "expected_floor_per_run": None,
    }


def test_discover_for_source_uses_plural_rss_feeds(monkeypatch) -> None:
    """Phase 122g — multiple RSS feeds under `discovery.rss_hint_urls`
    are all consulted; the per-feed yields are URL-unioned. This is the
    load-bearing change for bundesregierung's four-feed catalogue.
    """
    from main import _discover_for_source

    rss_calls: list[str] = []

    def fake_rss(url, since=None):
        rss_calls.append(url)
        # Each feed yields a distinct URL so the union is observable.
        if "feed-a" in url:
            return iter([("https://x/article-from-a", _NOW - timedelta(hours=1))])
        if "feed-b" in url:
            return iter([("https://x/article-from-b", _NOW - timedelta(hours=2))])
        return iter([])

    monkeypatch.setattr("main.discover_sitemap", lambda urls, since=None, strict_lastmod=True: iter([]))
    monkeypatch.setattr("main.discover_rss", fake_rss)

    source = {
        "name": "multi",
        "discovery": {
            "rss_hint_urls": [
                "https://x/feed-a.xml",
                "https://x/feed-b.xml",
            ],
        },
    }
    result = _discover_for_source(source, since=None)

    # Both feeds were consulted.
    assert sorted(rss_calls) == [
        "https://x/feed-a.xml",
        "https://x/feed-b.xml",
    ]
    # Union of yields, in newest-first order (a's pubDate is newer).
    assert [e.url for e in result] == [
        "https://x/article-from-a",
        "https://x/article-from-b",
    ]


def test_discover_for_source_unions_all_four_channels(monkeypatch) -> None:
    """Phase 122g — sitemap + RSS + html_sitemap + archive_index all
    contribute URLs in a single discovery pass. URL collision resolves
    in channel order (sitemap wins, then RSS, then html_sitemap, then
    archive_index). The four-channel union is the load-bearing
    architectural property: tagesschau (no sitemap, plural channels)
    and bundesregierung (sitemap-noise + plural RSS) both produce a
    single deduped URL list.
    """
    from main import _discover_for_source

    sitemap_entry = _make_url(
        "https://x/sitemap-article", _NOW - timedelta(days=2), "news"
    )
    _patch_discovery(
        monkeypatch,
        sitemap=[sitemap_entry],
        rss=[("https://x/rss-article", _NOW - timedelta(hours=3))],
        html_sitemap=[("https://x/html-sitemap-article", None)],
        archive_index=[("https://x/archive-article", _NOW - timedelta(days=5))],
    )

    source = {
        "name": "x",
        "discovery": {
            "sitemap_urls": ["https://x/sitemap.xml"],
            "rss_hint_urls": ["https://x/feed.xml"],
            "html_sitemap_urls": [
                {"url": "https://x/sitemap.html", "article_url_pattern": ".*"}
            ],
            "archive_index": {
                "url_template": "https://x/archiv?datum={date}",
                "article_url_pattern": ".*",
            },
        },
    }
    result = _discover_for_source(source, since=None)

    urls = [e.url for e in result]
    # All four channels contributed exactly one URL each; the union
    # contains all four entries with sitemap-first newest-first sort
    # (RSS pubDate is newest → first; sitemap lastmod → second; archive
    # lastmod older → third; html sitemap None → sinks to end).
    assert set(urls) == {
        "https://x/rss-article",
        "https://x/sitemap-article",
        "https://x/archive-article",
        "https://x/html-sitemap-article",
    }
    assert urls[0] == "https://x/rss-article"          # newest pubDate (3h ago)
    assert urls[1] == "https://x/sitemap-article"      # next-newest (2d ago)
    assert urls[2] == "https://x/archive-article"      # oldest dated (5d ago)
    assert urls[3] == "https://x/html-sitemap-article" # None lastmod sinks last


def test_discover_for_source_html_sitemap_loses_collision_to_sitemap(monkeypatch) -> None:
    """When the same URL is surfaced by BOTH the XML sitemap and the
    HTML sitemap, the XML-sitemap entry wins (it carries the canonical
    lastmod and the sitemap_section context). Pinned to prevent silent
    coverage regression if the channel-order changes."""
    from main import _discover_for_source

    sitemap_entry = _make_url(
        "https://x/article-100.html", datetime(2026, 5, 1, tzinfo=timezone.utc), "news"
    )
    _patch_discovery(
        monkeypatch,
        sitemap=[sitemap_entry],
        html_sitemap=[("https://x/article-100.html", None)],
    )

    source = {
        "name": "x",
        "discovery": {
            "sitemap_urls": ["https://x/sitemap.xml"],
            "html_sitemap_urls": [
                {"url": "https://x/sitemap.html", "article_url_pattern": ".*"}
            ],
        },
    }
    result = _discover_for_source(source, since=None)

    assert len(result) == 1
    # Sitemap-entry context (lastmod + section) preserved through the
    # collision.
    assert result[0].sitemap_lastmod is not None
    assert result[0].sitemap_section == "news"


def test_discover_for_source_aliases_legacy_rss_hint_url(monkeypatch) -> None:
    """Phase 122g — a source declared with the pre-122g singular
    `rss_hint_url` at root still discovers via that one feed (the
    alias path inside `_normalise_source_discovery`). Pinned so the
    migration window does not regress existing probe configs.
    """
    from main import _discover_for_source

    rss_calls: list[str] = []

    def fake_rss(url, since=None):
        rss_calls.append(url)
        return iter([("https://x/article", _NOW - timedelta(hours=1))])

    monkeypatch.setattr("main.discover_sitemap", lambda urls, since=None, strict_lastmod=True: iter([]))
    monkeypatch.setattr("main.discover_rss", fake_rss)

    source = {
        "name": "legacy",
        "sitemap_urls": [],
        "rss_hint_url": "https://x/feed.xml",
    }
    result = _discover_for_source(source, since=None)

    assert rss_calls == ["https://x/feed.xml"]
    assert [e.url for e in result] == ["https://x/article"]


# ---------------------------------------------------------------------------
# Phase 122e A1: regression test for ReactorNotRestartable
# ---------------------------------------------------------------------------


def test_cli_calls_start_exactly_once_for_multi_source(monkeypatch, tmp_path) -> None:
    """Phase 122e A1 — regression test for the original Phase-122
    ``twisted.internet.error.ReactorNotRestartable`` failure mode.

    The first Phase-122 implementation called ``CrawlerProcess.start()``
    once per source inside the ``cli()`` loop; Twisted's reactor is a
    process-wide singleton and crashed on the second source. The fix
    queues every source's spider on a SHARED CrawlerProcess and calls
    ``start()`` exactly once after the loop. This test pins that
    invariant: for a probe with N sources, ``build_crawler_process`` is
    called at most once and ``process.start`` is called exactly once.

    Mocks are set against `main.build_crawler_process` and
    `main.queue_source_crawl` (the two helpers exposed by
    `internal.fetch.scrapy_spider`). The test does not invoke any real
    Scrapy / Twisted machinery — it asserts on the call shape only.
    """
    from unittest.mock import MagicMock
    import main as crawler_main

    # Probe config with two sources — both will yield URLs.
    probe_yaml = tmp_path / "probe-test"
    probe_yaml.mkdir()
    (probe_yaml / "sources.yaml").write_text(
        yaml.safe_dump(
            {
                "probe": {"time_window_days": 30},
                "sources": [
                    {
                        "name": "src_a",
                        "sitemap_urls": ["https://a/sitemap.xml"],
                        "rss_hint_url": "https://a/feed.xml",
                    },
                    {
                        "name": "src_b",
                        "sitemap_urls": ["https://b/sitemap.xml"],
                        "rss_hint_url": "https://b/feed.xml",
                    },
                ],
            }
        ),
        encoding="utf-8",
    )

    # Discovery returns one URL per source.
    monkeypatch.setattr(
        "main.discover_sitemap",
        lambda urls, since=None, **_kw: iter(
            [
                _make_url(
                    f"https://{urls[0].split('/')[2]}/article-1",
                    datetime(2026, 5, 1, tzinfo=timezone.utc),
                )
            ]
        ),
    )
    monkeypatch.setattr(
        "main.discover_rss",
        lambda url, since=None, **_kw: iter([]),
    )

    # The single shared CrawlerProcess instance — its `crawl` and `start`
    # are observed by the test.
    fake_process = MagicMock(name="CrawlerProcess")
    build_calls: list[tuple] = []

    def fake_build(politeness, user_agent):
        build_calls.append((politeness, user_agent))
        return fake_process

    queue_calls: list[str] = []

    def fake_queue(process, **kwargs):
        # Assert the SAME process instance is reused for every source —
        # if the implementation regresses to "new process per source"
        # this assertion fires.
        assert process is fake_process, (
            f"queue_source_crawl received a different process instance "
            f"for source {kwargs.get('source_name')!r} — Phase 122e A1 "
            f"regression: ReactorNotRestartable will recur"
        )
        queue_calls.append(kwargs["source_name"])

    monkeypatch.setattr("main.build_crawler_process", fake_build)
    monkeypatch.setattr("main.queue_source_crawl", fake_queue)

    # Stub IngestionClient + CrawlerState so cli() doesn't try real I/O.
    fake_ingestion = MagicMock()
    fake_ingestion.resolve_source_id.side_effect = lambda name: {"src_a": 1, "src_b": 2}[name]
    monkeypatch.setattr("main.IngestionClient", lambda **kw: fake_ingestion)
    monkeypatch.setattr("main.CrawlerState", lambda dsn: MagicMock())

    # Run the CLI with both sources queued.
    rc = crawler_main.cli(
        [
            "--probe", "probe-test",
            "--config-dir", str(tmp_path),
            "--api-key", "test-key",
        ]
    )

    assert rc == 0, "cli should exit cleanly when both sources have URLs"
    # The load-bearing assertion: ONE process, TWO queues, ONE start.
    assert len(build_calls) == 1, (
        f"build_crawler_process called {len(build_calls)} times — must be "
        f"exactly 1 for multi-source crawls (Twisted reactor is a "
        f"process-wide singleton)"
    )
    assert queue_calls == ["src_a", "src_b"], (
        f"queue_source_crawl called for {queue_calls!r} — must queue both "
        f"sources in declaration order"
    )
    assert fake_process.start.call_count == 1, (
        f"process.start() called {fake_process.start.call_count} times — "
        f"must be exactly 1 to avoid ReactorNotRestartable"
    )
