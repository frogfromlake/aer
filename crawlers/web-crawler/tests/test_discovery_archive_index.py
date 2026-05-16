"""Phase 122e A20 / F-A20 — date-indexed HTML archive discovery."""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone
from typing import Any
from unittest.mock import MagicMock

from internal.discovery.archive_index import discover


@dataclass
class _FakeResponse:
    status_code: int = 200
    text: str = ""


@dataclass
class _FakeFetcher:
    """Records every (url, headers, timeout) call and returns a configured response."""

    by_url: dict[str, _FakeResponse] = field(default_factory=dict)
    default: _FakeResponse = field(default_factory=lambda: _FakeResponse(status_code=404))
    calls: list[tuple[str, dict, float]] = field(default_factory=list)

    def __call__(self, url, *, headers=None, timeout=None):
        self.calls.append((url, dict(headers or {}), float(timeout or 0)))
        return self.by_url.get(url, self.default)


_NOW = datetime(2026, 5, 9, 12, 0, tzinfo=timezone.utc)


def _config(template: str = "https://www.tagesschau.de/archiv?datum={date}") -> dict[str, Any]:
    return {
        "url_template": template,
        "date_format": "%Y-%m-%d",
        "article_url_pattern": r"^https://www\.tagesschau\.de/[^?#]+-\d{2,4}\.html$",
    }


def test_extracts_article_urls_matching_pattern():
    page = """
        <html><body>
            <a href="/inland/policy-100.html">policy</a>
            <a href="/ausland/europa/election-212.html">election</a>
            <a href="/inland">section landing — should be dropped</a>
            <a href="https://other-domain.com/article-300.html">cross-domain — drop</a>
            <a href="/static.css">asset — drop</a>
        </body></html>
    """
    fetcher = _FakeFetcher(
        by_url={"https://www.tagesschau.de/archiv?datum=2026-05-09": _FakeResponse(text=page)},
    )
    sleeps: list[float] = []

    results = list(
        discover(
            _config(),
            since=_NOW - timedelta(days=0),  # zero-day window: today only
            now=_NOW,
            http_get=fetcher,
            sleep=sleeps.append,
        )
    )

    urls = [u for u, _ in results]
    assert "https://www.tagesschau.de/inland/policy-100.html" in urls
    assert "https://www.tagesschau.de/ausland/europa/election-212.html" in urls
    assert "https://www.tagesschau.de/inland" not in urls
    assert all("other-domain.com" not in u for u in urls)
    # Politeness sleep ran exactly once (one date walked).
    assert len(sleeps) == 1


def test_walks_full_window_one_request_per_day():
    fetcher = _FakeFetcher(default=_FakeResponse(status_code=200, text=""))
    sleeps: list[float] = []

    list(
        discover(
            _config(),
            since=_NOW - timedelta(days=4),  # 5-day window (today + 4 prior)
            now=_NOW,
            http_get=fetcher,
            sleep=sleeps.append,
        )
    )

    fetched_dates = sorted(call[0].rsplit("=", 1)[-1] for call in fetcher.calls)
    assert fetched_dates == ["2026-05-05", "2026-05-06", "2026-05-07", "2026-05-08", "2026-05-09"]
    assert len(sleeps) == 5


def test_lastmod_carries_archive_page_date():
    page = '<a href="/inland/x-100.html">x</a>'
    fetcher = _FakeFetcher(by_url={
        "https://www.tagesschau.de/archiv?datum=2024-03-15": _FakeResponse(text=page),
    })

    results = list(
        discover(
            _config(),
            since=datetime(2024, 3, 15, tzinfo=timezone.utc),
            now=datetime(2024, 3, 15, 12, 0, tzinfo=timezone.utc),
            http_get=fetcher,
            sleep=lambda _s: None,
        )
    )

    assert len(results) == 1
    url, lastmod = results[0]
    assert url == "https://www.tagesschau.de/inland/x-100.html"
    assert lastmod == datetime(2024, 3, 15, tzinfo=timezone.utc)


def test_non_200_response_skips_date_does_not_abort():
    page = '<a href="/inland/x-100.html">x</a>'
    fetcher = _FakeFetcher(by_url={
        # Day 1: 200 OK with one article.
        "https://www.tagesschau.de/archiv?datum=2026-05-09": _FakeResponse(text=page),
        # Day 2: 502 Bad Gateway.
        "https://www.tagesschau.de/archiv?datum=2026-05-08": _FakeResponse(status_code=502),
        # Day 3: 200 OK with one article.
        "https://www.tagesschau.de/archiv?datum=2026-05-07": _FakeResponse(
            text='<a href="/inland/y-200.html">y</a>'
        ),
    })

    results = list(
        discover(
            _config(),
            since=_NOW - timedelta(days=2),
            now=_NOW,
            http_get=fetcher,
            sleep=lambda _s: None,
        )
    )

    urls = [u for u, _ in results]
    assert "https://www.tagesschau.de/inland/x-100.html" in urls
    assert "https://www.tagesschau.de/inland/y-200.html" in urls
    assert len(urls) == 2  # 502 day skipped; the other two contributed


def test_fetch_exception_skips_date_does_not_abort():
    def _flaky(url, *, headers=None, timeout=None):
        if "2026-05-08" in url:
            raise RuntimeError("network blip")
        return _FakeResponse(text='<a href="/inland/x-100.html">x</a>')

    results = list(
        discover(
            _config(),
            since=_NOW - timedelta(days=2),
            now=_NOW,
            http_get=_flaky,
            sleep=lambda _s: None,
        )
    )
    # 2 successful days × 1 unique URL each = 2 yields (URL is yielded
    # only once even if it appears on multiple days, but here each day
    # has the same URL so dedup folds them to 1).
    urls = [u for u, _ in results]
    assert urls == ["https://www.tagesschau.de/inland/x-100.html"]


def test_missing_url_template_returns_empty():
    results = list(
        discover(
            {"article_url_pattern": ".*"},  # no url_template
            since=_NOW - timedelta(days=2),
            now=_NOW,
            http_get=MagicMock(),
            sleep=MagicMock(),
        )
    )
    assert results == []


def test_missing_article_url_pattern_raises():
    """Phase 122g hard-stop: archive_index without article_url_pattern
    raises DiscoveryConfigurationError. Silent skip behaviour was
    dangerous — operator wouldn't notice the channel was inert until
    the underflow alert fired."""
    import pytest
    from internal.discovery import DiscoveryConfigurationError

    with pytest.raises(DiscoveryConfigurationError, match="empty"):
        list(
            discover(
                {"url_template": "https://example.com/x?d={date}"},
                since=_NOW - timedelta(days=2),
                now=_NOW,
                http_get=MagicMock(),
                sleep=MagicMock(),
            )
        )


def test_edit_me_placeholder_raises():
    """Phase 122g hard-stop: the audit-CLI placeholder must surface as
    a startup failure, not silent zero ingestion."""
    import pytest
    from internal.discovery import DiscoveryConfigurationError

    with pytest.raises(DiscoveryConfigurationError, match="placeholder"):
        list(
            discover(
                {
                    "url_template": "https://example.com/x?d={date}",
                    "article_url_pattern":
                        "EDIT-ME-REGEX-MATCHING-ARTICLE-URLS",
                },
                since=_NOW - timedelta(days=2),
                now=_NOW,
                http_get=MagicMock(),
                sleep=MagicMock(),
            )
        )


def test_invalid_regex_raises():
    """Phase 122g: invalid regex now raises rather than silently
    skipping the channel."""
    import pytest
    from internal.discovery import DiscoveryConfigurationError

    with pytest.raises(DiscoveryConfigurationError, match="invalid"):
        list(
            discover(
                {
                    "url_template": "https://example.com/x?d={date}",
                    "article_url_pattern": "(unbalanced",
                },
                since=_NOW - timedelta(days=2),
                now=_NOW,
                http_get=MagicMock(),
                sleep=MagicMock(),
            )
        )


def test_dedups_same_url_across_multiple_days():
    same_link = '<a href="/inland/feature-100.html">f</a>'
    fetcher = _FakeFetcher(default=_FakeResponse(text=same_link))

    results = list(
        discover(
            _config(),
            since=_NOW - timedelta(days=3),
            now=_NOW,
            http_get=fetcher,
            sleep=lambda _s: None,
        )
    )

    # 4 days walked, 1 unique article URL repeated on each → dedup to 1.
    assert len(results) == 1
    assert results[0][0] == "https://www.tagesschau.de/inland/feature-100.html"
    # The lastmod is the FIRST date the URL was seen — i.e. today (newest-
    # first walk). This matches the newest-first sort the caller uses.
    assert results[0][1] == datetime(2026, 5, 9, tzinfo=timezone.utc)


def test_monthly_granularity_walks_one_request_per_month():
    """Monthly stepping issues exactly one HTTP fetch per calendar month
    in the window, anchored on the first of the month."""
    fetcher = _FakeFetcher(default=_FakeResponse(status_code=200, text=""))
    sleeps: list[float] = []

    cfg = _config()
    cfg["granularity"] = "monthly"

    # Window: 2024-06-15 → back to 2024-04-10 spans Jun, May, Apr.
    list(
        discover(
            cfg,
            since=datetime(2024, 4, 10, tzinfo=timezone.utc),
            now=datetime(2024, 6, 15, 12, 0, tzinfo=timezone.utc),
            http_get=fetcher,
            sleep=sleeps.append,
        )
    )

    fetched = [call[0].rsplit("=", 1)[-1] for call in fetcher.calls]
    # First-of-month dates, walking newest-first.
    assert fetched == ["2024-06-01", "2024-05-01", "2024-04-01"]
    assert len(sleeps) == 3


def test_monthly_granularity_year_boundary():
    """Stepping back across a year boundary works (Jan → previous Dec)."""
    fetcher = _FakeFetcher(default=_FakeResponse(status_code=200, text=""))

    cfg = _config()
    cfg["granularity"] = "monthly"

    list(
        discover(
            cfg,
            since=datetime(2023, 11, 15, tzinfo=timezone.utc),
            now=datetime(2024, 1, 15, 12, 0, tzinfo=timezone.utc),
            http_get=fetcher,
            sleep=lambda _s: None,
        )
    )

    fetched = [call[0].rsplit("=", 1)[-1] for call in fetcher.calls]
    assert fetched == ["2024-01-01", "2023-12-01", "2023-11-01"]


def test_unknown_granularity_returns_empty():
    cfg = _config()
    cfg["granularity"] = "weekly"  # not supported
    results = list(
        discover(
            cfg,
            since=_NOW - timedelta(days=2),
            now=_NOW,
            http_get=MagicMock(),
            sleep=MagicMock(),
        )
    )
    assert results == []


def test_user_agent_passed_to_fetcher():
    fetcher = _FakeFetcher(default=_FakeResponse(text=""))

    list(
        discover(
            _config(),
            since=_NOW,
            now=_NOW,
            http_get=fetcher,
            sleep=lambda _s: None,
            user_agent="MyCustomAgent/1.0",
        )
    )

    assert fetcher.calls[0][1].get("User-Agent") == "MyCustomAgent/1.0"
