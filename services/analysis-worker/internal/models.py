from pydantic import BaseModel, Field, ValidationError
from datetime import datetime

class SilverRecord(BaseModel):
    """
    Unified schema for the AĒR Silver Layer — source-agnostic.
    All raw data from the Bronze Layer must be harmonized into this structure
    regardless of origin (Wikipedia, news feeds, social media, etc.).
    """
    title: str
    raw_text: str
    word_count: int = Field(default=0, ge=0)
    source: str
    status: str = Field(default="harmonized")
    metric_value: float = Field(default=0.0)
    timestamp: datetime

__all__ = ["SilverRecord", "ValidationError"]
