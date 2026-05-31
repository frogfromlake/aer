"""Phase 123c — `_resolve_timestamp` time-of-day upgrade.

Targets the pure resolver directly (no trafilatura), so it runs locally and in
CI alike. Covers the elysee bug (TESTING.md Issue 4): a date-only article page
date collapsing publication_hour to 0 when the RSS feed carried the real time.
"""

from __future__ import annotations

from datetime import datetime, timezone

from internal.adapters.web import _resolve_timestamp
from internal.adapters.web_meta import WebMeta

EVENT = datetime(2026, 5, 30, tzinfo=timezone.utc)


def _meta(published, sitemap_lastmod, source):
    m = WebMeta(source_type="web")
    m.published_date = published
    m.sitemap_lastmod = sitemap_lastmod
    m.timestamp_source = source
    return m


def test_date_only_published_upgrades_to_same_day_rss_time():
    """elysee: page exposes date only (midnight); RSS pubDate has the time on
    the same day → use the RSS time so publication_hour is real."""
    m = _meta(
        datetime(2026, 5, 29, 0, 0, tzinfo=timezone.utc),
        datetime(2026, 5, 29, 18, 15, tzinfo=timezone.utc),
        "html_meta_published",
    )
    ts = _resolve_timestamp(m, None, EVENT)
    assert ts.hour == 18
    assert ts.date() == datetime(2026, 5, 29).date()  # day authority unchanged
    assert m.timestamp_source == "rss_pubdate_time_upgrade"


def test_different_day_sitemap_lastmod_does_not_override():
    """A different-day sitemap_lastmod is the republication-trigger signal, NOT
    a more-precise publish time — keep the article's own published_date."""
    m = _meta(
        datetime(2026, 5, 29, 0, 0, tzinfo=timezone.utc),
        datetime(2026, 5, 20, 18, 15, tzinfo=timezone.utc),
        "html_meta_published",
    )
    ts = _resolve_timestamp(m, None, EVENT)
    assert ts.hour == 0
    assert ts.day == 29
    assert m.timestamp_source == "html_meta_published"


def test_published_with_time_is_untouched():
    """When the page date already carries a time, the RSS time is ignored."""
    m = _meta(
        datetime(2026, 5, 29, 9, 30, tzinfo=timezone.utc),
        datetime(2026, 5, 29, 18, 15, tzinfo=timezone.utc),
        "json_ld_published",
    )
    ts = _resolve_timestamp(m, None, EVENT)
    assert ts.hour == 9
    assert m.timestamp_source == "json_ld_published"


def test_no_sitemap_lastmod_keeps_date_only_published():
    """Date-only published with no RSS time available → unchanged (hour 0)."""
    m = _meta(
        datetime(2026, 5, 29, 0, 0, tzinfo=timezone.utc),
        None,
        "html_meta_published",
    )
    ts = _resolve_timestamp(m, None, EVENT)
    assert ts.hour == 0
    assert m.timestamp_source == "html_meta_published"
