import pytest
import json
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
def processor(mock_minio, mock_clickhouse):
    """Provides a DataProcessor instance with mocked infrastructure."""
    return DataProcessor(mock_minio, mock_clickhouse)

@pytest.fixture
def dummy_span():
    """Provides a dummy OpenTelemetry span for testing."""
    tracer = trace.get_tracer(__name__)
    with tracer.start_as_current_span("test-span") as span:
        yield span

def test_silver_contract_happy_path(processor, mock_minio, mock_clickhouse, dummy_span):
    """
    Tests if valid Bronze data is harmonized and correctly passed 
    to the Silver Layer (MinIO) and Gold Layer (ClickHouse).
    """
    # 1. Setup
    valid_bronze_data = json.dumps({
        "message": "Valid test message",
        "metric_value": 42.5
    }).encode('utf-8')
    
    mock_response = MagicMock()
    mock_response.read.return_value = valid_bronze_data
    mock_minio.get_object.return_value = mock_response
    
    # Assume file is not yet processed
    processor._is_already_processed = MagicMock(return_value=False)

    # 2. Execute
    processor.process_event("test_happy.json", dummy_span)

    # 3. Assert MinIO
    mock_minio.put_object.assert_called_once()
    args, kwargs = mock_minio.put_object.call_args
    assert args[0] == "silver" # Must be routed to silver
    assert args[1] == "test_happy.json"
    
    # 4. Assert ClickHouse
    mock_clickhouse.insert.assert_called_once()
    ch_args, ch_kwargs = mock_clickhouse.insert.call_args
    assert ch_args[0] == "aer_gold.metrics"

def test_silver_contract_corrupt_data(processor, mock_minio, mock_clickhouse, dummy_span):
    """
    Tests if data missing the required 'message' field is caught 
    by Pydantic and correctly routed to the Dead Letter Queue (DLQ).
    """
    # 1. Setup (Missing 'message' field)
    corrupt_bronze_data = json.dumps({
        "metric_value": 99.9
    }).encode('utf-8')
    
    mock_response = MagicMock()
    mock_response.read.return_value = corrupt_bronze_data
    mock_minio.get_object.return_value = mock_response
    
    processor._is_already_processed = MagicMock(return_value=False)

    # 2. Execute
    processor.process_event("test_corrupt.json", dummy_span)

    # 3. Assert MinIO (Must go to DLQ)
    mock_minio.put_object.assert_called_once()
    args, kwargs = mock_minio.put_object.call_args
    assert args[0] == "bronze-quarantine" # Dead Letter Queue
    assert args[1] == "test_corrupt.json"
    
    # 4. Assert ClickHouse (Must NOT be called)
    mock_clickhouse.insert.assert_not_called()

def test_idempotency_skip_duplicate(processor, mock_minio, mock_clickhouse, dummy_span):
    """
    Tests if an already processed event is skipped entirely.
    """
    # 1. Setup: Simulate that the file already exists in Silver or DLQ
    processor._is_already_processed = MagicMock(return_value=True)

    # 2. Execute
    processor.process_event("test_duplicate.json", dummy_span)

    # 3. Assert (Neither MinIO get/put nor ClickHouse insert should be called)
    mock_minio.get_object.assert_not_called()
    mock_minio.put_object.assert_not_called()
    mock_clickhouse.insert.assert_not_called()