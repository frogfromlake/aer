from unittest.mock import MagicMock
from conftest import DUMMY_EVENT_TIME


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
