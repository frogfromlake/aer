import json
from pathlib import Path
from unittest.mock import MagicMock
from internal.extractors import (
    WordCountExtractor, TemporalDistributionExtractor,
    LanguageDetectionExtractor, SentimentExtractor,
)
from conftest import VALID_RSS_BRONZE_DATA, _make_processor


def test_rss_adapter_happy_path(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span):
    """
    Tests end-to-end processing of an RSS document through the pipeline:
    RSS adapter → SilverCore + RssMeta → Silver upload → Gold insert.
    """
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, [WordCountExtractor()])

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_RSS_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response

    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    obj_key = "rss/tagesschau/abc123/2026-04-05.json"
    proc.process_event(obj_key, "2026-04-05T10:00:00.000Z", dummy_span)

    mock_minio.put_object.assert_called_once()
    args, _ = mock_minio.put_object.call_args
    assert args[0] == "silver"
    assert args[1] == obj_key

    silver_buffer = args[2]
    silver_buffer.seek(0)
    silver_data = json.loads(silver_buffer.read().decode('utf-8'))

    core = silver_data["core"]
    assert core["source_type"] == "rss"
    assert core["source"] == "tagesschau"
    assert core["schema_version"] == 2
    assert core["language"] == "de"
    assert core["url"] == "https://www.tagesschau.de/inland/klimaschutz-2026"
    assert core["raw_text"] == "Die Bundesregierung hat am Mittwoch ein umfassendes Maßnahmenpaket zum Klimaschutz verabschiedet."
    assert core["cleaned_text"] == core["raw_text"]
    assert core["word_count"] == 11
    assert len(core["document_id"]) == 64

    meta = silver_data["meta"]
    assert meta is not None
    assert meta["source_type"] == "rss"
    assert meta["feed_url"] == "https://www.tagesschau.de/index~rss2.xml"
    assert meta["categories"] == ["Klimaschutz", "Umwelt"]
    assert meta["author"] == "tagesschau.de"
    assert meta["feed_title"] == "tagesschau.de - Die Nachrichten der ARD"

    mock_clickhouse.insert.assert_called_once()
    ch_args, _ = mock_clickhouse.insert.call_args
    row = ch_args[1][0]
    assert row[1] == 11.0
    assert row[2] == "tagesschau"
    assert row[3] == "word_count"

    proc._update_document_status.assert_called_with(obj_key, "processed")


def test_rss_adapter_whitespace_normalization(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span):
    """Tests that RSS raw_text with irregular whitespace is properly normalized."""
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, [WordCountExtractor()])

    rss_data = json.dumps({
        "source": "bundesregierung",
        "source_type": "rss",
        "title": "Whitespace Test",
        "raw_text": "  Viel   unnötiger   Leerraum   hier  ",
        "url": "https://example.gov.de/test",
        "timestamp": "2026-04-05T10:00:00Z",
        "feed_url": "https://example.gov.de/feed.rss",
    }).encode('utf-8')

    mock_response = MagicMock()
    mock_response.read.return_value = rss_data
    mock_minio.get_object.return_value = mock_response

    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("rss/bundesregierung/xyz/2026-04-05.json", "2026-04-05T10:00:00.000Z", dummy_span)

    args, _ = mock_minio.put_object.call_args
    silver_buffer = args[2]
    silver_buffer.seek(0)
    silver_data = json.loads(silver_buffer.read().decode('utf-8'))

    assert silver_data["core"]["raw_text"] == "  Viel   unnötiger   Leerraum   hier  "
    assert silver_data["core"]["cleaned_text"] == "Viel unnötiger Leerraum hier"
    assert silver_data["core"]["word_count"] == 4


def test_full_extractor_pipeline_with_all_tier1(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span):
    """
    Integration test: process a German RSS document through the full extractor
    pipeline with all Tier 1 extractors (word_count, temporal, language, sentiment).
    Entity extractor excluded (requires spaCy model).
    Sentiment uses empty lexicon (no SentiWS files in test env).
    """
    extractors = [
        WordCountExtractor(),
        TemporalDistributionExtractor(),
        LanguageDetectionExtractor(),
        SentimentExtractor(sentiws_dir=Path("/nonexistent")),
    ]
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_RSS_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("rss/tagesschau/abc123/2026-04-05.json", "2026-04-05T10:00:00.000Z", dummy_span)

    # Two inserts: metrics + language_detections
    assert mock_clickhouse.insert.call_count == 2

    insert_calls = mock_clickhouse.insert.call_args_list
    metrics_call = [c for c in insert_calls if c[0][0] == "aer_gold.metrics"][0]
    rows = metrics_call[0][1]

    metric_names = [row[3] for row in rows]
    assert "word_count" in metric_names
    assert "publication_hour" in metric_names
    assert "publication_weekday" in metric_names
    assert "language_confidence" in metric_names
    assert "sentiment_score" not in metric_names  # no lexicon loaded

    for row in rows:
        name, value = row[3], row[1]
        if name == "publication_hour":
            assert 0.0 <= value <= 23.0
        elif name == "publication_weekday":
            assert 0.0 <= value <= 6.0
        elif name == "language_confidence":
            assert 0.0 <= value <= 1.0

    lang_tables = [c[0][0] for c in insert_calls]
    assert "aer_gold.language_detections" in lang_tables

    proc._update_document_status.assert_called_with(
        "rss/tagesschau/abc123/2026-04-05.json", "processed"
    )


def test_full_pipeline_with_language_detection_persistence(
    mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span
):
    """
    Integration test: process a German RSS document through the pipeline
    with all Tier 1 extractors including language detection persistence.
    Verifies both metrics and language_detections inserts happen.
    """
    extractors = [
        WordCountExtractor(),
        TemporalDistributionExtractor(),
        LanguageDetectionExtractor(),
        SentimentExtractor(sentiws_dir=Path("/nonexistent")),
    ]
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

    metrics_call = [c for c in insert_calls if c[0][0] == "aer_gold.metrics"][0]
    metric_names = [row[3] for row in metrics_call[0][1]]
    assert "word_count" in metric_names
    assert "publication_hour" in metric_names
    assert "language_confidence" in metric_names
