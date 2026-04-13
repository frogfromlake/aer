from datetime import datetime, timezone
from unittest.mock import MagicMock
from internal.models import SilverCore
from internal.extractors import (
    WordCountExtractor, MetricExtractor, LanguageDetectionExtractor, GoldLanguageDetection,
)
from conftest import VALID_BRONZE_DATA, VALID_RSS_BRONZE_DATA, DUMMY_EVENT_TIME, _make_processor


# ---------------------------------------------------------------------------
# Protocol conformance
# ---------------------------------------------------------------------------

def test_language_extractor_protocol_conformance():
    """Tests that LanguageDetectionExtractor satisfies the MetricExtractor protocol."""
    extractor = LanguageDetectionExtractor()
    assert isinstance(extractor, MetricExtractor)
    assert extractor.name == "language_detection"


def test_language_extractor_satisfies_metric_extractor():
    """Tests that LanguageDetectionExtractor satisfies the unified MetricExtractor protocol."""
    extractor = LanguageDetectionExtractor()
    assert isinstance(extractor, MetricExtractor)
    assert extractor.name == "language_detection"


# ---------------------------------------------------------------------------
# Single-extractor language detection behaviour
# ---------------------------------------------------------------------------

def test_language_extractor_german_text():
    """Tests that German text produces a language_confidence metric in [0, 1]."""
    extractor = LanguageDetectionExtractor()
    core = SilverCore(
        document_id="abc123",
        source="tagesschau",
        source_type="rss",
        raw_text="Die Bundesregierung hat am Mittwoch ein umfassendes Maßnahmenpaket zum Klimaschutz verabschiedet.",
        cleaned_text="Die Bundesregierung hat am Mittwoch ein umfassendes Maßnahmenpaket zum Klimaschutz verabschiedet.",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=11,
    )

    result = extractor.extract_all(core, "article-1")
    metrics = result.metrics
    assert len(metrics) == 1
    assert metrics[0].metric_name == "language_confidence"
    assert 0.0 <= metrics[0].value <= 1.0
    assert metrics[0].source == "tagesschau"


def test_language_extractor_short_text_returns_empty():
    """Tests that very short text (<10 chars) returns no metrics."""
    extractor = LanguageDetectionExtractor()
    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="Hi",
        cleaned_text="Hi",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=1,
    )

    result = extractor.extract_all(core, None)
    assert result.metrics == []


def test_language_extractor_empty_text_returns_empty():
    """Tests that empty cleaned_text returns no metrics."""
    extractor = LanguageDetectionExtractor()
    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="placeholder",
        cleaned_text="",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=0,
    )

    result = extractor.extract_all(core, None)
    assert result.metrics == []


def test_language_extractor_deterministic():
    """Tests that the same input produces the same confidence score (fixed seed)."""
    extractor = LanguageDetectionExtractor()
    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="Die Bundesregierung hat ein neues Gesetz beschlossen über den Klimaschutz.",
        cleaned_text="Die Bundesregierung hat ein neues Gesetz beschlossen über den Klimaschutz.",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=11,
    )

    results = [extractor.extract_all(core, None).metrics[0].value for _ in range(5)]
    assert len(set(results)) == 1  # all identical


# ---------------------------------------------------------------------------
# Language detection persistence (ExtractionResult.language_detections)
# ---------------------------------------------------------------------------

def test_language_extractor_extract_all_german_text():
    """Tests that extract_all() returns an ExtractionResult with metrics and language_detections."""
    extractor = LanguageDetectionExtractor()
    core = SilverCore(
        document_id="abc123",
        source="tagesschau",
        source_type="rss",
        raw_text="Die Bundesregierung hat am Mittwoch ein umfassendes Maßnahmenpaket zum Klimaschutz verabschiedet.",
        cleaned_text="Die Bundesregierung hat am Mittwoch ein umfassendes Maßnahmenpaket zum Klimaschutz verabschiedet.",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=11,
    )

    result = extractor.extract_all(core, "article-1")
    metrics = result.metrics
    detections = result.language_detections

    assert len(metrics) == 1
    assert metrics[0].metric_name == "language_confidence"
    assert 0.0 <= metrics[0].value <= 1.0

    assert len(detections) >= 1
    assert all(isinstance(d, GoldLanguageDetection) for d in detections)

    assert detections[0].rank == 1
    assert detections[0].detected_language
    assert 0.0 <= detections[0].confidence <= 1.0
    assert detections[0].source == "tagesschau"
    assert detections[0].article_id == "article-1"

    for i, d in enumerate(detections):
        assert d.rank == i + 1


def test_language_extractor_extract_all_short_text():
    """Tests that extract_all() returns empty ExtractionResult for short text."""
    extractor = LanguageDetectionExtractor()
    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="Hi",
        cleaned_text="Hi",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=1,
    )

    result = extractor.extract_all(core, None)
    assert result.metrics == []
    assert result.language_detections == []


def test_language_extractor_language_detections_via_extract_all():
    """Tests that language_detections are accessible via extract_all().language_detections."""
    extractor = LanguageDetectionExtractor()
    core = SilverCore(
        document_id="abc123",
        source="tagesschau",
        source_type="rss",
        raw_text="Die Bundesregierung hat am Mittwoch ein umfassendes Maßnahmenpaket zum Klimaschutz verabschiedet.",
        cleaned_text="Die Bundesregierung hat am Mittwoch ein umfassendes Maßnahmenpaket zum Klimaschutz verabschiedet.",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=11,
    )

    result = extractor.extract_all(core, "article-1")
    detections = result.language_detections
    assert len(detections) >= 1
    assert all(isinstance(d, GoldLanguageDetection) for d in detections)


# ---------------------------------------------------------------------------
# Language detection persistence via processor
# ---------------------------------------------------------------------------

def test_processor_language_detections_insert(
    mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span
):
    """
    Tests that the processor inserts language detections into
    aer_gold.language_detections when LanguageDetectionExtractor is used.
    """
    extractors = [LanguageDetectionExtractor()]
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_RSS_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("rss/tagesschau/abc123/2026-04-05.json", "2026-04-05T10:00:00.000Z", dummy_span)

    assert mock_clickhouse.insert.call_count == 2

    insert_calls = mock_clickhouse.insert.call_args_list
    tables = [call[0][0] for call in insert_calls]
    assert "aer_gold.metrics" in tables
    assert "aer_gold.language_detections" in tables

    lang_call = [c for c in insert_calls if c[0][0] == "aer_gold.language_detections"][0]
    lang_rows = lang_call[0][1]
    assert len(lang_rows) >= 1
    assert lang_call[1]['column_names'] == ['timestamp', 'source', 'article_id', 'detected_language', 'confidence', 'rank', 'ingestion_version']

    assert lang_rows[0][5] == 1  # rank column


def test_processor_no_language_detection_insert_without_language_extractor(
    mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span
):
    """
    Tests that extractors not producing language_detections (empty list in
    ExtractionResult) do not trigger a language_detections ClickHouse insert.
    """
    extractors = [WordCountExtractor()]
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("test-source/test-article/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    mock_clickhouse.insert.assert_called_once()
    assert mock_clickhouse.insert.call_args[0][0] == "aer_gold.metrics"
