"""Phase 62 test coverage — DiscourseContext propagation in RssAdapter.

Split out from tests/test_discourse.py during Phase 72 (test coverage
retrofit). Covers the WP-001 Functional Probe Taxonomy propagation path:
from the source_classifications PostgreSQL table, through the RssAdapter
at harmonization time, into RssMeta.discourse_context.
"""

from datetime import datetime, timezone
from unittest.mock import MagicMock, patch

from internal.adapters.rss import RssAdapter, RssMeta
from internal.models.discourse import DiscourseContext, ProbeEticTag, ProbeEmicTag


# ---------------------------------------------------------------------------
# Pydantic model invariants
# ---------------------------------------------------------------------------


def test_discourse_context_construction():
    ctx = DiscourseContext(
        primary_function="epistemic_authority",
        secondary_function="power_legitimation",
        emic_designation="Tagesschau",
    )
    assert ctx.primary_function == "epistemic_authority"
    assert ctx.secondary_function == "power_legitimation"
    assert ctx.emic_designation == "Tagesschau"


def test_discourse_context_optional_secondary():
    ctx = DiscourseContext(
        primary_function="power_legitimation",
        emic_designation="Bundesregierung",
    )
    assert ctx.secondary_function is None


def test_probe_etic_tag_function_weights_default_none():
    """function_weights are intentionally optional — NULL in DB → None in model."""
    tag = ProbeEticTag(
        primary_function="epistemic_authority",
        secondary_function="power_legitimation",
    )
    assert tag.function_weights is None


def test_probe_etic_tag_with_weights():
    tag = ProbeEticTag(
        primary_function="epistemic_authority",
        function_weights={"epistemic_authority": 0.7, "power_legitimation": 0.3},
    )
    assert tag.function_weights["epistemic_authority"] == 0.7


def test_probe_emic_tag():
    tag = ProbeEmicTag(
        emic_designation="Tagesschau",
        emic_context="State-funded public broadcaster (ARD).",
        emic_language="de",
    )
    assert tag.emic_language == "de"


# ---------------------------------------------------------------------------
# RssAdapter — DiscourseContext propagation
# ---------------------------------------------------------------------------


def _rss_raw(source: str = "tagesschau") -> dict:
    return {
        "source": source,
        "source_type": "rss",
        "raw_text": "Die Bundesregierung hat einen Beschluss gefasst.",
        "url": "https://example.com/article",
    }


def _event_time() -> datetime:
    return datetime(2026, 4, 11, 10, 0, 0, tzinfo=timezone.utc)


def test_rss_adapter_no_pg_pool_yields_none_context():
    adapter = RssAdapter()
    _, meta = adapter.harmonize(_rss_raw(), _event_time(), "rss/tagesschau/test.json")
    assert meta.discourse_context is None


def test_rss_adapter_populates_discourse_context_from_classification():
    mock_pool = MagicMock()
    with patch("internal.adapters.rss.get_source_classification") as mock_get:
        mock_get.return_value = {
            "primary_function": "epistemic_authority",
            "secondary_function": "power_legitimation",
            "emic_designation": "Tagesschau",
        }
        adapter = RssAdapter(pg_pool=mock_pool)
        _, meta = adapter.harmonize(_rss_raw(), _event_time(), "rss/tagesschau/test.json")

    assert meta.discourse_context is not None
    assert meta.discourse_context.primary_function == "epistemic_authority"
    assert meta.discourse_context.secondary_function == "power_legitimation"
    assert meta.discourse_context.emic_designation == "Tagesschau"
    mock_get.assert_called_once_with(mock_pool, "tagesschau")


def test_rss_adapter_no_classification_row_is_graceful():
    """No classification for the source → discourse_context=None, no crash."""
    mock_pool = MagicMock()
    with patch("internal.adapters.rss.get_source_classification") as mock_get:
        mock_get.return_value = None
        adapter = RssAdapter(pg_pool=mock_pool)
        _, meta = adapter.harmonize(
            _rss_raw("unknown-source"), _event_time(), "rss/unknown/test.json"
        )

    assert meta.discourse_context is None


def test_rss_adapter_classification_query_failure_is_graceful():
    mock_pool = MagicMock()
    with patch("internal.adapters.rss.get_source_classification") as mock_get:
        mock_get.side_effect = Exception("Connection refused")
        adapter = RssAdapter(pg_pool=mock_pool)
        core, meta = adapter.harmonize(_rss_raw(), _event_time(), "rss/tagesschau/test.json")

    assert meta.discourse_context is None
    assert core.source == "tagesschau"


def test_rss_adapter_secondary_function_null_is_propagated_as_none():
    """secondary_function = NULL in the DB row → None in DiscourseContext."""
    mock_pool = MagicMock()
    with patch("internal.adapters.rss.get_source_classification") as mock_get:
        mock_get.return_value = {
            "primary_function": "power_legitimation",
            "secondary_function": None,
            "emic_designation": "Bundesregierung",
        }
        adapter = RssAdapter(pg_pool=mock_pool)
        _, meta = adapter.harmonize(
            _rss_raw("bundesregierung"), _event_time(), "rss/br/test.json"
        )

    assert meta.discourse_context is not None
    assert meta.discourse_context.primary_function == "power_legitimation"
    assert meta.discourse_context.secondary_function is None


def test_discourse_context_serialization_in_silver():
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
    meta = RssMeta(
        source_type="rss",
        feed_url="https://example.com/feed",
    )
    data = meta.model_dump()
    assert data["discourse_context"] is None
