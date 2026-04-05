import json
import io
import structlog
from datetime import datetime
from minio import Minio
from psycopg2.pool import ThreadedConnectionPool
from internal.models import SilverEnvelope, ValidationError
from internal.adapters.registry import AdapterRegistry
from internal.metrics import (
    events_processed_total,
    events_quarantined_total,
    event_processing_duration_seconds,
    dlq_size,
)

logger = structlog.get_logger()

class DataProcessor:
    """
    Source-agnostic data processor for the AĒR Medallion Architecture.

    Fetches Bronze data, delegates harmonization to source-specific adapters
    via the AdapterRegistry, validates the SilverCore contract, writes Silver,
    extracts Gold metrics, and manages the Dead Letter Queue (DLQ).
    """
    def __init__(self, minio_client: Minio, ch_client, pg_pool: ThreadedConnectionPool, adapter_registry: AdapterRegistry):
        self.minio = minio_client
        self.ch = ch_client
        self.pg = pg_pool
        self.adapter_registry = adapter_registry

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
            self._move_to_quarantine(obj_key, raw_content)
            self._update_document_status(obj_key, "quarantined")
            events_quarantined_total.inc()
            dlq_size.inc()
            span.set_attribute("aer.status", "quarantined")
            span.set_attribute("aer.quarantine_reason", f"unknown_source_type:{source_type}")
            return

        # --- 4. Harmonization (Bronze → Silver, via adapter) ---
        try:
            core, meta = adapter.harmonize(raw_content, event_time, obj_key)
        except (ValidationError, ValueError) as e:
            logger.warning("Harmonization failed. Moving to DLQ.", object=obj_key, error=str(e))
            self._move_to_quarantine(obj_key, raw_content)
            self._update_document_status(obj_key, "quarantined")
            events_quarantined_total.inc()
            dlq_size.inc()
            span.set_attribute("aer.status", "quarantined")
            return

        # --- 5. Validation (The Silver Contract) ---
        try:
            if not core.cleaned_text:
                raise ValueError("cleaned_text field cannot be empty after harmonization.")
            if not core.raw_text:
                raise ValueError("raw_text field cannot be empty after harmonization.")
        except ValueError as e:
            logger.warning("Silver contract validation failed. Moving to DLQ.", object=obj_key, error=str(e))
            self._move_to_quarantine(obj_key, raw_content)
            self._update_document_status(obj_key, "quarantined")
            events_quarantined_total.inc()
            dlq_size.inc()
            span.set_attribute("aer.status", "quarantined")
            return

        # --- 6. Upload to Silver Layer ---
        envelope = SilverEnvelope(core=core, meta=meta)
        silver_payload = envelope.model_dump_json().encode('utf-8')
        self.minio.put_object(
            "silver", obj_key,
            io.BytesIO(silver_payload), len(silver_payload),
            content_type="application/json"
        )
        logger.info("Silver layer updated", object=obj_key, source=core.source, word_count=core.word_count, schema_version=core.schema_version)

        # --- 7. Extract and load to Gold Layer (ClickHouse) ---
        key_parts = obj_key.split("/")
        article_id = key_parts[1] if len(key_parts) >= 3 else None

        self.ch.insert(
            'aer_gold.metrics',
            [[core.timestamp, float(core.word_count), core.source, "word_count", article_id]],
            column_names=['timestamp', 'value', 'source', 'metric_name', 'article_id']
        )
        logger.info("Gold layer updated", metric=core.word_count, timestamp=str(core.timestamp), source=core.source, article_id=article_id)

        # --- 8. Commit Success ---
        self._update_document_status(obj_key, "processed")
        events_processed_total.inc()

        span.set_attribute("aer.word_count", core.word_count)
        span.set_attribute("aer.source_type", core.source_type)
        span.set_attribute("aer.schema_version", core.schema_version)
        span.set_attribute("aer.status", "processed")

    def _get_document_status(self, obj_key: str) -> str | None:
        """Fetches the current processing status from PostgreSQL."""
        conn = self.pg.getconn()
        try:
            with conn.cursor() as cur:
                cur.execute("SELECT status FROM documents WHERE bronze_object_key = %s", (obj_key,))
                res = cur.fetchone()
                return res[0] if res else None
        finally:
            self.pg.putconn(conn)

    def _update_document_status(self, obj_key: str, status: str):
        """Updates the document status in PostgreSQL."""
        conn = self.pg.getconn()
        try:
            with conn.cursor() as cur:
                cur.execute("UPDATE documents SET status = %s WHERE bronze_object_key = %s", (status, obj_key))
            conn.commit()
        finally:
            self.pg.putconn(conn)

    def _move_to_quarantine(self, obj_key: str, raw_content: dict):
        """Moves unprocessable raw data to the quarantine bucket."""
        payload = json.dumps(raw_content).encode('utf-8')
        self.minio.put_object(
            "bronze-quarantine", obj_key,
            io.BytesIO(payload), len(payload),
            content_type="application/json"
        )
