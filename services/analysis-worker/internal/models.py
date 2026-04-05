from pydantic import BaseModel, Field, SerializeAsAny, ValidationError
from datetime import datetime
from typing import Optional
import hashlib


class SilverCore(BaseModel):
    """
    Universal minimum contract for the AĒR Silver Layer (v2).

    Every Bronze document, regardless of source, must be harmonized into this
    structure before promotion to Silver. Fields are instrumentally motivated
    (the pipeline needs them), not scientifically motivated.

    See ADR-015 for the architectural decision and schema evolution strategy.
    """
    document_id: str
    source: str
    source_type: str
    raw_text: str
    cleaned_text: str
    language: str = Field(default="und")
    timestamp: datetime
    url: str = Field(default="")
    schema_version: int = Field(default=2)
    word_count: int = Field(default=0, ge=0)


class SilverMeta(BaseModel):
    """
    Base class for source-specific metadata envelopes.

    Each source type defines its own subclass (e.g., RssMeta, ForumMeta).
    The meta envelope is explicitly unstable — source adapters may add, rename,
    or restructure fields without a formal ADR. Only SilverCore changes require an ADR.
    """
    source_type: str


class SilverEnvelope(BaseModel):
    """
    Complete Silver record combining the universal core with optional source-specific metadata.
    This is the structure written to the Silver MinIO bucket.
    """
    core: SilverCore
    meta: Optional[SerializeAsAny[SilverMeta]] = None


def generate_document_id(source: str, bronze_object_key: str) -> str:
    """Deterministic SHA-256 hash of source + bronze_object_key."""
    return hashlib.sha256(f"{source}{bronze_object_key}".encode("utf-8")).hexdigest()


__all__ = ["SilverCore", "SilverMeta", "SilverEnvelope", "generate_document_id", "ValidationError"]
