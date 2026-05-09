"""Five-tier WebMeta envelope for the Phase 122 web-crawl source type.

The WebAdapter produces (SilverCore, WebMeta) pairs from raw HTML stored
verbatim in Bronze. Field richness is structured into five tiers so that
downstream analysis can opt into a "clean" structured-data-only subset
(``extraction_method ∈ {'json_ld', 'microdata'}``) or include heuristic
extractions for full coverage. ADR-015 explicitly marks SilverMeta
subclasses as unstable; Tier-D / Tier-E may grow new keys without an ADR
amendment.

Tiers:

* Tier-A — universal mandatory: missing → DLQ. Works on every probe in
  every language. ``canonical_url``, ``original_url``, ``fetch_at``,
  ``http_status``, ``html_lang``, ``title``.
* Tier-B — standard news metadata almost-always-present via JSON-LD or
  OpenGraph: published_date, modified_date, author, description,
  categories, tags, section, image_url, article_type, word_count.
* Tier-C — rich metadata captured when present: comment counts, editor,
  reading_time_minutes, dateline_location, paywall_status, correction
  notice, editorial_labels, external_citations, images, social share
  counts, revision_date.
* Tier-D — verbatim ``extruct`` dump (``structured_data``). The
  insurance policy: fields we don't think to use today are available the
  moment we want them, without re-crawling.
* Tier-E — source-specific extras (``source_extras``) populated from
  per-source XPath/CSS rules declared in ``probes/<id>/sources.yaml >
  custom_extractors:``. Empty in Phase 122; the slot is reserved.

Provenance markers
------------------

Every Tier-B and Tier-C field has a sibling entry in
``extraction_methods`` keyed by the field name. Allowed values:
``json_ld``, ``open_graph``, ``microdata``, ``rdfa``, ``html_meta``,
``xpath_rule_<rule_id>``, ``heuristic_htmldate``, ``derived``, or
``None`` (the field was not populated). Storing methods in a parallel
dict keeps the WebMeta surface readable while preserving full per-field
provenance.

Timestamp resolution
--------------------

``timestamp_source`` records which origin the WebAdapter used to set
``SilverCore.timestamp``: the priority chain is
``json_ld_published`` → ``open_graph_published`` →
``html_meta_published`` → ``sitemap_lastmod`` → ``http_last_modified``
→ ``fetch_at_fallback``. Anything resolving to ``fetch_at_fallback`` is
flagged so analysis can filter it out as a Negative-Space population
(Brief §7.7).
"""

from datetime import datetime
from typing import Any, Optional

from pydantic import Field

from internal.models import SilverMeta
from internal.models.bias import BiasContext
from internal.models.discourse import DiscourseContext


class ImageRef(SilverMeta):
    """Single image reference associated with the article body.

    Subclassing ``SilverMeta`` is purely a convenience for Pydantic — image
    refs do not stand alone as Silver envelopes. ``source_type`` is set to
    ``"web_image"`` so accidentally serialised refs are recognisable.
    """

    source_type: str = Field(default="web_image")
    url: str = Field(default="")
    alt_text: str = Field(default="")
    caption: str = Field(default="")


class WebMeta(SilverMeta):
    """Source-specific metadata for full-article web crawls (source_type="web")."""

    # ----- Tier-A: universal mandatory ------------------------------------
    canonical_url: str = Field(default="")
    original_url: str = Field(default="")
    fetch_at: Optional[datetime] = None
    http_status: int = Field(default=0)
    html_lang: str = Field(default="")
    title: str = Field(default="")

    # ----- Tier-B: standard news metadata ---------------------------------
    published_date: Optional[datetime] = None
    modified_date: Optional[datetime] = None
    author: str = Field(default="")
    description: str = Field(default="")
    categories: list[str] = Field(default_factory=list)
    tags: list[str] = Field(default_factory=list)
    section: str = Field(default="")
    image_url: str = Field(default="")
    article_type: str = Field(default="")
    word_count: int = Field(default=0)

    # ----- Tier-C: rich metadata ------------------------------------------
    comment_count: Optional[int] = None
    comment_url: str = Field(default="")
    editor: str = Field(default="")
    reading_time_minutes: Optional[int] = None
    dateline_location: str = Field(default="")
    paywall_status: Optional[bool] = None
    correction_notice: str = Field(default="")
    editorial_labels: list[str] = Field(default_factory=list)
    external_citations: list[str] = Field(default_factory=list)
    images: list[ImageRef] = Field(default_factory=list)
    social_share_counts: dict[str, int] = Field(default_factory=dict)
    revision_date: Optional[datetime] = None

    # ----- Tier-D: verbatim structured-data dump --------------------------
    structured_data: dict[str, Any] = Field(default_factory=dict)

    # ----- Tier-E: per-source bespoke extras ------------------------------
    source_extras: dict[str, Any] = Field(default_factory=dict)

    # ----- Provenance markers ---------------------------------------------
    extraction_methods: dict[str, Optional[str]] = Field(default_factory=dict)
    extraction_fallback: Optional[str] = None
    timestamp_source: str = Field(default="")

    # ----- Sitemap context -------------------------------------------------
    sitemap_section: Optional[str] = None
    sitemap_lastmod: Optional[datetime] = None

    # ----- Cross-cutting context (mirrors RssMeta) ------------------------
    discourse_context: Optional[DiscourseContext] = None
    bias_context: Optional[BiasContext] = None


# Allowed values for the extraction-method provenance markers. Used as a
# defensive whitelist by tests — the WebAdapter is the source of truth at
# runtime.
ALLOWED_EXTRACTION_METHODS: frozenset[str] = frozenset(
    {
        "json_ld",
        "open_graph",
        "microdata",
        "rdfa",
        "html_meta",
        "heuristic_htmldate",
        "derived",
    }
)

# Allowed values for ``timestamp_source``. ``fetch_at_fallback`` is the
# Negative-Space sentinel — analysis filters on this to exclude documents
# whose timestamp is the crawl moment rather than a publication-date
# signal.
ALLOWED_TIMESTAMP_SOURCES: frozenset[str] = frozenset(
    {
        "json_ld_published",
        "open_graph_published",
        "html_meta_published",
        "sitemap_lastmod",
        "http_last_modified",
        "fetch_at_fallback",
    }
)
