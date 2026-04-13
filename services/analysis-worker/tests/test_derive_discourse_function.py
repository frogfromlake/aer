"""Phase 76 — unit tests for the `_derive_discourse_function` helper.

Isolates the single sanctioned point where `SilverMeta` influences Gold
row assembly so the contract is explicit and unit-testable.
"""

from internal.adapters.rss import RssMeta
from internal.models.discourse import DiscourseContext
from internal.processor import _derive_discourse_function


def test_none_meta_returns_empty_string():
    assert _derive_discourse_function(None) == ""


def test_meta_without_discourse_context_returns_empty_string():
    meta = RssMeta(source_type="rss")
    assert _derive_discourse_function(meta) == ""


def test_meta_with_discourse_context_returns_primary_function():
    meta = RssMeta(
        source_type="rss",
        discourse_context=DiscourseContext(
            primary_function="epistemic_authority",
            emic_designation="Tagesschau",
        ),
    )
    assert _derive_discourse_function(meta) == "epistemic_authority"


def test_meta_with_empty_primary_function_returns_empty_string():
    meta = RssMeta(
        source_type="rss",
        discourse_context=DiscourseContext(
            primary_function="",
            emic_designation="Tagesschau",
        ),
    )
    assert _derive_discourse_function(meta) == ""
