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
from typing import Any, Optional

from internal.adapters.web_meta import (
    ALLOWED_TIMESTAMP_SOURCES,
    WebMeta,
)

from internal.adapters.web_extract_deps import (
    EXTRACTION_AVAILABLE,
    trafilatura,
    extruct,
    ReadabilityDocument,
    get_base_url,
)
from internal.adapters.web_extract_sources import (
    canonical_url_or,
    _detect_html_lang,
    _extract_jsonld,
    _extract_microdata,
    _extract_open_graph,
    _record,
    _stringify,
)
from internal.adapters.web_extract_images import (
    _dedupe_images,
    _extract_image_url,
    _image_identity,
)
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

logger = logging.getLogger(__name__)

# Public surface of the extraction package. web_extract.py is the facade: it
# owns extract_web_document + the body/extruct/title helpers and re-exports the
# pieces the production WebAdapter (web.py: EXTRACTION_AVAILABLE, canonical_url_or)
# and the tests (_extract_image_url, _dedupe_images, _image_identity) import.
# Listed in __all__ so the lint dead-import ratchet treats the re-exports as used.
__all__ = [
    "extract_web_document",
    "EXTRACTION_AVAILABLE",
    "canonical_url_or",
    "_extract_image_url",
    "_dedupe_images",
    "_image_identity",
]


def _extract_body(html: str) -> tuple[str, Optional[str]]:
    """Run trafilatura with readability fallback. Returns (cleaned_text,
    fallback_marker). ``fallback_marker`` is ``"readability"`` when
    readability salvaged the body, ``None`` otherwise.

    Phase 131a (BUG 1.1) — extraction tightened to ``favor_precision``
    so ``cleaned_text`` carries article prose (headline, lead, body)
    only, never publisher chrome (navigation menus, footer links,
    "Mehr aus …" rail items, video module captions). Every consumer
    of ``SilverCore.cleaned_text`` (NER, sentiment, word-count,
    BERTopic, …) inherits the cleaner input, so the co-occurrence
    network no longer contains outlet self-references like
    "ARD-aktuell" / "tagesschau.de" / "Video Tagesschau" as nodes.

    The previous ``favor_recall=True`` was greedy by design — it
    pulled in any text block trafilatura was uncertain about, which
    on news templates is mostly navigation and the related-articles
    rail. The Phase-122 cut-over comment trail records no
    investigated reason for it; this restores trafilatura to its
    document-default behaviour with an explicit precision bias.

    Crucially, ``include_links`` keeps its default (True) — news
    articles routinely wrap named entities ("Olaf Scholz", "SPD",
    "Bundestag") inside hyperlinks to dossier/Wikipedia/internal
    pages. Stripping link text would remove those entities entirely
    from NER input, regressing the very co-occurrence network this
    phase is meant to repair.

    If ``favor_precision`` returns empty text on a real article (rare;
    seen on aggressively-templated pages) the readability fallback
    below picks it up just as before.
    """
    cleaned = ""
    if trafilatura is not None:
        try:
            cleaned = (
                trafilatura.extract(
                    html,
                    include_comments=False,
                    include_tables=False,
                    favor_precision=True,
                    # deduplicate=False: extract every article independently.
                    # trafilatura's dedup keeps a PROCESS-GLOBAL LRU of seen text
                    # segments, so in the long-running worker a legitimate article
                    # whose paragraphs recur across articles (institutional
                    # boilerplate) gets silently discarded -> empty cleaned_text
                    # -> a false ExtractionFailedError -> DLQ. Article-level dedup
                    # is the crawler's job (the crawler is the dedup-state SoT,
                    # see web.py). Surfaced by an order-dependent unit-test flake.
                    deduplicate=False,
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
        return (
            extruct.extract(
                html,
                base_url=base_url or None,
                syntaxes=["json-ld", "microdata", "opengraph", "rdfa", "microformat"],
                uniform=True,
            )
            or {}
        )
    except Exception as exc:  # pragma: no cover - defensive
        logger.warning(
            "extruct.extract failed; structured_data will be empty (%s)", exc
        )
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
    if jsonld is not None and (
        _stringify(jsonld.get("headline") or jsonld.get("name"))
    ):
        return "json_ld"
    if _stringify(og.get("og:title")):
        return "open_graph"
    if _TITLE_TAG_RE.search(html):
        return "html_meta"
    return None
