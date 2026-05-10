"""Phase 122e A17 / F-A17 — probe language-scope quarantine filter.

Verifies that documents whose detected language falls outside the
source's allow-list are quarantined before the Silver write, while
in-scope documents continue through the normal pipeline.
"""

from unittest.mock import MagicMock

from internal.adapters import AdapterRegistry, RssAdapter
from internal.extractors import LanguageDetectionExtractor, WordCountExtractor
from internal.models.probe_scope import ProbeLanguageScope
from internal.processor import DataProcessor

from conftest import VALID_RSS_BRONZE_DATA, gold_insert_calls


def _make_scoped_processor(mock_minio, mock_clickhouse, mock_pg_pool, allow_map):
    """Build a DataProcessor with an explicit ProbeLanguageScope."""
    adapter_registry = AdapterRegistry({"rss": RssAdapter(pg_pool=mock_pg_pool)})
    scope = ProbeLanguageScope(allow_map)
    return DataProcessor(
        mock_minio,
        mock_clickhouse,
        mock_pg_pool,
        adapter_registry,
        [LanguageDetectionExtractor(), WordCountExtractor()],
        language_scope=scope,
    )


def test_in_scope_document_processes_normally(
    mock_minio, mock_clickhouse, mock_pg_pool, dummy_span
):
    """German doc from a German-only source is not quarantined."""
    proc = _make_scoped_processor(
        mock_minio, mock_clickhouse, mock_pg_pool, {"tagesschau": ["de"]}
    )
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_RSS_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response

    proc.process_event("rss/tagesschau/2026-04-05.json", "2026-04-05T10:00:00.000Z", dummy_span)

    proc._update_document_status.assert_called_with(
        "rss/tagesschau/2026-04-05.json", "processed"
    )
    # Gold inserts happened (language_detections + word_count metric).
    assert len(gold_insert_calls(mock_clickhouse)) >= 1


def test_out_of_scope_document_quarantined_before_silver(
    mock_minio, mock_clickhouse, mock_pg_pool, dummy_span, monkeypatch
):
    """Source configured for [de] but detector returns 'fr' → quarantine, no Gold inserts."""
    # Force the language detector to return French regardless of input —
    # we are testing the scope filter, not the detector itself.

    class _ForceFrenchDetector(LanguageDetectionExtractor):
        def extract_all(self, core, article_id):
            from internal.extractors.base import (
                ExtractionResult,
                GoldLanguageDetection,
            )

            return ExtractionResult(
                metrics=[],
                entities=[],
                entity_links=[],
                language_detections=[
                    GoldLanguageDetection(
                        timestamp=core.timestamp,
                        source=core.source,
                        article_id=article_id,
                        detected_language="fr",
                        confidence=0.99,
                        rank=1,
                    )
                ],
            )

    adapter_registry = AdapterRegistry({"rss": RssAdapter(pg_pool=mock_pg_pool)})
    scope = ProbeLanguageScope({"tagesschau": ["de"]})
    proc = DataProcessor(
        mock_minio,
        mock_clickhouse,
        mock_pg_pool,
        adapter_registry,
        [_ForceFrenchDetector()],
        language_scope=scope,
    )

    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_RSS_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response

    proc.process_event("rss/tagesschau/2026-04-05.json", "2026-04-05T10:00:00.000Z", dummy_span)

    # Quarantine path took over — Silver upload was never reached.
    proc._update_document_status.assert_called_with(
        "rss/tagesschau/2026-04-05.json", "quarantined"
    )
    assert mock_minio.put_object.call_args_list == [] or not any(
        c.args[0] == "silver" for c in mock_minio.put_object.call_args_list
    )
    # No Gold inserts — quarantined before extraction.
    assert len(gold_insert_calls(mock_clickhouse)) == 0


def test_source_with_no_scope_entry_is_not_filtered(
    mock_minio, mock_clickhouse, mock_pg_pool, dummy_span
):
    """A source without an entry in the scope config is treated as in-scope."""
    proc = _make_scoped_processor(
        mock_minio, mock_clickhouse, mock_pg_pool, {"someothersource": ["en"]}
    )
    proc._get_document_status = MagicMock(return_value=None)
    proc._update_document_status = MagicMock()

    mock_response = MagicMock()
    mock_response.read.return_value = VALID_RSS_BRONZE_DATA
    mock_minio.get_object.return_value = mock_response

    proc.process_event("rss/tagesschau/2026-04-05.json", "2026-04-05T10:00:00.000Z", dummy_span)

    # tagesschau has no entry → no scope filter applies → processed.
    proc._update_document_status.assert_called_with(
        "rss/tagesschau/2026-04-05.json", "processed"
    )


def test_probe_language_scope_loader_rejects_malformed_entries(tmp_path):
    """Loader rejects non-list values for sources.<name>."""
    import yaml
    import pytest

    bad = tmp_path / "bad.yaml"
    bad.write_text(yaml.safe_dump({"sources": {"tagesschau": "de"}}))
    with pytest.raises(ValueError):
        ProbeLanguageScope.load(bad)
