import pytest
import json
from datetime import datetime
from unittest.mock import MagicMock
from opentelemetry import trace
from internal.processor import DataProcessor
from internal.adapters import AdapterRegistry, LegacyAdapter, RssAdapter
from internal.models import generate_document_id

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
def processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry):
    """Provides a DataProcessor instance with mocked infrastructure."""
    return DataProcessor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry)

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
