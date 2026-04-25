import pytest
import json
from unittest.mock import MagicMock
from opentelemetry import trace
from internal.processor import DataProcessor
from internal.adapters import AdapterRegistry, LegacyAdapter, RssAdapter
from internal.extractors import (
    WordCountExtractor, GoldMetric, ExtractionResult,
)

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

DUMMY_EVENT_TIME = "2023-10-25T12:34:56.000Z"

VALID_RAW_TEXT = "Hello world from the source"
EXPECTED_WORD_COUNT = 5  # len("Hello world from the source".split())

VALID_BRONZE_DATA = json.dumps({
    "source": "test-source",
    "title": "Test Article",
    "raw_text": VALID_RAW_TEXT,
    "url": "https://example.com/test-article",
    "timestamp": "2023-10-25T12:34:56Z",
}).encode('utf-8')

VALID_RSS_BRONZE_DATA = json.dumps({
    "source": "tagesschau",
    "source_type": "rss",
    "title": "Bundesregierung beschließt neues Klimaschutzpaket",
    "raw_text": "Die Bundesregierung hat am Mittwoch ein umfassendes Maßnahmenpaket zum Klimaschutz verabschiedet.",
    "url": "https://www.tagesschau.de/inland/klimaschutz-2026",
    "timestamp": "2026-04-05T10:00:00Z",
    "feed_url": "https://www.tagesschau.de/index~rss2.xml",
    "categories": ["Klimaschutz", "Umwelt"],
    "author": "tagesschau.de",
    "feed_title": "tagesschau.de - Die Nachrichten der ARD",
}).encode('utf-8')

# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------

def gold_insert_calls(mock_ch):
    """Return mock_ch.insert call_args_list excluding aer_silver projection writes.

    Phase 103b adds a per-document `aer_silver.documents` insert alongside
    the existing Gold inserts. Tests written before that change asserted
    only on the Gold call(s); this helper preserves those assertions
    without baking the projection into every test.
    """
    return [c for c in mock_ch.insert.call_args_list if not c.args or c.args[0] != "aer_silver.documents"]


@pytest.fixture
def mock_minio():
    """Provides a mocked MinIO client."""
    return MagicMock()


@pytest.fixture
def mock_clickhouse():
    """Provides a mocked ClickHouse client."""
    return MagicMock()


@pytest.fixture
def mock_pg_pool():
    """Provides a mocked PostgreSQL connection pool."""
    return MagicMock()


@pytest.fixture
def adapter_registry():
    """Provides an AdapterRegistry with the legacy and rss adapters registered."""
    return AdapterRegistry({"legacy": LegacyAdapter(), "rss": RssAdapter()})


@pytest.fixture
def extractors():
    """Provides the default extractor pipeline."""
    return [WordCountExtractor()]


@pytest.fixture
def processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors):
    """Provides a DataProcessor instance with mocked infrastructure."""
    return DataProcessor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)


@pytest.fixture
def dummy_span():
    """Provides a dummy OpenTelemetry span for testing."""
    tracer = trace.get_tracer(__name__)
    with tracer.start_as_current_span("test-span") as span:
        yield span


# ---------------------------------------------------------------------------
# Helper classes
# ---------------------------------------------------------------------------

class StubExtractor:
    """A test extractor that produces a fixed metric."""
    def __init__(self, metric_name: str = "stub_metric", value: float = 42.0):
        self._name = metric_name
        self._value = value

    @property
    def name(self) -> str:
        return self._name

    def extract_all(self, core, article_id: str | None) -> ExtractionResult:
        return ExtractionResult(
            metrics=[
                GoldMetric(
                    timestamp=core.timestamp,
                    value=self._value,
                    source=core.source,
                    metric_name=self._name,
                    article_id=article_id,
                )
            ]
        )


class FailingExtractor:
    """A test extractor that always raises an exception."""
    @property
    def name(self) -> str:
        return "failing_extractor"

    def extract_all(self, core, article_id: str | None) -> ExtractionResult:
        raise RuntimeError("Simulated extractor failure")


class MalformedExtractor:
    """A test extractor that omits extract_all() — triggers AttributeError at dispatch time."""
    @property
    def name(self) -> str:
        return "malformed_extractor"
    # No extract_all() method


# ---------------------------------------------------------------------------
# Helper functions
# ---------------------------------------------------------------------------

def _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors):
    """Helper to create a processor with custom extractors."""
    return DataProcessor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)
