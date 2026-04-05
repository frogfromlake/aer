from datetime import datetime
from pydantic import Field
from internal.models import SilverCore, SilverMeta, generate_document_id


class RssMeta(SilverMeta):
    """Source-specific metadata for RSS feed items."""
    feed_url: str = Field(default="")
    categories: list[str] = Field(default_factory=list)
    author: str = Field(default="")
    feed_title: str = Field(default="")


class RssAdapter:
    """
    Source adapter for RSS feed data (source_type="rss").

    Maps RSS-specific raw Bronze fields to SilverCore + RssMeta.
    Registered in the AdapterRegistry under source_type="rss".
    """

    def harmonize(self, raw: dict, event_time: datetime, bronze_object_key: str) -> tuple[SilverCore, RssMeta]:
        source = raw.get("source", "")
        raw_text = raw.get("raw_text", "")
        cleaned_text = " ".join(raw_text.split())
        word_count = len(cleaned_text.split()) if cleaned_text else 0

        core = SilverCore(
            document_id=generate_document_id(source, bronze_object_key),
            source=source,
            source_type="rss",
            raw_text=raw_text,
            cleaned_text=cleaned_text,
            language="de",  # Probe 0: German institutional feeds
            timestamp=event_time,
            url=raw.get("url", raw.get("link", "")),
            schema_version=2,
            word_count=word_count,
        )

        meta = RssMeta(
            source_type="rss",
            feed_url=raw.get("feed_url", ""),
            categories=raw.get("categories", []),
            author=raw.get("author", ""),
            feed_title=raw.get("feed_title", ""),
        )

        return core, meta
