import json
import io
import structlog
from datetime import datetime
from minio import Minio
from minio.error import S3Error
from internal.models import SilverRecord, ValidationError

logger = structlog.get_logger()

class DataProcessor:
    """
    Handles the transformation of raw Bronze data into harmonized Silver data,
    calculates Gold metrics, and manages the Dead Letter Queue (DLQ).
    """
    def __init__(self, minio_client: Minio, ch_client):
        self.minio = minio_client
        self.ch = ch_client

    def process_event(self, obj_key: str, span):
        # --- 1. IDEMPOTENCY CHECK ---
        if self._is_already_processed(obj_key):
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
            span.set_attribute("aer.status", "quarantined")
            return

        # --- 5. Upload to Silver Layer ---
        silver_payload = record.model_dump_json().encode('utf-8')
        self.minio.put_object(
            "silver", obj_key, 
            io.BytesIO(silver_payload), len(silver_payload),
            content_type="application/json"
        )
        logger.info("Silver layer updated", object=obj_key)

        # --- 6. Extract and load to Gold Layer (ClickHouse) ---
        self.ch.insert(
            'aer_gold.metrics', 
            [[datetime.now(), record.metric_value]], 
            column_names=['timestamp', 'value']
        )
        logger.info("Gold layer updated", metric=record.metric_value)
        
        span.set_attribute("aer.metric_value", record.metric_value)
        span.set_attribute("aer.status", "processed")

    def _is_already_processed(self, obj_key: str) -> bool:
        """
        Checks if the object has already been harmonized (exists in Silver)
        or quarantined (exists in DLQ) to guarantee idempotency.
        """
        for bucket in ["silver", "bronze-quarantine"]:
            try:
                self.minio.stat_object(bucket, obj_key)
                return True # Object found! It was already processed.
            except S3Error as e:
                if e.code == "NoSuchKey":
                    continue # Not in this bucket, check the next one.
                else:
                    raise # An unexpected storage error occurred.
        return False

    def _move_to_quarantine(self, obj_key: str, raw_content: dict):
        """
        Moves unprocessable raw data to the quarantine bucket.
        """
        payload = json.dumps(raw_content).encode('utf-8')
        self.minio.put_object(
            "bronze-quarantine", obj_key,
            io.BytesIO(payload), len(payload),
            content_type="application/json"
        )