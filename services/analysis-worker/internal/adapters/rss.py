import structlog
from datetime import datetime
from typing import Optional
from pydantic import Field
from psycopg2.pool import ThreadedConnectionPool
from internal.models import SilverCore, SilverMeta, generate_document_id
from internal.models.bias import BiasContext
from internal.models.discourse import DiscourseContext
from internal.storage.postgres_client import get_source_classification

logger = structlog.get_logger()


class RssMeta(SilverMeta):
    """Source-specific metadata for RSS feed items."""
    feed_url: str = Field(default="")
    categories: list[str] = Field(default_factory=list)
    author: str = Field(default="")
    feed_title: str = Field(default="")
    discourse_context: Optional[DiscourseContext] = None
    bias_context: Optional[BiasContext] = None


class RssAdapter:
    """
    Source adapter for RSS feed data (source_type="rss").

    Maps RSS-specific raw Bronze fields to SilverCore + RssMeta.
    Registered in the AdapterRegistry under source_type="rss".

    When a PostgreSQL connection pool is provided, the adapter reads
    the source_classifications table to populate DiscourseContext in
    RssMeta. If no pool or no classification exists, discourse_context
    is None and the pipeline continues without failure.
    """

    def __init__(self, pg_pool: ThreadedConnectionPool | None = None):
        self._pg_pool = pg_pool

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

        discourse_context = None
        if self._pg_pool is not None:
            try:
                classification = get_source_classification(self._pg_pool, source)
                if classification:
                    discourse_context = DiscourseContext(
                        primary_function=classification["primary_function"],
                        secondary_function=classification["secondary_function"],
                        emic_designation=classification["emic_designation"],
                    )
            except Exception as e:
                logger.warning(
                    "Failed to fetch source classification. Continuing without discourse context.",
                    source=source,
                    error=str(e),
                )

        meta = RssMeta(
            source_type="rss",
            feed_url=raw.get("feed_url", ""),
            categories=raw.get("categories", []),
            author=raw.get("author", ""),
            feed_title=raw.get("feed_title", ""),
            discourse_context=discourse_context,
            bias_context=BiasContext(
                platform_type="rss",
                access_method="public_rss",
                visibility_mechanism="chronological",
                moderation_context="editorial",
                engagement_data_available=False,
                account_metadata_available=False,
            ),
        )

        return core, meta
