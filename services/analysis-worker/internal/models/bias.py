from pydantic import BaseModel


class BiasContext(BaseModel):
    """
    Platform bias metadata following the WP-003 "document, don't filter" approach.

    Every document carries a BiasContext describing the structural biases of its
    source platform. This enables downstream consumers to interpret metrics with
    awareness of platform-specific selection effects, visibility mechanisms, and
    data availability constraints.

    Fields are platform-level constants (not per-document), populated by source
    adapters at harmonization time.
    """
    platform_type: str
    access_method: str
    visibility_mechanism: str
    moderation_context: str
    engagement_data_available: bool
    account_metadata_available: bool
