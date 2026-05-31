"""Source adapter for full-article web crawls (Phase 122 / source_type="web").

The crawler stores raw HTML + a fetch envelope verbatim in Bronze; this
adapter is the medallion-architecture-correct boundary at which derived
metadata (cleaned text, structured-data tiers, publication date, author)
is computed. Trafilatura version upgrades replay archived Bronze through
this code without re-crawling — see ADR-028.

The adapter is graceful: when the heavy NLP-stack dependencies
(trafilatura / extruct / htmldate / courlan / readability-lxml) are
absent at import time the adapter still constructs and instantiates,
but every ``harmonize`` call raises :class:`ExtractionFailedError`. The
processor catches it and routes the document to the Bronze DLQ with
reason ``extraction_failed``.
"""

from __future__ import annotations

import threading
import time
from collections import OrderedDict
from datetime import datetime, timezone
from typing import Any, Optional

import structlog
from psycopg2.pool import ThreadedConnectionPool

from internal.adapters.web_extract import (
    EXTRACTION_AVAILABLE,
    canonical_url_or,
    extract_web_document,
)
from internal.adapters.web_meta import WebMeta
from internal.models import SilverCore, generate_document_id
from internal.models.bias import BiasContext
from internal.models.discourse import DiscourseContext
from internal.storage.postgres_client import get_source_classification
from internal.wayback import WaybackCDXClient

logger = structlog.get_logger()


class ExtractionFailedError(ValueError):
    """Raised by the WebAdapter when both trafilatura and readability fail
    to recover a non-empty article body. Subclasses ``ValueError`` so the
    existing harmonization-error path still catches it; the processor
    routes it to the DLQ with reason ``extraction_failed``.
    """


CLASSIFICATION_CACHE_TTL_SECONDS = 60.0
CLASSIFICATION_CACHE_MAX_ENTRIES = 4096


class WebAdapter:
    """Source adapter for ``source_type == "web"``.

    Pulls raw HTML from the Bronze payload, runs the
    :func:`extract_web_document` pipeline, and constructs
    ``(SilverCore, WebMeta)`` with the canonical timestamp-resolution
    chain populated.

    The classification cache mirrors :class:`RssAdapter` so the
    ``DiscourseContext`` lookup does not become an N+1 query during
    high-throughput batch ingest.
    """

    def __init__(
        self,
        pg_pool: ThreadedConnectionPool | None = None,
        wayback_client: WaybackCDXClient | None = None,
    ):
        self._pg_pool = pg_pool
        self._classification_cache: OrderedDict[str, tuple[Optional[dict], float]] = OrderedDict()
        self._cache_lock = threading.Lock()
        # Phase 122d.0 — Silent-Edit Observability. None disables the
        # lookup entirely; the WebMeta `wayback_lookup_status` is left
        # empty so the Gold-row writer skips this article. A
        # non-disabled-but-failing lookup returns a `failed` status
        # which IS recorded on WebMeta — those rows surface as the
        # "we tried, IA was down" signal on the dashboard.
        self._wayback_client = wayback_client

    # ------------------------------------------------------------------
    # Classification cache (mirrors RssAdapter pattern)
    # ------------------------------------------------------------------
    def _get_classification_cached(self, source: str) -> Optional[dict]:
        now = time.monotonic()
        with self._cache_lock:
            entry = self._classification_cache.get(source)
            if entry is not None and now - entry[1] < CLASSIFICATION_CACHE_TTL_SECONDS:
                self._classification_cache.move_to_end(source)
                return entry[0]
        classification = get_source_classification(self._pg_pool, source)
        with self._cache_lock:
            self._classification_cache[source] = (classification, now)
            self._classification_cache.move_to_end(source)
            if len(self._classification_cache) > CLASSIFICATION_CACHE_MAX_ENTRIES:
                self._classification_cache.popitem(last=False)
        return classification

    # ------------------------------------------------------------------
    # SourceAdapter protocol
    # ------------------------------------------------------------------
    def harmonize(
        self,
        raw: dict,
        event_time: datetime,
        bronze_object_key: str,
    ) -> tuple[SilverCore, WebMeta]:
        if not EXTRACTION_AVAILABLE:
            raise ExtractionFailedError(
                "web_extract dependencies missing — cannot harmonise web Bronze"
            )

        source = raw.get("source", "")
        raw_html = raw.get("raw_html") or raw.get("raw_text") or ""
        if not isinstance(raw_html, str) or not raw_html.strip():
            raise ExtractionFailedError("raw_html missing or empty in Bronze payload")

        original_url = raw.get("original_url") or raw.get("url") or ""
        custom_extractors = raw.get("custom_extractors") or {}

        cleaned_text, meta = extract_web_document(
            html=raw_html,
            original_url=original_url,
            custom_extractors=custom_extractors,
        )

        if not cleaned_text.strip():
            raise ExtractionFailedError(
                "extraction produced empty cleaned_text (trafilatura + readability fallback both failed)"
            )

        # ---- Cross-cutting envelope fields the pure pipeline cannot fill --
        meta.fetch_at = _parse_iso_or_event_time(raw.get("fetch_at"), event_time)
        meta.http_status = int(raw.get("http_status") or 0)
        meta.sitemap_lastmod = _parse_iso(raw.get("sitemap_lastmod"))
        meta.sitemap_section = raw.get("sitemap_section") or None

        # Crawler-supplied canonical_url wins when present; otherwise fall
        # back to the courlan output computed by the extractor. Both ought
        # to agree, but the crawler is the dedup-state SoT.
        crawler_canonical = raw.get("canonical_url")
        if isinstance(crawler_canonical, str) and crawler_canonical:
            meta.canonical_url = crawler_canonical
        elif not meta.canonical_url:
            meta.canonical_url = canonical_url_or(original_url)

        http_last_modified = _parse_iso(raw.get("http_last_modified"))

        # ---- Timestamp resolution ----------------------------------------
        timestamp = _resolve_timestamp(meta, http_last_modified, event_time)

        # ---- Silent-Edit Observability (Phase 122d.0) --------------------
        # The CDX lookup is the last step of `harmonize()` per ADR-032:
        # by this point `meta.canonical_url` is final and the
        # cleaned_text / WebMeta tiers are stable. The client itself is
        # fail-silent — every error mode collapses to a typed status on
        # `meta.wayback_lookup_status` and `meta.wayback_revisions`
        # remains the empty list. A CDX outage NEVER produces a DLQ
        # event (enforced by the catch-all in `_resolve_wayback`).
        self._resolve_wayback(meta)

        # ---- DiscourseContext / BiasContext ------------------------------
        discourse_context = self._lookup_discourse(source)
        meta.discourse_context = discourse_context
        meta.bias_context = BiasContext(
            platform_type="web",
            access_method="public_web",
            visibility_mechanism="editorial_homepage_and_sitemap",
            moderation_context="editorial",
            engagement_data_available=False,
            account_metadata_available=False,
        )

        # ---- Build SilverCore -------------------------------------------
        core = SilverCore(
            document_id=generate_document_id(source, bronze_object_key),
            source=source,
            source_type="web",
            raw_text=raw_html,
            cleaned_text=cleaned_text,
            language="und",  # Real value patched in by LanguageDetectionExtractor.
            timestamp=timestamp,
            url=meta.canonical_url or original_url,
            schema_version=2,
            word_count=meta.word_count or len(cleaned_text.split()),
        )
        meta.word_count = core.word_count
        return core, meta

    # ------------------------------------------------------------------
    # Helpers
    # ------------------------------------------------------------------
    def _resolve_wayback(self, meta: WebMeta) -> None:
        """Populate `meta.wayback_revisions` + `wayback_lookup_status`.

        Fail-silent invariant (Phase 122d.0 / ADR-032): a missing
        client, missing URL, or any exception inside the client
        collapses to a typed status on the meta. We never raise out
        of this method — the caller (`harmonize`) treats Wayback as
        purely augmentative provenance.
        """
        if self._wayback_client is None:
            # Worker boot disabled the integration (e.g. tests). Leave
            # the fields blank so the Gold writer skips this article;
            # any non-empty status value would be misleading.
            return
        try:
            result = self._wayback_client.lookup(meta.canonical_url)
        except Exception as exc:
            # Defence-in-depth — `WaybackCDXClient.lookup` is documented
            # to never raise, but a future refactor could regress. We
            # log at INFO and move on.
            logger.info(
                "Wayback CDX adapter call raised; collapsing to failed.",
                canonical_url=meta.canonical_url,
                error=str(exc),
                error_type=type(exc).__name__,
            )
            meta.wayback_lookup_status = "failed"
            meta.wayback_revisions = []
            return
        meta.wayback_lookup_status = result.status
        meta.wayback_revisions = [r.to_dict() for r in result.revisions]

    def _lookup_discourse(self, source: str) -> Optional[DiscourseContext]:
        if self._pg_pool is None or not source:
            return None
        try:
            classification = self._get_classification_cached(source)
        except Exception as exc:
            logger.warning(
                "Failed to fetch source classification. Continuing without discourse context.",
                source=source,
                error=str(exc),
            )
            return None
        if not classification:
            return None
        return DiscourseContext(
            primary_function=classification["primary_function"],
            secondary_function=classification["secondary_function"],
            emic_designation=classification["emic_designation"],
        )


# ---------------------------------------------------------------------------
# Module-level helpers
# ---------------------------------------------------------------------------


def _parse_iso(value: Any) -> Optional[datetime]:
    if not value:
        return None
    if isinstance(value, datetime):
        return value if value.tzinfo else value.replace(tzinfo=timezone.utc)
    if not isinstance(value, str):
        return None
    candidate = value.strip().replace("Z", "+00:00")
    try:
        parsed = datetime.fromisoformat(candidate)
    except ValueError:
        return None
    return parsed if parsed.tzinfo else parsed.replace(tzinfo=timezone.utc)


def _parse_iso_or_event_time(value: Any, event_time: datetime) -> datetime:
    parsed = _parse_iso(value)
    if parsed is not None:
        return parsed
    if event_time.tzinfo is None:
        return event_time.replace(tzinfo=timezone.utc)
    return event_time


def _is_date_only(dt: datetime) -> bool:
    """True when a datetime carries no time-of-day (exact midnight).

    Publishers that expose only a calendar date parse to 00:00:00; this is the
    signal that `publication_hour`/`publication_weekday` would be meaningless
    unless a more precise source (RSS pubDate via `sitemap_lastmod`) exists.
    """
    return dt.hour == 0 and dt.minute == 0 and dt.second == 0 and dt.microsecond == 0


def _same_utc_date(a: datetime, b: datetime) -> bool:
    """True when both datetimes fall on the same calendar day in UTC."""
    ua = a.astimezone(timezone.utc) if a.tzinfo else a.replace(tzinfo=timezone.utc)
    ub = b.astimezone(timezone.utc) if b.tzinfo else b.replace(tzinfo=timezone.utc)
    return ua.date() == ub.date()


def _resolve_timestamp(
    meta: WebMeta,
    http_last_modified: Optional[datetime],
    event_time: datetime,
) -> datetime:
    """Apply the canonical priority chain documented in ADR-028:

    ``published_date`` → ``sitemap_lastmod`` → ``http_last_modified``
    → ``fetch_at``.

    Records the chosen origin in ``meta.timestamp_source``. Anything
    resolving to ``fetch_at_fallback`` is the Negative-Space sentinel
    (Brief §7.7).
    """
    if meta.published_date is not None:
        # ``timestamp_source`` was already set by extract_web_document
        # (json_ld_published / open_graph_published / html_meta_published).
        if not meta.timestamp_source:
            meta.timestamp_source = "html_meta_published"

        # Phase 123c — time-of-day upgrade. Some publishers expose only a
        # DATE on the article page (e.g. elysee.fr → published_date at exact
        # midnight) while the RSS feed that surfaced the URL carries the full
        # timestamp (pubDate "Fri, 29 May 2026 18:15:00"), propagated here as
        # `sitemap_lastmod`. A date-only published_date makes publication_hour
        # collapse to 0 for every such article (TESTING.md Issue 4). When the
        # RSS timestamp falls on the SAME calendar day and carries a real
        # time-of-day, prefer it: the day authority is unchanged, only the hour
        # is recovered. Different-day sitemap_lastmod is left untouched (that is
        # the republication-trigger signal, not a more-precise publish time).
        if (
            _is_date_only(meta.published_date)
            and meta.sitemap_lastmod is not None
            and not _is_date_only(meta.sitemap_lastmod)
            and _same_utc_date(meta.published_date, meta.sitemap_lastmod)
        ):
            meta.timestamp_source = "rss_pubdate_time_upgrade"
            return meta.sitemap_lastmod
        return meta.published_date

    if meta.sitemap_lastmod is not None:
        meta.timestamp_source = "sitemap_lastmod"
        return meta.sitemap_lastmod

    if http_last_modified is not None:
        meta.timestamp_source = "http_last_modified"
        return http_last_modified

    meta.timestamp_source = "fetch_at_fallback"
    if meta.fetch_at is not None:
        return meta.fetch_at
    if event_time.tzinfo is None:
        return event_time.replace(tzinfo=timezone.utc)
    return event_time
