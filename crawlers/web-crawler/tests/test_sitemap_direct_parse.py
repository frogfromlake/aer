"""Phase 123 — sitemap discovery parses ONLY the configured URL.

Regression guard for the scalability fix: `discover()` must fetch and parse the
exact sitemap URL the operator configured, NOT treat it as a homepage and walk
robots.txt + the publisher's entire sitemap tree (which fanned franceinfo out
into thousands of archive-sitemap fetches per crawl).
"""

from __future__ import annotations

from datetime import datetime, timezone
from unittest.mock import MagicMock, patch

from internal.discovery import sitemap


def _resp(body: bytes):
    m = MagicMock()
    m.content = body
    m.raise_for_status = MagicMock()
    return m


FLAT_URLSET = b"""<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://www.franceinfo.fr/a_8000001.html</loc><lastmod>2026-05-30T10:00:00+02:00</lastmod></url>
  <url><loc>https://www.franceinfo.fr/b_8000002.html</loc><lastmod>2020-01-01T10:00:00+02:00</lastmod></url>
  <url><loc>https://www.franceinfo.fr/c_8000003.html</loc></url>
</urlset>"""


def test_flat_urlset_parsed_directly_no_tree_walk():
    """A configured flat news sitemap yields its URLs from ONE fetch — and the
    fetch targets the configured URL verbatim (no robots.txt homepage walk)."""
    with patch("requests.get", return_value=_resp(FLAT_URLSET)) as mock_get:
        out = list(sitemap.discover(["https://www.franceinfo.fr/sitemap_news.xml"], since=None))

    # Exactly one network fetch, aimed at the configured URL.
    assert mock_get.call_count == 1
    assert mock_get.call_args.args[0] == "https://www.franceinfo.fr/sitemap_news.xml"
    # All three <loc> entries surface when there is no temporal cutoff.
    urls = {d.url for d in out}
    assert urls == {
        "https://www.franceinfo.fr/a_8000001.html",
        "https://www.franceinfo.fr/b_8000002.html",
        "https://www.franceinfo.fr/c_8000003.html",
    }


def test_temporal_filter_strict_drops_old_and_undated():
    """With a `since` watermark + strict mode: old entries and undated entries
    are dropped; only the fresh dated one survives."""
    since = datetime(2026, 5, 1, tzinfo=timezone.utc)
    with patch("requests.get", return_value=_resp(FLAT_URLSET)):
        out = list(sitemap.discover(
            ["https://www.franceinfo.fr/sitemap_news.xml"],
            since=since,
            strict_lastmod=True,
        ))
    assert [d.url for d in out] == ["https://www.franceinfo.fr/a_8000001.html"]


SITEMAP_INDEX = b"""<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap><loc>https://example.com/child-1.xml</loc></sitemap>
</sitemapindex>"""

CHILD = b"""<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/article-1.html</loc><lastmod>2026-05-30</lastmod></url>
</urlset>"""


def test_sitemapindex_recurses_only_listed_children():
    """A configured index recurses into the children it lists (and only those)."""
    def fake_get(url, *a, **k):
        if url.endswith("index.xml"):
            return _resp(SITEMAP_INDEX)
        if url.endswith("child-1.xml"):
            return _resp(CHILD)
        raise AssertionError(f"unexpected fetch: {url}")

    with patch("requests.get", side_effect=fake_get) as mock_get:
        out = list(sitemap.discover(["https://example.com/index.xml"], since=None))

    assert [d.url for d in out] == ["https://example.com/article-1.html"]
    # Two fetches only: the index + its single listed child. No robots.txt.
    assert mock_get.call_count == 2
