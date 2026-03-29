import json
import io
import structlog
from datetime import datetime
from minio import Minio
from psycopg2.pool import ThreadedConnectionPool
from internal.models import SilverRecord, ValidationError

logger = structlog.get_logger()

class DataProcessor:
    """
    Handles the transformation of raw Bronze data into harmonized Silver data,
    calculates Gold metrics, and manages the Dead Letter Queue (DLQ).
    """
    def __init__(self, minio_client: Minio, ch_client, pg_pool: ThreadedConnectionPool):
        self.minio = minio_client
        self.ch = ch_client
        self.pg = pg_pool

    def process_event(self, obj_key: str, span):
        # --- 1. IDEMPOTENCY CHECK (Fast PG Lookup) ---
        status = self._get_document_status(obj_key)
        
        # If status is None, the Go Ingestion API hasn't written the metadata yet.
        # If it's processed/quarantined, we skip.
        if status in ("processed", "quarantined"):
            logger.info("Event already processed. Skipping duplicate.", object=obj_key)
            span.set_attribute("aer.status", "skipped_duplicate")
            return

        # --- 2. Fetch raw data (Bronze Layer) ---
        response = self.minio.get_object("bronze", obj_key)
        raw_content = json.loads(response.read().decode('utf-8'))
        
        # --- 3. Harmonization (Map raw data to Silver format) ---
        harmonized_data = {
            "message": raw_content.get("message", "").lower(),
            "status": "harmonized",
            "metric_value": raw_content.get("metric_value", 0.0)
        }
        
        # --- 4. Validation (The Silver Contract) ---
        try:
            record = SilverRecord(**harmonized_data)
            if not record.message:
                raise ValueError("Message field cannot be empty after harmonization.")
        except (ValidationError, ValueError) as e:
            logger.warning("Harmonization failed. Moving to DLQ.", object=obj_key, error=str(e))
            self._move_to_quarantine(obj_key, raw_content)
            self._update_document_status(obj_key, "quarantined")
            span.set_attribute("aer.status", "quarantined")
            return

        # --- 5. Upload to Silver Layer ---
        # This is idempotent! If ClickHouse failed previously, we safely overwrite Silver.
        silver_payload = record.model_dump_json().encode('utf-8')
        self.minio.put_object(
            "silver", obj_key, 
            io.BytesIO(silver_payload), len(silver_payload),
            content_type="application/json"
        )
        logger.info("Silver layer updated", object=obj_key)

        # --- 6. Extract and load to Gold Layer (ClickHouse) ---
        # If this fails, an exception is thrown, the NATS message is NAK'd, 
        # and the PG status remains 'pending'/'uploaded' for retry!
        self.ch.insert(
            'aer_gold.metrics', 
            [[datetime.now(), record.metric_value]], 
            column_names=['timestamp', 'value']
        )
        logger.info("Gold layer updated", metric=record.metric_value)
        
        # --- 7. Commit Success (Solve Partial Failures) ---
        self._update_document_status(obj_key, "processed")
        
        span.set_attribute("aer.metric_value", record.metric_value)
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