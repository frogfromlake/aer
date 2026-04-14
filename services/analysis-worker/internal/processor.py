import json
import os
import uuid
import structlog
from datetime import datetime
from urllib.parse import unquote
from minio import Minio
from psycopg2.pool import ThreadedConnectionPool
from internal.models import ValidationError
from internal.adapters.registry import AdapterRegistry
from internal.extractors.base import MetricExtractor, ProvenanceExtractor, GoldMetric, GoldEntity, GoldLanguageDetection
from internal.metrics import (
    events_processed_total,
    event_processing_duration_seconds,
    events_quarantined_total,
    dlq_size,
    analysis_worker_poison_messages_total,
)
from internal.storage.postgres_client import get_document_status, update_document_status
from internal import quarantine as _quarantine_module
from internal import silver as _silver_module

logger = structlog.get_logger()


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


class DataProcessor:
    """
    Source-agnostic data processor for the AĒR Medallion Architecture.

    Fetches Bronze data, delegates harmonization to source-specific adapters
    via the AdapterRegistry, validates the SilverCore contract, writes Silver,
    extracts Gold metrics via the extractor pipeline, and manages the Dead
    Letter Queue (DLQ).
    """
    def __init__(self, minio_client: Minio, ch_client, pg_pool: ThreadedConnectionPool, adapter_registry: AdapterRegistry, extractors: list[MetricExtractor] | None = None):
        self.minio = minio_client
        self.ch = ch_client
        self.pg = pg_pool
        self.adapter_registry = adapter_registry
        self.extractors: list[MetricExtractor] = extractors or []
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
            self._process_event_inner(obj_key, event_time_str, span)

    def _process_event_inner(self, obj_key: str, event_time_str: str, span):
        # --- 1. IDEMPOTENCY CHECK (Fast PG Lookup) ---
        status = self._get_document_status(obj_key)

        if status in ("processed", "quarantined"):
            logger.info("Event already processed. Skipping duplicate.", object=obj_key)
            span.set_attribute("aer.status", "skipped_duplicate")
            return

        # Parse the ISO 8601 string from MinIO Event
        event_time = datetime.fromisoformat(event_time_str.replace('Z', '+00:00'))

        # --- 2. Fetch raw data (Bronze Layer) ---
        response = self.minio.get_object(self.bronze_bucket, obj_key)
        raw_content = json.loads(response.read().decode('utf-8'))

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

        # --- 6. Upload to Silver Layer ---
        _silver_module.upload_silver(self.minio, obj_key, core, meta, self._extraction_provenance)

        # --- 7. Extract and load to Gold Layer (ClickHouse) via Extractor Pipeline ---
        article_id = core.document_id

        all_metrics: list[GoldMetric] = []
        all_entities: list[GoldEntity] = []
        all_language_detections: list[GoldLanguageDetection] = []
        for extractor in self.extractors:
            try:
                result = extractor.extract_all(core, article_id)
                all_metrics.extend(result.metrics)
                all_entities.extend(result.entities)
                all_language_detections.extend(result.language_detections)
            except Exception as e:
                logger.error(
                    "Extractor failed. Skipping this extractor; other extractors continue.",
                    extractor=extractor.name,
                    object=obj_key,
                    error=str(e),
                )

        discourse_fn = _derive_discourse_function(meta)

        # Phase 74: monotone ingestion_version derived from the deterministic
        # MinIO event time lets ReplacingMergeTree collapse duplicate rows from
        # NATS redelivery. Redelivered events share the same event_time → same
        # version → last-write-wins is a no-op on identical payloads.
        ingestion_version = int(event_time.timestamp() * 1_000_000_000)

        if all_metrics:
            rows = [[m.timestamp, m.value, m.source, m.metric_name, m.article_id, discourse_fn, ingestion_version] for m in all_metrics]
            self.ch.insert(
                'aer_gold.metrics',
                rows,
                column_names=['timestamp', 'value', 'source', 'metric_name', 'article_id', 'discourse_function', 'ingestion_version']
            )
            logger.info(
                "Gold layer updated",
                metrics_count=len(all_metrics),
                extractors=[m.metric_name for m in all_metrics],
                timestamp=str(core.timestamp),
                source=core.source,
                article_id=article_id,
            )

        if all_entities:
            entity_rows = [
                [e.timestamp, e.source, e.article_id, e.entity_text, e.entity_label, e.start_char, e.end_char, discourse_fn, ingestion_version]
                for e in all_entities
            ]
            self.ch.insert(
                'aer_gold.entities',
                entity_rows,
                column_names=['timestamp', 'source', 'article_id', 'entity_text', 'entity_label', 'start_char', 'end_char', 'discourse_function', 'ingestion_version']
            )
            logger.info(
                "Gold entities updated",
                entity_count=len(all_entities),
                timestamp=str(core.timestamp),
                source=core.source,
                article_id=article_id,
            )

        if all_language_detections:
            lang_rows = [
                [d.timestamp, d.source, d.article_id, d.detected_language, d.confidence, d.rank, ingestion_version]
                for d in all_language_detections
            ]
            self.ch.insert(
                'aer_gold.language_detections',
                lang_rows,
                column_names=['timestamp', 'source', 'article_id', 'detected_language', 'confidence', 'rank', 'ingestion_version']
            )
            logger.info(
                "Gold language detections updated",
                detection_count=len(all_language_detections),
                timestamp=str(core.timestamp),
                source=core.source,
                article_id=article_id,
            )

        # --- 8. Commit Success ---
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
            raw_content = json.loads(response.read().decode("utf-8"))
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
