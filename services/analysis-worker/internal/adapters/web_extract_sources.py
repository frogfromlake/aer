"""Structured-data parsers + date/url/lang helpers — extracted from web_extract.py (Phase 141)."""

from __future__ import annotations

import logging
import re
from datetime import datetime, timezone
from typing import Any, Iterable, Optional

from internal.adapters.web_meta import (
    ALLOWED_EXTRACTION_METHODS,
    WebMeta,
)

from internal.adapters.web_extract_deps import courlan

logger = logging.getLogger(__name__)

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


def _pick_news_article(
    jsonld_blocks: Iterable[dict[str, Any]],
) -> Optional[dict[str, Any]]:
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
