"""Phase 64 test coverage — BiasContext propagation in RssAdapter.

Split out from tests/test_discourse.py during Phase 72. Covers the WP-003
"document, don't filter" approach: every document carries platform-level
bias metadata populated by source adapters at harmonization time.
"""

from datetime import datetime, timezone

import pytest
from pydantic import ValidationError

from internal.adapters.rss import RssAdapter, RssMeta
from internal.models.bias import BiasContext


# ---------------------------------------------------------------------------
# Pydantic model invariants
# ---------------------------------------------------------------------------


def test_bias_context_construction_with_all_fields():
    ctx = BiasContext(
        platform_type="rss",
        access_method="public_rss",
        visibility_mechanism="chronological",
        moderation_context="editorial",
        engagement_data_available=False,
        account_metadata_available=False,
    )
    assert ctx.platform_type == "rss"
    assert ctx.access_method == "public_rss"
    assert ctx.visibility_mechanism == "chronological"
    assert ctx.moderation_context == "editorial"
    assert ctx.engagement_data_available is False
    assert ctx.account_metadata_available is False


def test_bias_context_missing_required_field_raises_validation_error():
    """BiasContext fields are all required — missing any raises ValidationError."""
    with pytest.raises(ValidationError):
        BiasContext(  # type: ignore[call-arg]
            platform_type="rss",
            access_method="public_rss",
            visibility_mechanism="chronological",
            # moderation_context deliberately omitted
            engagement_data_available=False,
            account_metadata_available=False,
        )


def test_bias_context_rejects_wrong_type_for_bool_field():
    with pytest.raises(ValidationError):
        BiasContext(
            platform_type="rss",
            access_method="public_rss",
            visibility_mechanism="chronological",
            moderation_context="editorial",
            engagement_data_available="not-a-bool",  # type: ignore[arg-type]
            account_metadata_available=False,
        )


# ---------------------------------------------------------------------------
# RssAdapter — BiasContext population
# ---------------------------------------------------------------------------


def test_rss_adapter_populates_bias_context_with_static_rss_values():
    adapter = RssAdapter()
    raw = {
        "source": "tagesschau",
        "source_type": "rss",
        "raw_text": "Some German text content.",
        "url": "https://example.com",
    }
    event_time = datetime(2026, 4, 11, 10, 0, 0, tzinfo=timezone.utc)
    _, meta = adapter.harmonize(raw, event_time, "rss/tagesschau/test.json")

    assert meta.bias_context is not None
    assert meta.bias_context.platform_type == "rss"
    assert meta.bias_context.access_method == "public_rss"
    assert meta.bias_context.visibility_mechanism == "chronological"
    assert meta.bias_context.moderation_context == "editorial"
    assert meta.bias_context.engagement_data_available is False
    assert meta.bias_context.account_metadata_available is False


def test_rss_adapter_bias_context_fields_are_non_null():
    """Every BiasContext field must be non-null for RSS sources."""
    adapter = RssAdapter()
    raw = {
        "source": "tagesschau",
        "source_type": "rss",
        "raw_text": "Text",
        "url": "https://example.com",
    }
    _, meta = adapter.harmonize(
        raw, datetime(2026, 4, 11, 10, 0, 0, tzinfo=timezone.utc), "rss/t/x.json"
    )

    assert meta.bias_context is not None
    data = meta.bias_context.model_dump()
    for field, value in data.items():
        assert value is not None, f"bias_context.{field} must be non-null for RSS"


def test_bias_context_serialization_in_silver():
    meta = RssMeta(
        source_type="rss",
        feed_url="https://example.com/feed",
        bias_context=BiasContext(
            platform_type="rss",
            access_method="public_rss",
            visibility_mechanism="chronological",
            moderation_context="editorial",
            engagement_data_available=False,
            account_metadata_available=False,
        ),
    )
    data = meta.model_dump()
    assert data["bias_context"]["platform_type"] == "rss"
    assert data["bias_context"]["engagement_data_available"] is False
