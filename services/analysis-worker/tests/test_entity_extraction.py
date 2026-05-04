import pytest
from datetime import datetime, timezone
from unittest.mock import MagicMock
from internal.models import SilverCore
from internal.extractors import (
    WordCountExtractor, GoldEntity, MetricExtractor, ExtractionResult, GoldMetric,
    NamedEntityExtractor,
)
from conftest import VALID_BRONZE_DATA, DUMMY_EVENT_TIME, _make_processor, gold_insert_calls


# ---------------------------------------------------------------------------
# StubEntityExtractor — produces both metrics and entities
# ---------------------------------------------------------------------------

class StubEntityExtractor:
    """A test extractor that produces both metrics and entities."""

    @property
    def name(self) -> str:
        return "stub_entity"

    def extract_all(self, core, article_id: str | None) -> ExtractionResult:
        return ExtractionResult(
            metrics=[
                GoldMetric(
                    timestamp=core.timestamp,
                    value=2.0,
                    source=core.source,
                    metric_name="entity_count",
                    article_id=article_id,
                )
            ],
            entities=[
                GoldEntity(
                    timestamp=core.timestamp,
                    source=core.source,
                    article_id=article_id,
                    entity_text="Berlin",
                    entity_label="LOC",
                    start_char=0,
                    end_char=6,
                ),
                GoldEntity(
                    timestamp=core.timestamp,
                    source=core.source,
                    article_id=article_id,
                    entity_text="Merkel",
                    entity_label="PER",
                    start_char=10,
                    end_char=16,
                ),
            ],
        )


# ---------------------------------------------------------------------------
# Protocol conformance
# ---------------------------------------------------------------------------

def test_named_entity_extractor_protocol_conformance():
    """Tests that NamedEntityExtractor satisfies the MetricExtractor protocol."""
    extractor = NamedEntityExtractor()
    assert isinstance(extractor, MetricExtractor)
    assert extractor.name == "named_entity"


def test_named_entity_extractor_satisfies_metric_extractor():
    """Tests that NamedEntityExtractor satisfies the unified MetricExtractor protocol.

    Phase 118a (ADR-024): language→model routing comes from the Capability
    Manifest. Pointing the `de` block at a non-installed model exercises the
    OSError fallback path without requiring a real spaCy model on disk.
    """
    from internal.models.language_capability import (
        CapabilityManifest,
        LanguageCapability,
        NerCapability,
    )

    manifest = CapabilityManifest(
        manifest_version=1,
        languages={
            "de": LanguageCapability(
                iso_code="de",
                display_name="German",
                ner=NerCapability(tier=1.5, model="nonexistent_model_for_test", model_version="0"),
            ),
        },
    )
    extractor = NamedEntityExtractor(manifest=manifest)
    assert isinstance(extractor, MetricExtractor)
    assert extractor.name == "named_entity"


# ---------------------------------------------------------------------------
# NER extraction
# ---------------------------------------------------------------------------

def test_named_entity_extractor_german_text():
    """
    Tests NER on a German sentence with known entities.
    Note: exact entities depend on the spaCy model version.
    """
    extractor = NamedEntityExtractor()
    if not extractor._nlp_by_language:
        pytest.skip("spaCy de_core_news_lg not installed")

    core = SilverCore(
        document_id="abc123",
        source="tagesschau",
        source_type="rss",
        raw_text="Bundeskanzler Olaf Scholz traf sich in Berlin mit dem französischen Präsidenten Emmanuel Macron.",
        cleaned_text="Bundeskanzler Olaf Scholz traf sich in Berlin mit dem französischen Präsidenten Emmanuel Macron.",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=13,
    )

    result = extractor.extract_all(core, "article-1")
    metrics = result.metrics
    entities = result.entities

    assert len(metrics) == 1
    assert metrics[0].metric_name == "entity_count"
    assert metrics[0].value >= 1
    assert metrics[0].source == "tagesschau"

    assert len(entities) >= 1
    assert all(isinstance(e, GoldEntity) for e in entities)

    for entity in entities:
        assert entity.entity_text
        assert entity.entity_label in ("PER", "ORG", "LOC", "MISC")
        assert entity.start_char >= 0
        assert entity.end_char > entity.start_char
        assert entity.source == "tagesschau"
        assert entity.article_id == "article-1"


def test_named_entity_extractor_empty_text():
    """Tests that empty text returns no entities and no metrics."""
    extractor = NamedEntityExtractor()
    if not extractor._nlp_by_language:
        pytest.skip("spaCy de_core_news_lg not installed")

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
    assert result.entities == []


def test_named_entity_extractor_entity_count_matches():
    """Tests that entity_count metric matches the number of extracted entities."""
    extractor = NamedEntityExtractor()
    if not extractor._nlp_by_language:
        pytest.skip("spaCy de_core_news_lg not installed")

    core = SilverCore(
        document_id="abc123",
        source="test",
        source_type="rss",
        raw_text="Angela Merkel und Emmanuel Macron trafen sich in Paris.",
        cleaned_text="Angela Merkel und Emmanuel Macron trafen sich in Paris.",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=8,
    )

    result = extractor.extract_all(core, None)
    metrics = result.metrics
    entities = result.entities

    entity_count_metric = metrics[0]
    assert entity_count_metric.value == float(len(entities))


# ---------------------------------------------------------------------------
# Entity insertion via processor
# ---------------------------------------------------------------------------

def test_processor_inserts_entities(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span):
    """
    Tests that the processor inserts entities into aer_gold.entities
    when an extractor returns non-empty entities in its ExtractionResult.
    """
    extractors = [WordCountExtractor(), StubEntityExtractor()]
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("test-source/test-article/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    # Two inserts: one for metrics, one for entities
    assert len(gold_insert_calls(mock_clickhouse)) == 2

    metrics_call = gold_insert_calls(mock_clickhouse)[0]
    assert metrics_call[0][0] == "aer_gold.metrics"
    assert len(metrics_call[0][1]) == 2  # word_count + entity_count

    entities_call = gold_insert_calls(mock_clickhouse)[1]
    assert entities_call[0][0] == "aer_gold.entities"
    entity_rows = entities_call[0][1]
    assert len(entity_rows) == 2
    assert entity_rows[0][3] == "Berlin"
    assert entity_rows[0][4] == "LOC"
    assert entity_rows[1][3] == "Merkel"
    assert entity_rows[1][4] == "PER"
    assert entities_call[1]["column_names"] == [
        "timestamp", "source", "article_id", "entity_text", "entity_label", "start_char", "end_char", "discourse_function", "ingestion_version"
    ]


def test_processor_no_entity_insert_without_entity_extractor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, dummy_span):
    """
    Tests that extractors returning empty entities in their ExtractionResult
    do not trigger entity insertion — only the metrics insert happens.
    """
    extractors = [WordCountExtractor()]
    proc = _make_processor(mock_minio, mock_clickhouse, mock_pg_pool, adapter_registry, extractors)

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    proc.process_event("test-source/test-article/2023-10-25.json", DUMMY_EVENT_TIME, dummy_span)

    assert len(gold_insert_calls(mock_clickhouse)) == 1
    assert gold_insert_calls(mock_clickhouse)[0][0][0] == "aer_gold.metrics"


# ── Entity filter guards ─────────────────────────────────────────────────────

def test_is_valid_entity_rejects_long_text():
    """Sentence-length spans must be rejected regardless of label."""
    long = "And we must stay focused on achieving our common goals in difficult times"
    assert not NamedEntityExtractor._is_valid_entity(long)


def test_is_valid_entity_rejects_too_many_words():
    """More than _MAX_ENTITY_WORDS words → not a named entity."""
    seven_words = "one two three four five six seven"
    assert not NamedEntityExtractor._is_valid_entity(seven_words)


def test_is_valid_entity_accepts_short_phrases():
    """Normal named entities must pass the filter."""
    assert NamedEntityExtractor._is_valid_entity("Friedrich Merz")
    assert NamedEntityExtractor._is_valid_entity("Berlin")
    assert NamedEntityExtractor._is_valid_entity("CDU/CSU")
    assert NamedEntityExtractor._is_valid_entity(
        "Bundesministerium für wirtschaftliche Zusammenarbeit und Entwicklung"
    )


def test_is_valid_entity_rejects_empty():
    assert not NamedEntityExtractor._is_valid_entity("")
    assert not NamedEntityExtractor._is_valid_entity("   ")


def test_ner_skips_non_german_language():
    """NER must not run when core.language is a non-German ISO code."""
    extractor = NamedEntityExtractor()
    if not extractor._nlp_by_language:
        pytest.skip("spaCy de_core_news_lg not installed")

    core = SilverCore(
        document_id="en-doc",
        source="test",
        source_type="rss",
        raw_text="Prime Minister Keir Starmer met with President Biden in Washington.",
        cleaned_text="Prime Minister Keir Starmer met with President Biden in Washington.",
        language="en",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=11,
    )

    result = extractor.extract_all(core, "en-article-1")
    assert result.metrics == []
    assert result.entities == []


def test_ner_runs_on_undetermined_language():
    """language='und' must not suppress NER — it means detection did not run."""
    extractor = NamedEntityExtractor()
    if not extractor._nlp_by_language:
        pytest.skip("spaCy de_core_news_lg not installed")

    core = SilverCore(
        document_id="und-doc",
        source="tagesschau",
        source_type="rss",
        raw_text="Bundeskanzler Olaf Scholz sprach in Berlin.",
        cleaned_text="Bundeskanzler Olaf Scholz sprach in Berlin.",
        language="und",
        timestamp=datetime(2026, 4, 5, 10, 0, 0, tzinfo=timezone.utc),
        word_count=7,
    )

    result = extractor.extract_all(core, "und-article-1")
    assert len(result.entities) >= 1
