"""Phase 62 test coverage — discourse_function ClickHouse column.

Split out from tests/test_discourse.py during Phase 72. Verifies that the
DataProcessor propagates the DiscourseContext from SilverMeta into the Gold
layer as a dedicated ``discourse_function`` column on ``aer_gold.metrics``.
"""

import json
from unittest.mock import MagicMock, patch

from internal.adapters import AdapterRegistry, LegacyAdapter
from internal.adapters.rss import RssAdapter
from internal.extractors import WordCountExtractor
from internal.processor import DataProcessor

from conftest import DUMMY_EVENT_TIME


def _bronze_payload() -> bytes:
    return json.dumps({
        "source": "tagesschau",
        "source_type": "rss",
        "raw_text": "Die Bundesregierung hat am Mittwoch ein Paket verabschiedet.",
        "url": "https://example.com/article",
    }).encode("utf-8")


def _wire_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter):
    registry = AdapterRegistry({"legacy": LegacyAdapter(), "rss": adapter})
    processor = DataProcessor(
        mock_minio, mock_clickhouse, mock_pg_pool, registry, [WordCountExtractor()]
    )
    mock_response = MagicMock()
    mock_response.read.return_value = _bronze_payload()
    mock_minio.get_object.return_value = mock_response
    processor._get_document_status = MagicMock(return_value=None)
    processor._update_document_status = MagicMock()
    return processor


def test_discourse_function_written_when_classification_present(
    mock_minio, mock_clickhouse, mock_pg_pool, dummy_span
):
    mock_pool = MagicMock()
    with patch("internal.adapters.rss.get_source_classification") as mock_get:
        mock_get.return_value = {
            "primary_function": "epistemic_authority",
            "secondary_function": "power_legitimation",
            "emic_designation": "Tagesschau",
        }
        adapter = RssAdapter(pg_pool=mock_pool)
        processor = _wire_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter)
        processor.process_event("rss/tagesschau/test.json", DUMMY_EVENT_TIME, dummy_span)

    insert_call = mock_clickhouse.insert.call_args
    assert insert_call is not None
    _, kwargs = insert_call
    assert "discourse_function" in kwargs["column_names"]
    row = insert_call[0][1][0]
    assert row[-1] == "epistemic_authority"


def test_discourse_function_empty_string_without_classification(
    mock_minio, mock_clickhouse, mock_pg_pool, dummy_span
):
    """No DiscourseContext → discourse_function column contains '' (ClickHouse DEFAULT '')."""
    adapter = RssAdapter()  # No pg_pool
    processor = _wire_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter)
    processor.process_event("rss/tagesschau/test.json", DUMMY_EVENT_TIME, dummy_span)

    insert_call = mock_clickhouse.insert.call_args
    assert insert_call is not None
    _, kwargs = insert_call
    assert "discourse_function" in kwargs["column_names"]
    row = insert_call[0][1][0]
    assert row[-1] == ""
