"""Phase 133 (Slice 1) — unit tests for `derive_metadata_metrics`.

The single sanctioned point where scalar WebMeta fields are promoted to Gold
`metric_name`s. The load-bearing contract is **disclose-never-coerce**
(WP-003 §3.2): a field that the WebAdapter did not extract emits NO metric row
(never a `0`), while a field that WAS extracted and legitimately holds zero
items emits a real `0.0`.
"""

from internal.adapters.rss import RssMeta
from internal.adapters.web_meta import ImageRef, WebMeta
from internal.metadata_coverage import COVERAGE_FIELDS
from internal.metadata_metrics import (
    SCALAR_METADATA_METRIC_NAMES,
    _SCALAR_METADATA_FIELDS,
    derive_metadata_metrics,
)


def _as_dict(pairs):
    return dict(pairs)


def test_non_web_meta_returns_empty():
    assert derive_metadata_metrics(None) == []
    assert derive_metadata_metrics(RssMeta(source_type="rss")) == []


def test_web_meta_with_no_extraction_methods_emits_nothing():
    # Values present on the object but never recorded as extracted → absent.
    meta = WebMeta(source_type="web", paywall_status=True, images=[ImageRef(url="x")])
    assert derive_metadata_metrics(meta) == []


def test_paywall_true_and_false_map_to_1_and_0_when_extracted():
    t = WebMeta(
        source_type="web",
        paywall_status=True,
        extraction_methods={"paywall_status": "json_ld"},
    )
    assert _as_dict(derive_metadata_metrics(t))["paywall_status"] == 1.0

    f = WebMeta(
        source_type="web",
        paywall_status=False,
        extraction_methods={"paywall_status": "json_ld"},
    )
    assert _as_dict(derive_metadata_metrics(f))["paywall_status"] == 0.0


def test_defensive_method_present_empty_list_counts_zero():
    # DEFENSIVE / local-behaviour only: if a method were recorded for an empty
    # list, len()==0 would emit 0.0. In PRODUCTION the WebAdapter records a
    # method for images/external_citations only when the list is non-empty
    # (web_extract.py), so this state never occurs — "0 structured images" is
    # indistinguishable from "field absent" and is correctly treated as absence
    # (see the test below), not a coerced 0.
    meta = WebMeta(
        source_type="web",
        images=[],
        extraction_methods={"images": "json_ld"},
    )
    assert _as_dict(derive_metadata_metrics(meta))["image_count"] == 0.0


def test_out_of_vocabulary_method_is_not_present():
    # Presence is shared with the coverage matrix: an out-of-vocabulary method
    # collapses to "null" (absent) in BOTH, so promotion never disagrees with
    # coverage. (A new method tag — e.g. custom_css in Slice 6 — must be added
    # to ALLOWED_EXTRACTION_METHODS to count as present in either surface.)
    meta = WebMeta(
        source_type="web",
        images=[ImageRef(url="a")],
        extraction_methods={"images": "custom_css"},  # not (yet) in the vocabulary
    )
    assert "image_count" not in _as_dict(derive_metadata_metrics(meta))


def test_unextracted_list_is_omitted_even_if_object_has_items():
    # Disclose-never-coerce: a value present without an extraction method is
    # NOT analytically present (coverage records it as null), so no row.
    meta = WebMeta(
        source_type="web",
        images=[ImageRef(url="a"), ImageRef(url="b")],  # present on the object…
        extraction_methods={},  # …but never recorded as extracted
    )
    assert "image_count" not in _as_dict(derive_metadata_metrics(meta))


def test_counts_use_cardinality():
    meta = WebMeta(
        source_type="web",
        images=[ImageRef(url="a"), ImageRef(url="b"), ImageRef(url="c")],
        external_citations=["u1", "u2"],
        extraction_methods={"images": "json_ld", "external_citations": "json_ld"},
    )
    d = _as_dict(derive_metadata_metrics(meta))
    assert d["image_count"] == 3.0
    assert d["external_citation_count"] == 2.0


def test_optional_int_none_is_omitted_even_when_method_recorded():
    # An extracted-but-None numeric (shouldn't normally happen) is omitted, not 0.
    meta = WebMeta(
        source_type="web",
        comment_count=None,
        reading_time_minutes=5,
        extraction_methods={"comment_count": "json_ld", "reading_time_minutes": "json_ld"},
    )
    d = _as_dict(derive_metadata_metrics(meta))
    assert "comment_count" not in d
    assert d["reading_time_minutes"] == 5.0


def test_all_declared_metric_names_are_unique_and_count_suffixed_where_lists():
    # Guard the public name set so picker/content/provenance stay in lockstep.
    assert len(set(SCALAR_METADATA_METRIC_NAMES)) == len(SCALAR_METADATA_METRIC_NAMES)
    assert "image_count" in SCALAR_METADATA_METRIC_NAMES
    assert "external_citation_count" in SCALAR_METADATA_METRIC_NAMES
    assert "paywall_status" in SCALAR_METADATA_METRIC_NAMES


def test_scalar_coverage_fields_are_tracked_by_the_coverage_matrix():
    # Drift guard: the WebMeta field each scalar metric reads MUST be in the
    # coverage matrix, so promotion (this module) and coverage agree on presence.
    cov_fields = {coverage_field for coverage_field, _ in _SCALAR_METADATA_FIELDS.values()}
    missing = [f for f in cov_fields if f not in COVERAGE_FIELDS]
    assert missing == [], f"scalar coverage_fields not in COVERAGE_FIELDS: {missing}"
