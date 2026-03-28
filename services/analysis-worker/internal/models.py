from pydantic import BaseModel, Field, ValidationError

class SilverRecord(BaseModel):
    """
    Unified schema for the AĒR Silver Layer.
    All raw data from the Bronze Layer must be harmonized into this structure.
    """
    message: str
    status: str = Field(default="harmonized")
    metric_value: float = Field(default=0.0)

__all__ = ["SilverRecord", "ValidationError"]