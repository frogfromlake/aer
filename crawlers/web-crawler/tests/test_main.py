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

from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Iterator

import pytest
import yaml

from internal.discovery.sitemap import DiscoveredUrl


# --- _load_probe_config ----------------------------------------------------


def _write_probe_yaml(tmp_path: Path, payload: dict) -> Path:
    probe_dir = tmp_path / "probe0"
    probe_dir.mkdir()
    (probe_dir / "sources.yaml").write_text(yaml.safe_dump(payload), encoding="utf-8")
    return tmp_path


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


def test_load_probe_config_rejects_non_integer(tmp_path: Path) -> None:
    from main import _load_probe_config

    config_dir = _write_probe_yaml(
        tmp_path,
        {
            "probe": {"time_window_days": "ninety"},
            "sources": [{"name": "x", "sitemap_urls": ["https://x"]}],
        },
    )
    with pytest.raises(ValueError, match="must be an integer"):
        _load_probe_config("probe0", config_dir)


def test_load_probe_config_rejects_zero_or_negative(tmp_path: Path) -> None:
    from main import _load_probe_config

    config_dir = _write_probe_yaml(
        tmp_path,
        {
            "probe": {"time_window_days": 0},
            "sources": [{"name": "x", "sitemap_urls": ["https://x"]}],
        },
    )
    with pytest.raises(ValueError, match="must be positive"):
        _load_probe_config("probe0", config_dir)


def test_load_probe_config_rejects_missing_file(tmp_path: Path) -> None:
    from main import _load_probe_config

    with pytest.raises(FileNotFoundError):
        _load_probe_config("nonexistent", tmp_path)


def test_load_probe_config_rejects_empty_sources(tmp_path: Path) -> None:
    from main import _load_probe_config

    config_dir = _write_probe_yaml(tmp_path, {"sources": []})
    with pytest.raises(ValueError, match="no sources configured"):
        _load_probe_config("probe0", config_dir)


# --- _discover_for_source ordering -----------------------------------------


def test_discover_for_source_sorts_newest_first(monkeypatch) -> None:
    """`sitemap_lastmod desc` ordering. `None` lastmods sink to the end."""
    from main import _discover_for_source

    now = datetime(2026, 5, 9, tzinfo=timezone.utc)
    fake_urls = [
        DiscoveredUrl(
            url="https://x/old",
            sitemap_lastmod=now - timedelta(days=10),
            sitemap_section=None,
        ),
        DiscoveredUrl(
            url="https://x/new",
            sitemap_lastmod=now - timedelta(days=1),
            sitemap_section=None,
        ),
        DiscoveredUrl(
            url="https://x/none",
            sitemap_lastmod=None,
            sitemap_section=None,
        ),
        DiscoveredUrl(
            url="https://x/medium",
            sitemap_lastmod=now - timedelta(days=5),
            sitemap_section=None,
        ),
    ]

    def fake_sitemap_discover(_urls, since=None) -> Iterator[DiscoveredUrl]:
        yield from fake_urls

    def fake_rss_discover(_url, since=None) -> Iterator[str]:
        return iter([])

    monkeypatch.setattr("main.discover_sitemap", fake_sitemap_discover)
    monkeypatch.setattr("main.discover_rss", fake_rss_discover)

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

    def capture_sitemap(_urls, since=None) -> Iterator[DiscoveredUrl]:
        received["sitemap"] = since
        return iter([])

    def capture_rss(_url, since=None) -> Iterator[str]:
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


def test_discover_for_source_dedup_prefers_sitemap_entry(monkeypatch) -> None:
    """When the same URL appears in both sitemap and RSS, the sitemap
    entry (with `sitemap_lastmod`) wins over the RSS entry (no
    lastmod)."""
    from main import _discover_for_source

    sitemap_entry = DiscoveredUrl(
        url="https://x/dup",
        sitemap_lastmod=datetime(2026, 5, 1, tzinfo=timezone.utc),
        sitemap_section="news",
    )

    monkeypatch.setattr(
        "main.discover_sitemap", lambda urls, since=None: iter([sitemap_entry])
    )
    monkeypatch.setattr(
        "main.discover_rss", lambda url, since=None: iter(["https://x/dup"])
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
