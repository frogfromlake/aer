"""Phase 148d (WP-007) — measured declared denominator + indeterminate flag.

Completeness is `collected / declared`, where `declared` is the
publisher-advertised, in-window inventory measured at each channel's parse
boundary (before AĒR's filters). `declared_indeterminate` is the honest
companion: it fires whenever `declared` is only a *lower bound* — a
fetch/parse error swallowed entries, a walk/fetch cap truncated the
channel, or the channel surfaced advertised-but-undatable content. When it
fires, completeness is reported as indeterminate (Negative Space), never a
clean ratio and never 100 % (WP-007 §3, §5).

These tests pin the per-channel `ChannelStats` behaviour and the
`_discover_for_source` wiring that folds it into `ChannelCount`.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone
from typing import Any, Optional
from unittest.mock import patch

from internal.discovery import ChannelStats
from internal.discovery.archive_index import discover as discover_archive
from internal.discovery.html_sitemap import discover as discover_html
from internal.discovery.rss_hint import discover as discover_rss
from internal.discovery.sitemap import discover as discover_sitemap

_NOW = datetime(2026, 5, 9, 12, 0, tzinfo=timezone.utc)
_SINCE = _NOW - timedelta(days=30)


# --------------------------------------------------------------------------
# ChannelStats primitive
# --------------------------------------------------------------------------


def test_channel_stats_count_and_mark() -> None:
    s = ChannelStats()
    assert s.declared is None and s.indeterminate is False
    s.count()
    s.count(2)
    assert s.declared == 3
    s.mark_indeterminate()
    assert s.indeterminate is True and s.declared == 3


def test_channel_stats_mark_initialises_declared_to_zero() -> None:
    """A channel that errors before counting anything reports declared=0
    (a measured lower bound), not None (never measured)."""
    s = ChannelStats()
    s.mark_indeterminate()
    assert s.declared == 0 and s.indeterminate is True


# --------------------------------------------------------------------------
# Sitemap channel
# --------------------------------------------------------------------------


def test_sitemap_declared_counts_in_window_dated_entries() -> None:
    entries = [
        ("https://x/a", _NOW - timedelta(days=1)),  # in window → declared
        ("https://x/b", _NOW - timedelta(days=10)),  # in window → declared
        ("https://x/old", _NOW - timedelta(days=90)),  # out of window → excluded
    ]
    stats = ChannelStats()
    with patch(
        "internal.discovery.sitemap._iter_sitemap_entries", return_value=iter(entries)
    ):
        urls = list(discover_sitemap(["https://x/sitemap.xml"], since=_SINCE, stats=stats))
    assert len(urls) == 2
    assert stats.declared == 2
    assert stats.indeterminate is False  # clean, fully-dated, no cap/error


def test_sitemap_fetch_error_marks_indeterminate() -> None:
    """A swallowed leaf (fetch/parse error) leaves the declared count a
    lower bound — the fail-silent path now flags it instead of hiding it."""
    from internal.discovery import sitemap as sm

    stats = ChannelStats()
    with patch("requests.get", side_effect=RuntimeError("network")):
        out = list(sm._iter_sitemap_entries("https://x/sitemap.xml", stats=stats))
    assert out == []
    assert stats.indeterminate is True


def test_sitemap_fetch_cap_marks_indeterminate(monkeypatch) -> None:
    """When the fetch budget is exhausted with child sitemaps still
    un-walked, the publisher exposes more than we sampled → indeterminate."""
    from internal.discovery import sitemap as sm

    index_xml = (
        b'<?xml version="1.0"?>'
        b'<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">'
        b"<sitemap><loc>https://x/child-1.xml</loc></sitemap>"
        b"<sitemap><loc>https://x/child-2.xml</loc></sitemap>"
        b"</sitemapindex>"
    )

    class _R:
        content = index_xml

        def raise_for_status(self):
            return None

    monkeypatch.setattr(sm, "_MAX_SITEMAP_FETCHES", 1)
    stats = ChannelStats()
    with patch("requests.get", return_value=_R()):
        list(sm._iter_sitemap_entries("https://x/index.xml", stats=stats))
    assert stats.indeterminate is True


def test_sitemap_strict_undated_marks_indeterminate() -> None:
    """An advertised-but-undatable entry (no <lastmod>) dropped in strict
    mode makes the in-window denominator a lower bound."""
    entries = [
        ("https://x/dated", _NOW - timedelta(days=2)),
        ("https://x/undated", None),
    ]
    stats = ChannelStats()
    with patch(
        "internal.discovery.sitemap._iter_sitemap_entries", return_value=iter(entries)
    ):
        urls = list(discover_sitemap(["https://x"], since=_SINCE, stats=stats))
    assert {u.url for u in urls} == {"https://x/dated"}
    assert stats.declared == 1
    assert stats.indeterminate is True


# --------------------------------------------------------------------------
# RSS channel
# --------------------------------------------------------------------------


class _FakeFeed:
    def __init__(self, entries: list[dict]) -> None:
        self.entries = entries


def _rss_entry(link: str, dt: Optional[datetime]) -> dict:
    e: dict[str, Any] = {"link": link}
    if dt is not None:
        e["published_parsed"] = dt.timetuple()
    return e


def test_rss_declared_counts_dated_in_window() -> None:
    feed = _FakeFeed(
        [
            _rss_entry("https://x/a", _NOW - timedelta(days=1)),
            _rss_entry("https://x/b", _NOW - timedelta(days=3)),
        ]
    )
    stats = ChannelStats()
    with patch("feedparser.parse", return_value=feed):
        urls = list(discover_rss("https://x/feed.xml", since=_SINCE, stats=stats))
    assert len(urls) == 2
    assert stats.declared == 2
    assert stats.indeterminate is False


def test_rss_parse_failure_marks_indeterminate() -> None:
    """A feed that fails to parse yields nothing AND flags the count as a
    lower bound — never a silent zero that reads as 'no content'."""
    stats = ChannelStats()
    with patch("feedparser.parse", side_effect=RuntimeError("boom")):
        urls = list(discover_rss("https://x/feed.xml", since=_SINCE, stats=stats))
    assert urls == []
    assert stats.indeterminate is True
    assert stats.declared == 0


def test_rss_undated_entry_marks_indeterminate() -> None:
    feed = _FakeFeed(
        [
            _rss_entry("https://x/dated", _NOW - timedelta(days=1)),
            _rss_entry("https://x/undated", None),
        ]
    )
    stats = ChannelStats()
    with patch("feedparser.parse", return_value=feed):
        urls = list(discover_rss("https://x/feed.xml", since=_SINCE, stats=stats))
    # Undated entries are still surfaced (yielded) but do not count toward
    # the trustworthy in-window denominator.
    assert len(urls) == 2
    assert stats.declared == 1
    assert stats.indeterminate is True


# --------------------------------------------------------------------------
# HTML sitemap channel — structurally dateless → always indeterminate
# --------------------------------------------------------------------------


@dataclass
class _Resp:
    status_code: int = 200
    text: str = ""


def test_html_sitemap_always_indeterminate_with_lower_bound_count() -> None:
    html = (
        '<a href="https://x/inland/policy-100.html">a</a>'
        '<a href="https://x/ausland/election-212.html">b</a>'
        '<a href="/nav">drop</a>'
    )
    cfg = [{"url": "https://x/sitemap.html", "article_url_pattern": r"-\d+\.html$"}]
    stats = ChannelStats()
    urls = list(
        discover_html(cfg, since=_SINCE, http_get=lambda *a, **k: _Resp(text=html), stats=stats)
    )
    assert len(urls) == 2
    assert stats.declared == 2  # lower-bound count of what the page listed
    assert stats.indeterminate is True  # dateless channel cannot prove completeness


# --------------------------------------------------------------------------
# Archive walker (Decision B) — count links on walked pages; 5xx/transport
# error => indeterminate, 4xx (legitimate empty date) => not.
# --------------------------------------------------------------------------


@dataclass
class _Fetcher:
    by_url: dict[str, _Resp] = field(default_factory=dict)
    default: _Resp = field(default_factory=lambda: _Resp(status_code=404))

    def __call__(self, url, *, headers=None, timeout=None):
        return self.by_url.get(url, self.default)


def _archive_cfg() -> dict[str, Any]:
    return {
        "url_template": "https://x/archiv?datum={date}",
        "date_format": "%Y-%m-%d",
        "article_url_pattern": r"-\d+\.html$",
    }


def test_archive_declared_counts_walked_links_404_is_not_indeterminate() -> None:
    page = '<a href="https://x/a-1.html">a</a><a href="https://x/b-2.html">b</a>'
    fetcher = _Fetcher(by_url={"https://x/archiv?datum=2026-05-09": _Resp(text=page)})
    stats = ChannelStats()
    urls = list(
        discover_archive(
            _archive_cfg(),
            since=_NOW - timedelta(days=0),  # today only
            now=_NOW,
            http_get=fetcher,
            sleep=lambda *_: None,
            stats=stats,
        )
    )
    assert len(urls) == 2
    assert stats.declared == 2
    # The default 404 for other dates is a legitimate empty, not lost data.
    assert stats.indeterminate is False


def test_archive_5xx_marks_indeterminate() -> None:
    fetcher = _Fetcher(default=_Resp(status_code=503))
    stats = ChannelStats()
    list(
        discover_archive(
            _archive_cfg(),
            since=_NOW - timedelta(days=0),
            now=_NOW,
            http_get=fetcher,
            sleep=lambda *_: None,
            stats=stats,
        )
    )
    assert stats.indeterminate is True


# --------------------------------------------------------------------------
# main._discover_for_source wiring — ChannelCount carries declared +
# indeterminate; fallback declared=discovered when the channel left the
# sink at the sentinel (mocked double).
# --------------------------------------------------------------------------


def test_discover_for_source_folds_declared_into_channel_count(monkeypatch) -> None:
    from main import _discover_for_source

    def fake_sitemap(urls, since=None, strict_lastmod=True, stats=None):
        if stats is not None:
            stats.count()
            stats.mark_indeterminate()
        return iter([type("E", (), {"url": "https://x/a", "sitemap_lastmod": _NOW})()])

    monkeypatch.setattr("main.discover_sitemap", fake_sitemap)
    monkeypatch.setattr(
        "main.discover_rss", lambda url, since=None, **_kw: iter([])
    )
    monkeypatch.setattr(
        "main.discover_html_sitemap", lambda cfgs, since=None, **_kw: iter([])
    )

    source = {"name": "s", "discovery": {"sitemap_urls": ["https://x/sm.xml"]}}
    result = _discover_for_source(source, since=_SINCE)
    by_channel = {cc.channel: cc for cc in result.channel_counts}
    assert by_channel["sitemap"].declared == 1
    assert by_channel["sitemap"].declared_indeterminate is True


def test_discover_for_source_declared_falls_back_to_discovered(monkeypatch) -> None:
    """A mocked channel that ignores the stats sink must not regress the
    denominator to a spurious zero — declared falls back to discovered."""
    from main import _discover_for_source
    from internal.discovery.sitemap import DiscoveredUrl

    def fake_sitemap(urls, since=None, strict_lastmod=True, **_kw):
        return iter(
            [
                DiscoveredUrl("https://x/a", _NOW, None),
                DiscoveredUrl("https://x/b", _NOW, None),
            ]
        )

    monkeypatch.setattr("main.discover_sitemap", fake_sitemap)
    monkeypatch.setattr("main.discover_rss", lambda url, since=None, **_kw: iter([]))
    monkeypatch.setattr(
        "main.discover_html_sitemap", lambda cfgs, since=None, **_kw: iter([])
    )

    source = {"name": "s", "discovery": {"sitemap_urls": ["https://x/sm.xml"]}}
    result = _discover_for_source(source, since=_SINCE)
    by_channel = {cc.channel: cc for cc in result.channel_counts}
    assert by_channel["sitemap"].urls_discovered == 2
    assert by_channel["sitemap"].declared == 2  # fallback, not 0/None
    assert by_channel["sitemap"].declared_indeterminate is False
