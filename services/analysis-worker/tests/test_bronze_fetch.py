import json
import pytest
from unittest.mock import MagicMock
from conftest import VALID_BRONZE_DATA, DUMMY_EVENT_TIME


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
