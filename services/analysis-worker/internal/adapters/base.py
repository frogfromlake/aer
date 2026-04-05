from typing import Protocol, runtime_checkable
from datetime import datetime
from internal.models import SilverCore, SilverMeta


@runtime_checkable
class SourceAdapter(Protocol):
    """
    Protocol for source-specific data harmonization.

    Each data source (RSS, forum, social, legacy) implements this protocol
    to map its raw Bronze data to the universal SilverCore contract plus
    an optional source-specific SilverMeta envelope.
    """

    def harmonize(self, raw: dict, event_time: datetime, bronze_object_key: str) -> tuple[SilverCore, SilverMeta | None]:
        """
        Transform raw Bronze data into a harmonized SilverCore record.

        Args:
            raw: The raw JSON document from the Bronze bucket.
            event_time: Deterministic timestamp from the MinIO event metadata.
            bronze_object_key: The MinIO object key, used for document_id generation.

        Returns:
            A tuple of (SilverCore, optional SilverMeta).
        """
        ...
