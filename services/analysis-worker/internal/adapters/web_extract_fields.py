"""Per-field resolvers (date/author/section/image/categories/type/tier-c) — extracted from web_extract.py (Phase 141)."""

from __future__ import annotations

import logging
from typing import Any, Optional

from internal.adapters.web_meta import (
    ImageRef,
    WebMeta,
)

from internal.adapters.web_extract_deps import htmldate
from internal.adapters.web_extract_images import (
    _dedupe_images,
    _extract_image_url,
)
from internal.adapters.web_extract_sources import (
    _extract_html_meta_published,
    _is_year_floor_sentinel,
    _list_of_strings,
    _log_midnight_htmldate_observation,
    _parse_iso_or_none,
    _record,
    _stringify,
)

logger = logging.getLogger(__name__)


def _resolve_published_date(
    meta: WebMeta,
    jsonld: Optional[dict[str, Any]],
    og: dict[str, Any],
    microdata: Optional[dict[str, Any]],
    html: str,
    canonical_url: str,
) -> None:
    """Populate ``published_date`` and the timestamp_source provenance
    via the canonical priority chain (JSON-LD → OG → microdata →
    HTML5 ``<time datetime>`` / ``<meta>`` → htmldate heuristic).

    Phase 122e enforces three correctness invariants over the
    pre-existing chain:

    * ``<time datetime="...">`` and `<meta>` tags are read **before**
      falling through to htmldate's heuristic (Probe 0's
      bundesregierung.de news pages emit only these — no NewsArticle
      JSON-LD, no ``article:published_time`` OG tag).
    * ``extraction_methods.published_date`` and ``timestamp_source``
      are written as a consistent pair on every code path. Previously
      ``microdata`` left ``timestamp_source = "open_graph_published"``
      (folded buckets) — corrected to ``html_meta_published``.
    * htmldate's "year-floor sentinel" output (``YYYY-01-01 00:00:00``
      with no corroborating element in the HTML) is rewritten to
      ``published_date = None`` so the article is correctly classified
      as Negative-Space (per Brief §7.7) instead of polluting timeline
      analyses with a fake-precise stamp.
    """
    # Tier-1: JSON-LD NewsArticle.
    if jsonld is not None:
        candidate = _parse_iso_or_none(jsonld.get("datePublished"))
        if candidate is not None:
            meta.published_date = candidate
            _record(meta, "published_date", "json_ld")
            meta.timestamp_source = "json_ld_published"
            return

    # Tier-2: OpenGraph article:published_time.
    og_published = og.get("article:published_time") or og.get("og:published_time")
    candidate = _parse_iso_or_none(og_published)
    if candidate is not None:
        meta.published_date = candidate
        _record(meta, "published_date", "open_graph")
        meta.timestamp_source = "open_graph_published"
        return

    # Tier-3: Schema.org microdata datePublished.
    if microdata is not None:
        candidate = _parse_iso_or_none(microdata.get("datePublished"))
        if candidate is not None:
            meta.published_date = candidate
            _record(meta, "published_date", "microdata")
            meta.timestamp_source = "html_meta_published"
            return

    # Tier-4 (Phase 122e — F-A3): HTML5 `<time datetime="...">` and
    # `<meta>` publishedTime variants. These are publisher-emitted
    # explicit dates that pre-empt htmldate's whole-document heuristic.
    candidate = _extract_html_meta_published(html)
    if candidate is not None:
        meta.published_date = candidate
        _record(meta, "published_date", "html_meta")
        meta.timestamp_source = "html_meta_published"
        return

    # Tier-5: htmldate's whole-document heuristic. Last resort. Phase
    # 122e — F-A4: detect the year-floor sentinel and refuse to record
    # it as a publication date. Phase 122e A14: emit a defensive
    # monitoring log line for the softer "midnight stamp without a
    # `<time>` element" case — date is still accepted, but the
    # operator can audit how often it fires.
    if htmldate is not None:
        try:
            heuristic = htmldate.find_date(
                html, original_date=True, url=canonical_url or None
            )
        except Exception:
            heuristic = None
        candidate = _parse_iso_or_none(heuristic) if heuristic else None
        if candidate is not None and not _is_year_floor_sentinel(candidate, html):
            meta.published_date = candidate
            _record(meta, "published_date", "heuristic_htmldate")
            meta.timestamp_source = "html_meta_published"
            _log_midnight_htmldate_observation(candidate, html, canonical_url)
            return

    # No authoritative date available. Caller is expected to cascade to
    # sitemap_lastmod / http_last_modified / fetch_at and update
    # `meta.timestamp_source` accordingly. Leave both null/empty here so
    # the cascade is observable in the WebAdapter.
    _record(meta, "published_date", None)


def _resolve_modified_date(
    meta: WebMeta,
    jsonld: Optional[dict[str, Any]],
    og: dict[str, Any],
) -> None:
    if jsonld is not None:
        candidate = _parse_iso_or_none(jsonld.get("dateModified"))
        if candidate is not None:
            meta.modified_date = candidate
            _record(meta, "modified_date", "json_ld")
            return
    candidate = _parse_iso_or_none(og.get("article:modified_time"))
    if candidate is not None:
        meta.modified_date = candidate
        _record(meta, "modified_date", "open_graph")
        return
    _record(meta, "modified_date", None)


def _resolve_author(
    meta: WebMeta,
    jsonld: Optional[dict[str, Any]],
    og: dict[str, Any],
) -> None:
    if jsonld is not None:
        candidate = _stringify(jsonld.get("author"))
        if candidate:
            meta.author = candidate
            _record(meta, "author", "json_ld")
            return
    og_author = og.get("article:author") or og.get("author")
    candidate = _stringify(og_author)
    if candidate:
        meta.author = candidate
        _record(meta, "author", "open_graph")
        return
    _record(meta, "author", None)


def _resolve_description(
    meta: WebMeta,
    jsonld: Optional[dict[str, Any]],
    og: dict[str, Any],
    microdata: Optional[dict[str, Any]],
) -> None:
    if jsonld is not None:
        candidate = _stringify(jsonld.get("description"))
        if candidate:
            meta.description = candidate
            _record(meta, "description", "json_ld")
            return
    candidate = _stringify(og.get("og:description") or og.get("description"))
    if candidate:
        meta.description = candidate
        _record(meta, "description", "open_graph")
        return
    if microdata is not None:
        candidate = _stringify(microdata.get("description"))
        if candidate:
            meta.description = candidate
            _record(meta, "description", "microdata")
            return
    _record(meta, "description", None)


def _resolve_section(
    meta: WebMeta,
    jsonld: Optional[dict[str, Any]],
    og: dict[str, Any],
) -> None:
    """Populate ``section`` and its provenance via the canonical chain
    JSON-LD ``articleSection`` → OpenGraph ``article:section`` → JSON-LD
    ``about[0].name`` → JSON-LD ``keywords[0]``.

    Phase 122e A12 adds the two JSON-LD fallback steps. tagesschau's
    ``NewsArticle`` JSON-LD does NOT carry ``articleSection``; instead
    it carries ``about: [{"@type":"Thing","name":"Wetter","sameAs":...}]``
    and ``keywords: ["Wetter"]``. Falling back to ``about[0].name``
    captures the most-semantically-explicit topical anchor (Schema.org
    ``Thing`` references with `sameAs` URLs), and ``keywords[0]`` is the
    final fallback when ``about`` is absent. The provenance marker
    stays ``"json_ld"`` because the source IS JSON-LD; the path within
    JSON-LD differs and is captured by the field-level write.
    """
    if jsonld is not None:
        candidate = _stringify(jsonld.get("articleSection"))
        if candidate:
            meta.section = candidate
            _record(meta, "section", "json_ld")
            return
    candidate = _stringify(og.get("article:section"))
    if candidate:
        meta.section = candidate
        _record(meta, "section", "open_graph")
        return
    if jsonld is not None:
        # `about` is a list of Schema.org Thing references; first one
        # is conventionally the article's primary topic.
        about_value = jsonld.get("about")
        if isinstance(about_value, list) and about_value:
            first = about_value[0]
            if isinstance(first, dict):
                name = _stringify(first.get("name"))
                if name:
                    meta.section = name
                    _record(meta, "section", "json_ld")
                    return
        elif isinstance(about_value, dict):
            name = _stringify(about_value.get("name"))
            if name:
                meta.section = name
                _record(meta, "section", "json_ld")
                return
        # Final JSON-LD fallback: first keyword.
        keywords_value = jsonld.get("keywords")
        first_keyword = ""
        if isinstance(keywords_value, list) and keywords_value:
            first_keyword = _stringify(keywords_value[0])
        elif isinstance(keywords_value, str):
            first_keyword = (
                keywords_value.split(",")[0].strip() if keywords_value else ""
            )
        if first_keyword:
            meta.section = first_keyword
            _record(meta, "section", "json_ld")
            return
    _record(meta, "section", None)


def _resolve_image(
    meta: WebMeta,
    jsonld: Optional[dict[str, Any]],
    og: dict[str, Any],
) -> None:
    if jsonld is not None:
        url = _extract_image_url(jsonld.get("image"))
        if url:
            meta.image_url = url
            _record(meta, "image_url", "json_ld")
            return
    candidate = _stringify(og.get("og:image"))
    if candidate:
        meta.image_url = candidate
        _record(meta, "image_url", "open_graph")
        return
    _record(meta, "image_url", None)


def _resolve_categories_and_tags(
    meta: WebMeta,
    jsonld: Optional[dict[str, Any]],
    og: dict[str, Any],
) -> None:
    """Populate ``tags`` and ``categories`` from JSON-LD / OpenGraph.

    Phase 122e A12 / A7 refinement: also accept ``about[*].name`` as a
    fallback for ``categories`` when ``articleSection`` is absent.
    tagesschau emits ``about: [{"@type":"Thing","name":"Wetter",...}, ...]``
    on its ``NewsArticle`` blocks instead of ``articleSection``.
    """
    if jsonld is not None:
        keywords = _list_of_strings(jsonld.get("keywords"))
        if keywords:
            meta.tags = keywords
            _record(meta, "tags", "json_ld")
        section = _list_of_strings(jsonld.get("articleSection"))
        if section:
            meta.categories = section
            _record(meta, "categories", "json_ld")
        else:
            # A12 fallback: derive categories from `about[*].name` —
            # the canonical Schema.org topical anchor when the
            # publisher does not emit `articleSection` directly.
            about_value = jsonld.get("about")
            about_names: list[str] = []
            if isinstance(about_value, list):
                for entry in about_value:
                    if isinstance(entry, dict):
                        name = _stringify(entry.get("name"))
                        if name:
                            about_names.append(name)
            elif isinstance(about_value, dict):
                name = _stringify(about_value.get("name"))
                if name:
                    about_names.append(name)
            if about_names:
                meta.categories = about_names
                _record(meta, "categories", "json_ld")
        if meta.tags or meta.categories:
            return

    og_tags = _list_of_strings(og.get("article:tag"))
    if og_tags:
        meta.tags = og_tags
        _record(meta, "tags", "open_graph")

    if "categories" not in meta.extraction_methods:
        _record(meta, "categories", None)
    if "tags" not in meta.extraction_methods:
        _record(meta, "tags", None)


def _resolve_article_type(
    meta: WebMeta,
    jsonld: Optional[dict[str, Any]],
) -> None:
    if jsonld is None:
        _record(meta, "article_type", None)
        return
    article_type = jsonld.get("@type")
    if isinstance(article_type, list):
        article_type = next((t for t in article_type if isinstance(t, str)), "")
    if isinstance(article_type, str) and article_type:
        meta.article_type = article_type
        _record(meta, "article_type", "json_ld")
        return
    _record(meta, "article_type", None)


def _resolve_tier_c(
    meta: WebMeta,
    jsonld: Optional[dict[str, Any]],
) -> None:
    if jsonld is None:
        for field in (
            "comment_count",
            "editor",
            "reading_time_minutes",
            "dateline_location",
            "paywall_status",
            "correction_notice",
            "editorial_labels",
            "external_citations",
            "images",
            "social_share_counts",
            "revision_date",
        ):
            _record(meta, field, None)
        return

    editor = _stringify(jsonld.get("editor"))
    if editor:
        meta.editor = editor
        _record(meta, "editor", "json_ld")
    else:
        _record(meta, "editor", None)

    location = _stringify(jsonld.get("contentLocation"))
    if location:
        meta.dateline_location = location
        _record(meta, "dateline_location", "json_ld")
    else:
        _record(meta, "dateline_location", None)

    accessible = jsonld.get("isAccessibleForFree")
    if isinstance(accessible, bool):
        meta.paywall_status = not accessible
        _record(meta, "paywall_status", "json_ld")
    elif isinstance(accessible, str) and accessible.strip().lower() in {
        "true",
        "false",
    }:
        meta.paywall_status = accessible.strip().lower() == "false"
        _record(meta, "paywall_status", "json_ld")
    else:
        _record(meta, "paywall_status", None)

    correction = _stringify(jsonld.get("correction"))
    if correction:
        meta.correction_notice = correction
        _record(meta, "correction_notice", "json_ld")
    else:
        _record(meta, "correction_notice", None)

    genres = _list_of_strings(jsonld.get("genre"))
    if genres:
        meta.editorial_labels = genres
        _record(meta, "editorial_labels", "json_ld")
    else:
        _record(meta, "editorial_labels", None)

    citations = _list_of_strings(jsonld.get("citation"))
    if citations:
        meta.external_citations = citations
        _record(meta, "external_citations", "json_ld")
    else:
        _record(meta, "external_citations", None)

    revision = _parse_iso_or_none(jsonld.get("dateModified"))
    if revision is not None and meta.modified_date is None:
        meta.revision_date = revision
        _record(meta, "revision_date", "json_ld")
    elif meta.modified_date is not None:
        meta.revision_date = meta.modified_date
        _record(meta, "revision_date", meta.extraction_methods.get("modified_date"))
    else:
        _record(meta, "revision_date", None)

    interactions = jsonld.get("interactionStatistic") or jsonld.get("commentCount")
    comment_count: Optional[int] = None
    if isinstance(interactions, list):
        for entry in interactions:
            if not isinstance(entry, dict):
                continue
            interaction_type = _stringify(entry.get("interactionType"))
            if "Comment" in interaction_type:
                count = entry.get("userInteractionCount")
                if isinstance(count, (int, str)):
                    try:
                        comment_count = int(count)
                        break
                    except (TypeError, ValueError):
                        continue
    elif isinstance(interactions, (int, str)):
        try:
            comment_count = int(interactions)
        except (TypeError, ValueError):
            comment_count = None
    if comment_count is not None:
        meta.comment_count = comment_count
        _record(meta, "comment_count", "json_ld")
    else:
        _record(meta, "comment_count", None)

    comment_url = _stringify(jsonld.get("discussionUrl"))
    if comment_url:
        meta.comment_url = comment_url
        _record(meta, "comment_url", "json_ld")
    else:
        _record(meta, "comment_url", None)

    word_count_jsonld = jsonld.get("wordCount")
    if isinstance(word_count_jsonld, (int, str)):
        try:
            wc = int(word_count_jsonld)
            if wc > 0:
                # word_count is Tier-B; record method here too to clean up
                # the heuristic later.
                if not meta.word_count:
                    meta.word_count = wc
                    _record(meta, "word_count", "json_ld")
        except (TypeError, ValueError):
            pass

    images: list[ImageRef] = []
    raw_images = jsonld.get("image")
    if isinstance(raw_images, list):
        for entry in raw_images:
            if isinstance(entry, dict):
                images.append(
                    ImageRef(
                        url=_stringify(entry.get("url") or entry.get("contentUrl")),
                        alt_text=_stringify(entry.get("name")),
                        caption=_stringify(entry.get("caption")),
                    )
                )
            elif isinstance(entry, str):
                images.append(ImageRef(url=entry))
    elif isinstance(raw_images, dict):
        images.append(
            ImageRef(
                url=_stringify(raw_images.get("url") or raw_images.get("contentUrl")),
                alt_text=_stringify(raw_images.get("name")),
                caption=_stringify(raw_images.get("caption")),
            )
        )
    if images:
        # Collapse responsive renditions of the same image so `image_count`
        # (len) measures DISTINCT images, not crops (Phase 133 data-quality).
        meta.images = _dedupe_images(images)
        _record(meta, "images", "json_ld")
    else:
        _record(meta, "images", None)

    # Reading-time hint (Schema.org timeRequired is ISO-8601 duration).
    time_required = _stringify(jsonld.get("timeRequired"))
    if time_required.startswith("PT") and time_required.endswith("M"):
        try:
            meta.reading_time_minutes = int(time_required[2:-1])
            _record(meta, "reading_time_minutes", "json_ld")
        except ValueError:
            _record(meta, "reading_time_minutes", None)
    else:
        _record(meta, "reading_time_minutes", None)

    # social_share_counts has no canonical Schema.org field; left empty.
    _record(meta, "social_share_counts", None)


# ---------------------------------------------------------------------------
# Body extraction
# ---------------------------------------------------------------------------
