"""Unit tests for the web-adapter structured-data parsers + date/url/lang helpers.

All functions under test are pure (no I/O) — they parse already-extracted
JSON-LD / OpenGraph / microdata payloads and HTML strings.
"""

from __future__ import annotations

from datetime import datetime, timezone
from types import SimpleNamespace

from internal.adapters.web_meta import ALLOWED_EXTRACTION_METHODS
from internal.adapters.web_extract_sources import (
    _detect_html_lang,
    _extract_html_meta_published,
    _extract_jsonld,
    _extract_microdata,
    _extract_open_graph,
    _flatten_jsonld,
    _is_year_floor_sentinel,
    _list_of_strings,
    _parse_iso_or_none,
    _pick_news_article,
    _record,
    _stringify,
    canonical_url_or,
)


# --- canonical_url_or --------------------------------------------------------


def test_canonical_url_or_empty_returns_empty():
    assert canonical_url_or("") == ""


def test_canonical_url_or_returns_string_preserving_host():
    out = canonical_url_or("https://example.com/a/b")
    assert isinstance(out, str)
    assert "example.com" in out


# --- _parse_iso_or_none ------------------------------------------------------


def test_parse_iso_none_and_non_string():
    assert _parse_iso_or_none(None) is None
    assert _parse_iso_or_none(123) is None
    assert _parse_iso_or_none("   ") is None


def test_parse_iso_z_suffix_to_utc():
    got = _parse_iso_or_none("2026-05-12T08:30:00Z")
    assert got == datetime(2026, 5, 12, 8, 30, tzinfo=timezone.utc)


def test_parse_iso_naive_gets_utc():
    got = _parse_iso_or_none("2026-05-12T08:30:00")
    assert got.tzinfo == timezone.utc


def test_parse_iso_datetime_passthrough_and_naive_fill():
    aware = datetime(2026, 5, 12, tzinfo=timezone.utc)
    assert _parse_iso_or_none(aware) is aware
    naive = datetime(2026, 5, 12)
    assert _parse_iso_or_none(naive).tzinfo == timezone.utc


def test_parse_iso_with_microseconds_parses():
    got = _parse_iso_or_none("2026-05-12T08:30:00.500000Z")
    assert got is not None
    assert (got.year, got.month, got.day, got.hour, got.minute) == (2026, 5, 12, 8, 30)
    assert got.tzinfo == timezone.utc


def test_parse_iso_garbage_returns_none():
    assert _parse_iso_or_none("not-a-date") is None


# --- _detect_html_lang -------------------------------------------------------


def test_detect_html_lang_strips_region_and_lowercases():
    assert _detect_html_lang('<html lang="de-DE">') == "de"
    assert _detect_html_lang('<html LANG="EN">') == "en"


def test_detect_html_lang_empty_and_missing():
    assert _detect_html_lang("") == ""
    assert _detect_html_lang("<html>no lang</html>") == ""


# --- _flatten_jsonld ---------------------------------------------------------


def test_flatten_jsonld_handles_graph_lists_and_scalars():
    assert list(_flatten_jsonld(None)) == []
    plain = {"@type": "NewsArticle"}
    assert list(_flatten_jsonld(plain)) == [plain]
    graph = {"@graph": [{"@type": "Article"}, {"@type": "Person"}]}
    out = list(_flatten_jsonld(graph))
    assert {"@type": "Article"} in out and {"@type": "Person"} in out
    nested = [{"@type": "A"}, [{"@type": "B"}]]
    types = [o.get("@type") for o in _flatten_jsonld(nested)]
    assert "A" in types and "B" in types


# --- _pick_news_article ------------------------------------------------------


def test_pick_news_article_prefers_newsarticle():
    blocks = [{"@type": "Article", "x": 1}, {"@type": "NewsArticle", "x": 2}]
    assert _pick_news_article(blocks)["x"] == 2


def test_pick_news_article_type_list_and_reportage():
    assert _pick_news_article([{"@type": ["WebPage", "ReportageNewsArticle"]}]) is not None


def test_pick_news_article_falls_back_to_article():
    assert _pick_news_article([{"@type": "Article", "x": 9}])["x"] == 9


def test_pick_news_article_none_when_no_article():
    assert _pick_news_article([{"@type": "Person"}, {"no_type": True}]) is None


# --- _stringify --------------------------------------------------------------


def test_stringify_variants():
    assert _stringify(None) == ""
    assert _stringify("  hi  ") == "hi"
    assert _stringify({"@value": "v"}) == "v"
    assert _stringify({"name": "n"}) == "n"
    assert _stringify({"other": "x"}) == ""
    assert _stringify(["", "first"]) == "first"
    assert _stringify(42) == "42"


# --- _list_of_strings --------------------------------------------------------


def test_list_of_strings_variants():
    assert _list_of_strings(None) == []
    assert _list_of_strings("a, b; c") == ["a", "b", "c"]
    assert _list_of_strings(["x", {"@value": "y"}, ""]) == ["x", "y"]
    assert _list_of_strings(42) == []


# --- _record -----------------------------------------------------------------


def test_record_accepts_allowed_method():
    method = next(iter(ALLOWED_EXTRACTION_METHODS))
    meta = SimpleNamespace(extraction_methods={})
    _record(meta, "author", method)
    assert meta.extraction_methods["author"] == method


def test_record_rejects_unknown_method():
    meta = SimpleNamespace(extraction_methods={})
    _record(meta, "author", "totally-made-up-method")
    assert meta.extraction_methods["author"] is None


# --- _extract_jsonld / _open_graph / _microdata ------------------------------


def test_extract_jsonld_picks_article():
    sd = {"json-ld": [{"@type": "NewsArticle", "headline": "H"}]}
    assert _extract_jsonld(sd)["headline"] == "H"
    assert _extract_jsonld({"json-ld": None}) is None
    assert _extract_jsonld({}) is None


def test_extract_open_graph_dict_and_list():
    assert _extract_open_graph({"opengraph": {"og:title": "T"}}) == {"og:title": "T"}
    listed = {"opengraph": [{"properties": [("og:title", "T2"), ("og:type", "article")]}]}
    assert _extract_open_graph(listed) == {"og:title": "T2", "og:type": "article"}
    assert _extract_open_graph({}) == {}


def test_extract_microdata_article_props():
    sd = {
        "microdata": [
            {"type": "https://schema.org/NewsArticle", "properties": {"headline": "H"}},
            {"type": "https://schema.org/Person", "properties": {"name": "N"}},
        ]
    }
    assert _extract_microdata(sd) == {"headline": "H"}
    assert _extract_microdata({"microdata": []}) is None
    assert _extract_microdata({}) is None


# --- _extract_html_meta_published + _is_year_floor_sentinel ------------------


def test_extract_html_meta_published_from_time_element():
    html = '<time datetime="2026-05-12T08:30:00Z">12 May</time>'
    assert _extract_html_meta_published(html) == datetime(2026, 5, 12, 8, 30, tzinfo=timezone.utc)


def test_extract_html_meta_published_from_meta_tag():
    html = '<meta property="article:published_time" content="2026-03-01T00:00:00Z">'
    assert _extract_html_meta_published(html) == datetime(2026, 3, 1, tzinfo=timezone.utc)


def test_extract_html_meta_published_none():
    assert _extract_html_meta_published("<p>no date here</p>") is None


def test_year_floor_sentinel_true_without_corroboration():
    candidate = datetime(2026, 1, 1, 0, 0, 0, tzinfo=timezone.utc)
    assert _is_year_floor_sentinel(candidate, "<footer>© 2026</footer>") is True


def test_year_floor_sentinel_false_with_corroborating_time():
    candidate = datetime(2026, 1, 1, 0, 0, 0, tzinfo=timezone.utc)
    html = '<time datetime="2026-01-01T00:00:00Z">New Year</time>'
    assert _is_year_floor_sentinel(candidate, html) is False


def test_year_floor_sentinel_false_for_non_jan1_or_non_midnight():
    assert _is_year_floor_sentinel(datetime(2026, 5, 12, tzinfo=timezone.utc), "") is False
    assert _is_year_floor_sentinel(datetime(2026, 1, 1, 9, 0, tzinfo=timezone.utc), "") is False
