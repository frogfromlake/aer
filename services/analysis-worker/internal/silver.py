import io
import structlog
from minio import Minio
from internal.models import SilverEnvelope

logger = structlog.get_logger()

SILVER_BUCKET = "silver"


def upload_silver(
    minio_client: Minio,
    obj_key: str,
    core,
    meta,
    extraction_provenance: dict,
) -> None:
    """
    Constructs a SilverEnvelope from harmonized core and source-specific meta,
    then uploads the JSON payload to the Silver MinIO bucket.
    """
    envelope = SilverEnvelope(core=core, meta=meta, extraction_provenance=extraction_provenance)
    silver_payload = envelope.model_dump_json().encode('utf-8')
    minio_client.put_object(
        SILVER_BUCKET, obj_key,
        io.BytesIO(silver_payload), len(silver_payload),
        content_type="application/json"
    )
    logger.info(
        "Silver layer updated",
        object=obj_key,
        source=core.source,
        word_count=core.word_count,
        schema_version=core.schema_version,
    )
