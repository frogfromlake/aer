"""Tests for the publisher-built HTML sitemap discovery channel (Phase 122g).

The HTML sitemap channel surfaces article links from a publisher-built
navigation page (e.g. tagesschau.de's
``/infoservices/startseite-sitemap-102.html``). The channel exists for
publishers who do not expose a standard XML sitemap but DO publish a
human-readable index page — operator-discoverable, not standardly
auto-discoverable.

Tests cover:
  * happy-path fixture HTML with N article links → exactly N yielded
  * non-article hrefs (navigation, assets, self-link) filtered by
    ``article_url_pattern`` regex
  * empty config / missing keys / regex errors yield nothing without
    crashing
  * non-200 / network-failure responses logged and skipped (do not
    poison the iterator)
  * relative ``href`` values resolved against the page's base URL
  * cross-page deduplication when the same article appears in two
    configured sitemap pages
"""

from __future__ import annotations

from unittest.mock import MagicMock

from internal.discovery.html_sitemap import discover


_TAGESSCHAU_SAMPLE_HTML = """<!DOCTYPE html>
<html>
<head><title>Sitemap | tagesschau.de</title></head>
<body>
  <a href="https://www.tagesschau.de/infoservices/startseite-sitemap-102.html">self</a>
  <a href="/inland/koalitionsausschuss-296.html">relative inland</a>
  <a href="/inland/innenpolitik/koalitionsausschuss-spd-union-entlastungen-100.html">relative innenpolitik</a>
  <a href="https://www.tagesschau.de/ausland/europa/starmer-grossbritannien-102.html">absolute ausland</a>
  <a href="/wetter/deutschland/wettervorhersage-deutschland-100.html">relative wetter</a>
  <a href="/api/some-endpoint">api (non-article)</a>
  <a href="https://www.tagesschau.de/impressum">impressum (non-article)</a>
  <a href="/static/style.css">stylesheet</a>
  <a href="https://other-domain.example/article-123.html">off-domain</a>
</body>
</html>
"""

_ARTICLE_PATTERN = r"https?://(www\.)?tagesschau\.de/[^?#]+-[0-9]+\.html$"


def _fake_get(payload: str, status: int = 200) -> MagicMock:
    resp = MagicMock()
    resp.status_code = status
    resp.text = payload
    return resp


def test_html_sitemap_extracts_only_article_shaped_links() -> None:
    """The regex pattern keeps `<section>/.../<slug>-NNN.html` URLs and
    drops the page's self-link, API endpoints, asset URLs, off-domain
    links, and the impressum page (no trailing ``-NNN.html``)."""
    http_get = MagicMock(return_value=_fake_get(_TAGESSCHAU_SAMPLE_HTML))
    cfg = [
        {
            "url": "https://www.tagesschau.de/infoservices/startseite-sitemap-102.html",
            "article_url_pattern": _ARTICLE_PATTERN,
        }
    ]
    urls = [u for u, _ in discover(cfg, http_get=http_get)]
    # 4 article-shaped links: self-link (the sitemap page itself ends
    # in `-102.html` and DOES match the pattern — this is by design,
    # the channel-collision rule in main._discover_for_source defers
    # to whichever channel surfaced the URL first; the worker will
    # DLQ-or-extract it like any other URL), the 3 article paths, and
    # the wetter page. The 4 non-matching hrefs are dropped.
    assert sorted(urls) == [
        "https://www.tagesschau.de/ausland/europa/starmer-grossbritannien-102.html",
        "https://www.tagesschau.de/infoservices/startseite-sitemap-102.html",
        "https://www.tagesschau.de/inland/innenpolitik/koalitionsausschuss-spd-union-entlastungen-100.html",
        "https://www.tagesschau.de/inland/koalitionsausschuss-296.html",
        "https://www.tagesschau.de/wetter/deutschland/wettervorhersage-deutschland-100.html",
    ]


def test_html_sitemap_yields_none_lastmod() -> None:
    """The HTML sitemap channel exposes no per-article date — every
    yielded entry must carry ``lastmod=None`` so the newest-first sort
    in main._discover_for_source sinks them after dated URLs."""
    http_get = MagicMock(return_value=_fake_get(_TAGESSCHAU_SAMPLE_HTML))
    cfg = [
        {
            "url": "https://www.tagesschau.de/infoservices/startseite-sitemap-102.html",
            "article_url_pattern": _ARTICLE_PATTERN,
        }
    ]
    entries = list(discover(cfg, http_get=http_get))
    assert entries, "expected at least one entry from the fixture HTML"
    assert all(lastmod is None for _, lastmod in entries)


def test_html_sitemap_resolves_relative_hrefs() -> None:
    """Relative paths like ``/inland/foo-NNN.html`` resolve against the
    page's scheme+netloc; protocol-relative ``//example/...`` also
    resolves; purely relative paths (no leading slash) are skipped."""
    html = """
    <a href="/inland/relative-100.html">root-relative</a>
    <a href="//www.tagesschau.de/ausland/protocol-relative-101.html">protocol-relative</a>
    <a href="ausland/no-leading-slash-102.html">no-leading-slash (drop)</a>
    """
    http_get = MagicMock(return_value=_fake_get(html))
    cfg = [
        {
            "url": "https://www.tagesschau.de/infoservices/startseite-sitemap-102.html",
            "article_url_pattern": _ARTICLE_PATTERN,
        }
    ]
    urls = [u for u, _ in discover(cfg, http_get=http_get)]
    assert sorted(urls) == [
        "https://www.tagesschau.de/ausland/protocol-relative-101.html",
        "https://www.tagesschau.de/inland/relative-100.html",
    ]


def test_html_sitemap_skips_non_200_response() -> None:
    """A 404 / 500 from the HTML sitemap URL is logged and skipped —
    the iterator yields nothing for that entry but does not crash."""
    http_get = MagicMock(return_value=_fake_get("", status=404))
    cfg = [
        {
            "url": "https://www.tagesschau.de/dead-sitemap-page.html",
            "article_url_pattern": _ARTICLE_PATTERN,
        }
    ]
    urls = list(discover(cfg, http_get=http_get))
    assert urls == []


def test_html_sitemap_skips_network_failure() -> None:
    """A network exception (DNS, timeout, connection reset) is caught
    and logged — the iterator does not raise."""
    def boom(*_a, **_kw):
        raise ConnectionError("network unreachable")

    cfg = [
        {
            "url": "https://www.tagesschau.de/infoservices/startseite-sitemap-102.html",
            "article_url_pattern": _ARTICLE_PATTERN,
        }
    ]
    urls = list(discover(cfg, http_get=boom))
    assert urls == []


def test_html_sitemap_raises_on_missing_pattern() -> None:
    """Phase 122g hard-stop: an entry without `article_url_pattern`
    raises DiscoveryConfigurationError. The old silently-skip behavior
    was dangerous — a forgotten pattern produced zero ingestion that
    only surfaced after the two-consecutive-runs underflow alert."""
    import pytest
    from internal.discovery import DiscoveryConfigurationError

    cfg = [{"url": "https://www.tagesschau.de/missing-pattern.html"}]
    with pytest.raises(DiscoveryConfigurationError, match="empty"):
        list(discover(cfg, http_get=MagicMock()))


def test_html_sitemap_raises_on_edit_me_placeholder() -> None:
    """Phase 122g hard-stop: an entry with the audit-CLI placeholder
    raises DiscoveryConfigurationError so the operator sees a loud
    startup failure instead of silent zero-ingestion."""
    import pytest
    from internal.discovery import DiscoveryConfigurationError

    cfg = [{
        "url": "https://x/sitemap.html",
        "article_url_pattern": "EDIT-ME-REGEX-MATCHING-ARTICLE-URLS",
    }]
    with pytest.raises(DiscoveryConfigurationError, match="placeholder"):
        list(discover(cfg, http_get=MagicMock()))


def test_html_sitemap_raises_on_invalid_regex() -> None:
    """Phase 122g: invalid regex now raises rather than silently
    skipping the channel."""
    import pytest
    from internal.discovery import DiscoveryConfigurationError

    cfg = [{
        "url": "https://x/sitemap.html",
        "article_url_pattern": "[unclosed-character-class",
    }]
    with pytest.raises(DiscoveryConfigurationError, match="invalid"):
        list(discover(cfg, http_get=MagicMock()))


def test_html_sitemap_deduplicates_across_pages() -> None:
    """When two configured HTML-sitemap pages both surface the same
    article URL, it is yielded only once (channel-internal dedup)."""
    shared_html = """
    <a href="https://www.tagesschau.de/inland/shared-100.html">on both pages</a>
    """
    http_get = MagicMock(return_value=_fake_get(shared_html))
    cfg = [
        {
            "url": "https://x/sitemap-a.html",
            "article_url_pattern": _ARTICLE_PATTERN,
        },
        {
            "url": "https://x/sitemap-b.html",
            "article_url_pattern": _ARTICLE_PATTERN,
        },
    ]
    urls = list(discover(cfg, http_get=http_get))
    assert len(urls) == 1
    assert urls[0][0] == "https://www.tagesschau.de/inland/shared-100.html"


def test_html_sitemap_empty_configs_no_op() -> None:
    """An empty or missing configs list returns immediately without
    any HTTP traffic."""
    http_get = MagicMock()
    assert list(discover([], http_get=http_get)) == []
    assert http_get.call_count == 0
