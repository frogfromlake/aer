"""Unit tests for the web-adapter per-field resolvers.

Each resolver is pure: it reads already-parsed JSON-LD / OpenGraph / microdata
dicts (plus HTML for the date chain) and stamps a `WebMeta` field + its
provenance marker. Tests assert both the resolved value and the recorded method.
"""

from __future__ import annotations

from datetime import datetime, timezone

from internal.adapters.web_meta import WebMeta
from internal.adapters.web_extract_fields import (
    _resolve_article_type,
    _resolve_author,
    _resolve_categories_and_tags,
    _resolve_description,
    _resolve_image,
    _resolve_modified_date,
    _resolve_published_date,
    _resolve_section,
    _resolve_tier_c,
)

UTC = timezone.utc


def _meta() -> WebMeta:
    return WebMeta(source_type="web")


# --- _resolve_published_date (priority chain) --------------------------------


def test_published_date_jsonld_wins():
    m = _meta()
    _resolve_published_date(m, {"datePublished": "2026-05-12T08:00:00Z"}, {}, None, "", "")
    assert m.published_date == datetime(2026, 5, 12, 8, tzinfo=UTC)
    assert m.timestamp_source == "json_ld_published"
    assert m.extraction_methods["published_date"] == "json_ld"


def test_published_date_opengraph_tier():
    m = _meta()
    _resolve_published_date(m, None, {"article:published_time": "2026-05-12T09:00:00Z"}, None, "", "")
    assert m.published_date.hour == 9
    assert m.timestamp_source == "open_graph_published"
    assert m.extraction_methods["published_date"] == "open_graph"


def test_published_date_microdata_tier():
    m = _meta()
    _resolve_published_date(m, None, {}, {"datePublished": "2026-05-12T10:00:00Z"}, "", "")
    assert m.published_date.hour == 10
    assert m.timestamp_source == "html_meta_published"
    assert m.extraction_methods["published_date"] == "microdata"


def test_published_date_html_time_tier():
    m = _meta()
    _resolve_published_date(m, None, {}, None, '<time datetime="2026-05-12T11:00:00Z">x</time>', "")
    assert m.published_date.hour == 11
    assert m.extraction_methods["published_date"] == "html_meta"


def test_published_date_none_when_no_signal():
    m = _meta()
    _resolve_published_date(m, None, {}, None, "", "")
    assert m.published_date is None
    assert m.extraction_methods["published_date"] is None


# --- _resolve_modified_date --------------------------------------------------


def test_modified_date_jsonld_then_og_then_none():
    m = _meta()
    _resolve_modified_date(m, {"dateModified": "2026-05-12T08:00:00Z"}, {})
    assert m.extraction_methods["modified_date"] == "json_ld"

    m2 = _meta()
    _resolve_modified_date(m2, None, {"article:modified_time": "2026-05-12T08:00:00Z"})
    assert m2.extraction_methods["modified_date"] == "open_graph"

    m3 = _meta()
    _resolve_modified_date(m3, None, {})
    assert m3.extraction_methods["modified_date"] is None


# --- _resolve_author ---------------------------------------------------------


def test_author_jsonld_og_none():
    m = _meta()
    _resolve_author(m, {"author": {"@type": "Person", "name": "Jane Doe"}}, {})
    assert m.author == "Jane Doe"
    assert m.extraction_methods["author"] == "json_ld"

    m2 = _meta()
    _resolve_author(m2, None, {"article:author": "Og Author"})
    assert m2.author == "Og Author"
    assert m2.extraction_methods["author"] == "open_graph"

    m3 = _meta()
    _resolve_author(m3, None, {})
    assert m3.extraction_methods["author"] is None


# --- _resolve_description ----------------------------------------------------


def test_description_chain():
    m = _meta()
    _resolve_description(m, {"description": "from jsonld"}, {}, None)
    assert m.description == "from jsonld"
    assert m.extraction_methods["description"] == "json_ld"

    m2 = _meta()
    _resolve_description(m2, None, {"og:description": "from og"}, None)
    assert m2.description == "from og"

    m3 = _meta()
    _resolve_description(m3, None, {}, {"description": "from microdata"})
    assert m3.description == "from microdata"
    assert m3.extraction_methods["description"] == "microdata"


# --- _resolve_section (with the two JSON-LD fallbacks) -----------------------


def test_section_article_section():
    m = _meta()
    _resolve_section(m, {"articleSection": "Politik"}, {})
    assert m.section == "Politik"
    assert m.extraction_methods["section"] == "json_ld"


def test_section_opengraph():
    m = _meta()
    _resolve_section(m, None, {"article:section": "Sport"})
    assert m.section == "Sport"
    assert m.extraction_methods["section"] == "open_graph"


def test_section_about_name_fallback():
    m = _meta()
    _resolve_section(m, {"about": [{"@type": "Thing", "name": "Wetter"}]}, {})
    assert m.section == "Wetter"
    assert m.extraction_methods["section"] == "json_ld"


def test_section_keywords_fallback():
    m = _meta()
    _resolve_section(m, {"keywords": ["Klima", "Energie"]}, {})
    assert m.section == "Klima"


def test_section_none():
    m = _meta()
    _resolve_section(m, None, {})
    assert m.extraction_methods["section"] is None


# --- _resolve_image ----------------------------------------------------------


def test_image_opengraph_and_none():
    m = _meta()
    _resolve_image(m, None, {"og:image": "https://img/og.jpg"})
    assert m.image_url == "https://img/og.jpg"
    assert m.extraction_methods["image_url"] == "open_graph"

    m2 = _meta()
    _resolve_image(m2, None, {})
    assert m2.extraction_methods["image_url"] is None


# --- _resolve_categories_and_tags --------------------------------------------


def test_categories_and_tags_from_jsonld():
    m = _meta()
    _resolve_categories_and_tags(m, {"keywords": ["a", "b"], "articleSection": "News"}, {})
    assert m.tags == ["a", "b"]
    assert m.categories == ["News"]
    assert m.extraction_methods["tags"] == "json_ld"


def test_categories_about_fallback():
    m = _meta()
    _resolve_categories_and_tags(
        m, {"keywords": ["k"], "about": [{"name": "Topic1"}, {"name": "Topic2"}]}, {}
    )
    assert m.categories == ["Topic1", "Topic2"]


def test_tags_from_opengraph_when_no_jsonld():
    m = _meta()
    _resolve_categories_and_tags(m, None, {"article:tag": ["t1", "t2"]})
    assert m.tags == ["t1", "t2"]
    assert m.extraction_methods["tags"] == "open_graph"
    assert m.extraction_methods["categories"] is None


# --- _resolve_article_type ---------------------------------------------------


def test_article_type_string_list_none():
    m = _meta()
    _resolve_article_type(m, {"@type": "NewsArticle"})
    assert m.article_type == "NewsArticle"

    m2 = _meta()
    _resolve_article_type(m2, {"@type": ["WebPage", "ReportageNewsArticle"]})
    assert m2.article_type == "WebPage"

    m3 = _meta()
    _resolve_article_type(m3, None)
    assert m3.extraction_methods["article_type"] is None


# --- _resolve_tier_c ---------------------------------------------------------


def test_tier_c_rich_jsonld():
    m = _meta()
    _resolve_tier_c(
        m,
        {
            "editor": "Jane Editor",
            "contentLocation": "Berlin",
            "isAccessibleForFree": False,
            "correction": "Fixed a typo",
            "genre": ["opinion", "analysis"],
            "citation": ["https://a", "https://b"],
            "interactionStatistic": [
                {"interactionType": "CommentAction", "userInteractionCount": 42}
            ],
            "discussionUrl": "https://comments",
            "wordCount": 850,
            "image": [{"url": "https://img/1.jpg", "name": "alt", "caption": "cap"}],
            "timeRequired": "PT5M",
        },
    )
    assert m.editor == "Jane Editor"
    assert m.dateline_location == "Berlin"
    assert m.paywall_status is True  # not accessible for free → paywalled
    assert m.correction_notice == "Fixed a typo"
    assert m.editorial_labels == ["opinion", "analysis"]
    assert m.external_citations == ["https://a", "https://b"]
    assert m.comment_count == 42
    assert m.comment_url == "https://comments"
    assert m.word_count == 850
    assert len(m.images) == 1
    assert m.reading_time_minutes == 5


def test_tier_c_paywall_string_true_means_free():
    m = _meta()
    _resolve_tier_c(m, {"isAccessibleForFree": "true"})
    assert m.paywall_status is False


def test_tier_c_none_records_all_none():
    m = _meta()
    _resolve_tier_c(m, None)
    for field in ("editor", "dateline_location", "paywall_status", "images", "reading_time_minutes"):
        assert m.extraction_methods[field] is None
