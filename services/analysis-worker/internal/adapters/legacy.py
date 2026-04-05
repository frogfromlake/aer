from datetime import datetime
from internal.models import SilverCore, SilverMeta, generate_document_id


class LegacyAdapter:
    """
    Backward-compatible adapter for existing Wikipedia-era Bronze objects.

    These objects predate the source_type field and follow the original
    SilverRecord format. This adapter maps them to SilverCore with
    source_type="legacy" and schema_version=1.

    This adapter should be maintained until all pre-Phase 39 Bronze objects
    have expired from the 90-day ILM window.
    """

    def harmonize(self, raw: dict, event_time: datetime, bronze_object_key: str) -> tuple[SilverCore, SilverMeta | None]:
        source = raw.get("source", "")
        raw_text = raw.get("raw_text", "")
        cleaned_text = " ".join(raw_text.split())
        word_count = len(cleaned_text.split()) if cleaned_text else 0

        core = SilverCore(
            document_id=generate_document_id(source, bronze_object_key),
            source=source,
            source_type="legacy",
            raw_text=raw_text,
            cleaned_text=cleaned_text,
            language="und",
            timestamp=event_time,
            url=raw.get("url", ""),
            schema_version=1,
            word_count=word_count,
        )

        return core, None
