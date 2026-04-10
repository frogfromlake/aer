import json
from datetime import datetime, timezone
from unittest.mock import MagicMock
from internal.adapters import LegacyAdapter, RssAdapter
from conftest import VALID_BRONZE_DATA, VALID_RAW_TEXT, DUMMY_EVENT_TIME


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

    mock_minio.put_object.assert_called_once()
    args, _ = mock_minio.put_object.call_args
    assert args[0] == "silver"

    silver_buffer = args[2]
    silver_buffer.seek(0)
    silver_data = __import__('json').loads(silver_buffer.read().decode('utf-8'))

    assert silver_data["core"]["source_type"] == "legacy"
    assert silver_data["core"]["schema_version"] == 1
    assert silver_data["core"]["raw_text"] == VALID_RAW_TEXT
    assert silver_data["core"]["cleaned_text"] == VALID_RAW_TEXT
    assert silver_data["core"]["language"] == "und"
    assert silver_data["meta"] is None


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

    mock_minio.put_object.assert_called_once()
    args, _ = mock_minio.put_object.call_args
    assert args[0] == "silver"

    processor._update_document_status.assert_called_with(
        "wikipedia/old/2023-10-25.json", "processed"
    )


def test_rss_adapter_registry_lookup(adapter_registry):
    """Tests that the RSS adapter is registered and returned."""
    adapter = adapter_registry.get("rss")
    assert adapter is not None
    assert isinstance(adapter, RssAdapter)


def test_rss_adapter_empty_categories():
    """Tests that RssMeta handles missing categories gracefully."""
    adapter = RssAdapter()
    raw = {
        "source": "test",
        "raw_text": "Some text",
        "url": "https://example.com",
    }
    event_time = datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc)
    core, meta = adapter.harmonize(raw, event_time, "rss/test/abc/2026-04-05.json")

    assert core.source_type == "rss"
    assert meta.categories == []
    assert meta.author == ""
    assert meta.feed_url == ""
