"""Pure-function HTML → (cleaned_text, WebMeta) extraction pipeline.

Runs at the Silver boundary, never in the crawler. Bronze is verbatim
raw HTML; trafilatura version upgrades replay archived Bronze through
this module without re-crawling — the operational realisation of the
medallion architecture's collection-vs-derivation decoupling (ADR-028).

The module is import-tolerant: if the heavy NLP-stack dependencies
(trafilatura, extruct, htmldate, courlan, readability-lxml) are absent
at import time, ``EXTRACTION_AVAILABLE`` flips to ``False`` and
``extract_web_document`` raises a clear ``RuntimeError``. The worker's
graceful-degradation hook in ``init_extractors``-style call sites is
expected to surface the failure as a DLQ reason rather than crashing.

Public surface:

* :func:`canonical_url_or` — courlan wrapper with a deterministic
  fallback when courlan is unavailable.
* :func:`extract_web_document` — the canonical pure function. Inputs:
  raw HTML, original (post-redirect) URL, an opaque ``custom_extractors``
  mapping (Tier-E rules, empty in Phase 122). Outputs: ``(cleaned_text,
  WebMeta)``. The caller is responsible for filling in ``fetch_at``,
  ``http_status``, ``sitemap_*``, and the ``BiasContext`` /
  ``DiscourseContext`` cross-cutting fields on the returned ``WebMeta``.
"""

from __future__ import annotations

import logging
import re
from datetime import datetime, timezone
from typing import Any, Iterable, Optional

from internal.adapters.web_meta import (
    ALLOWED_EXTRACTION_METHODS,
    ALLOWED_TIMESTAMP_SOURCES,
    ImageRef,
    WebMeta,
)

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Optional-dependency probe.
# ---------------------------------------------------------------------------
try:  # pragma: no cover - import shim only
    import trafilatura  # type: ignore
    import extruct  # type: ignore
    import htmldate  # type: ignore
    import courlan  # type: ignore
    from readability import Document as ReadabilityDocument  # type: ignore
    from w3lib.html import get_base_url  # type: ignore

    EXTRACTION_AVAILABLE = True
except Exception as _extract_import_error:  # pragma: no cover - tested via DLQ path
    trafilatura = None  # type: ignore
    extruct = None  # type: ignore
    htmldate = None  # type: ignore
    courlan = None  # type: ignore
    ReadabilityDocument = None  # type: ignore
    get_base_url = None  # type: ignore
    EXTRACTION_AVAILABLE = False
    _IMPORT_ERROR = _extract_import_error
    logger.warning(
        "web_extract: optional NLP dependencies missing — extraction disabled (%s)",
        _extract_import_error,
    )


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_HTML_LANG_RE = re.compile(r"<html[^>]*\blang\s*=\s*['\"]([^'\"]+)['\"]", re.IGNORECASE)

# Phase 122e — F-A3 / F-A4. The `<time datetime="...">` element is the
# canonical HTML5 way to mark a publication date when JSON-LD doesn't
# carry a NewsArticle. Probe 0's bundesregierung.de news pages emit only
# this — no NewsArticle JSON-LD, no `article:published_time` OG tag.
# Without reading it the WebAdapter falls all the way through to
# htmldate's heuristic, which produces `YYYY-01-01 00:00:00` from year-
# only strings in page footers — a fake-precise stamp that collapses
# every article to a single instant in time and breaks every downstream
# temporal analysis. The first match below is the article-level date.
_TIME_DATETIME_RE = re.compile(
    r"<time[^>]*\bdatetime\s*=\s*['\"]([^'\"]+)['\"][^>]*>",
    re.IGNORECASE,
)
# Also catch `<meta property="article:published_time">` and the variants
# `name="published_time"`, `itemprop="datePublished"` — three more
# publisher-emitted signals that pre-empt htmldate's heuristic.
_META_PUBLISHED_RE = re.compile(
    r"<meta[^>]+(?:property|name|itemprop)\s*=\s*['\"]"
    r"(?:article:published_time|published_time|date|pubdate|publishdate|datePublished)"
    r"['\"][^>]+content\s*=\s*['\"]([^'\"]+)['\"][^>]*>"
    r"|"
    r"<meta[^>]+content\s*=\s*['\"]([^'\"]+)['\"][^>]+(?:property|name|itemprop)\s*=\s*['\"]"
    r"(?:article:published_time|published_time|date|pubdate|publishdate|datePublished)"
    r"['\"][^>]*>",
    re.IGNORECASE,
)


def _extract_html_meta_published(html: str) -> Optional[datetime]:
    """Pull the article's publication date from publisher-emitted HTML5
    elements (`<time datetime="...">`) and `<meta>` tags
    (`article:published_time`, `pubdate`, `datePublished`, ...). This
    runs after JSON-LD / OG resolution but before falling through to
    htmldate's heuristic. Returns the first parsable timestamp.
    """
    # `<time datetime="...">` first — it's the most semantically explicit
    # publisher signal in HTML5.
    for match in _TIME_DATETIME_RE.finditer(html):
        candidate = _parse_iso_or_none(match.group(1))
        if candidate is not None:
            return candidate
    # `<meta>` variants second.
    for match in _META_PUBLISHED_RE.finditer(html):
        # The regex has two alternative groups depending on attribute order.
        value = match.group(1) or match.group(2)
        candidate = _parse_iso_or_none(value)
        if candidate is not None:
            return candidate
    return None


def _is_year_floor_sentinel(candidate: datetime, html: str) -> bool:
    """Phase 122e — F-A4. htmldate's heuristic sometimes returns
    ``YYYY-01-01 00:00:00`` (start-of-year) when no real publication
    date is present in the HTML — typically picked up from a year string
    in a footer copyright notice. Treating that as an authoritative
    publication date collapses every article on the same site to a
    single instant in time (Probe 0's bundesregierung sample produced
    `2026-01-01 00:00:00` for ALL 201 articles).

    Detect the year-floor pattern: midnight on January 1st with no
    corroborating `<time datetime="YYYY-01-01...">` or `<meta>` tag
    bearing exactly the same date. A real Jan 1 publication will have a
    matching authoritative element; a sentinel-only stamp will not.
    """
    if candidate.month != 1 or candidate.day != 1:
        return False
    if candidate.hour != 0 or candidate.minute != 0 or candidate.second != 0:
        return False
    target_prefix = candidate.strftime("%Y-01-01")
    # Look for any element that explicitly says this exact date.
    for match in _TIME_DATETIME_RE.finditer(html):
        if target_prefix in (match.group(1) or ""):
            return False
    for match in _META_PUBLISHED_RE.finditer(html):
        value = match.group(1) or match.group(2) or ""
        if target_prefix in value:
            return False
    return True


def _log_midnight_htmldate_observation(
    candidate: datetime, html: str, canonical_url: str
) -> None:
    """Phase 122e A14 — defensive monitoring for htmldate's midnight
    stamps that pass the year-floor sentinel check.

    When htmldate resolves a publication date whose time component is
    exactly ``00:00:00`` AND no ``<time datetime="...">`` element
    exists in the source HTML, the date is most likely derived from a
    date STRING in the headline / footer (e.g. ``"7. Mai 2026"``)
    rather than a structured publisher signal. The date may be
    accurate (speeches and press releases are often dated to "the
    day" without a time component), but the precision is undocumented
    and could mask a finer-grained signal that downstream temporal
    analyses might want to weight.

    This is **monitoring only**: the date is still recorded, the
    extraction_method already records ``heuristic_htmldate`` honestly,
    and analyses that need precision can filter on
    ``extraction_method != "heuristic_htmldate"`` already. The log
    line lets us measure how often this case fires across the corpus
    and lets us escalate to a real bug if downstream analyses prove
    sensitive to midnight imprecision (Phase 122e A14 → escalation
    threshold lives in the operations playbook).

    Cross-reference: F-A4 (year-floor sentinel) is a HARDER rejection
    that fires only on ``YYYY-01-01 00:00:00`` with no corroboration.
    A14 is a SOFTER observation that fires on any midnight stamp from
    htmldate without a `<time>` element — it captures real-but-
    imprecise dates the year-floor check correctly accepts.
    """
    if candidate.hour != 0 or candidate.minute != 0 or candidate.second != 0:
        return
    if _TIME_DATETIME_RE.search(html):
        return
    logger.info(
        "htmldate_midnight_observation: published_date=%s url=%s",
        candidate.isoformat(),
        canonical_url or "<unknown>",
    )


def canonical_url_or(original_url: str) -> str:
    """Return courlan-canonicalised URL, or the input verbatim if courlan
    is unavailable. Pure: no I/O.
    """
    if not original_url:
        return ""
    if courlan is None:
        return original_url
    try:
        normalised = courlan.normalize_url(original_url)
        if isinstance(normalised, tuple):
            normalised = normalised[0]
        return normalised or original_url
    except Exception:
        return original_url


def _parse_iso_or_none(value: Any) -> Optional[datetime]:
    """Parse an ISO-8601 string into a UTC ``datetime``. Forgiving: returns
    ``None`` on any failure rather than raising — extraction is best-effort.
    """
    if value is None:
        return None
    if isinstance(value, datetime):
        return value if value.tzinfo else value.replace(tzinfo=timezone.utc)
    if not isinstance(value, str) or not value.strip():
        return None
    candidate = value.strip().replace("Z", "+00:00")
    try:
        parsed = datetime.fromisoformat(candidate)
    except ValueError:
        # Looser fallback: drop fractional seconds and trailing junk.
        candidate = re.sub(r"\.\d+", "", candidate)
        try:
            parsed = datetime.fromisoformat(candidate)
        except ValueError:
            return None
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=timezone.utc)
    return parsed


def _detect_html_lang(html: str) -> str:
    if not html:
        return ""
    match = _HTML_LANG_RE.search(html)
    if not match:
        return ""
    return match.group(1).strip().split("-")[0].lower()


def _flatten_jsonld(blob: Any) -> Iterable[dict[str, Any]]:
    """Yield every JSON-LD object (including ``@graph`` entries) under the
    given JSON-LD payload. Robust to lists, dicts, and ``@graph``-wrapped
    structures.
    """
    if blob is None:
        return
    if isinstance(blob, list):
        for item in blob:
            yield from _flatten_jsonld(item)
        return
    if isinstance(blob, dict):
        graph = blob.get("@graph")
        if isinstance(graph, list):
            for item in graph:
                yield from _flatten_jsonld(item)
        yield blob


def _pick_news_article(jsonld_blocks: Iterable[dict[str, Any]]) -> Optional[dict[str, Any]]:
    """Pick the first JSON-LD object whose ``@type`` matches a news-article
    Schema.org type. Falls back to the first ``Article`` if no
    ``NewsArticle``/``ReportageNewsArticle`` is present.
    """
    candidates: list[dict[str, Any]] = []
    for item in jsonld_blocks:
        if not isinstance(item, dict):
            continue
        item_type = item.get("@type")
        types: list[str]
        if isinstance(item_type, list):
            types = [t for t in item_type if isinstance(t, str)]
        elif isinstance(item_type, str):
            types = [item_type]
        else:
            continue
        if any(t.endswith("NewsArticle") or t == "ReportageNewsArticle" for t in types):
            return item
        if "Article" in types:
            candidates.append(item)
    return candidates[0] if candidates else None


def _stringify(value: Any) -> str:
    """Return a printable string for a JSON-LD value that may be a dict
    (``{"@value": "..."}``), a list, or a scalar.
    """
    if value is None:
        return ""
    if isinstance(value, str):
        return value.strip()
    if isinstance(value, dict):
        for key in ("@value", "name", "headline"):
            inner = value.get(key)
            if isinstance(inner, str) and inner.strip():
                return inner.strip()
        return ""
    if isinstance(value, list):
        for item in value:
            text = _stringify(item)
            if text:
                return text
    return str(value)


def _list_of_strings(value: Any) -> list[str]:
    if value is None:
        return []
    if isinstance(value, str):
        return [s.strip() for s in re.split(r"[,;]", value) if s.strip()]
    if isinstance(value, list):
        out: list[str] = []
        for item in value:
            text = _stringify(item)
            if text:
                out.append(text)
        return out
    return []


def _record(meta: WebMeta, field_name: str, method: Optional[str]) -> None:
    """Stamp the provenance marker for a Tier-B/C field."""
    if method is not None and method not in ALLOWED_EXTRACTION_METHODS:
        # Defensive: never write an out-of-vocabulary method. The whitelist
        # is also enforced in tests.
        method = None
    meta.extraction_methods[field_name] = method


# ---------------------------------------------------------------------------
# Sub-extractors
# ---------------------------------------------------------------------------


def _extract_jsonld(structured_data: dict[str, Any]) -> Optional[dict[str, Any]]:
    blob = structured_data.get("json-ld") or structured_data.get("json_ld")
    if not blob:
        return None
    return _pick_news_article(_flatten_jsonld(blob))


def _extract_open_graph(structured_data: dict[str, Any]) -> dict[str, Any]:
    og_blob = structured_data.get("opengraph") or structured_data.get("open_graph")
    if not og_blob:
        return {}
    if isinstance(og_blob, list) and og_blob:
        # extruct emits a list of OG blocks; first entry is the article-level set.
        first = og_blob[0]
        if isinstance(first, dict):
            properties = first.get("properties")
            if isinstance(properties, list):
                return {k: v for k, v in properties if isinstance(k, str)}
            if isinstance(first, dict):
                return first
    if isinstance(og_blob, dict):
        return og_blob
    return {}


def _extract_microdata(structured_data: dict[str, Any]) -> Optional[dict[str, Any]]:
    md_blob = structured_data.get("microdata")
    if not md_blob:
        return None
    if isinstance(md_blob, list):
        for entry in md_blob:
            if not isinstance(entry, dict):
                continue
            entry_type = entry.get("type")
            types = entry_type if isinstance(entry_type, list) else [entry_type]
            if any(isinstance(t, str) and "Article" in t for t in types):
                props = entry.get("properties")
                if isinstance(props, dict):
                    return props
    return None


# ---------------------------------------------------------------------------
# Tier-B/C resolution
# ---------------------------------------------------------------------------


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
            heuristic = htmldate.find_date(html, original_date=True, url=canonical_url or None)
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
            first_keyword = keywords_value.split(",")[0].strip() if keywords_value else ""
        if first_keyword:
            meta.section = first_keyword
            _record(meta, "section", "json_ld")
            return
    _record(meta, "section", None)


def _extract_image_url(value: Any) -> str:
    """Return a single URL string from a JSON-LD ``image`` value.

    Phase 122e A25 / F-A25 — JSON-LD ``image`` may be:
      * a bare URL string: ``"https://example.com/foo.jpg"``;
      * a single ``ImageObject`` dict:
        ``{"@type": "ImageObject", "url": "https://..."}`` — the URL
        sits under ``url`` (preferred), ``@id``, or ``contentUrl``;
      * an array of any of the above (the most common pattern when an
        article carries multiple promotional images).

    The previous implementation called ``_stringify(image)`` which
    fell through to ``str(value)`` for a list of ImageObjects,
    producing a stringified Python list-of-dict like
    ``"[{'@type': 'ImageObject', 'url': '...'}]"`` instead of the URL.
    The schema (``WebMeta.image_url: str``) requires a URL string.

    Returns the first URL it finds, or ``""`` if none.
    """
    if value is None:
        return ""
    if isinstance(value, str):
        return value.strip()
    if isinstance(value, dict):
        for key in ("url", "@id", "contentUrl"):
            inner = value.get(key)
            if isinstance(inner, str) and inner.strip():
                return inner.strip()
        return ""
    if isinstance(value, list):
        for item in value:
            url = _extract_image_url(item)
            if url:
                return url
    return ""


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
    elif isinstance(accessible, str) and accessible.strip().lower() in {"true", "false"}:
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
        meta.images = images
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


def _extract_body(html: str) -> tuple[str, Optional[str]]:
    """Run trafilatura with readability fallback. Returns (cleaned_text,
    fallback_marker). ``fallback_marker`` is ``"readability"`` when
    readability salvaged the body, ``None`` otherwise.
    """
    cleaned = ""
    if trafilatura is not None:
        try:
            cleaned = (
                trafilatura.extract(
                    html,
                    include_comments=False,
                    include_tables=False,
                    favor_recall=True,
                    deduplicate=True,
                )
                or ""
            )
        except Exception:
            cleaned = ""

    if cleaned.strip():
        return cleaned.strip(), None

    # Phase 122 fallback: readability salvages obvious-but-tricky article
    # pages (missing JSON-LD, weird template). Only attempted when the
    # HTML smells article-shaped.
    if (
        ReadabilityDocument is not None
        and len(html) > 5000
        and ("<article" in html.lower() or "schema.org/Article" in html)
    ):
        try:
            doc = ReadabilityDocument(html)
            summary_html = doc.summary(html_partial=True)
            text = re.sub(r"<[^>]+>", " ", summary_html)
            text = re.sub(r"\s+", " ", text).strip()
            if text:
                return text, "readability"
        except Exception:
            pass

    return "", None


def _extruct_safe(html: str, base_url: str) -> dict[str, Any]:
    if extruct is None:
        return {}
    try:
        return extruct.extract(
            html,
            base_url=base_url or None,
            syntaxes=["json-ld", "microdata", "opengraph", "rdfa", "microformat"],
            uniform=True,
        ) or {}
    except Exception as exc:  # pragma: no cover - defensive
        logger.warning("extruct.extract failed; structured_data will be empty (%s)", exc)
        return {}


# ---------------------------------------------------------------------------
# Custom extractors (Tier-E)
# ---------------------------------------------------------------------------


def _apply_custom_extractors(
    html: str,
    custom_extractors: dict[str, Any],
) -> dict[str, Any]:
    """Apply per-source XPath/CSS rules. Empty in Phase 122; the slot is
    reserved. The format is intentionally minimal:

    .. code-block:: yaml

       custom_extractors:
         dossier_label:
           xpath: //meta[@name='dossier']/@content
         live_blog_flag:
           css: ".liveblog-banner"

    The first match per rule is stored verbatim. Failure (no match,
    invalid expression) is silent — the value is simply absent.
    """
    if not custom_extractors:
        return {}
    try:
        from lxml import html as lxml_html  # type: ignore
    except Exception:
        return {}

    try:
        tree = lxml_html.fromstring(html)
    except Exception:
        return {}

    out: dict[str, Any] = {}
    for rule_id, rule in custom_extractors.items():
        if not isinstance(rule, dict):
            continue
        try:
            if "xpath" in rule:
                results = tree.xpath(rule["xpath"])
            elif "css" in rule:
                results = tree.cssselect(rule["css"])
            else:
                continue
            if not results:
                continue
            first = results[0]
            if hasattr(first, "text_content"):
                out[rule_id] = first.text_content().strip()
            else:
                out[rule_id] = str(first).strip()
        except Exception:
            continue
    return out


# ---------------------------------------------------------------------------
# Top-level entrypoint
# ---------------------------------------------------------------------------


def extract_web_document(
    html: str,
    original_url: str,
    custom_extractors: Optional[dict[str, Any]] = None,
) -> tuple[str, WebMeta]:
    """Pure HTML → (cleaned_text, WebMeta) extraction.

    The caller fills in the cross-cutting fields the pure pipeline cannot
    know: ``fetch_at``, ``http_status``, ``sitemap_section``,
    ``sitemap_lastmod``, ``original_url`` (mirrored back from the
    argument), ``BiasContext``, ``DiscourseContext``, and any
    timestamp-source override (e.g. ``sitemap_lastmod`` /
    ``http_last_modified``) when ``timestamp_source`` is empty after
    extraction.
    """
    if not EXTRACTION_AVAILABLE:
        raise RuntimeError(
            "web_extract: optional NLP dependencies missing; cannot extract "
            "(install trafilatura, extruct, htmldate, courlan, readability-lxml)."
        )

    if not isinstance(html, str):
        raise TypeError("html must be a str (pre-decoded UTF-8 / declared encoding)")

    canonical = canonical_url_or(original_url)
    base_url = canonical or original_url
    if base_url and get_base_url is not None:
        try:
            base_url = get_base_url(html, base_url)
        except Exception:
            pass

    structured_data = _extruct_safe(html, base_url=base_url)
    jsonld = _extract_jsonld(structured_data)
    og = _extract_open_graph(structured_data)
    microdata = _extract_microdata(structured_data)

    cleaned_text, fallback_marker = _extract_body(html)
    word_count = len(cleaned_text.split()) if cleaned_text else 0

    meta = WebMeta(
        source_type="web",
        canonical_url=canonical,
        original_url=original_url,
        html_lang=_detect_html_lang(html),
        title=_resolve_title(jsonld, og, html),
        word_count=word_count,
        structured_data=structured_data,
        extraction_fallback=fallback_marker,
    )

    _record(meta, "title", _title_method(jsonld, og, html))
    _record(meta, "word_count", "derived")

    _resolve_published_date(meta, jsonld, og, microdata, html, canonical)
    _resolve_modified_date(meta, jsonld, og)
    _resolve_author(meta, jsonld, og)
    _resolve_description(meta, jsonld, og, microdata)
    _resolve_section(meta, jsonld, og)
    _resolve_image(meta, jsonld, og)
    _resolve_categories_and_tags(meta, jsonld, og)
    _resolve_article_type(meta, jsonld)
    _resolve_tier_c(meta, jsonld)

    if custom_extractors:
        meta.source_extras = _apply_custom_extractors(html, custom_extractors)

    # Tier-A guarantees: ``timestamp_source`` may still be empty if neither
    # JSON-LD nor OG nor htmldate produced a publication date. The caller
    # is expected to extend the resolution chain with sitemap_lastmod /
    # http_last_modified / fetch_at before sealing the SilverEnvelope.
    if meta.timestamp_source and meta.timestamp_source not in ALLOWED_TIMESTAMP_SOURCES:
        meta.timestamp_source = ""

    return cleaned_text, meta


# ---------------------------------------------------------------------------
# Title resolution helpers (separate so the method tag is symmetrical).
# ---------------------------------------------------------------------------


_TITLE_TAG_RE = re.compile(r"<title[^>]*>(.*?)</title>", re.IGNORECASE | re.DOTALL)


def _resolve_title(
    jsonld: Optional[dict[str, Any]], og: dict[str, Any], html: str
) -> str:
    if jsonld is not None:
        title = _stringify(jsonld.get("headline") or jsonld.get("name"))
        if title:
            return title
    title = _stringify(og.get("og:title"))
    if title:
        return title
    match = _TITLE_TAG_RE.search(html)
    if match:
        return re.sub(r"\s+", " ", match.group(1)).strip()
    return ""


def _title_method(
    jsonld: Optional[dict[str, Any]], og: dict[str, Any], html: str
) -> Optional[str]:
    if jsonld is not None and (_stringify(jsonld.get("headline") or jsonld.get("name"))):
        return "json_ld"
    if _stringify(og.get("og:title")):
        return "open_graph"
    if _TITLE_TAG_RE.search(html):
        return "html_meta"
    return None
