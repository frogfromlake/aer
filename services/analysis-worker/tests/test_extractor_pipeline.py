from datetime import datetime, timezone
from pathlib import Path
from unittest.mock import MagicMock
from internal.models import SilverCore
from internal.extractors import (
    WordCountExtractor, MetricExtractor,
    TemporalDistributionExtractor, SentimentExtractor,
    NamedEntityExtractor, LanguageDetectionExtractor,
)
from conftest import (
    VALID_BRONZE_DATA, DUMMY_EVENT_TIME, EXPECTED_WORD_COUNT,
    StubExtractor, FailingExtractor, MalformedExtractor, _make_processor,
    gold_insert_calls,
)


# ---------------------------------------------------------------------------
# Basic pipeline behaviour
# ---------------------------------------------------------------------------

def test_extractor_pipeline_multiple_extractors(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span):
    """
    Tests that multiple extractors each contribute metrics and all are
    batch-inserted into ClickHouse in a single round-trip.
    """
    extractors = [WordCountExtractor(), StubExtractor("sentiment_score", 0.85)]
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("test-source/test-article/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    assert len(gold_insert_calls(mock_clickhouse)) == 1
    ch_args, ch_kwargs = gold_insert_calls(mock_clickhouse)[0]
    rows = ch_args[1]
    assert len(rows) == 2
    assert rows[0][3] == "word_count"
    assert rows[0][1] == float(EXPECTED_WORD_COUNT)
    assert rows[1][3] == "sentiment_score"
    assert rows[1][1] == 0.85


def test_extractor_pipeline_failing_extractor_does_not_block_others(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span):
    """
    Tests that a failing extractor is skipped while other extractors still
    produce their metrics. The document is NOT sent to the DLQ.
    """
    extractors = [FailingExtractor(), WordCountExtractor()]
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("test-source/test-article/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    assert len(gold_insert_calls(mock_clickhouse)) == 1
    ch_args, _ = gold_insert_calls(mock_clickhouse)[0]
    rows = ch_args[1]
    assert len(rows) == 1
    assert rows[0][3] == "word_count"

    proc._update_document_status.assert_called_with(
        "test-source/test-article/2023-10-25.json", "processed"
    )


def test_extractor_pipeline_no_extractors(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span):
    """
    Tests that the processor works with an empty extractor list.
    No ClickHouse insert occurs but the document is still processed.
    """
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, [])

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("test-source/test-article/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    assert len(gold_insert_calls(mock_clickhouse)) == 0
    proc._update_document_status.assert_called_with(
        "test-source/test-article/2023-10-25.json", "processed"
    )


def test_extractor_pipeline_all_extractors_fail(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span):
    """
    Tests that if all extractors fail, no ClickHouse insert occurs but the
    document is still marked as processed (partial extraction is acceptable).
    """
    extractors = [FailingExtractor()]
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("test-source/test-article/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    assert len(gold_insert_calls(mock_clickhouse)) == 0
    proc._update_document_status.assert_called_with(
        "test-source/test-article/2023-10-25.json", "processed"
    )


def test_extractor_registration_add_remove():
    """
    Tests that adding or removing an extractor from the list does not affect
    other extractors. Extractors are independent.
    """
    e1 = WordCountExtractor()
    e2 = StubExtractor("test_metric")

    pipeline = [e1, e2]
    assert len(pipeline) == 2
    assert pipeline[0].name == "word_count"
    assert pipeline[1].name == "test_metric"

    pipeline.remove(e2)
    assert len(pipeline) == 1
    assert pipeline[0].name == "word_count"


def test_extractor_missing_extract_all_is_skipped_gracefully(
    mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span
):
    """
    Tests that an extractor not implementing extract_all() is skipped gracefully.
    The AttributeError is caught by the per-extractor exception handler.
    Other extractors in the pipeline continue normally.
    """
    extractors = [MalformedExtractor(), WordCountExtractor()]
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("test-source/test-article/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    assert len(gold_insert_calls(mock_clickhouse)) == 1
    ch_args, _ = gold_insert_calls(mock_clickhouse)[0]
    rows = ch_args[1]
    assert len(rows) == 1
    assert rows[0][3] == "word_count"

    proc._update_document_status.assert_called_with(
        "test-source/test-article/2023-10-25.json", "processed"
    )


# ---------------------------------------------------------------------------
# Protocol conformance
# ---------------------------------------------------------------------------

def test_word_count_extractor_protocol_conformance():
    """Tests that WordCountExtractor satisfies the MetricExtractor protocol."""
    extractor = WordCountExtractor()
    assert isinstance(extractor, MetricExtractor)
    assert extractor.name == "word_count"


def test_stub_extractor_protocol_conformance():
    """Tests that the test StubExtractor satisfies the MetricExtractor protocol."""
    extractor = StubExtractor()
    assert isinstance(extractor, MetricExtractor)


def test_all_extractors_satisfy_metric_extractor_protocol():
    """Tests that all registered extractors satisfy the unified MetricExtractor protocol."""
    extractors = [
        WordCountExtractor(),
        TemporalDistributionExtractor(),
        LanguageDetectionExtractor(),
        SentimentExtractor(sentiws_dir=Path("/nonexistent")),
        NamedEntityExtractor(language_to_model={"de": "nonexistent_model_for_test"}),
    ]
    for extractor in extractors:
        assert isinstance(extractor, MetricExtractor), f"{extractor.name} does not satisfy MetricExtractor"


# ---------------------------------------------------------------------------
# Temporal Distribution Extractor
# ---------------------------------------------------------------------------

def test_temporal_extractor_protocol_conformance():
    """Tests that TemporalDistributionExtractor satisfies the MetricExtractor protocol."""
    extractor = TemporalDistributionExtractor()
    assert isinstance(extractor, MetricExtractor)
    assert extractor.name == "temporal_distribution"


def test_temporal_extractor_produces_hour_and_weekday():
    """Tests that temporal extractor produces both publication_hour and publication_weekday."""
    extractor = TemporalDistributionExtractor()
    # Wednesday 2026-04-01 at 14:30 UTC
    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="Test text",
        cleaned_text="Test text",
        timestamp=datetime(2026, 4, 1, 14, 30, 0, tzinfo=timezone.utc),
        word_count=2,
    )

    result = extractor.extract_all(core, "article-1")
    metrics = result.metrics
    assert len(metrics) == 2

    hour_metric = next(m for m in metrics if m.metric_name == "publication_hour")
    weekday_metric = next(m for m in metrics if m.metric_name == "publication_weekday")

    assert hour_metric.value == 14.0
    assert 0.0 <= hour_metric.value <= 23.0

    # 2026-04-01 is a Wednesday → weekday() = 2
    assert weekday_metric.value == 2.0
    assert 0.0 <= weekday_metric.value <= 6.0

    assert hour_metric.source == "test"
    assert hour_metric.article_id == "article-1"


def test_temporal_extractor_midnight_sunday():
    """Tests boundary values: midnight (hour=0) and Sunday (weekday=6)."""
    extractor = TemporalDistributionExtractor()
    # Sunday 2026-04-05 at 00:00 UTC
    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="Test",
        cleaned_text="Test",
        timestamp=datetime(2026, 4, 5, 0, 0, 0, tzinfo=timezone.utc),
        word_count=1,
    )

    result = extractor.extract_all(core, None)
    metrics = result.metrics
    hour_metric = next(m for m in metrics if m.metric_name == "publication_hour")
    weekday_metric = next(m for m in metrics if m.metric_name == "publication_weekday")

    assert hour_metric.value == 0.0
    assert weekday_metric.value == 6.0  # Sunday


def test_temporal_extractor_naive_datetime_returns_empty():
    """Defense-in-depth: naive datetime must produce an empty list, not wrong metrics."""
    extractor = TemporalDistributionExtractor()
    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="Test",
        cleaned_text="Test",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=1,
    )
    # Replace the timestamp with a naive one to exercise the extractor guard
    object.__setattr__(core, "timestamp", datetime(2026, 4, 5, 10, 0, 0))

    result = extractor.extract_all(core, "article-naive")
    assert result.metrics == []


# ---------------------------------------------------------------------------
# Sentiment Extractor
# ---------------------------------------------------------------------------

def test_sentiment_extractor_protocol_conformance():
    """Tests that SentimentExtractor satisfies the MetricExtractor protocol."""
    extractor = SentimentExtractor(sentiws_dir=Path("/nonexistent"))
    assert isinstance(extractor, MetricExtractor)
    assert extractor.name == "sentiment"


def test_sentiment_extractor_no_lexicon_returns_empty():
    """Tests that an extractor with no loaded lexicon produces no metrics."""
    extractor = SentimentExtractor(sentiws_dir=Path("/nonexistent"))
    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="Positive text here",
        cleaned_text="Positive text here",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=3,
    )

    result = extractor.extract_all(core, None)
    assert result.metrics == []


def test_sentiment_extractor_with_inline_lexicon(tmp_path):
    """
    Tests sentiment extraction with a minimal inline SentiWS lexicon.
    Creates temporary lexicon files to test the scoring algorithm.
    """
    pos_file = tmp_path / "SentiWS_v2.0_Positive.txt"
    neg_file = tmp_path / "SentiWS_v2.0_Negative.txt"

    pos_file.write_text("Glück|NN\t0.5765\tGlücks,Glückes\ngut|ADJX\t0.5040\tguten,guter,gutes\n", encoding="utf-8")
    neg_file.write_text("schlecht|ADJX\t-0.4771\tschlechten,schlechter\nKrise|NN\t-0.3544\tKrisen\n", encoding="utf-8")

    extractor = SentimentExtractor(sentiws_dir=tmp_path)
    assert len(extractor._lexicon) > 0

    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="Das ist gut und bringt Glück",
        cleaned_text="Das ist gut und bringt Glück",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=6,
    )

    result = extractor.extract_all(core, "article-1")
    metrics = result.metrics
    assert len(metrics) == 1
    assert metrics[0].metric_name == "sentiment_score_sentiws"  # Phase 117 rename
    assert all(m.metric_name != "lexicon_version" for m in metrics)

    # "gut" (0.504) + "glück" (0.5765) → mean = ~0.54
    assert -1.0 <= metrics[0].value <= 1.0
    assert metrics[0].value > 0  # positive text → positive score

    # Provenance is exposed via version_hash, not as a metric
    assert extractor.version_hash != "empty"
    assert len(extractor.version_hash) == 16


def test_sentiment_extractor_negative_text(tmp_path):
    """Tests that negative text produces a negative sentiment score."""
    pos_file = tmp_path / "SentiWS_v2.0_Positive.txt"
    neg_file = tmp_path / "SentiWS_v2.0_Negative.txt"

    pos_file.write_text("gut|ADJX\t0.5040\n", encoding="utf-8")
    neg_file.write_text("schlecht|ADJX\t-0.4771\tschlechten,schlechter\nKrise|NN\t-0.3544\tKrisen\n", encoding="utf-8")

    extractor = SentimentExtractor(sentiws_dir=tmp_path)

    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="Die Krise ist schlecht",
        cleaned_text="Die Krise ist schlecht",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=4,
    )

    result = extractor.extract_all(core, None)
    sentiment = next(m for m in result.metrics if m.metric_name == "sentiment_score_sentiws")
    assert sentiment.value < 0
    assert -1.0 <= sentiment.value <= 1.0


def test_sentiment_extractor_no_matches_returns_zero(tmp_path):
    """Tests that text with no lexicon matches returns sentiment_score = 0."""
    pos_file = tmp_path / "SentiWS_v2.0_Positive.txt"
    neg_file = tmp_path / "SentiWS_v2.0_Negative.txt"

    pos_file.write_text("gut|ADJX\t0.5040\n", encoding="utf-8")
    neg_file.write_text("schlecht|ADJX\t-0.4771\n", encoding="utf-8")

    extractor = SentimentExtractor(sentiws_dir=tmp_path)

    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="Ein neutraler Satz ohne Wertung",
        cleaned_text="Ein neutraler Satz ohne Wertung",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=5,
    )

    result = extractor.extract_all(core, None)
    sentiment = next(m for m in result.metrics if m.metric_name == "sentiment_score_sentiws")
    assert sentiment.value == 0.0


def test_sentiment_lexicon_hash_deterministic(tmp_path):
    """Tests that the lexicon hash is deterministic across loads."""
    pos_file = tmp_path / "SentiWS_v2.0_Positive.txt"
    neg_file = tmp_path / "SentiWS_v2.0_Negative.txt"
    pos_file.write_text("gut|ADJX\t0.5040\n", encoding="utf-8")
    neg_file.write_text("schlecht|ADJX\t-0.4771\n", encoding="utf-8")

    e1 = SentimentExtractor(sentiws_dir=tmp_path)
    e2 = SentimentExtractor(sentiws_dir=tmp_path)
    assert e1.lexicon_hash == e2.lexicon_hash
    assert e1.lexicon_hash != "empty"
