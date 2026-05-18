from unittest.mock import patch
from conftest import DUMMY_EVENT_TIME


def test_idempotency_skip_duplicate(processor, mock_minio, mock_clickhouse, dummy_span):
    """Tests if an already processed event is skipped entirely.

    Phase 122e A27 replaced the SELECT-status pattern with the atomic
    `try_claim_document` compare-and-swap. A document whose status is
    already `processed` fails the CAS — the claim returns False and
    `_process_event_inner` returns before any MinIO / CH side-effects.
    """
    with patch("internal.processor.try_claim_document", return_value=False):
        processor.process_event("test-source/duplicate/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    mock_minio.get_object.assert_not_called()
    mock_minio.put_object.assert_not_called()
    mock_clickhouse.insert.assert_not_called()


def test_idempotency_skip_quarantined(processor, mock_minio, mock_clickhouse, dummy_span):
    """Tests that an event already in 'quarantined' state is also skipped."""
    with patch("internal.processor.try_claim_document", return_value=False):
        processor.process_event("test-source/quarantined/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    mock_minio.get_object.assert_not_called()
    mock_minio.put_object.assert_not_called()
    mock_clickhouse.insert.assert_not_called()
