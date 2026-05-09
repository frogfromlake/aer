"""Tests for sitemap + RSS-hint discovery filters (Phase 122b).

Covers the temporal-symmetry cutoff: entries strictly older than
``since`` are dropped at discovery time; entries with no parsable date
fall through (worker classifies them as Negative-Space via
``timestamp_source = "fetch_at_fallback"``).
"""

from __future__ import annotations

import time
from datetime import datetime, timedelta, timezone
from typing import Optional
from unittest.mock import MagicMock, patch

from internal.discovery.sitemap import discover as discover_sitemap
from internal.discovery.rss_hint import discover as discover_rss


# ----- Sitemap filter ------------------------------------------------------


def _fake_page(url: str, last_modified: Optional[datetime]) -> MagicMock:
    page = MagicMock()
    page.url = url
    page.last_modified = last_modified
    return page


def _fake_tree(pages: list[MagicMock]) -> MagicMock:
    tree = MagicMock()
    tree.all_pages.return_value = pages
    return tree


def test_sitemap_drops_entry_strictly_before_since() -> None:
    now = datetime(2026, 5, 9, tzinfo=timezone.utc)
    since = now - timedelta(days=30)

    pages = [_fake_page("https://x/old", now - timedelta(days=60))]
    with patch(
        "usp.tree.sitemap_tree_for_homepage", return_value=_fake_tree(pages)
    ):
        result = list(discover_sitemap(["https://x/sitemap.xml"], since=since))

    assert result == []


def test_sitemap_keeps_entry_at_or_after_since() -> None:
    now = datetime(2026, 5, 9, tzinfo=timezone.utc)
    since = now - timedelta(days=30)

    pages = [
        _fake_page("https://x/edge", since),  # exactly at cutoff — kept
        _fake_page("https://x/new", now - timedelta(days=10)),  # newer — kept
    ]
    with patch(
        "usp.tree.sitemap_tree_for_homepage", return_value=_fake_tree(pages)
    ):
        urls = {entry.url for entry in discover_sitemap(["https://x"], since=since)}

    assert urls == {"https://x/edge", "https://x/new"}


def test_sitemap_falls_through_when_lastmod_is_none() -> None:
    """Entries without a `last_modified` are kept regardless of `since`.
    The worker classifies them as Negative-Space via
    `timestamp_source = "fetch_at_fallback"`; we do not silently drop
    coverage on publishers with sparse sitemaps.
    """
    since = datetime(2026, 5, 1, tzinfo=timezone.utc)
    pages = [_fake_page("https://x/no-date", None)]
    with patch(
        "usp.tree.sitemap_tree_for_homepage", return_value=_fake_tree(pages)
    ):
        result = list(discover_sitemap(["https://x"], since=since))

    assert len(result) == 1
    assert result[0].url == "https://x/no-date"
    assert result[0].sitemap_lastmod is None


def test_sitemap_no_filter_when_since_is_none() -> None:
    """Backward-compatible: omitting `since` keeps every entry."""
    pages = [
        _fake_page("https://x/very-old", datetime(1995, 1, 1, tzinfo=timezone.utc)),
        _fake_page("https://x/none", None),
    ]
    with patch(
        "usp.tree.sitemap_tree_for_homepage", return_value=_fake_tree(pages)
    ):
        urls = {entry.url for entry in discover_sitemap(["https://x"])}

    assert urls == {"https://x/very-old", "https://x/none"}


# ----- RSS hint filter -----------------------------------------------------


def _fake_feed(entries: list[dict]) -> MagicMock:
    feed = MagicMock()
    feed.entries = entries
    return feed


def _struct_for(dt: datetime) -> time.struct_time:
    return dt.utctimetuple()


def test_rss_hint_drops_entry_before_since() -> None:
    now = datetime(2026, 5, 9, tzinfo=timezone.utc)
    since = now - timedelta(days=30)

    entries = [
        {"link": "https://x/old", "published_parsed": _struct_for(now - timedelta(days=60))},
        {"link": "https://x/new", "published_parsed": _struct_for(now - timedelta(days=5))},
    ]
    with patch("feedparser.parse", return_value=_fake_feed(entries)):
        urls = list(discover_rss("https://x/feed.xml", since=since))

    assert urls == ["https://x/new"]


def test_rss_hint_falls_through_when_no_parsable_date() -> None:
    """Defensive: if neither `published_parsed` nor `updated_parsed` is
    parsable, the entry is kept (RSS feeds are nearly always recent;
    we'd rather over-include than silently drop)."""
    since = datetime(2026, 5, 1, tzinfo=timezone.utc)
    entries = [{"link": "https://x/no-date"}]
    with patch("feedparser.parse", return_value=_fake_feed(entries)):
        urls = list(discover_rss("https://x/feed.xml", since=since))

    assert urls == ["https://x/no-date"]


def test_rss_hint_no_filter_when_since_is_none() -> None:
    entries = [
        {"link": "https://x/old", "published_parsed": _struct_for(datetime(1990, 1, 1, tzinfo=timezone.utc))},
    ]
    with patch("feedparser.parse", return_value=_fake_feed(entries)):
        urls = list(discover_rss("https://x/feed.xml"))

    assert urls == ["https://x/old"]
