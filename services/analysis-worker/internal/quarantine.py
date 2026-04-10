import json
import io
import structlog
from minio import Minio
from internal.metrics import events_quarantined_total, dlq_size

logger = structlog.get_logger()

QUARANTINE_BUCKET = "bronze-quarantine"


def move_to_quarantine(minio_client: Minio, obj_key: str, raw_content: dict) -> None:
    """Moves unprocessable raw data to the quarantine bucket."""
    payload = json.dumps(raw_content).encode('utf-8')
    minio_client.put_object(
        QUARANTINE_BUCKET, obj_key,
        io.BytesIO(payload), len(payload),
        content_type="application/json"
    )


def quarantine(minio_client: Minio, obj_key: str, raw_content: dict, reason: str, span, update_status_fn) -> None:
    """
    Routes a document to the DLQ with standard bookkeeping.

    `update_status_fn` is a callable matching the signature
    ``(obj_key: str, status: str) -> None`` so that callers can inject
    the status-update implementation (enables mocking in tests).
    """
    move_to_quarantine(minio_client, obj_key, raw_content)
    update_status_fn(obj_key, "quarantined")
    events_quarantined_total.inc()
    dlq_size.inc()
    span.set_attribute("aer.status", "quarantined")
    span.set_attribute("aer.quarantine_reason", reason)
