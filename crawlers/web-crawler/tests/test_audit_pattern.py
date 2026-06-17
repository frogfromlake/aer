"""Unit tests for article-URL-pattern inference + CMS detection.

Pure string/regex logic (no network, no DB) — the audit_pattern module only
depends on the shared regexes/constants in audit_probe, which the conftest's
mocked optional deps let us import cleanly.
"""

from __future__ import annotations

import re

from audit_pattern import (
    CMS_PATTERN_TEMPLATES,
    _detect_cms,
    _extract_article_url_candidates,
    _extract_non_article_links,
    _infer_article_url_pattern,
    _validate_article_listing_page,
    cms_pattern_suggestions,
    infer_safe_pattern,
    validate_inferred_pattern,
)

BASE = "https://news.example.com/"
ORIGIN = "https://news.example.com"


def _articles(n: int, *, segment: str = "inland", ext: str = "-{i}.html") -> list[str]:
    return [f"{ORIGIN}/{segment}/slug{i}{ext.format(i=i)}" for i in range(1, n + 1)]


# --- _extract_article_url_candidates -----------------------------------------


def test_extract_candidates_keeps_only_article_shaped_links():
    body = """
    <a href="/inland/foo-bar-123.html">A</a>
    <a href="/sport/baz-qux-456.html">B</a>
    <a href="/about">About</a>
    <a href="https://partner.com/x/y-1.html">Partner</a>
    <a href="mailto:editor@example.com">Mail</a>
    <a href="/assets/logo.png">Logo</a>
    <a href="/inland/foo-bar-123.html">Dup</a>
    """
    out = _extract_article_url_candidates(body, BASE)
    assert out == [
        "https://news.example.com/inland/foo-bar-123.html",
        "https://news.example.com/sport/baz-qux-456.html",
    ]


def test_extract_candidates_excludes_self_url():
    body = '<a href="/a/one-1.html">1</a><a href="/a/two-2.html">2</a>'
    out = _extract_article_url_candidates(
        body, BASE, self_url="https://news.example.com/a/one-1.html"
    )
    assert out == ["https://news.example.com/a/two-2.html"]


def test_extract_candidates_empty_body():
    assert _extract_article_url_candidates("", BASE) == []


def test_extract_candidates_invalid_base():
    assert _extract_article_url_candidates('<a href="/a/b-1.html">x</a>', "not-a-url") == []


# --- _infer_article_url_pattern ----------------------------------------------


def test_infer_pattern1_slug_numeric_html():
    pattern = _infer_article_url_pattern(_articles(5), ORIGIN)
    assert pattern is not None
    assert r"-\d+\.html$" in pattern
    assert re.match(pattern, "https://news.example.com/inland/slug1-1.html")


def test_infer_pattern2_segment_plus_html():
    urls = [f"{ORIGIN}/aktuelles/story{i}.html" for i in range(5)]
    pattern = _infer_article_url_pattern(urls, ORIGIN)
    assert pattern is not None
    assert "aktuelles" in pattern
    assert pattern.endswith(r"\.html$")
    assert r"-\d+" not in pattern


def test_infer_pattern2_segment_without_extension():
    urls = [f"{ORIGIN}/news/story-{i}" for i in range(5)]
    pattern = _infer_article_url_pattern(urls, ORIGIN)
    assert pattern == r"^https?://(www\.)?news\.example\.com/news/[^?#]+$"


def test_infer_pattern3_generic_html_no_dominant_segment():
    urls = [f"{ORIGIN}/{seg}/page.html" for seg in ("a", "b", "c", "d", "e")]
    pattern = _infer_article_url_pattern(urls, ORIGIN)
    assert pattern == r"^https?://(www\.)?news\.example\.com/[^?#]+\.html$"


def test_infer_returns_none_below_min_sample():
    assert _infer_article_url_pattern(_articles(4), ORIGIN) is None


def test_infer_returns_none_when_no_pattern():
    urls = [f"{ORIGIN}/{seg}/plain" for seg in ("a", "b", "c", "d", "e")]
    assert _infer_article_url_pattern(urls, ORIGIN) is None


def test_infer_returns_none_for_hostless_origin():
    assert _infer_article_url_pattern(_articles(5), "no-scheme-host") is None


# --- cms_pattern_suggestions -------------------------------------------------


def test_cms_suggestions_known_family_substitutes_host():
    out = cms_pattern_suggestions("WordPress", "https://blog.example.com")
    assert len(out) == len(CMS_PATTERN_TEMPLATES["wordpress"])
    label, regex = out[0]
    assert "WordPress" in label
    assert r"blog\.example\.com" in regex
    re.compile(regex)  # every emitted suggestion must compile


def test_cms_suggestions_no_family():
    assert cms_pattern_suggestions(None, ORIGIN) == []


def test_cms_suggestions_unknown_family():
    assert cms_pattern_suggestions("nonesuch-cms", ORIGIN) == []


def test_cms_suggestions_hostless_origin():
    assert cms_pattern_suggestions("wordpress", "no-host") == []


# --- _validate_article_listing_page ------------------------------------------


def test_validate_listing_page_accepts_real_listing():
    body = "".join(f'<a href="/sec/story-{i}-{i}.html">{i}</a>' for i in range(6))
    result = _validate_article_listing_page(body, "https://news.example.com/list")
    assert result["is_listing"] is True
    assert len(result["article_urls"]) == 6


def test_validate_listing_page_rejects_navigation_page():
    body = '<a href="/sec/only-1.html">1</a><a href="/about">a</a>'
    result = _validate_article_listing_page(body, "https://news.example.com/list")
    assert result["is_listing"] is False
    assert "threshold" in result["reason"]


# --- validate_inferred_pattern -----------------------------------------------


PATTERN1 = r"^https?://(www\.)?news\.example\.com/[^?#]+-\d+\.html$"


def test_validate_inferred_pattern_perfect():
    result = validate_inferred_pattern(
        PATTERN1,
        article_urls=["https://news.example.com/a/x-1.html", "https://news.example.com/b/y-2.html"],
        non_article_urls=["https://news.example.com/about"],
    )
    assert result["valid"] is True
    assert result["article_matched"] == 2
    assert result["non_article_matched"] == 0


def test_validate_inferred_pattern_uncompilable():
    result = validate_inferred_pattern(
        "[unclosed",
        article_urls=["https://news.example.com/a/x-1.html"],
        non_article_urls=[],
    )
    assert result["valid"] is False
    assert result["missed_articles"] == ["https://news.example.com/a/x-1.html"]


def test_validate_inferred_pattern_under_matches():
    result = validate_inferred_pattern(
        PATTERN1,
        article_urls=["https://news.example.com/a/x-1.html", "https://news.example.com/b/plain"],
        non_article_urls=[],
    )
    assert result["article_matched"] == 1
    assert result["article_total"] == 2


def test_validate_inferred_pattern_false_positive():
    result = validate_inferred_pattern(
        PATTERN1,
        article_urls=["https://news.example.com/a/x-1.html"],
        non_article_urls=["https://news.example.com/ad/banner-9.html"],
    )
    assert result["non_article_matched"] == 1


# --- infer_safe_pattern ------------------------------------------------------


def test_infer_safe_pattern_accepts_clean_slug_numeric():
    articles = _articles(5)
    pattern, diag = infer_safe_pattern(articles, ["https://news.example.com/about"], ORIGIN)
    assert pattern is not None
    assert diag["accepted"] is True


def test_infer_safe_pattern_rejects_when_no_pattern():
    pattern, diag = infer_safe_pattern(_articles(3), [], ORIGIN)
    assert pattern is None
    assert "rejected_reason" in diag


def test_infer_safe_pattern_rejects_permissive():
    articles = [f"{ORIGIN}/aktuelles/story{i}.html" for i in range(5)]
    pattern, diag = infer_safe_pattern(articles, [], ORIGIN)
    assert pattern is None
    assert "permissive" in diag["rejected_reason"]


def test_infer_safe_pattern_rejects_false_positive():
    articles = [f"{ORIGIN}/news/story-{i}.html" for i in range(5)]
    non_articles = ["https://news.example.com/promo-9.html"]
    pattern, diag = infer_safe_pattern(articles, non_articles, ORIGIN)
    assert pattern is None
    assert "false-positive" in diag["rejected_reason"]


# --- _extract_non_article_links ----------------------------------------------


def test_extract_non_article_links_returns_excluded_set():
    body = """
    <a href="/inland/real-1.html">article</a>
    <a href="/about">nav</a>
    <a href="https://partner.com/x/y">partner</a>
    <a href="/assets/logo.png">asset</a>
    """
    out = _extract_non_article_links(body, BASE)
    assert "https://news.example.com/about" in out
    assert "https://partner.com/x/y" in out
    assert "https://news.example.com/assets/logo.png" in out
    assert "https://news.example.com/inland/real-1.html" not in out


def test_extract_non_article_links_empty_body():
    assert _extract_non_article_links("", BASE) == []


# --- _detect_cms -------------------------------------------------------------


def test_detect_cms_known_family():
    html = '<meta name="generator" content="WordPress 6.5" />'
    assert _detect_cms(html) == "wordpress"


def test_detect_cms_unknown_generator_returns_raw():
    html = '<meta name="generator" content="CustomCMS 9" />'
    assert _detect_cms(html) == "CustomCMS 9"


def test_detect_cms_no_meta():
    assert _detect_cms("<html><body>nothing</body></html>") is None


def test_detect_cms_empty():
    assert _detect_cms("") is None
