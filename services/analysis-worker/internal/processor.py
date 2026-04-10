import json
import structlog
from datetime import datetime
from minio import Minio
from psycopg2.pool import ThreadedConnectionPool
from internal.models import ValidationError
from internal.adapters.registry import AdapterRegistry
from internal.extractors.base import MetricExtractor, ProvenanceExtractor, GoldMetric, GoldEntity, GoldLanguageDetection
from internal.metrics import events_processed_total, event_processing_duration_seconds
from internal.storage.postgres_client import get_document_status, update_document_status
from internal import quarantine as _quarantine_module
from internal import silver as _silver_module

logger = structlog.get_logger()


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
        response = self.minio.get_object("bronze", obj_key)
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

        if all_metrics:
            rows = [[m.timestamp, m.value, m.source, m.metric_name, m.article_id] for m in all_metrics]
            self.ch.insert(
                'aer_gold.metrics',
                rows,
                column_names=['timestamp', 'value', 'source', 'metric_name', 'article_id']
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
                [e.timestamp, e.source, e.article_id, e.entity_text, e.entity_label, e.start_char, e.end_char]
                for e in all_entities
            ]
            self.ch.insert(
                'aer_gold.entities',
                entity_rows,
                column_names=['timestamp', 'source', 'article_id', 'entity_text', 'entity_label', 'start_char', 'end_char']
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
                [d.timestamp, d.source, d.article_id, d.detected_language, d.confidence, d.rank]
                for d in all_language_detections
            ]
            self.ch.insert(
                'aer_gold.language_detections',
                lang_rows,
                column_names=['timestamp', 'source', 'article_id', 'detected_language', 'confidence', 'rank']
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
