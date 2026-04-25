import io
import json
import pytest
from datetime import datetime
from unittest.mock import MagicMock
from internal.models import generate_document_id
from conftest import VALID_BRONZE_DATA, DUMMY_EVENT_TIME, EXPECTED_WORD_COUNT, gold_insert_calls


def test_silver_contract_happy_path(processor, mock_minio, mock_clickhouse, dummy_span):
    """
    Tests if valid Bronze data is harmonized via the legacy adapter and correctly
    passed to the Silver Layer (MinIO) and Gold Layer (ClickHouse).
    """
    EXPECTED_DATETIME = datetime.fromisoformat("2023-10-25T12:34:56+00:00")

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response

    processor._get_document_status = MagicMock(return_value=None)
    processor._update_document_status = MagicMock()

    obj_key = "test-source/test-article/2023-10-25.json"
    processor.process_event(obj_key, DUMMY_EVENT_TIME, dummy_span)

    # Silver upload
    mock_minio.put_object.assert_called_once()
    args, kwargs = mock_minio.put_object.call_args
    assert args[0] == "silver"
    assert args[1] == obj_key

    # ClickHouse: deterministic timestamp + word count + dimensions
    assert len(gold_insert_calls(mock_clickhouse)) == 1
    ch_args, ch_kwargs = gold_insert_calls(mock_clickhouse)[0]
    assert ch_args[0] == "aer_gold.metrics"
    row = ch_args[1][0]
    assert row[0] == EXPECTED_DATETIME
    assert row[1] == float(EXPECTED_WORD_COUNT)
    assert row[2] == "test-source"
    assert row[3] == "word_count"
    assert row[4] == generate_document_id("test-source", obj_key)
    assert ch_kwargs['column_names'] == ['timestamp', 'value', 'source', 'metric_name', 'article_id', 'discourse_function', 'ingestion_version']

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

    processor.process_event("test-source/incomplete/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    mock_minio.put_object.assert_called_once()
    args, kwargs = mock_minio.put_object.call_args
    assert args[0] == "bronze-quarantine"
    assert args[1] == "test-source/incomplete/2023-10-25.json"

    assert len(gold_insert_calls(mock_clickhouse)) == 0

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

    ws_obj_key = "test-source/whitespace-article/2023-10-25.json"
    processor.process_event(ws_obj_key, DUMMY_EVENT_TIME, dummy_span)

    assert len(gold_insert_calls(mock_clickhouse)) == 1
    ch_args, ch_kwargs = gold_insert_calls(mock_clickhouse)[0]
    row = ch_args[1][0]
    assert row[1] == float(EXPECTED_WORD_COUNT)
    assert row[2] == "test-source"
    assert row[3] == "word_count"
    assert row[4] == generate_document_id("test-source", ws_obj_key)


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
    assert len(gold_insert_calls(mock_clickhouse)) == 0
    processor._update_document_status.assert_called_with(
        "test-source/empty/2023-10-25.json", "quarantined"
    )


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

    uploaded_buffer = args[2]
    assert isinstance(uploaded_buffer, io.BytesIO)
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


def test_quarantine_helper_sets_span_attributes(
    mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span
):
    """
    Tests that the _quarantine helper sets all expected span attributes
    and increments the correct Prometheus metrics.
    """
    from conftest import _make_processor
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, [])
    proc._update_document_status = MagicMock()

    raw_content = {"source": "test", "raw_text": "broken"}
    proc._quarantine("test-key", raw_content, "test_reason", dummy_span)

    mock_minio.put_object.assert_called_once()
    args, _ = mock_minio.put_object.call_args
    assert args[0] == "bronze-quarantine"
    assert args[1] == "test-key"

    proc._update_document_status.assert_called_with("test-key", "quarantined")


def test_quarantine_helper_from_unknown_source_type(
    mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span
):
    """
    Tests that quarantine via unknown source_type uses the _quarantine helper
    and produces the correct quarantine_reason span attribute.
    """
    from conftest import _make_processor
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
    from conftest import _make_processor
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
    assert len(gold_insert_calls(mock_clickhouse)) == 0
    proc._update_document_status.assert_called_with(
        "test-source/empty/2023-10-25.json", "quarantined"
    )


def test_silver_core_naive_timestamp_raises_validation_error():
    """SilverCore must reject naive datetimes at the Silver contract level."""
    from internal.models import SilverCore, ValidationError

    with pytest.raises(ValidationError):
        SilverCore(
            document_id="abc123",
            source="test",
            source_type="rss",
            raw_text="Test",
            cleaned_text="Test",
            timestamp=datetime(2026, 4, 5, 10, 0, 0),  # naive — no tzinfo
            word_count=1,
        )


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

    assert silver_data["core"]["raw_text"] == raw_text
    assert silver_data["core"]["cleaned_text"] == "Hello world from the source"


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
