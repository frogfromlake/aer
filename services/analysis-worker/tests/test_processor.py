import pytest
import json
from datetime import datetime
from unittest.mock import MagicMock
from opentelemetry import trace
from internal.processor import DataProcessor
from internal.adapters import AdapterRegistry, LegacyAdapter, RssAdapter
from internal.models import generate_document_id
from internal.extractors import (
    WordCountExtractor, GoldMetric, GoldEntity, MetricExtractor, EntityExtractor,
    TemporalDistributionExtractor, LanguageDetectionExtractor,
    SentimentExtractor, NamedEntityExtractor,
)

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

# Dummy timestamp string mimicking the NATS MinIO event
DUMMY_EVENT_TIME = "2023-10-25T12:34:56.000Z"
EXPECTED_DATETIME = datetime.fromisoformat("2023-10-25T12:34:56+00:00")

# Generic Bronze payload conforming to the AĒR Ingestion Contract
VALID_RAW_TEXT = "Hello world from the source"
EXPECTED_WORD_COUNT = 5  # len("Hello world from the source".split())

VALID_BRONZE_DATA = json.dumps({
    "source": "test-source",
    "title": "Test Article",
    "raw_text": VALID_RAW_TEXT,
    "url": "https://example.com/test-article",
    "timestamp": "2023-10-25T12:34:56Z",
}).encode('utf-8')


def test_silver_contract_happy_path(processor, mock_minio, mock_clickhouse, dummy_span):
    """
    Tests if valid Bronze data is harmonized via the legacy adapter and correctly
    passed to the Silver Layer (MinIO) and Gold Layer (ClickHouse).
    """
    # 1. Setup
    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response

    processor._get_document_status = MagicMock(return_value=None)
    processor._update_document_status = MagicMock()

    obj_key = "test-source/test-article/2023-10-25.json"

    # 2. Execute
    processor.process_event(obj_key, DUMMY_EVENT_TIME, dummy_span)

    # 3. Assert MinIO (Silver upload)
    mock_minio.put_object.assert_called_once()
    args, kwargs = mock_minio.put_object.call_args
    assert args[0] == "silver"
    assert args[1] == obj_key

    # 4. Assert ClickHouse: deterministic timestamp + word count as metric value + dimensions
    mock_clickhouse.insert.assert_called_once()
    ch_args, ch_kwargs = mock_clickhouse.insert.call_args
    assert ch_args[0] == "aer_gold.metrics"
    row = ch_args[1][0]
    assert row[0] == EXPECTED_DATETIME          # Must NOT be datetime.now()
    assert row[1] == float(EXPECTED_WORD_COUNT)  # word_count as metric
    assert row[2] == "test-source"               # source dimension
    assert row[3] == "word_count"                # metric_name dimension
    assert row[4] == "test-article"              # article_id derived from obj_key
    assert ch_kwargs['column_names'] == ['timestamp', 'value', 'source', 'metric_name', 'article_id']

    # 5. Assert DB Update (Commit Success)
    processor._update_document_status.assert_called_with(obj_key, "processed")


def test_silver_contract_missing_raw_text(processor, mock_minio, mock_clickhouse, dummy_span):
    """
    Tests if a document missing the 'raw_text' field is caught by validation
    and correctly routed to the Dead Letter Queue (DLQ).
    """
    corrupt_bronze_data = json.dumps({
        "source": "test-source",
        "title": "Incomplete Article",
        "url": "https://example.com/incomplete",
        "timestamp": "2023-10-25T12:34:56Z",
    }).encode('utf-8')

    mock_response = MagicMock()
    mock_response.read.return_value = corrupt_bronze_data
    mock_minio.get_object.return_value = mock_response

    processor._get_document_status = MagicMock(return_value=None)
    processor._update_document_status = MagicMock()

    # Execute
    processor.process_event("test-source/incomplete/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    # Assert MinIO (Must go to DLQ)
    mock_minio.put_object.assert_called_once()
    args, kwargs = mock_minio.put_object.call_args
    assert args[0] == "bronze-quarantine"
    assert args[1] == "test-source/incomplete/2023-10-25.json"

    # Assert ClickHouse (Must NOT be called)
    mock_clickhouse.insert.assert_not_called()

    # Assert DB Update (Quarantined)
    processor._update_document_status.assert_called_with(
        "test-source/incomplete/2023-10-25.json", "quarantined"
    )


def test_whitespace_normalization(processor, mock_minio, mock_clickhouse, dummy_span):
    """
    Tests that leading/trailing and internal whitespace in raw_text is normalized.
    The word count must reflect the cleaned text, not the raw whitespace-padded string.
    """
    raw_text = "  Hello   world  from   the   source  "
    bronze_data = json.dumps({
        "source": "test-source",
        "title": "Whitespace Article",
        "raw_text": raw_text,
        "url": "https://example.com/whitespace-article",
        "timestamp": "2023-10-25T12:34:56Z",
    }).encode('utf-8')

    mock_response = MagicMock()
    mock_response.read.return_value = bronze_data
    mock_minio.get_object.return_value = mock_response

    processor._get_document_status = MagicMock(return_value=None)
    processor._update_document_status = MagicMock()

    # Execute
    processor.process_event("test-source/whitespace-article/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    # Assert: word_count must equal 5 (same as the clean version)
    mock_clickhouse.insert.assert_called_once()
    ch_args, ch_kwargs = mock_clickhouse.insert.call_args
    row = ch_args[1][0]
    assert row[1] == float(EXPECTED_WORD_COUNT)
    assert row[2] == "test-source"
    assert row[3] == "word_count"
    assert row[4] == "whitespace-article"


def test_idempotency_skip_duplicate(processor, mock_minio, mock_clickhouse, dummy_span):
    """Tests if an already processed event is skipped entirely."""
    processor._get_document_status = MagicMock(return_value="processed")

    processor.process_event("test-source/duplicate/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    mock_minio.get_object.assert_not_called()
    mock_minio.put_object.assert_not_called()
    mock_clickhouse.insert.assert_not_called()


def test_idempotency_skip_quarantined(processor, mock_minio, mock_clickhouse, dummy_span):
    """Tests that an event already in 'quarantined' state is also skipped."""
    processor._get_document_status = MagicMock(return_value="quarantined")

    processor.process_event("test-source/quarantined/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    mock_minio.get_object.assert_not_called()
    mock_minio.put_object.assert_not_called()
    mock_clickhouse.insert.assert_not_called()


def test_raw_text_only_whitespace_quarantined(processor, mock_minio, mock_clickhouse, dummy_span):
    """
    Tests that a document whose 'raw_text' is only whitespace is routed to the DLQ.
    After normalization the cleaned_text becomes '' — an invalid Silver record.
    """
    bronze_data = json.dumps({
        "source": "test-source",
        "title": "Empty Content Article",
        "raw_text": "   \t  \n  ",
        "url": "https://example.com/empty",
        "timestamp": "2023-10-25T12:34:56Z",
    }).encode('utf-8')

    mock_response = MagicMock()
    mock_response.read.return_value = bronze_data
    mock_minio.get_object.return_value = mock_response

    processor._get_document_status = MagicMock(return_value=None)
    processor._update_document_status = MagicMock()

    processor.process_event("test-source/empty/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    mock_minio.put_object.assert_called_once()
    args, _ = mock_minio.put_object.call_args
    assert args[0] == "bronze-quarantine"
    mock_clickhouse.insert.assert_not_called()
    processor._update_document_status.assert_called_with(
        "test-source/empty/2023-10-25.json", "quarantined"
    )


def test_nested_raw_text_raises_unhandled_exception(processor, mock_minio, mock_clickhouse, dummy_span):
    """
    Tests that a document where 'raw_text' is a nested JSON object (not a string) raises
    an AttributeError that propagates out of process_event uncaught.
    """
    nested_bronze_data = json.dumps({
        "source": "test-source",
        "title": "Nested Article",
        "raw_text": {"nested": "object", "depth": 1},
        "url": "https://example.com/nested",
        "timestamp": "2023-10-25T12:34:56Z",
    }).encode('utf-8')

    mock_response = MagicMock()
    mock_response.read.return_value = nested_bronze_data
    mock_minio.get_object.return_value = mock_response

    processor._get_document_status = MagicMock(return_value=None)
    processor._update_document_status = MagicMock()

    with pytest.raises(AttributeError):
        processor.process_event("test-source/nested/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    mock_minio.put_object.assert_not_called()
    mock_clickhouse.insert.assert_not_called()
    processor._update_document_status.assert_not_called()


def test_silver_upload_failure_propagates(processor, mock_minio, mock_clickhouse, dummy_span):
    """
    Tests that a transient network error during the Silver MinIO upload propagates
    as an unhandled exception, allowing NATS to NAK and retry the message.
    """
    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    mock_minio.put_object.side_effect = Exception("MinIO: connection timeout")

    processor._get_document_status = MagicMock(return_value=None)
    processor._update_document_status = MagicMock()

    with pytest.raises(Exception, match="MinIO: connection timeout"):
        processor.process_event("test-source/upload-fail/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    mock_clickhouse.insert.assert_not_called()
    processor._update_document_status.assert_not_called()


def test_move_to_quarantine_payload_encoding(processor, mock_minio):
    """
    Unit-tests _move_to_quarantine in isolation.
    Verifies: correct target bucket, unchanged key, and exact JSON serialization.
    """
    obj_key = "test-source/broken/2023-10-25.json"
    raw_content = {"source": "test-source", "title": "Broken", "raw_text": None}

    processor._move_to_quarantine(obj_key, raw_content)

    mock_minio.put_object.assert_called_once()
    args, kwargs = mock_minio.put_object.call_args

    assert args[0] == "bronze-quarantine"
    assert args[1] == obj_key
    assert kwargs.get("content_type") == "application/json"

    import io as _io
    uploaded_buffer = args[2]
    assert isinstance(uploaded_buffer, _io.BytesIO)
    uploaded_buffer.seek(0)
    assert json.loads(uploaded_buffer.read().decode('utf-8')) == raw_content


def test_move_to_quarantine_length_matches_payload(processor, mock_minio):
    """
    Verifies that the 'length' argument passed to put_object exactly matches
    the byte length of the serialized payload.
    """
    obj_key = "test-source/length-check/2023-10-25.json"
    raw_content = {"source": "unicode-test", "raw_text": "Ünïcödé chäräctérs 日本語"}

    processor._move_to_quarantine(obj_key, raw_content)

    args, _ = mock_minio.put_object.call_args
    payload_buffer = args[2]
    declared_length = args[3]

    payload_buffer.seek(0)
    actual_length = len(payload_buffer.read())
    assert declared_length == actual_length


# ==================== Phase 39: New Tests ====================


def test_adapter_registry_lookup(adapter_registry):
    """Tests that the registry returns the correct adapter for a known source_type."""
    adapter = adapter_registry.get("legacy")
    assert adapter is not None
    assert isinstance(adapter, LegacyAdapter)


def test_adapter_registry_unknown_type(adapter_registry):
    """Tests that the registry returns None for an unknown source_type."""
    adapter = adapter_registry.get("forum")
    assert adapter is None


def test_adapter_registry_supported_types(adapter_registry):
    """Tests that supported_types returns all registered keys."""
    assert "legacy" in adapter_registry.supported_types()
    assert "rss" in adapter_registry.supported_types()


def test_unknown_source_type_quarantined(processor, mock_minio, mock_clickhouse, dummy_span):
    """
    Tests that a document with an unknown source_type (no registered adapter)
    is routed to the DLQ with a clear error.
    """
    bronze_data = json.dumps({
        "source": "unknown-feed",
        "source_type": "forum",
        "raw_text": "Some forum post content",
        "url": "https://example.com/forum/post",
        "timestamp": "2023-10-25T12:34:56Z",
    }).encode('utf-8')

    mock_response = MagicMock()
    mock_response.read.return_value = bronze_data
    mock_minio.get_object.return_value = mock_response

    processor._get_document_status = MagicMock(return_value=None)
    processor._update_document_status = MagicMock()

    processor.process_event("unknown-feed/post/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    # Must go to DLQ
    mock_minio.put_object.assert_called_once()
    args, _ = mock_minio.put_object.call_args
    assert args[0] == "bronze-quarantine"

    mock_clickhouse.insert.assert_not_called()
    processor._update_document_status.assert_called_with(
        "unknown-feed/post/2023-10-25.json", "quarantined"
    )


def test_legacy_adapter_backward_compatibility(processor, mock_minio, mock_clickhouse, dummy_span):
    """
    Tests that Bronze objects without a source_type field (pre-Phase 39)
    are handled by the legacy adapter with source_type='legacy' and schema_version=1.
    """
    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response

    processor._get_document_status = MagicMock(return_value=None)
    processor._update_document_status = MagicMock()

    obj_key = "test-source/test-article/2023-10-25.json"
    processor.process_event(obj_key, DUMMY_EVENT_TIME, dummy_span)

    # Verify the Silver payload contains the envelope structure with legacy metadata
    mock_minio.put_object.assert_called_once()
    args, _ = mock_minio.put_object.call_args
    assert args[0] == "silver"

    # Parse the uploaded Silver payload
    silver_buffer = args[2]
    silver_buffer.seek(0)
    silver_data = json.loads(silver_buffer.read().decode('utf-8'))

    assert silver_data["core"]["source_type"] == "legacy"
    assert silver_data["core"]["schema_version"] == 1
    assert silver_data["core"]["raw_text"] == VALID_RAW_TEXT
    assert silver_data["core"]["cleaned_text"] == VALID_RAW_TEXT  # already clean
    assert silver_data["core"]["language"] == "und"
    assert silver_data["meta"] is None


def test_schema_version_written_to_silver(processor, mock_minio, mock_clickhouse, dummy_span):
    """Tests that schema_version is present in the Silver object."""
    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response

    processor._get_document_status = MagicMock(return_value=None)
    processor._update_document_status = MagicMock()

    processor.process_event("test-source/test-article/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    args, _ = mock_minio.put_object.call_args
    silver_buffer = args[2]
    silver_buffer.seek(0)
    silver_data = json.loads(silver_buffer.read().decode('utf-8'))

    assert "schema_version" in silver_data["core"]
    assert silver_data["core"]["schema_version"] == 1  # legacy adapter sets v1


def test_document_id_determinism():
    """
    Tests that generate_document_id produces the same hash for the same inputs
    and different hashes for different inputs.
    """
    id1 = generate_document_id("source-a", "path/to/doc.json")
    id2 = generate_document_id("source-a", "path/to/doc.json")
    id3 = generate_document_id("source-b", "path/to/doc.json")

    assert id1 == id2  # same inputs → same hash
    assert id1 != id3  # different source → different hash

    # Verify it's a valid SHA-256 hex string
    assert len(id1) == 64
    int(id1, 16)  # must parse as hex


def test_document_id_in_silver_payload(processor, mock_minio, mock_clickhouse, dummy_span):
    """Tests that document_id is present and deterministic in the Silver payload."""
    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response

    processor._get_document_status = MagicMock(return_value=None)
    processor._update_document_status = MagicMock()

    obj_key = "test-source/test-article/2023-10-25.json"
    processor.process_event(obj_key, DUMMY_EVENT_TIME, dummy_span)

    args, _ = mock_minio.put_object.call_args
    silver_buffer = args[2]
    silver_buffer.seek(0)
    silver_data = json.loads(silver_buffer.read().decode('utf-8'))

    expected_id = generate_document_id("test-source", obj_key)
    assert silver_data["core"]["document_id"] == expected_id


def test_raw_text_preserved_separately_from_cleaned(processor, mock_minio, mock_clickhouse, dummy_span):
    """
    Tests that raw_text preserves the original text while cleaned_text
    contains the normalized version. Provenance must not be destroyed.
    """
    raw_text = "  Hello   world  from   the   source  "
    bronze_data = json.dumps({
        "source": "test-source",
        "title": "Provenance Test",
        "raw_text": raw_text,
        "url": "https://example.com/provenance",
        "timestamp": "2023-10-25T12:34:56Z",
    }).encode('utf-8')

    mock_response = MagicMock()
    mock_response.read.return_value = bronze_data
    mock_minio.get_object.return_value = mock_response

    processor._get_document_status = MagicMock(return_value=None)
    processor._update_document_status = MagicMock()

    processor.process_event("test-source/provenance/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    args, _ = mock_minio.put_object.call_args
    silver_buffer = args[2]
    silver_buffer.seek(0)
    silver_data = json.loads(silver_buffer.read().decode('utf-8'))

    # raw_text preserved as-is from Bronze
    assert silver_data["core"]["raw_text"] == raw_text
    # cleaned_text is whitespace-normalized
    assert silver_data["core"]["cleaned_text"] == "Hello world from the source"


def test_missing_source_type_defaults_to_legacy(processor, mock_minio, mock_clickhouse, dummy_span):
    """
    Tests that Bronze objects without a source_type field default to 'legacy',
    ensuring backward compatibility with pre-Phase 39 data.
    """
    bronze_data = json.dumps({
        "source": "wikipedia",
        "title": "Old Article",
        "raw_text": "Some old content",
        "url": "https://en.wikipedia.org/wiki/Old",
        "timestamp": "2023-10-25T12:34:56Z",
    }).encode('utf-8')

    mock_response = MagicMock()
    mock_response.read.return_value = bronze_data
    mock_minio.get_object.return_value = mock_response

    processor._get_document_status = MagicMock(return_value=None)
    processor._update_document_status = MagicMock()

    processor.process_event("wikipedia/old/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    # Should succeed (not quarantined) — legacy adapter handles it
    mock_minio.put_object.assert_called_once()
    args, _ = mock_minio.put_object.call_args
    assert args[0] == "silver"

    processor._update_document_status.assert_called_with(
        "wikipedia/old/2023-10-25.json", "processed"
    )


# ==================== Phase 40: RSS Adapter Tests ====================


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


def test_rss_adapter_registry_lookup(adapter_registry):
    """Tests that the RSS adapter is registered and returned."""
    adapter = adapter_registry.get("rss")
    assert adapter is not None
    assert isinstance(adapter, RssAdapter)


def test_rss_adapter_happy_path(processor, mock_minio, mock_clickhouse, dummy_span):
    """
    Tests end-to-end processing of an RSS document through the pipeline:
    RSS adapter → SilverCore + RssMeta → Silver upload → Gold insert.
    """
    mock_response = MagicMock()
    mock_response.read.return_value = VALID_RSS_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response

    processor._get_document_status = MagicMock(return_value=None)
    processor._update_document_status = MagicMock()

    obj_key = "rss/tagesschau/abc123/2026-04-05.json"
    processor.process_event(obj_key, "2026-04-05T10:00:00.000Z", dummy_span)

    # Silver upload succeeded
    mock_minio.put_object.assert_called_once()
    args, _ = mock_minio.put_object.call_args
    assert args[0] == "silver"
    assert args[1] == obj_key

    # Parse Silver payload
    silver_buffer = args[2]
    silver_buffer.seek(0)
    silver_data = json.loads(silver_buffer.read().decode('utf-8'))

    # SilverCore assertions
    core = silver_data["core"]
    assert core["source_type"] == "rss"
    assert core["source"] == "tagesschau"
    assert core["schema_version"] == 2
    assert core["language"] == "de"
    assert core["url"] == "https://www.tagesschau.de/inland/klimaschutz-2026"
    assert core["raw_text"] == "Die Bundesregierung hat am Mittwoch ein umfassendes Maßnahmenpaket zum Klimaschutz verabschiedet."
    assert core["cleaned_text"] == core["raw_text"]  # already clean
    assert core["word_count"] == 11
    assert len(core["document_id"]) == 64  # SHA-256 hex

    # RssMeta assertions
    meta = silver_data["meta"]
    assert meta is not None
    assert meta["source_type"] == "rss"
    assert meta["feed_url"] == "https://www.tagesschau.de/index~rss2.xml"
    assert meta["categories"] == ["Klimaschutz", "Umwelt"]
    assert meta["author"] == "tagesschau.de"
    assert meta["feed_title"] == "tagesschau.de - Die Nachrichten der ARD"

    # Gold insert
    mock_clickhouse.insert.assert_called_once()
    ch_args, _ = mock_clickhouse.insert.call_args
    row = ch_args[1][0]
    assert row[1] == 11.0  # word_count
    assert row[2] == "tagesschau"
    assert row[3] == "word_count"

    # Status committed
    processor._update_document_status.assert_called_with(obj_key, "processed")


def test_rss_adapter_whitespace_normalization(processor, mock_minio, mock_clickhouse, dummy_span):
    """Tests that RSS raw_text with irregular whitespace is properly normalized."""
    rss_data = json.dumps({
        "source": "bundesregierung",
        "source_type": "rss",
        "title": "Whitespace Test",
        "raw_text": "  Viel   unnötiger   Leerraum   hier  ",
        "url": "https://example.gov.de/test",
        "timestamp": "2026-04-05T10:00:00Z",
        "feed_url": "https://example.gov.de/feed.rss",
    }).encode('utf-8')

    mock_response = MagicMock()
    mock_response.read.return_value = rss_data
    mock_minio.get_object.return_value = mock_response

    processor._get_document_status = MagicMock(return_value=None)
    processor._update_document_status = MagicMock()

    processor.process_event("rss/bundesregierung/xyz/2026-04-05.json", "2026-04-05T10:00:00.000Z", dummy_span)

    args, _ = mock_minio.put_object.call_args
    silver_buffer = args[2]
    silver_buffer.seek(0)
    silver_data = json.loads(silver_buffer.read().decode('utf-8'))

    # raw_text preserved, cleaned_text normalized
    assert silver_data["core"]["raw_text"] == "  Viel   unnötiger   Leerraum   hier  "
    assert silver_data["core"]["cleaned_text"] == "Viel unnötiger Leerraum hier"
    assert silver_data["core"]["word_count"] == 4


def test_rss_adapter_empty_categories():
    """Tests that RssMeta handles missing categories gracefully."""
    adapter = RssAdapter()
    raw = {
        "source": "test",
        "raw_text": "Some text",
        "url": "https://example.com",
    }
    from datetime import timezone
    event_time = datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc)
    core, meta = adapter.harmonize(raw, event_time, "rss/test/abc/2026-04-05.json")

    assert core.source_type == "rss"
    assert meta.categories == []
    assert meta.author == ""
    assert meta.feed_url == ""


# ==================== Phase 41: Extractor Pipeline Tests ====================


class StubExtractor:
    """A test extractor that produces a fixed metric."""
    def __init__(self, metric_name: str = "stub_metric", value: float = 42.0):
        self._name = metric_name
        self._value = value

    @property
    def name(self) -> str:
        return self._name

    def extract(self, core, article_id: str | None) -> list[GoldMetric]:
        return [
            GoldMetric(
                timestamp=core.timestamp,
                value=self._value,
                source=core.source,
                metric_name=self._name,
                article_id=article_id,
            )
        ]


class FailingExtractor:
    """A test extractor that always raises an exception."""
    @property
    def name(self) -> str:
        return "failing_extractor"

    def extract(self, core, article_id: str | None) -> list[GoldMetric]:
        raise RuntimeError("Simulated extractor failure")


def _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors):
    """Helper to create a processor with custom extractors."""
    return DataProcessor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)


def test_extractor_pipeline_multiple_extractors(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span):
    """
    Tests that multiple extractors each contribute metrics and all are
    batch-inserted into ClickHouse in a single round-trip.
    """
    extractors = [WordCountExtractor(), StubExtractor("sentiment_score", 0.85)]
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("test-source/test-article/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    # Single batch insert with 2 rows
    mock_clickhouse.insert.assert_called_once()
    ch_args, ch_kwargs = mock_clickhouse.insert.call_args
    rows = ch_args[1]
    assert len(rows) == 2
    assert rows[0][3] == "word_count"
    assert rows[0][1] == float(EXPECTED_WORD_COUNT)
    assert rows[1][3] == "sentiment_score"
    assert rows[1][1] == 0.85


def test_extractor_pipeline_failing_extractor_does_not_block_others(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span):
    """
    Tests that a failing extractor is skipped while other extractors still
    produce their metrics. The document is NOT sent to the DLQ.
    """
    extractors = [FailingExtractor(), WordCountExtractor()]
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("test-source/test-article/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    # word_count still inserted despite failing_extractor
    mock_clickhouse.insert.assert_called_once()
    ch_args, _ = mock_clickhouse.insert.call_args
    rows = ch_args[1]
    assert len(rows) == 1
    assert rows[0][3] == "word_count"

    # Document is marked processed, not quarantined
    proc._update_document_status.assert_called_with(
        "test-source/test-article/2023-10-25.json", "processed"
    )


def test_extractor_pipeline_no_extractors(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span):
    """
    Tests that the processor works with an empty extractor list.
    No ClickHouse insert occurs but the document is still processed.
    """
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, [])

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("test-source/test-article/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    # No metrics → no ClickHouse insert
    mock_clickhouse.insert.assert_not_called()

    # Still processed successfully (Silver was uploaded)
    proc._update_document_status.assert_called_with(
        "test-source/test-article/2023-10-25.json", "processed"
    )


def test_extractor_pipeline_all_extractors_fail(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span):
    """
    Tests that if all extractors fail, no ClickHouse insert occurs but the
    document is still marked as processed (partial extraction is acceptable).
    """
    extractors = [FailingExtractor()]
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("test-source/test-article/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    mock_clickhouse.insert.assert_not_called()
    proc._update_document_status.assert_called_with(
        "test-source/test-article/2023-10-25.json", "processed"
    )


def test_extractor_registration_add_remove():
    """
    Tests that adding or removing an extractor from the list does not affect
    other extractors. Extractors are independent.
    """
    e1 = WordCountExtractor()
    e2 = StubExtractor("test_metric")

    pipeline = [e1, e2]
    assert len(pipeline) == 2
    assert pipeline[0].name == "word_count"
    assert pipeline[1].name == "test_metric"

    # Remove one — the other is unaffected
    pipeline.remove(e2)
    assert len(pipeline) == 1
    assert pipeline[0].name == "word_count"


def test_word_count_extractor_protocol_conformance():
    """Tests that WordCountExtractor satisfies the MetricExtractor protocol."""
    extractor = WordCountExtractor()
    assert isinstance(extractor, MetricExtractor)
    assert extractor.name == "word_count"


def test_stub_extractor_protocol_conformance():
    """Tests that the test StubExtractor satisfies the MetricExtractor protocol."""
    extractor = StubExtractor()
    assert isinstance(extractor, MetricExtractor)


# ==================== Phase 42: Tier 1 Extractor Tests ====================


# --- Temporal Distribution Extractor ---


def test_temporal_extractor_protocol_conformance():
    """Tests that TemporalDistributionExtractor satisfies the MetricExtractor protocol."""
    extractor = TemporalDistributionExtractor()
    assert isinstance(extractor, MetricExtractor)
    assert extractor.name == "temporal_distribution"


def test_temporal_extractor_produces_hour_and_weekday():
    """Tests that temporal extractor produces both publication_hour and publication_weekday."""
    from datetime import timezone
    from internal.models import SilverCore

    extractor = TemporalDistributionExtractor()
    # Wednesday 2026-04-01 at 14:30 UTC
    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="Test text",
        cleaned_text="Test text",
        timestamp=datetime(2026, 4, 1, 14, 30, 0, tzinfo=timezone.utc),
        word_count=2,
    )

    metrics = extractor.extract(core, "article-1")
    assert len(metrics) == 2

    hour_metric = next(m for m in metrics if m.metric_name == "publication_hour")
    weekday_metric = next(m for m in metrics if m.metric_name == "publication_weekday")

    assert hour_metric.value == 14.0
    assert 0.0 <= hour_metric.value <= 23.0

    # 2026-04-01 is a Wednesday → weekday() = 2
    assert weekday_metric.value == 2.0
    assert 0.0 <= weekday_metric.value <= 6.0

    assert hour_metric.source == "test"
    assert hour_metric.article_id == "article-1"


def test_temporal_extractor_midnight_sunday():
    """Tests boundary values: midnight (hour=0) and Sunday (weekday=6)."""
    from datetime import timezone
    from internal.models import SilverCore

    extractor = TemporalDistributionExtractor()
    # Sunday 2026-04-05 at 00:00 UTC
    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="Test",
        cleaned_text="Test",
        timestamp=datetime(2026, 4, 5, 0, 0, 0, tzinfo=timezone.utc),
        word_count=1,
    )

    metrics = extractor.extract(core, None)
    hour_metric = next(m for m in metrics if m.metric_name == "publication_hour")
    weekday_metric = next(m for m in metrics if m.metric_name == "publication_weekday")

    assert hour_metric.value == 0.0
    assert weekday_metric.value == 6.0  # Sunday


# --- Language Detection Extractor ---


def test_language_extractor_protocol_conformance():
    """Tests that LanguageDetectionExtractor satisfies the MetricExtractor protocol."""
    extractor = LanguageDetectionExtractor()
    assert isinstance(extractor, MetricExtractor)
    assert extractor.name == "language_detection"


def test_language_extractor_german_text():
    """Tests that German text produces a language_confidence metric in [0, 1]."""
    from datetime import timezone
    from internal.models import SilverCore

    extractor = LanguageDetectionExtractor()
    core = SilverCore(
        document_id="abc123",
        source="tagesschau",
        source_type="rss",
        raw_text="Die Bundesregierung hat am Mittwoch ein umfassendes Maßnahmenpaket zum Klimaschutz verabschiedet.",
        cleaned_text="Die Bundesregierung hat am Mittwoch ein umfassendes Maßnahmenpaket zum Klimaschutz verabschiedet.",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=11,
    )

    metrics = extractor.extract(core, "article-1")
    assert len(metrics) == 1
    assert metrics[0].metric_name == "language_confidence"
    assert 0.0 <= metrics[0].value <= 1.0
    assert metrics[0].source == "tagesschau"


def test_language_extractor_short_text_returns_empty():
    """Tests that very short text (<10 chars) returns no metrics."""
    from datetime import timezone
    from internal.models import SilverCore

    extractor = LanguageDetectionExtractor()
    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="Hi",
        cleaned_text="Hi",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=1,
    )

    metrics = extractor.extract(core, None)
    assert metrics == []


def test_language_extractor_empty_text_returns_empty():
    """Tests that empty cleaned_text returns no metrics."""
    from datetime import timezone
    from internal.models import SilverCore

    extractor = LanguageDetectionExtractor()
    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="placeholder",
        cleaned_text="",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=0,
    )

    metrics = extractor.extract(core, None)
    assert metrics == []


def test_language_extractor_deterministic():
    """Tests that the same input produces the same confidence score (fixed seed)."""
    from datetime import timezone
    from internal.models import SilverCore

    extractor = LanguageDetectionExtractor()
    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="Die Bundesregierung hat ein neues Gesetz beschlossen über den Klimaschutz.",
        cleaned_text="Die Bundesregierung hat ein neues Gesetz beschlossen über den Klimaschutz.",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=11,
    )

    results = [extractor.extract(core, None)[0].value for _ in range(5)]
    assert len(set(results)) == 1  # all identical


# --- Sentiment Extractor (SentiWS) ---


def test_sentiment_extractor_protocol_conformance():
    """Tests that SentimentExtractor satisfies the MetricExtractor protocol."""
    from pathlib import Path
    # Use a temp dir with no lexicon files — extractor loads empty
    extractor = SentimentExtractor(sentiws_dir=Path("/nonexistent"))
    assert isinstance(extractor, MetricExtractor)
    assert extractor.name == "sentiment"


def test_sentiment_extractor_no_lexicon_returns_empty():
    """Tests that an extractor with no loaded lexicon produces no metrics."""
    from pathlib import Path
    from datetime import timezone
    from internal.models import SilverCore

    extractor = SentimentExtractor(sentiws_dir=Path("/nonexistent"))
    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="Positive text here",
        cleaned_text="Positive text here",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=3,
    )

    metrics = extractor.extract(core, None)
    assert metrics == []


def test_sentiment_extractor_with_inline_lexicon(tmp_path):
    """
    Tests sentiment extraction with a minimal inline SentiWS lexicon.
    Creates temporary lexicon files to test the scoring algorithm.
    """
    from datetime import timezone
    from internal.models import SilverCore

    # Create minimal SentiWS-format lexicon files
    pos_file = tmp_path / "SentiWS_v2.0_Positive.txt"
    neg_file = tmp_path / "SentiWS_v2.0_Negative.txt"

    pos_file.write_text("Glück|NN\t0.5765\tGlücks,Glückes\ngut|ADJX\t0.5040\tguten,guter,gutes\n", encoding="utf-8")
    neg_file.write_text("schlecht|ADJX\t-0.4771\tschlechten,schlechter\nKrise|NN\t-0.3544\tKrisen\n", encoding="utf-8")

    extractor = SentimentExtractor(sentiws_dir=tmp_path)
    assert len(extractor._lexicon) > 0

    # Positive text
    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="Das ist gut und bringt Glück",
        cleaned_text="Das ist gut und bringt Glück",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=6,
    )

    metrics = extractor.extract(core, "article-1")
    assert len(metrics) == 2

    sentiment_metric = next(m for m in metrics if m.metric_name == "sentiment_score")
    lexicon_metric = next(m for m in metrics if m.metric_name == "lexicon_version")

    # "gut" (0.504) + "glück" (0.5765) → mean = ~0.54
    assert -1.0 <= sentiment_metric.value <= 1.0
    assert sentiment_metric.value > 0  # positive text → positive score

    assert lexicon_metric.value > 0  # hash as numeric


def test_sentiment_extractor_negative_text(tmp_path):
    """Tests that negative text produces a negative sentiment score."""
    from datetime import timezone
    from internal.models import SilverCore

    pos_file = tmp_path / "SentiWS_v2.0_Positive.txt"
    neg_file = tmp_path / "SentiWS_v2.0_Negative.txt"

    pos_file.write_text("gut|ADJX\t0.5040\n", encoding="utf-8")
    neg_file.write_text("schlecht|ADJX\t-0.4771\tschlechten,schlechter\nKrise|NN\t-0.3544\tKrisen\n", encoding="utf-8")

    extractor = SentimentExtractor(sentiws_dir=tmp_path)

    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="Die Krise ist schlecht",
        cleaned_text="Die Krise ist schlecht",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=4,
    )

    metrics = extractor.extract(core, None)
    sentiment = next(m for m in metrics if m.metric_name == "sentiment_score")
    assert sentiment.value < 0  # negative text → negative score
    assert -1.0 <= sentiment.value <= 1.0


def test_sentiment_extractor_no_matches_returns_zero(tmp_path):
    """Tests that text with no lexicon matches returns sentiment_score = 0."""
    from datetime import timezone
    from internal.models import SilverCore

    pos_file = tmp_path / "SentiWS_v2.0_Positive.txt"
    neg_file = tmp_path / "SentiWS_v2.0_Negative.txt"

    pos_file.write_text("gut|ADJX\t0.5040\n", encoding="utf-8")
    neg_file.write_text("schlecht|ADJX\t-0.4771\n", encoding="utf-8")

    extractor = SentimentExtractor(sentiws_dir=tmp_path)

    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="Ein neutraler Satz ohne Wertung",
        cleaned_text="Ein neutraler Satz ohne Wertung",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=5,
    )

    metrics = extractor.extract(core, None)
    sentiment = next(m for m in metrics if m.metric_name == "sentiment_score")
    assert sentiment.value == 0.0


def test_sentiment_lexicon_hash_deterministic(tmp_path):
    """Tests that the lexicon hash is deterministic across loads."""
    pos_file = tmp_path / "SentiWS_v2.0_Positive.txt"
    neg_file = tmp_path / "SentiWS_v2.0_Negative.txt"
    pos_file.write_text("gut|ADJX\t0.5040\n", encoding="utf-8")
    neg_file.write_text("schlecht|ADJX\t-0.4771\n", encoding="utf-8")

    e1 = SentimentExtractor(sentiws_dir=tmp_path)
    e2 = SentimentExtractor(sentiws_dir=tmp_path)
    assert e1.lexicon_hash == e2.lexicon_hash
    assert e1.lexicon_hash != "empty"


# --- Named Entity Extractor ---


def test_named_entity_extractor_protocol_conformance():
    """Tests that NamedEntityExtractor satisfies the MetricExtractor protocol."""
    extractor = NamedEntityExtractor()
    assert isinstance(extractor, MetricExtractor)
    assert extractor.name == "named_entity"


def test_named_entity_extractor_german_text():
    """
    Tests NER on a German sentence with known entities.
    Note: exact entities depend on the spaCy model version.
    """
    from datetime import timezone
    from internal.models import SilverCore

    extractor = NamedEntityExtractor()
    if extractor._nlp is None:
        pytest.skip("spaCy de_core_news_lg not installed")

    core = SilverCore(
        document_id="abc123",
        source="tagesschau",
        source_type="rss",
        raw_text="Bundeskanzler Olaf Scholz traf sich in Berlin mit dem französischen Präsidenten Emmanuel Macron.",
        cleaned_text="Bundeskanzler Olaf Scholz traf sich in Berlin mit dem französischen Präsidenten Emmanuel Macron.",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=13,
    )

    # Test metric output
    metrics = extractor.extract(core, "article-1")
    assert len(metrics) == 1
    assert metrics[0].metric_name == "entity_count"
    assert metrics[0].value >= 1  # at least some entities found
    assert metrics[0].source == "tagesschau"

    # Test entity output
    entities = extractor.extract_entities(core, "article-1")
    assert len(entities) >= 1
    assert all(isinstance(e, GoldEntity) for e in entities)

    # Verify entity structure
    for entity in entities:
        assert entity.entity_text  # non-empty
        assert entity.entity_label in ("PER", "ORG", "LOC", "MISC")
        assert entity.start_char >= 0
        assert entity.end_char > entity.start_char
        assert entity.source == "tagesschau"
        assert entity.article_id == "article-1"


def test_named_entity_extractor_empty_text():
    """Tests that empty text returns no entities and no metrics."""
    from datetime import timezone
    from internal.models import SilverCore

    extractor = NamedEntityExtractor()
    if extractor._nlp is None:
        pytest.skip("spaCy de_core_news_lg not installed")

    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="placeholder",
        cleaned_text="",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=0,
    )

    assert extractor.extract(core, None) == []
    assert extractor.extract_entities(core, None) == []


def test_named_entity_extractor_entity_count_matches():
    """Tests that entity_count metric matches the number of extracted entities."""
    from datetime import timezone
    from internal.models import SilverCore

    extractor = NamedEntityExtractor()
    if extractor._nlp is None:
        pytest.skip("spaCy de_core_news_lg not installed")

    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="Angela Merkel und Emmanuel Macron trafen sich in Paris.",
        cleaned_text="Angela Merkel und Emmanuel Macron trafen sich in Paris.",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=8,
    )

    metrics = extractor.extract(core, None)
    entities = extractor.extract_entities(core, None)

    entity_count_metric = metrics[0]
    assert entity_count_metric.value == float(len(entities))


# --- Entity Insertion in Processor ---


class StubEntityExtractor:
    """A test extractor that produces both metrics and entities (EntityExtractor protocol)."""

    @property
    def name(self) -> str:
        return "stub_entity"

    def extract_all(self, core, article_id: str | None) -> tuple[list[GoldMetric], list[GoldEntity]]:
        return self.extract(core, article_id), self.extract_entities(core, article_id)

    def extract(self, core, article_id: str | None) -> list[GoldMetric]:
        return [
            GoldMetric(
                timestamp=core.timestamp,
                value=2.0,
                source=core.source,
                metric_name="entity_count",
                article_id=article_id,
            )
        ]

    def extract_entities(self, core, article_id: str | None) -> list[GoldEntity]:
        return [
            GoldEntity(
                timestamp=core.timestamp,
                source=core.source,
                article_id=article_id,
                entity_text="Berlin",
                entity_label="LOC",
                start_char=0,
                end_char=6,
            ),
            GoldEntity(
                timestamp=core.timestamp,
                source=core.source,
                article_id=article_id,
                entity_text="Merkel",
                entity_label="PER",
                start_char=10,
                end_char=16,
            ),
        ]


def test_processor_inserts_entities(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span):
    """
    Tests that the processor inserts entities into aer_gold.entities
    when an extractor provides extract_entities().
    """
    extractors = [WordCountExtractor(), StubEntityExtractor()]
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("test-source/test-article/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    # Two inserts: one for metrics, one for entities
    assert mock_clickhouse.insert.call_count == 2

    # First call: metrics
    metrics_call = mock_clickhouse.insert.call_args_list[0]
    assert metrics_call[0][0] == "aer_gold.metrics"
    assert len(metrics_call[0][1]) == 2  # word_count + entity_count

    # Second call: entities
    entities_call = mock_clickhouse.insert.call_args_list[1]
    assert entities_call[0][0] == "aer_gold.entities"
    entity_rows = entities_call[0][1]
    assert len(entity_rows) == 2
    assert entity_rows[0][3] == "Berlin"   # entity_text
    assert entity_rows[0][4] == "LOC"      # entity_label
    assert entity_rows[1][3] == "Merkel"
    assert entity_rows[1][4] == "PER"
    assert entities_call[1]["column_names"] == [
        "timestamp", "source", "article_id", "entity_text", "entity_label", "start_char", "end_char"
    ]


def test_processor_no_entity_insert_without_entity_extractor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span):
    """
    Tests that extractors without extract_entities() do not trigger
    entity insertion — only the metrics insert happens.
    """
    extractors = [WordCountExtractor()]
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("test-source/test-article/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    # Only one insert (metrics), no entity insert
    mock_clickhouse.insert.assert_called_once()
    assert mock_clickhouse.insert.call_args[0][0] == "aer_gold.metrics"


# --- Full Pipeline Integration Test ---


def test_full_extractor_pipeline_with_all_tier1(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span):
    """
    Integration test: process a German RSS document through the full extractor
    pipeline with all Tier 1 extractors (word_count, temporal, language, sentiment).
    Entity extractor excluded (requires spaCy model).
    Sentiment uses empty lexicon (no SentiWS files in test env).
    """
    from pathlib import Path

    extractors = [
        WordCountExtractor(),
        TemporalDistributionExtractor(),
        LanguageDetectionExtractor(),
        SentimentExtractor(sentiws_dir=Path("/nonexistent")),  # no lexicon → no metrics
    ]
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_RSS_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("rss/tagesschau/abc123/2026-04-05.json", "2026-04-05T10:00:00.000Z", dummy_span)

    # Two inserts: metrics + language_detections (Phase 45)
    assert mock_clickhouse.insert.call_count == 2

    insert_calls = mock_clickhouse.insert.call_args_list
    metrics_call = [c for c in insert_calls if c[0][0] == "aer_gold.metrics"][0]
    rows = metrics_call[0][1]

    metric_names = [row[3] for row in rows]
    assert "word_count" in metric_names
    assert "publication_hour" in metric_names
    assert "publication_weekday" in metric_names
    assert "language_confidence" in metric_names
    # sentiment_score not present (no lexicon loaded)
    assert "sentiment_score" not in metric_names

    # Verify value ranges
    for row in rows:
        name, value = row[3], row[1]
        if name == "publication_hour":
            assert 0.0 <= value <= 23.0
        elif name == "publication_weekday":
            assert 0.0 <= value <= 6.0
        elif name == "language_confidence":
            assert 0.0 <= value <= 1.0

    # Verify language_detections insert also happened
    lang_tables = [c[0][0] for c in insert_calls]
    assert "aer_gold.language_detections" in lang_tables

    proc._update_document_status.assert_called_with(
        "rss/tagesschau/abc123/2026-04-05.json", "processed"
    )


# ==================== Phase 44: Protocol Correctness & DRY Tests ====================


def test_named_entity_extractor_is_entity_extractor():
    """Tests that NamedEntityExtractor satisfies the EntityExtractor protocol."""
    extractor = NamedEntityExtractor(model_name="nonexistent_model_for_test")
    assert isinstance(extractor, EntityExtractor)


def test_entity_extractor_protocol_not_satisfied_by_metric_only():
    """Tests that a plain MetricExtractor does NOT satisfy EntityExtractor."""
    extractor = WordCountExtractor()
    assert isinstance(extractor, MetricExtractor)
    assert not isinstance(extractor, EntityExtractor)


def test_non_callable_extract_entities_does_not_crash_processor(
    mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span
):
    """
    Tests that an extractor with a non-callable extract_entities attribute
    does not crash the processor — isinstance(EntityExtractor) returns False
    because the protocol requires callable methods.
    """
    class BadExtractor:
        extract_entities = "not a callable"

        @property
        def name(self) -> str:
            return "bad_extractor"

        def extract(self, core, article_id):
            return [
                GoldMetric(
                    timestamp=core.timestamp,
                    value=1.0,
                    source=core.source,
                    metric_name="bad_metric",
                    article_id=article_id,
                )
            ]

    extractors = [BadExtractor()]
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    # Must not raise — bad extractor is treated as a plain MetricExtractor
    proc.process_event("test-source/test-article/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    mock_clickhouse.insert.assert_called_once()
    ch_args, _ = mock_clickhouse.insert.call_args
    rows = ch_args[1]
    assert len(rows) == 1
    assert rows[0][3] == "bad_metric"

    proc._update_document_status.assert_called_with(
        "test-source/test-article/2023-10-25.json", "processed"
    )


def test_quarantine_helper_sets_span_attributes(
    mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span
):
    """
    Tests that the _quarantine helper sets all expected span attributes
    and increments the correct Prometheus metrics.
    """
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, [])
    proc._update_document_status = MagicMock()

    raw_content = {"source": "test", "raw_text": "broken"}
    proc._quarantine("test-key", raw_content, "test_reason", dummy_span)

    # Verify MinIO quarantine upload
    mock_minio.put_object.assert_called_once()
    args, _ = mock_minio.put_object.call_args
    assert args[0] == "bronze-quarantine"
    assert args[1] == "test-key"

    # Verify DB update
    proc._update_document_status.assert_called_with("test-key", "quarantined")


def test_quarantine_helper_from_unknown_source_type(
    mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span
):
    """
    Tests that quarantine via unknown source_type uses the _quarantine helper
    and produces the correct quarantine_reason span attribute.
    """
    bronze_data = json.dumps({
        "source": "forum-feed",
        "source_type": "forum",
        "raw_text": "Some forum post",
        "url": "https://example.com/forum",
        "timestamp": "2023-10-25T12:34:56Z",
    }).encode('utf-8')

    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, [])

    mock_response = MagicMock()
    mock_response.read.return_value = bronze_data
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("forum-feed/post/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    mock_minio.put_object.assert_called_once()
    args, _ = mock_minio.put_object.call_args
    assert args[0] == "bronze-quarantine"
    proc._update_document_status.assert_called_with(
        "forum-feed/post/2023-10-25.json", "quarantined"
    )


def test_quarantine_helper_from_validation_failure(
    mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span
):
    """
    Tests that quarantine via Silver contract validation failure uses
    the _quarantine helper correctly.
    """
    bronze_data = json.dumps({
        "source": "test-source",
        "title": "Empty Content",
        "raw_text": "   \t  \n  ",
        "url": "https://example.com/empty",
        "timestamp": "2023-10-25T12:34:56Z",
    }).encode('utf-8')

    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, [])

    mock_response = MagicMock()
    mock_response.read.return_value = bronze_data
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("test-source/empty/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    mock_minio.put_object.assert_called_once()
    args, _ = mock_minio.put_object.call_args
    assert args[0] == "bronze-quarantine"
    mock_clickhouse.insert.assert_not_called()
    proc._update_document_status.assert_called_with(
        "test-source/empty/2023-10-25.json", "quarantined"
    )


# ==================== Phase 45: Language Detection Persistence Tests ====================


def test_language_extractor_is_language_detection_persist_extractor():
    """Tests that LanguageDetectionExtractor satisfies the LanguageDetectionPersistExtractor protocol."""
    from internal.extractors import LanguageDetectionPersistExtractor
    extractor = LanguageDetectionExtractor()
    assert isinstance(extractor, LanguageDetectionPersistExtractor)


def test_language_extractor_not_entity_extractor():
    """Tests that LanguageDetectionExtractor does NOT satisfy EntityExtractor."""
    extractor = LanguageDetectionExtractor()
    assert not isinstance(extractor, EntityExtractor)


def test_language_extractor_extract_all_german_text():
    """Tests that extract_all() returns both metrics and language detections."""
    from datetime import timezone
    from internal.models import SilverCore
    from internal.extractors import GoldLanguageDetection

    extractor = LanguageDetectionExtractor()
    core = SilverCore(
        document_id="abc123",
        source="tagesschau",
        source_type="rss",
        raw_text="Die Bundesregierung hat am Mittwoch ein umfassendes Maßnahmenpaket zum Klimaschutz verabschiedet.",
        cleaned_text="Die Bundesregierung hat am Mittwoch ein umfassendes Maßnahmenpaket zum Klimaschutz verabschiedet.",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=11,
    )

    metrics, detections = extractor.extract_all(core, "article-1")

    # Metrics: language_confidence as before
    assert len(metrics) == 1
    assert metrics[0].metric_name == "language_confidence"
    assert 0.0 <= metrics[0].value <= 1.0

    # Detections: at least one ranked candidate
    assert len(detections) >= 1
    assert all(isinstance(d, GoldLanguageDetection) for d in detections)

    # Rank 1 should be the top candidate
    assert detections[0].rank == 1
    assert detections[0].detected_language  # non-empty
    assert 0.0 <= detections[0].confidence <= 1.0
    assert detections[0].source == "tagesschau"
    assert detections[0].article_id == "article-1"

    # Ranks must be sequential
    for i, d in enumerate(detections):
        assert d.rank == i + 1


def test_language_extractor_extract_all_short_text():
    """Tests that extract_all() returns empty lists for short text."""
    from datetime import timezone
    from internal.models import SilverCore

    extractor = LanguageDetectionExtractor()
    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="Hi",
        cleaned_text="Hi",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=1,
    )

    metrics, detections = extractor.extract_all(core, None)
    assert metrics == []
    assert detections == []


def test_language_extractor_extract_language_detections():
    """Tests that extract_language_detections() returns only detections."""
    from datetime import timezone
    from internal.models import SilverCore
    from internal.extractors import GoldLanguageDetection

    extractor = LanguageDetectionExtractor()
    core = SilverCore(
        document_id="abc123",
        source="tagesschau",
        source_type="rss",
        raw_text="Die Bundesregierung hat am Mittwoch ein umfassendes Maßnahmenpaket zum Klimaschutz verabschiedet.",
        cleaned_text="Die Bundesregierung hat am Mittwoch ein umfassendes Maßnahmenpaket zum Klimaschutz verabschiedet.",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=11,
    )

    detections = extractor.extract_language_detections(core, "article-1")
    assert len(detections) >= 1
    assert all(isinstance(d, GoldLanguageDetection) for d in detections)


def test_processor_language_detections_insert(
    mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span
):
    """
    Tests that the processor inserts language detections into
    aer_gold.language_detections when LanguageDetectionExtractor is used.
    """
    extractors = [LanguageDetectionExtractor()]
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_RSS_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("rss/tagesschau/abc123/2026-04-05.json", "2026-04-05T10:00:00.000Z", dummy_span)

    # Should have two inserts: metrics + language_detections
    assert mock_clickhouse.insert.call_count == 2

    insert_calls = mock_clickhouse.insert.call_args_list
    tables = [call[0][0] for call in insert_calls]
    assert "aer_gold.metrics" in tables
    assert "aer_gold.language_detections" in tables

    # Verify language_detections insert structure
    lang_call = [c for c in insert_calls if c[0][0] == "aer_gold.language_detections"][0]
    lang_rows = lang_call[0][1]
    assert len(lang_rows) >= 1
    assert lang_call[1]['column_names'] == ['timestamp', 'source', 'article_id', 'detected_language', 'confidence', 'rank']

    # First row should be rank 1
    assert lang_rows[0][5] == 1  # rank column


def test_processor_no_language_detection_insert_without_persist_extractor(
    mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span
):
    """
    Tests that extractors without LanguageDetectionPersistExtractor protocol
    do not trigger language_detections insertion.
    """
    extractors = [WordCountExtractor()]
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("test-source/test-article/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    # Only one insert (metrics), no language_detections insert
    mock_clickhouse.insert.assert_called_once()
    assert mock_clickhouse.insert.call_args[0][0] == "aer_gold.metrics"


def test_full_pipeline_with_language_detection_persistence(
    mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span
):
    """
    Integration test: process a German RSS document through the pipeline
    with all Tier 1 extractors including language detection persistence.
    Verifies both metrics and language_detections inserts happen.
    """
    from pathlib import Path

    extractors = [
        WordCountExtractor(),
        TemporalDistributionExtractor(),
        LanguageDetectionExtractor(),
        SentimentExtractor(sentiws_dir=Path("/nonexistent")),
    ]
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_RSS_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("rss/tagesschau/abc123/2026-04-05.json", "2026-04-05T10:00:00.000Z", dummy_span)

    # Two inserts: metrics + language_detections
    assert mock_clickhouse.insert.call_count == 2

    insert_calls = mock_clickhouse.insert.call_args_list
    tables = [call[0][0] for call in insert_calls]
    assert "aer_gold.metrics" in tables
    assert "aer_gold.language_detections" in tables

    # Verify metrics still include all expected names
    metrics_call = [c for c in insert_calls if c[0][0] == "aer_gold.metrics"][0]
    metric_names = [row[3] for row in metrics_call[0][1]]
    assert "word_count" in metric_names
    assert "publication_hour" in metric_names
    assert "language_confidence" in metric_names
