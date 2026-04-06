from typing import TYPE_CHECKING, Protocol, runtime_checkable
from dataclasses import dataclass
from datetime import datetime


@dataclass(frozen=True, slots=True)
class GoldMetric:
    """
    A single metric extracted from a SilverCore document.
    Maps 1:1 to the aer_gold.metrics ClickHouse schema.
    """
    timestamp: datetime
    value: float
    source: str
    metric_name: str
    article_id: str | None


@dataclass(frozen=True, slots=True)
class GoldEntity:
    """
    A structured entity extracted from a SilverCore document.
    Maps to the future aer_gold.entities ClickHouse table (Phase 42).
    """
    timestamp: datetime
    source: str
    article_id: str | None
    entity_text: str
    entity_label: str
    start_char: int
    end_char: int


@dataclass(frozen=True, slots=True)
class GoldLanguageDetection:
    """
    A detected language for a SilverCore document.
    Maps to the aer_gold.language_detections ClickHouse table (Phase 45).
    """
    timestamp: datetime
    source: str
    article_id: str | None
    detected_language: str
    confidence: float
    rank: int


@dataclass(frozen=True, slots=True)
class TimeWindow:
    """Defines a time range for corpus-level batch extraction."""
    start: datetime
    end: datetime


@runtime_checkable
class MetricExtractor(Protocol):
    """
    Protocol for per-document metric extraction.

    Each implementation extracts one or more Gold metrics from a single
    SilverCore record. Extractors are registered in the DataProcessor
    pipeline and executed sequentially after Silver validation.

    One extractor can produce multiple metrics per document (e.g.,
    sentiment produces sentiment_score + sentiment_subjectivity).
    """

    @property
    def name(self) -> str:
        """Human-readable identifier for this extractor (used in logging)."""
        ...

    def extract(self, core: "SilverCore", article_id: str | None) -> list[GoldMetric]:
        """
        Extract metrics from a single harmonized document.

        Args:
            core: The validated SilverCore record.
            article_id: Document identifier derived from the MinIO object key.

        Returns:
            A list of GoldMetric instances. May be empty if the extractor
            determines no meaningful metric can be derived.
        """
        ...


@runtime_checkable
class EntityExtractor(MetricExtractor, Protocol):
    """
    Protocol for extractors that produce both GoldMetric and GoldEntity results.

    Extends MetricExtractor with an additional extract_entities() method.
    The processor checks isinstance(extractor, EntityExtractor) to determine
    whether to call extract_entities() — no hasattr() required.

    Implementations should process the document once and return both metrics
    and entities from a single pass (see extract_all()).

    Extractors must be stateless between documents — no mutable instance-level
    caching of intermediate results (e.g., spaCy docs).
    """

    def extract_entities(self, core: "SilverCore", article_id: str | None) -> list[GoldEntity]:
        """
        Extract structured entities from a single harmonized document.

        Args:
            core: The validated SilverCore record.
            article_id: Document identifier derived from the MinIO object key.

        Returns:
            A list of GoldEntity instances. May be empty if the extractor
            determines no meaningful entities can be derived.
        """
        ...

    def extract_all(self, core: "SilverCore", article_id: str | None) -> tuple[list[GoldMetric], list[GoldEntity]]:
        """
        Single-pass extraction returning both metrics and entities.

        This is the preferred entry point for EntityExtractor instances.
        The processor calls this instead of calling extract() and
        extract_entities() separately, avoiding redundant document processing.

        Args:
            core: The validated SilverCore record.
            article_id: Document identifier derived from the MinIO object key.

        Returns:
            A tuple of (metrics, entities).
        """
        ...


@runtime_checkable
class LanguageDetectionPersistExtractor(MetricExtractor, Protocol):
    """
    Protocol for extractors that produce both GoldMetric and GoldLanguageDetection results.

    Extends MetricExtractor with extract_language_detections() and extract_all().
    The processor checks isinstance(extractor, LanguageDetectionPersistExtractor)
    to determine whether to call extract_all() — following the EntityExtractor pattern.

    Implementations should process the document once and return both metrics
    and language detections from a single pass (see extract_all()).
    """

    def extract_language_detections(self, core: "SilverCore", article_id: str | None) -> list[GoldLanguageDetection]:
        """
        Extract structured language detection records from a single document.

        Returns:
            A list of GoldLanguageDetection instances, ranked by confidence.
        """
        ...

    def extract_all(self, core: "SilverCore", article_id: str | None) -> tuple[list[GoldMetric], list[GoldLanguageDetection]]:
        """
        Single-pass extraction returning both metrics and language detections.

        Returns:
            A tuple of (metrics, language_detections).
        """
        ...


@runtime_checkable
class ProvenanceExtractor(MetricExtractor, Protocol):
    """
    Protocol for extractors that expose a version hash for provenance tracking.

    Extractors implementing this protocol contribute a ``(name, version_hash)``
    entry to the Silver envelope's ``extraction_provenance`` field. This keeps
    provenance in the metadata layer (Silver) rather than polluting the
    time-series Gold layer with non-analytical values.

    Currently implemented by ``SentimentExtractor`` (SentiWS lexicon hash).
    """

    @property
    def version_hash(self) -> str:
        """Deterministic version identifier for the extractor's resource (e.g. lexicon hash)."""
        ...


@runtime_checkable
class CorpusExtractor(Protocol):
    """
    Protocol for corpus-level batch extraction (interface only).

    Methods like TF-IDF, topic modeling (LDA), and co-occurrence networks
    require statistics across the entire corpus or time windows. These cannot
    run per-document -- they need batch processing on accumulated Silver data.

    This protocol exists to ensure the per-document extractor pipeline does
    not architecturally preclude corpus-level analysis. No implementations
    exist in this phase.

    Future scheduling mechanism: cron or NATS-triggered batch jobs (see
    Chapter 11, Risks).
    """

    @property
    def name(self) -> str:
        """Human-readable identifier for this corpus extractor."""
        ...

    def extract_batch(self, cores: list["SilverCore"], window: TimeWindow) -> list[GoldMetric]:
        """
        Extract metrics from a batch of documents within a time window.

        Args:
            cores: A list of SilverCore records within the window.
            window: The time range defining the batch.

        Returns:
            A list of GoldMetric instances derived from corpus-level analysis.
        """
        ...

if TYPE_CHECKING:
    from internal.models import SilverCore
