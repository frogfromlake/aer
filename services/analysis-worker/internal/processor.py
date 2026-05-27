import json
import os
import uuid
import structlog
from datetime import datetime, timedelta, timezone
from urllib.parse import unquote, urlparse
from minio import Minio
from psycopg2.pool import ThreadedConnectionPool
from internal.adapters.web import ExtractionFailedError
from internal.models import ValidationError
from internal.adapters.registry import AdapterRegistry
from internal.extractors.base import MetricExtractor, ProvenanceExtractor, GoldMetric, GoldEntity, GoldEntityLink, GoldLanguageDetection, ExtractionResult
from internal.metrics import (
    events_processed_total,
    event_processing_duration_seconds,
    events_quarantined_total,
    dlq_size,
    analysis_worker_poison_messages_total,
    analysis_worker_archived_only_total,
)
from internal.storage.postgres_client import (
    get_document_status,
    release_document_claim,
    try_claim_document,
    update_document_article_id,
    update_document_status,
)
from internal import quarantine as _quarantine_module
from internal import silver as _silver_module
from internal import silver_projection as _silver_projection_module
from internal import metadata_coverage as _metadata_coverage_module
from internal import article_revisions as _article_revisions_module
from internal.models.probe_scope import ProbeLanguageScope

logger = structlog.get_logger()


# Phase 122e A26 / F-A26 — the worker's analytical window. Articles whose
# extracted `core.timestamp` falls outside this window are preserved in the
# Silver MinIO archive layer but excluded from CH analytics inserts (which
# would otherwise be evicted by the schema's TTL on next merge anyway,
# producing transient raw-vs-MV drift). Default 365 days matches the TTL
# on `aer_silver.documents` and `aer_gold.metrics`.
DEFAULT_ANALYTICAL_WINDOW_DAYS = 365


def _analytical_window_days() -> int:
    raw = os.getenv("WORKER_ANALYTICAL_WINDOW_DAYS", "").strip()
    if not raw:
        return DEFAULT_ANALYTICAL_WINDOW_DAYS
    try:
        value = int(raw)
        if value <= 0:
            raise ValueError
        return value
    except (TypeError, ValueError):
        logger.warning(
            "WORKER_ANALYTICAL_WINDOW_DAYS unparseable; falling back to default",
            raw=raw,
            default=DEFAULT_ANALYTICAL_WINDOW_DAYS,
        )
        return DEFAULT_ANALYTICAL_WINDOW_DAYS


def _is_within_analytical_window(article_timestamp: datetime, window_days: int) -> bool:
    """Return True iff ``article_timestamp`` is within the analytical
    window — i.e. not so old that the CH TTL would evict it on insert.

    A naive datetime is assumed to be UTC (matches WebAdapter convention
    where ``core.timestamp`` carries the parsed JSON-LD/meta date, which
    is normalised to UTC at extraction time).
    """
    if article_timestamp.tzinfo is None:
        article_timestamp = article_timestamp.replace(tzinfo=timezone.utc)
    cutoff = datetime.now(tz=timezone.utc) - timedelta(days=window_days)
    return article_timestamp >= cutoff


def _derive_discourse_function(meta) -> str:
    """Return the discourse primary_function from a SilverMeta, or "".

    This is the single sanctioned point where Gold-row assembly reads
    `SilverMeta`. Isolating it here keeps the `MetricExtractor` protocol
    meta-agnostic (Phase 76) and gives a clear extension point should
    further meta-derived Gold columns appear.
    """
    if meta is None:
        return ""
    ctx = getattr(meta, "discourse_context", None)
    if ctx is None:
        return ""
    return ctx.primary_function or ""


# Phase 116: TLD heuristic for German variety. A coarse metadata signal,
# NOT a dialect classifier — documents the publishing locale of the feed,
# not the linguistic variety of the prose.
_GERMAN_VARIETY_BY_TLD = {".at": "de-AT", ".ch": "de-CH"}


def _derive_language_variety(meta, detected_language: str) -> str:
    """Return a coarse language-variety tag derived from RssMeta.feed_url TLD.

    Only emits a non-empty value for German texts (Phase 116 scope). The
    TLD lookup runs against the host portion of `RssMeta.feed_url`.

    This is the single sanctioned point where the `language_detections`
    Gold-row assembly reads `SilverMeta`, parallel to
    `_derive_discourse_function` for the metrics/entities tables.
    """
    if detected_language != "de":
        return ""
    if meta is None:
        return ""
    feed_url = getattr(meta, "feed_url", "") or ""
    if not feed_url:
        return "de-DE"
    try:
        host = urlparse(feed_url).hostname or ""
    except ValueError:
        return "de-DE"
    host = host.lower()
    for suffix, variety in _GERMAN_VARIETY_BY_TLD.items():
        if host.endswith(suffix):
            return variety
    return "de-DE"


class DataProcessor:
    """
    Source-agnostic data processor for the AĒR Medallion Architecture.

    Fetches Bronze data, delegates harmonization to source-specific adapters
    via the AdapterRegistry, validates the SilverCore contract, writes Silver,
    extracts Gold metrics via the extractor pipeline, and manages the Dead
    Letter Queue (DLQ).
    """
    def __init__(self, minio_client: Minio, ch_client, pg_pool: ThreadedConnectionPool, adapter_registry: AdapterRegistry, extractors: list[MetricExtractor] | None = None, language_scope: ProbeLanguageScope | None = None, analytical_window_days: int | None = None):
        self.minio = minio_client
        self.ch = ch_client
        self.pg = pg_pool
        self.adapter_registry = adapter_registry
        self.extractors: list[MetricExtractor] = extractors or []
        self.language_scope = language_scope
        # Phase 122e A26 / F-A26.
        self.analytical_window_days = (
            analytical_window_days
            if analytical_window_days is not None
            else _analytical_window_days()
        )
        # Bronze bucket name, shared with ingestion-api via .env / compose.
        # Two services, one truth: if ingestion-api writes to a renamed
        # bucket, the worker must read from the same one.
        self.bronze_bucket = os.getenv("WORKER_BRONZE_BUCKET", "bronze")
        self._extraction_provenance: dict[str, str] = {
            e.name: e.version_hash
            for e in self.extractors
            if isinstance(e, ProvenanceExtractor)
        }

    def process_event(self, obj_key: str, event_time_str: str, span):
        with event_processing_duration_seconds.time():
            try:
                self._process_event_inner(obj_key, event_time_str, span)
            except BaseException:
                # Phase 122e A27 / F-A27 — release the processing claim
                # if we exit non-terminally so NATS redelivery can pick
                # the message up again. CAS: only releases if status is
                # currently 'processing' (i.e., we held the claim and
                # neither succeeded nor quarantined). Quarantine /
                # processed paths set their terminal status before
                # returning, so the release is a no-op for those.
                try:
                    release_document_claim(self.pg, obj_key)
                except Exception as release_err:
                    logger.warning(
                        "Failed to release processing claim after exception",
                        object=obj_key,
                        error=str(release_err),
                    )
                raise

    def _process_event_inner(self, obj_key: str, event_time_str: str, span):
        # --- 1. ATOMIC IDEMPOTENCY CLAIM (Phase 122e A27 / F-A27). ---
        # Compare-and-swap status from {pending|uploaded|NULL} to
        # 'processing' in a single SQL UPDATE...RETURNING. Whichever
        # worker wins the row update gets to process; concurrent
        # NATS redeliveries see zero rows updated and skip immediately.
        # This eliminates the race window where two workers both
        # observed status='uploaded' and both proceeded to insert into
        # ClickHouse, producing silent raw-vs-MV drift (caught by
        # source-table dedup on raw but not propagated to dependent MVs
        # on non-Replicated engines).
        if not try_claim_document(self.pg, obj_key):
            logger.info(
                "Event already claimed (processed / quarantined / in-flight by another worker). Skipping duplicate.",
                object=obj_key,
            )
            span.set_attribute("aer.status", "skipped_duplicate")
            return

        # Parse the ISO 8601 string from MinIO Event
        event_time = datetime.fromisoformat(event_time_str.replace('Z', '+00:00'))

        # --- 2. Fetch raw data (Bronze Layer) ---
        response = self.minio.get_object(self.bronze_bucket, obj_key)
        try:
            raw_content = json.loads(response.read().decode('utf-8'))
        finally:
            response.close()
            response.release_conn()

        # --- 3. Resolve source adapter ---
        source_type = raw_content.get("source_type", "legacy")
        adapter = self.adapter_registry.get(source_type)

        if adapter is None:
            logger.warning(
                "Unknown source_type. No adapter registered. Moving to DLQ.",
                object=obj_key,
                source_type=source_type,
            )
            self._quarantine(obj_key, raw_content, f"unknown_source_type:{source_type}", span)
            return

        # --- 4. Harmonization (Bronze → Silver, via adapter) ---
        try:
            core, meta = adapter.harmonize(raw_content, event_time, obj_key)
        except ExtractionFailedError as e:
            logger.warning("Web extraction failed. Moving to DLQ.", object=obj_key, error=str(e))
            self._quarantine(obj_key, raw_content, "extraction_failed", span)
            return
        except (ValidationError, ValueError) as e:
            logger.warning("Harmonization failed. Moving to DLQ.", object=obj_key, error=str(e))
            self._quarantine(obj_key, raw_content, "harmonization_failed", span)
            return

        # --- 5. Validation (The Silver Contract) ---
        try:
            if not core.cleaned_text:
                raise ValueError("cleaned_text field cannot be empty after harmonization.")
            if not core.raw_text:
                raise ValueError("raw_text field cannot be empty after harmonization.")
        except ValueError as e:
            logger.warning("Silver contract validation failed. Moving to DLQ.", object=obj_key, error=str(e))
            self._quarantine(obj_key, raw_content, "silver_validation_failed", span)
            return

        # --- 6. Language detection (must precede Silver writes) ---
        # Phase 120b: detection runs *before* the Silver layers so the
        # rank=1 consensus winner replaces the adapter-set placeholder on
        # core.language for *every* downstream consumer — both Silver
        # writes (MinIO envelope + ClickHouse projection) and the
        # per-document extractors below. Pre-Phase-120b the detection
        # ran inside the extractor loop after Silver was already written,
        # so the ClickHouse Silver projection always carried "und" and
        # corpus-level extractors that partition by language (e.g. the
        # BERTopic loop in Phase 120) silently dropped every RSS doc.
        article_id = core.document_id
        working_core, detection_result = self._run_language_detection(
            core, article_id, obj_key
        )

        # --- 6b. Probe language-scope enforcement (Phase 122e A17 / F-A17). ---
        # Defense-in-depth against URL-filter regressions: even if the
        # crawler-side filter (sources.yaml `exclude_path_prefixes`) lets a
        # foreign-language URL through, the worker refuses to admit a doc
        # whose detected language falls outside the source's allow-list.
        # The crawler's URL filter is best-effort; the worker's content
        # filter is authoritative.
        if self.language_scope is not None and not self.language_scope.is_in_scope(
            working_core.source, working_core.language or ""
        ):
            logger.warning(
                "Language-scope violation. Detected language is outside the source's allow-list. Moving to DLQ.",
                object=obj_key,
                source=working_core.source,
                detected_language=working_core.language,
                allowed_languages=self.language_scope.allowed_languages(working_core.source),
            )
            self._quarantine(obj_key, raw_content, "language_scope_violation", span)
            return

        # --- 7. Upload to Silver Layer (envelope + ClickHouse projection) ---
        # Archive layer first — every successfully harmonised article is
        # preserved in MinIO Silver regardless of its analytical-window
        # status (Manifesto's "unaltered mirror": we record what we
        # observed; we don't silently drop coverage).
        _silver_module.upload_silver(self.minio, obj_key, working_core, meta, self._extraction_provenance)

        # --- 6c. Analytical-window gate (Phase 122e A26 / F-A26). ---
        # Articles whose extracted `published_date` is older than the
        # analytical window are preserved in Silver MinIO (above) but
        # excluded from ClickHouse analytics writes. This is the
        # archive-vs-analytics layered scientific-data pattern: the
        # archive is immutable; analytics are bounded.
        #
        # Without this gate, an old republished article (e.g., a
        # tagesschau podcast page whose RSS pubDate is recent but whose
        # actual `<time datetime>` is years old) would be inserted into
        # `aer_silver.documents` and `aer_gold.metrics`, then evicted on
        # the next merge by `TTL timestamp + toIntervalDay(365)`. The
        # `aer_gold.metrics_hourly` MV would retain the aggregate until
        # its own bucket-anchored TTL fires, producing transient
        # raw-vs-MV drift (F-A23, F-A26 in the iter-3/iter-5 forensics).
        # Skipping the analytics inserts up-front avoids both the wasted
        # extractor compute (NER, sentiment) and the transient MV drift.
        if not _is_within_analytical_window(working_core.timestamp, self.analytical_window_days):
            logger.info(
                "Article past analytical window — archived to Silver MinIO; skipping CH analytics inserts.",
                object=obj_key,
                source=working_core.source,
                article_id=article_id,
                article_timestamp=str(working_core.timestamp),
                analytical_window_days=self.analytical_window_days,
            )
            analysis_worker_archived_only_total.labels(source=working_core.source).inc()
            update_document_article_id(self.pg, obj_key, article_id)
            self._update_document_status(obj_key, "processed")
            events_processed_total.inc()
            span.set_attribute("aer.archived_only", True)
            span.set_attribute("aer.source_type", working_core.source_type)
            span.set_attribute("aer.status", "archived_only")
            return

        # --- 8. Extract and load to Gold Layer (ClickHouse) via Extractor Pipeline ---
        all_metrics: list[GoldMetric] = list(detection_result.metrics)
        all_entities: list[GoldEntity] = list(detection_result.entities)
        all_entity_links: list[GoldEntityLink] = list(detection_result.entity_links)
        all_language_detections: list[GoldLanguageDetection] = list(detection_result.language_detections)

        for extractor in self.extractors:
            if extractor.name == "language_detection":
                continue  # already ran in step 6
            try:
                result = extractor.extract_all(working_core, article_id)
                all_metrics.extend(result.metrics)
                all_entities.extend(result.entities)
                all_entity_links.extend(result.entity_links)
            except Exception as e:
                logger.error(
                    "Extractor failed. Skipping this extractor; other extractors continue.",
                    extractor=extractor.name,
                    object=obj_key,
                    error=str(e),
                )

        discourse_fn = _derive_discourse_function(meta)

        # Phase 122e A18: surface the provenance of `core.timestamp` to Gold.
        # Non-web meta has no `timestamp_source` — degrade to "".
        timestamp_source = getattr(meta, "timestamp_source", "") or ""

        # Phase 74: monotone ingestion_version derived from the deterministic
        # MinIO event time lets ReplacingMergeTree collapse duplicate rows from
        # NATS redelivery. Redelivered events share the same event_time → same
        # version → last-write-wins is a no-op on identical payloads.
        ingestion_version = int(event_time.timestamp() * 1_000_000_000)

        # Phase 103b: write the Silver projection row to ClickHouse so the
        # aggregation endpoints can run as cheap GROUP BYs over
        # `aer_silver.documents` instead of scanning MinIO per request.
        # Uses `working_core` (with consensus language patched in by step 6)
        # so the projection's `language` column matches Gold rather than
        # carrying the adapter's "und" placeholder.
        _silver_projection_module.upload_silver_projection(
            self.ch, working_core, ingestion_version, obj_key, timestamp_source
        )

        # Phase 122f: append per-field provenance into the metadata-coverage
        # raw table. WP-003 §3.2 metadata-richness asymmetry is now a
        # runtime signal feeding the BFF coverage endpoints and the
        # dashboard's field-level Negative-Space rendering (Brief §7.7).
        # No-op for non-web meta — only WebMeta carries `extraction_methods`.
        _metadata_coverage_module.upload_metadata_coverage(
            self.ch, working_core, meta, ingestion_version, event_time
        )

        # Phase 91: wrap Gold inserts so a partial ClickHouse failure does not
        # NAK the message, causing a full reprocessing cycle.  Successfully
        # inserted rows are correct (ReplacingMergeTree deduplicates on
        # redeliver) and the extractor pipeline already degrades gracefully,
        # so marking "processed" on partial success is the consistent choice.
        gold_insert_failed = False

        if all_metrics:
            try:
                rows = [[m.timestamp, m.value, m.source, m.metric_name, m.article_id, discourse_fn, ingestion_version, timestamp_source] for m in all_metrics]
                self.ch.insert(
                    'aer_gold.metrics',
                    rows,
                    column_names=['timestamp', 'value', 'source', 'metric_name', 'article_id', 'discourse_function', 'ingestion_version', 'timestamp_source'],
                    deduplication_token=f"aer_gold.metrics:{article_id}:{ingestion_version}",
                )
                logger.info(
                    "Gold layer updated",
                    metrics_count=len(all_metrics),
                    extractors=[m.metric_name for m in all_metrics],
                    timestamp=str(core.timestamp),
                    source=core.source,
                    article_id=article_id,
                )
            except Exception as e:
                gold_insert_failed = True
                logger.error(
                    "Gold metrics insert failed. Continuing with remaining inserts.",
                    object=obj_key,
                    error=str(e),
                )

        if all_entities:
            try:
                entity_rows = [
                    [e.timestamp, e.source, e.article_id, e.entity_text, e.entity_label, e.start_char, e.end_char, discourse_fn, ingestion_version]
                    for e in all_entities
                ]
                self.ch.insert(
                    'aer_gold.entities',
                    entity_rows,
                    column_names=['timestamp', 'source', 'article_id', 'entity_text', 'entity_label', 'start_char', 'end_char', 'discourse_function', 'ingestion_version'],
                    deduplication_token=f"aer_gold.entities:{article_id}:{ingestion_version}",
                )
                logger.info(
                    "Gold entities updated",
                    entity_count=len(all_entities),
                    timestamp=str(core.timestamp),
                    source=core.source,
                    article_id=article_id,
                )
            except Exception as e:
                gold_insert_failed = True
                logger.error(
                    "Gold entities insert failed. Continuing with remaining inserts.",
                    object=obj_key,
                    error=str(e),
                )

        if all_entity_links:
            try:
                link_rows = [
                    [
                        link.timestamp,
                        link.article_id,
                        link.entity_text,
                        link.entity_label,
                        link.wikidata_qid,
                        link.link_confidence,
                        link.link_method,
                        ingestion_version,
                    ]
                    for link in all_entity_links
                ]
                self.ch.insert(
                    'aer_gold.entity_links',
                    link_rows,
                    column_names=['timestamp', 'article_id', 'entity_text', 'entity_label', 'wikidata_qid', 'link_confidence', 'link_method', 'ingestion_version'],
                    deduplication_token=f"aer_gold.entity_links:{article_id}:{ingestion_version}",
                )
                logger.info(
                    "Gold entity_links updated",
                    link_count=len(all_entity_links),
                    timestamp=str(core.timestamp),
                    source=core.source,
                    article_id=article_id,
                )
            except Exception as e:
                gold_insert_failed = True
                logger.error(
                    "Gold entity_links insert failed. Continuing with remaining inserts.",
                    object=obj_key,
                    error=str(e),
                )

        if all_language_detections:
            try:
                lang_rows = [
                    [
                        d.timestamp,
                        d.source,
                        d.article_id,
                        d.detected_language,
                        d.confidence,
                        d.rank,
                        ingestion_version,
                        _derive_language_variety(meta, d.detected_language),
                    ]
                    for d in all_language_detections
                ]
                self.ch.insert(
                    'aer_gold.language_detections',
                    lang_rows,
                    column_names=['timestamp', 'source', 'article_id', 'detected_language', 'confidence', 'rank', 'ingestion_version', 'language_variety'],
                    deduplication_token=f"aer_gold.language_detections:{article_id}:{ingestion_version}",
                )
                logger.info(
                    "Gold language detections updated",
                    detection_count=len(all_language_detections),
                    timestamp=str(core.timestamp),
                    source=core.source,
                    article_id=article_id,
                )
            except Exception as e:
                gold_insert_failed = True
                logger.error(
                    "Gold language_detections insert failed.",
                    object=obj_key,
                    error=str(e),
                )

        # Phase 122d.0 (ADR-032): Silent-Edit Observability. Project the
        # Wayback CDX result plus the publisher-side republication-trigger
        # signal into `aer_gold.article_revisions`. No-op for non-web meta
        # and for articles with no detected revisions — the absence of
        # rows is itself the signal.
        #
        # This write is ordered AFTER the canonical Gold inserts (metrics,
        # entities, entity_links, language_detections) so that a partial
        # Gold-insert failure does NOT leave a revision row visible for an
        # article that has no metrics row to anchor it. The L5 Evidence
        # Reader resolves an article via the k-anonymity-gated
        # /articles/{id} endpoint (which reads `aer_gold.metrics`); a
        # populated /articles/{id}/revisions with a 404'd parent would
        # render as an orphan. Skipping revisions on partial Gold failure
        # is the consistent choice — the operator can re-run the document
        # through the recovery path once Gold is healthy.
        if not gold_insert_failed:
            _article_revisions_module.upload_article_revisions(
                self.ch, working_core, meta, discourse_fn, ingestion_version
            )

        if gold_insert_failed:
            logger.warning(
                "Document marked processed despite partial Gold insert failure. "
                "Successfully inserted rows are correct; failed rows will be "
                "absent until the next reprocessing window. "
                "Article revisions (Phase 122d.0) skipped on this run to avoid "
                "orphan revision rows; will be re-attempted on next reprocessing.",
                object=obj_key,
            )

        # --- 8. Commit Success ---
        # Persist the article_id so the BFF L5 Evidence endpoint can resolve
        # an article_id back to its Bronze/Silver object key (Phase 101).
        update_document_article_id(self.pg, obj_key, article_id)
        self._update_document_status(obj_key, "processed")
        events_processed_total.inc()

        span.set_attribute("aer.word_count", core.word_count)
        span.set_attribute("aer.source_type", core.source_type)
        span.set_attribute("aer.schema_version", core.schema_version)
        span.set_attribute("aer.status", "processed")

    def _get_document_status(self, obj_key: str) -> str | None:
        """Fetches the current processing status from PostgreSQL."""
        return get_document_status(self.pg, obj_key)

    def _update_document_status(self, obj_key: str, status: str) -> None:
        """Updates the document status in PostgreSQL."""
        update_document_status(self.pg, obj_key, status)

    def _run_language_detection(self, core, article_id, obj_key):
        """Run the LanguageDetectionExtractor (if registered) before Silver writes.

        Returns ``(working_core, extraction_result)``. ``working_core``
        carries the rank=1 consensus winner on its ``language`` field when
        detection succeeded, otherwise the unmodified ``core`` so downstream
        consumers degrade to the adapter placeholder rather than crashing.
        ``extraction_result`` is the detector's full ExtractionResult so the
        caller can persist the ``language_confidence`` metric and the
        per-rank language_detection rows alongside the rest of the Gold
        inserts. An empty result is returned when detection is absent or
        crashed.
        """
        detection = next(
            (e for e in self.extractors if e.name == "language_detection"),
            None,
        )
        empty = ExtractionResult(metrics=[], entities=[], entity_links=[], language_detections=[])
        if detection is None:
            return core, empty
        try:
            result = detection.extract_all(core, article_id)
        except Exception as e:
            logger.error(
                "Language detection failed before Silver write. "
                "Continuing with adapter language; other extractors run.",
                object=obj_key,
                error=str(e),
            )
            return core, empty
        primary = next(
            (d for d in result.language_detections if d.rank == 1),
            None,
        )
        if primary is None or not primary.detected_language:
            return core, result
        working = core.model_copy(update={"language": primary.detected_language})
        return working, result

    def _quarantine(self, obj_key: str, raw_content: dict, reason: str, span) -> None:
        """Routes a document to the DLQ with standard bookkeeping."""
        _quarantine_module.quarantine(
            self.minio, obj_key, raw_content, reason, span,
            self._update_document_status,
        )

    def _move_to_quarantine(self, obj_key: str, raw_content: dict) -> None:
        """Moves unprocessable raw data to the quarantine bucket."""
        _quarantine_module.move_to_quarantine(self.minio, obj_key, raw_content)

    def quarantine_poison_message(self, msg_data: bytes, error_type: str, error_text: str) -> None:
        """
        Route a message that exhausted its NATS redelivery budget to the DLQ.

        Best-effort: tries to recover the original Bronze payload via the
        MinIO key parsed from the NATS event envelope. If parsing or fetch
        fails, writes a synthetic poison-pill envelope so the raw NATS
        payload is never silently dropped. Emits both the legacy
        `events_quarantined_total` / `dlq_size` signals and the Phase 83
        `analysis_worker_poison_messages_total` counter.
        """
        obj_key: str
        raw_content: dict
        try:
            event_data = json.loads(msg_data.decode("utf-8"))
            obj_key = unquote(event_data["Records"][0]["s3"]["object"]["key"])
        except Exception:
            obj_key = f"poison/unparseable/{uuid.uuid4().hex}.json"
            event_data = None

        try:
            response = self.minio.get_object(self.bronze_bucket, obj_key)
            try:
                raw_content = json.loads(response.read().decode("utf-8"))
            finally:
                response.close()
                response.release_conn()
        except Exception as fetch_err:
            raw_content = {
                "_poison": True,
                "_error_type": error_type,
                "_error": error_text,
                "_fetch_error": str(fetch_err),
                "_event_envelope": event_data,
            }

        _quarantine_module.move_to_quarantine(self.minio, obj_key, raw_content)
        try:
            self._update_document_status(obj_key, "quarantined")
        except Exception as status_err:
            logger.warning(
                "Failed to update poison document status",
                obj_key=obj_key,
                error=str(status_err),
            )
        events_quarantined_total.inc()
        dlq_size.inc()
        analysis_worker_poison_messages_total.labels(reason=error_type).inc()
        logger.error(
            "Poison message quarantined after max redeliveries",
            obj_key=obj_key,
            error_type=error_type,
            error=error_text,
        )
