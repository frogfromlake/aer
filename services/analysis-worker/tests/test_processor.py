import pytest
import json
from datetime import datetime
from unittest.mock import MagicMock
from opentelemetry import trace
from internal.processor import DataProcessor

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
def processor(mock_minio, mock_clickhouse, mock_pg_pool):
    """Provides a DataProcessor instance with mocked infrastructure."""
    return DataProcessor(mock_minio, mock_clickhouse, mock_pg_pool)

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
    Tests if valid Bronze data is harmonized and correctly passed to the Silver Layer
    (MinIO) and Gold Layer (ClickHouse) with a deterministic timestamp.
    """
    # 1. Setup
    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response

    # Assume file is pending (DB returns None)
    processor._get_document_status = MagicMock(return_value=None)
    processor._update_document_status = MagicMock()

    # 2. Execute
    processor.process_event("test-source/test-article/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    # 3. Assert MinIO (Silver upload)
    mock_minio.put_object.assert_called_once()
    args, kwargs = mock_minio.put_object.call_args
    assert args[0] == "silver"
    assert args[1] == "test-source/test-article/2023-10-25.json"

    # 4. Assert ClickHouse: deterministic timestamp + word count as metric value
    mock_clickhouse.insert.assert_called_once()
    ch_args, ch_kwargs = mock_clickhouse.insert.call_args
    assert ch_args[0] == "aer_gold.metrics"
    assert ch_args[1][0][0] == EXPECTED_DATETIME          # Must NOT be datetime.now()
    assert ch_args[1][0][1] == float(EXPECTED_WORD_COUNT) # word_count as metric

    # 5. Assert DB Update (Commit Success)
    processor._update_document_status.assert_called_with(
        "test-source/test-article/2023-10-25.json", "processed"
    )


def test_silver_contract_missing_raw_text(processor, mock_minio, mock_clickhouse, dummy_span):
    """
    Tests if a document missing the 'raw_text' field is caught by validation
    and correctly routed to the Dead Letter Queue (DLQ).
    """
    # 1. Setup (Missing 'raw_text' field)
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

    # 2. Execute
    processor.process_event("test-source/incomplete/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    # 3. Assert MinIO (Must go to DLQ)
    mock_minio.put_object.assert_called_once()
    args, kwargs = mock_minio.put_object.call_args
    assert args[0] == "bronze-quarantine"  # Dead Letter Queue
    assert args[1] == "test-source/incomplete/2023-10-25.json"

    # 4. Assert ClickHouse (Must NOT be called)
    mock_clickhouse.insert.assert_not_called()

    # 5. Assert DB Update (Quarantined)
    processor._update_document_status.assert_called_with(
        "test-source/incomplete/2023-10-25.json", "quarantined"
    )


def test_silver_contract_missing_title(processor, mock_minio, mock_clickhouse, dummy_span):
    """
    Tests if a document missing the 'title' field is sent to the DLQ.
    """
    # 1. Setup (Missing 'title' field)
    corrupt_bronze_data = json.dumps({
        "source": "test-source",
        "raw_text": "Some text without a title",
        "url": "https://example.com/unknown",
        "timestamp": "2023-10-25T12:34:56Z",
    }).encode('utf-8')

    mock_response = MagicMock()
    mock_response.read.return_value = corrupt_bronze_data
    mock_minio.get_object.return_value = mock_response

    processor._get_document_status = MagicMock(return_value=None)
    processor._update_document_status = MagicMock()

    # 2. Execute
    processor.process_event("test-source/unknown/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    # 3. Assert DLQ routing
    mock_minio.put_object.assert_called_once()
    args, _ = mock_minio.put_object.call_args
    assert args[0] == "bronze-quarantine"

    # 4. Assert ClickHouse not touched
    mock_clickhouse.insert.assert_not_called()


def test_whitespace_normalization(processor, mock_minio, mock_clickhouse, dummy_span):
    """
    Tests that leading/trailing and internal whitespace in raw_text is normalized.
    The word count must reflect the cleaned text, not the raw whitespace-padded string.
    """
    # 1. Setup: raw_text with irregular whitespace
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

    # 2. Execute
    processor.process_event("test-source/whitespace-article/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    # 3. Assert: word_count must equal 5 (same as the clean version)
    mock_clickhouse.insert.assert_called_once()
    ch_args, _ = mock_clickhouse.insert.call_args
    assert ch_args[1][0][1] == float(EXPECTED_WORD_COUNT)


def test_idempotency_skip_duplicate(processor, mock_minio, mock_clickhouse, dummy_span):
    """
    Tests if an already processed event is skipped entirely.
    """
    # 1. Setup: Simulate that the file already exists in DB as 'processed'
    processor._get_document_status = MagicMock(return_value="processed")

    # 2. Execute
    processor.process_event("test-source/duplicate/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    # 3. Assert (Neither MinIO get/put nor ClickHouse insert should be called)
    mock_minio.get_object.assert_not_called()
    mock_minio.put_object.assert_not_called()
    mock_clickhouse.insert.assert_not_called()
