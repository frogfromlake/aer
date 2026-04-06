from typing import TYPE_CHECKING, Protocol, runtime_checkable
from dataclasses import dataclass, field
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
    Maps to the aer_gold.entities ClickHouse table (Phase 42).
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


@dataclass(slots=True)
class ExtractionResult:
    """
    Unified result from a single-pass document extraction (Phase 52).

    All extractors return this structure from extract_all(). The processor
    accumulates metrics, entities, and language_detections from each extractor
    without requiring isinstance() dispatch — any extractor may populate any
    subset of the three result lists; empty lists are the default.
    """
    metrics: list[GoldMetric] = field(default_factory=list)
    entities: list[GoldEntity] = field(default_factory=list)
    language_detections: list[GoldLanguageDetection] = field(default_factory=list)


@dataclass(frozen=True, slots=True)
class TimeWindow:
    """Defines a time range for corpus-level batch extraction."""
    start: datetime
    end: datetime


@runtime_checkable
class MetricExtractor(Protocol):
    """
    Protocol for per-document extraction (Phase 52 unified interface).

    All extractors implement a single extract_all() method returning an
    ExtractionResult. This eliminates the isinstance() dispatch chains that
    previously routed LanguageDetectionPersistExtractor and EntityExtractor
    separately. Extractors populate whichever result fields they produce;
    the processor unconditionally accumulates all three.

    One extractor can produce multiple metrics per document and any number
    of entities or language detections. Graceful degradation is handled by
    the processor — a failing extractor is skipped, others continue.
    """

    @property
    def name(self) -> str:
        """Human-readable identifier for this extractor (used in logging)."""
        ...

    def extract_all(self, core: "SilverCore", article_id: str | None) -> ExtractionResult:
        """
        Single-pass extraction returning all results for this document.

        Args:
            core: The validated SilverCore record.
            article_id: Document identifier derived from the MinIO object key.

        Returns:
            An ExtractionResult with metrics, entities, and language_detections.
            Unpopulated fields default to empty lists.
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
