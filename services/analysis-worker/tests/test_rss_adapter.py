"""Phase 76 test coverage — RssAdapter data quality invariants.

Covers:
- `core.language` must be the ISO 639-3 sentinel "und" (undetermined).
  The `LanguageDetectionExtractor` is SSoT for detected language.
- Source-classification lookups are cached per-source with a TTL,
  eliminating the N+1 query pattern on bulk ingestion.
"""

from datetime import datetime, timezone
from unittest.mock import MagicMock, patch

from internal.adapters.rss import RssAdapter


def _rss_raw(source: str = "tagesschau") -> dict:
    return {
        "source": source,
        "source_type": "rss",
        "raw_text": "Die Bundesregierung hat einen Beschluss gefasst.",
        "url": "https://example.com/article",
    }


def _event_time() -> datetime:
    return datetime(2026, 4, 13, 10, 0, 0, tzinfo=timezone.utc)


def test_rss_adapter_language_is_undetermined_sentinel():
    """RssAdapter must not hardcode 'de'; LanguageDetectionExtractor is SSoT."""
    adapter = RssAdapter()
    core, _ = adapter.harmonize(_rss_raw(), _event_time(), "rss/tagesschau/test.json")
    assert core.language == "und"


def test_rss_adapter_caches_classification_within_ttl():
    """Repeated harmonize calls for the same source must hit the DB only once."""
    mock_pool = MagicMock()
    with patch("internal.adapters.rss.get_source_classification") as mock_get:
        mock_get.return_value = {
            "primary_function": "epistemic_authority",
            "secondary_function": None,
            "emic_designation": "Tagesschau",
        }
        adapter = RssAdapter(pg_pool=mock_pool)

        for i in range(5):
            adapter.harmonize(
                _rss_raw(), _event_time(), f"rss/tagesschau/{i}.json"
            )

        assert mock_get.call_count == 1


def test_rss_adapter_cache_is_per_source():
    """Different sources get independent cache entries."""
    mock_pool = MagicMock()
    with patch("internal.adapters.rss.get_source_classification") as mock_get:
        mock_get.return_value = None
        adapter = RssAdapter(pg_pool=mock_pool)

        adapter.harmonize(_rss_raw("a"), _event_time(), "rss/a/1.json")
        adapter.harmonize(_rss_raw("b"), _event_time(), "rss/b/1.json")
        adapter.harmonize(_rss_raw("a"), _event_time(), "rss/a/2.json")
        adapter.harmonize(_rss_raw("b"), _event_time(), "rss/b/2.json")

        assert mock_get.call_count == 2


def test_rss_adapter_cache_expires_after_ttl():
    """Past TTL, the adapter re-queries."""
    mock_pool = MagicMock()
    with patch("internal.adapters.rss.get_source_classification") as mock_get, \
            patch("internal.adapters.rss.time.monotonic") as mock_time:
        mock_get.return_value = None
        mock_time.side_effect = [0.0, 30.0, 120.0]
        adapter = RssAdapter(pg_pool=mock_pool)

        adapter.harmonize(_rss_raw(), _event_time(), "rss/t/1.json")
        adapter.harmonize(_rss_raw(), _event_time(), "rss/t/2.json")
        adapter.harmonize(_rss_raw(), _event_time(), "rss/t/3.json")

        assert mock_get.call_count == 2
