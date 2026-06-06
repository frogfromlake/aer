"""Phase 133 (Slice 2) — unit tests for the categorical article-metadata builder.

Load-bearing contracts:
  * list fields expand to one row per non-empty element;
  * disclose-never-coerce — empty string / empty list / whitespace → NO rows
    (absence is surfaced as Negative Space via metadata_coverage, never as a "");
  * non-WebMeta → no rows.
"""

from datetime import datetime, timezone

from internal.adapters.rss import RssMeta
from internal.adapters.web_meta import WebMeta
from internal.article_metadata import (
    CATEGORICAL_METADATA_FIELDS,
    build_article_metadata_rows,
    upload_article_metadata,
)

_TS = datetime(2026, 6, 6, tzinfo=timezone.utc)


def _rows(meta: WebMeta):
    return build_article_metadata_rows(
        source="tagesschau",
        article_id="a1",
        meta=meta,
        timestamp=_TS,
        discourse_function="epistemic_authority",
        timestamp_source="json_ld_published",
        ingestion_version=42,
    )


def _by_field(rows):
    # One row per field; columns: timestamp, source, article_id, field,
    # value (Array(String)), df, ts_source, version.
    return {r[3]: r[4] for r in rows}


def test_scalar_string_field_emits_one_row_with_singleton_array():
    rows = _rows(WebMeta(source_type="web", section="Politik"))
    assert len(rows) == 1
    assert _by_field(rows) == {"section": ["Politik"]}
    # column order + carried context
    r = rows[0]
    assert r[0] == _TS and r[1] == "tagesschau" and r[2] == "a1"
    assert r[5] == "epistemic_authority" and r[6] == "json_ld_published" and r[7] == 42


def test_list_field_emits_one_row_with_the_value_array():
    rows = _rows(WebMeta(source_type="web", tags=["Wahl", "Bundestag", "Wahl"]))
    # One row per field; `value` carries the full list (order + duplicates
    # preserved). The read path arrayJoins + uniqExact(article_id) to count
    # distinct articles per value, so a duplicate element never double-counts.
    assert len(rows) == 1
    assert _by_field(rows)["tags"] == ["Wahl", "Bundestag", "Wahl"]


def test_empty_and_whitespace_values_emit_no_rows():
    meta = WebMeta(
        source_type="web",
        section="",  # empty scalar
        author="   ",  # whitespace-only scalar
        tags=[],  # empty list
        categories=["", "  "],  # all-empty list elements (pydantic forbids None)
    )
    assert _rows(meta) == []


def test_mixed_fields_and_generic_field_set():
    meta = WebMeta(
        source_type="web",
        section="Inland",
        author="Hans Müller",
        article_type="NewsArticle",
        tags=["A", "B"],
        categories=["X"],
        editorial_labels=[],  # absent → no rows
    )
    by = _by_field(_rows(meta))
    assert by["section"] == ["Inland"]
    assert by["author"] == ["Hans Müller"]
    assert by["article_type"] == ["NewsArticle"]
    assert by["tags"] == ["A", "B"]
    assert by["categories"] == ["X"]
    assert "editorial_labels" not in by
    # the declared field set is the generic categorical set, list-aware
    assert CATEGORICAL_METADATA_FIELDS["tags"] is True
    assert CATEGORICAL_METADATA_FIELDS["section"] is False


def test_categorical_field_set_is_covered_by_the_coverage_matrix():
    # Drift guard: every analysable categorical field MUST be tracked by the
    # Phase-122f coverage matrix, because the availability gate (Slice 3) offers
    # a field only if coverage reports it present. A field analysable here but
    # missing from COVERAGE_FIELDS would be invisible (never offered).
    from internal.metadata_coverage import COVERAGE_FIELDS

    missing = [f for f in CATEGORICAL_METADATA_FIELDS if f not in COVERAGE_FIELDS]
    assert missing == [], f"categorical fields not in COVERAGE_FIELDS: {missing}"


def test_non_web_meta_writes_nothing():
    captured = []

    class _CH:
        def insert(self, *a, **k):
            captured.append((a, k))

    upload_article_metadata(_CH(), core=object(), meta=RssMeta(source_type="rss"), ingestion_version=1)
    assert captured == []


def test_upload_no_rows_skips_insert():
    captured = []

    class _CH:
        def insert(self, *a, **k):
            captured.append((a, k))

    class _Core:
        source = "bundesregierung"
        document_id = "b1"
        timestamp = _TS

    # An institutional article with no categorical metadata → no insert at all.
    upload_article_metadata(_CH(), core=_Core(), meta=WebMeta(source_type="web"), ingestion_version=1)
    assert captured == []
