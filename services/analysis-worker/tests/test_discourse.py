import json
from datetime import datetime, timezone
from unittest.mock import MagicMock, patch
from internal.models.discourse import DiscourseContext, ProbeEticTag, ProbeEmicTag
from internal.adapters.rss import RssAdapter, RssMeta
from internal.processor import DataProcessor
from internal.adapters import AdapterRegistry, LegacyAdapter
from internal.extractors import WordCountExtractor
from conftest import DUMMY_EVENT_TIME


# ---------------------------------------------------------------------------
# Pydantic model tests
# ---------------------------------------------------------------------------


def test_discourse_context_construction():
    """Tests DiscourseContext creation with all fields."""
    ctx = DiscourseContext(
        primary_function="epistemic_authority",
        secondary_function="power_legitimation",
        emic_designation="Tagesschau",
    )
    assert ctx.primary_function == "epistemic_authority"
    assert ctx.secondary_function == "power_legitimation"
    assert ctx.emic_designation == "Tagesschau"


def test_discourse_context_optional_secondary():
    """Tests DiscourseContext with no secondary function."""
    ctx = DiscourseContext(
        primary_function="power_legitimation",
        emic_designation="Bundesregierung",
    )
    assert ctx.secondary_function is None


def test_probe_etic_tag():
    """Tests ProbeEticTag construction with and without weights."""
    tag = ProbeEticTag(
        primary_function="epistemic_authority",
        secondary_function="power_legitimation",
    )
    assert tag.function_weights is None

    tag_weighted = ProbeEticTag(
        primary_function="epistemic_authority",
        function_weights={"epistemic_authority": 0.7, "power_legitimation": 0.3},
    )
    assert tag_weighted.function_weights["epistemic_authority"] == 0.7


def test_probe_emic_tag():
    """Tests ProbeEmicTag construction."""
    tag = ProbeEmicTag(
        emic_designation="Tagesschau",
        emic_context="State-funded public broadcaster (ARD).",
        emic_language="de",
    )
    assert tag.emic_language == "de"


# ---------------------------------------------------------------------------
# RssAdapter discourse context propagation tests
# ---------------------------------------------------------------------------


def test_rss_adapter_no_pg_pool():
    """Tests that RssAdapter without pg_pool produces None discourse_context."""
    adapter = RssAdapter()
    raw = {
        "source": "tagesschau",
        "source_type": "rss",
        "raw_text": "Some text",
        "url": "https://example.com",
    }
    event_time = datetime(2026, 4, 11, 10, 0, 0, tzinfo=timezone.utc)
    core, meta = adapter.harmonize(raw, event_time, "rss/tagesschau/test.json")

    assert meta.discourse_context is None


def test_rss_adapter_with_classification():
    """Tests that RssAdapter populates DiscourseContext from classification."""
    mock_pool = MagicMock()

    with patch("internal.adapters.rss.get_source_classification") as mock_get:
        mock_get.return_value = {
            "primary_function": "epistemic_authority",
            "secondary_function": "power_legitimation",
            "emic_designation": "Tagesschau",
        }

        adapter = RssAdapter(pg_pool=mock_pool)
        raw = {
            "source": "tagesschau",
            "source_type": "rss",
            "raw_text": "Some text",
            "url": "https://example.com",
        }
        event_time = datetime(2026, 4, 11, 10, 0, 0, tzinfo=timezone.utc)
        core, meta = adapter.harmonize(raw, event_time, "rss/tagesschau/test.json")

        assert meta.discourse_context is not None
        assert meta.discourse_context.primary_function == "epistemic_authority"
        assert meta.discourse_context.secondary_function == "power_legitimation"
        assert meta.discourse_context.emic_designation == "Tagesschau"
        mock_get.assert_called_once_with(mock_pool, "tagesschau")


def test_rss_adapter_no_classification_found():
    """Tests that RssAdapter handles missing classification gracefully."""
    mock_pool = MagicMock()

    with patch("internal.adapters.rss.get_source_classification") as mock_get:
        mock_get.return_value = None

        adapter = RssAdapter(pg_pool=mock_pool)
        raw = {
            "source": "unknown-source",
            "source_type": "rss",
            "raw_text": "Some text",
            "url": "https://example.com",
        }
        event_time = datetime(2026, 4, 11, 10, 0, 0, tzinfo=timezone.utc)
        core, meta = adapter.harmonize(raw, event_time, "rss/unknown/test.json")

        assert meta.discourse_context is None


def test_rss_adapter_classification_query_failure():
    """Tests that RssAdapter handles database errors gracefully."""
    mock_pool = MagicMock()

    with patch("internal.adapters.rss.get_source_classification") as mock_get:
        mock_get.side_effect = Exception("Connection refused")

        adapter = RssAdapter(pg_pool=mock_pool)
        raw = {
            "source": "tagesschau",
            "source_type": "rss",
            "raw_text": "Some text",
            "url": "https://example.com",
        }
        event_time = datetime(2026, 4, 11, 10, 0, 0, tzinfo=timezone.utc)
        core, meta = adapter.harmonize(raw, event_time, "rss/tagesschau/test.json")

        # Pipeline continues without failure
        assert meta.discourse_context is None
        assert core.source == "tagesschau"


# ---------------------------------------------------------------------------
# Processor discourse_function propagation tests
# ---------------------------------------------------------------------------


def test_processor_writes_discourse_function_to_metrics(
    mock_minio, mock_clickhouse, mock_pg_pool, dummy_span
):
    """Tests that processor includes discourse_function in ClickHouse metric inserts."""
    mock_pool = MagicMock()

    with patch("internal.adapters.rss.get_source_classification") as mock_get:
        mock_get.return_value = {
            "primary_function": "epistemic_authority",
            "secondary_function": "power_legitimation",
            "emic_designation": "Tagesschau",
        }

        adapter = RssAdapter(pg_pool=mock_pool)
        registry = AdapterRegistry({"legacy": LegacyAdapter(), "rss": adapter})
        extractors = [WordCountExtractor()]
        processor = DataProcessor(mock_minio, mock_clickhouse, mock_pg_pool, registry, extractors)

        bronze_data = json.dumps({
            "source": "tagesschau",
            "source_type": "rss",
            "raw_text": "Die Bundesregierung hat am Mittwoch ein Paket verabschiedet.",
            "url": "https://example.com/article",
        }).encode("utf-8")

        mock_response = MagicMock()
        mock_response.read.return_value = bronze_data
        mock_minio.get_object.return_value = mock_response
        processor._get_document_status = MagicMock(return_value=None)
        processor._update_document_status = MagicMock()

        processor.process_event("rss/tagesschau/test.json", DUMMY_EVENT_TIME, dummy_span)

        # Verify ClickHouse insert includes discourse_function
        insert_call = mock_clickhouse.insert.call_args
        assert insert_call is not None
        _, kwargs = insert_call
        assert "discourse_function" in kwargs["column_names"]
        # The row should have discourse_function as the last element
        row = insert_call[0][1][0]  # First positional arg (rows), first row
        assert row[-1] == "epistemic_authority"


def test_processor_empty_discourse_function_without_classification(
    mock_minio, mock_clickhouse, mock_pg_pool, dummy_span
):
    """Tests that processor writes empty discourse_function when no classification exists."""
    adapter = RssAdapter()  # No pg_pool
    registry = AdapterRegistry({"legacy": LegacyAdapter(), "rss": adapter})
    extractors = [WordCountExtractor()]
    processor = DataProcessor(mock_minio, mock_clickhouse, mock_pg_pool, registry, extractors)

    bronze_data = json.dumps({
        "source": "tagesschau",
        "source_type": "rss",
        "raw_text": "Some German text content here.",
        "url": "https://example.com/article",
    }).encode("utf-8")

    mock_response = MagicMock()
    mock_response.read.return_value = bronze_data
    mock_minio.get_object.return_value = mock_response
    processor._get_document_status = MagicMock(return_value=None)
    processor._update_document_status = MagicMock()

    processor.process_event("rss/tagesschau/test.json", DUMMY_EVENT_TIME, dummy_span)

    insert_call = mock_clickhouse.insert.call_args
    assert insert_call is not None
    _, kwargs = insert_call
    assert "discourse_function" in kwargs["column_names"]
    row = insert_call[0][1][0]
    assert row[-1] == ""


def test_discourse_context_serialization_in_silver():
    """Tests that DiscourseContext serializes correctly in RssMeta."""
    meta = RssMeta(
        source_type="rss",
        feed_url="https://example.com/feed",
        discourse_context=DiscourseContext(
            primary_function="epistemic_authority",
            secondary_function="power_legitimation",
            emic_designation="Tagesschau",
        ),
    )

    data = meta.model_dump()
    assert data["discourse_context"]["primary_function"] == "epistemic_authority"
    assert data["discourse_context"]["emic_designation"] == "Tagesschau"


def test_rss_meta_without_discourse_context():
    """Tests that RssMeta serializes correctly without discourse context."""
    meta = RssMeta(
        source_type="rss",
        feed_url="https://example.com/feed",
    )

    data = meta.model_dump()
    assert data["discourse_context"] is None
