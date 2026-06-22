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
    # Phase 148c storage lever — do NOT persist core.raw_text in the Silver
    # envelope. It is the raw HTML, a near-duplicate of the Bronze object (same
    # deterministic key, different bucket) and 80–96 % of every Silver object,
    # yet read by nothing analytical (extractors use cleaned_text; replay /
    # re-extraction reads Bronze). The L5 "Raw" view fetches it from Bronze
    # on-demand via the BFF. raw_text stays on the in-memory SilverCore (the
    # processor's non-empty contract guard + harmonization still hold); only the
    # serialized-to-MinIO form drops it → ~10× smaller Silver objects, ~3× less
    # lifetime storage growth, and Bronze=raw / Silver=refined is restored.
    silver_payload = envelope.model_dump_json(exclude={"core": {"raw_text"}}).encode("utf-8")
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
