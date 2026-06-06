"""
Scalar-metadata Gold metrics (Phase 133 / WP-003 §3.2).

A subset of WebMeta's Tier-B/C fields are numeric per-article quantities
(paywall status, image count, external-citation count, comment count, reading
time). Phase 133 promotes those into ``aer_gold.metrics`` as ordinary
``metric_name``s, so they flow through the *existing* distribution / time-series
/ scatter read paths, the ``/scope/available-metrics`` gate, the metric picker,
the content catalog and the provenance surface — with zero schema change.

These values live on ``WebMeta``, which the ``MetricExtractor`` protocol never
sees (``extract_all(core, article_id)`` takes only ``SilverCore``). So — exactly
like ``_derive_discourse_function`` in the processor — they are assembled where
``meta`` is in scope, NOT as a ``MetricExtractor``.

Disclose-never-coerce (WP-003 §3.2): a field is promoted ONLY when the
WebAdapter actually extracted it (its ``extraction_methods`` entry is a real,
in-vocabulary method — the same presence test the coverage matrix uses, shared
via :func:`metadata_coverage.is_extraction_present`). An unextracted field emits
NO metric row — never a ``0``. Absence is surfaced as Negative Space via
``aer_gold.metadata_coverage``, never as a fabricated value.

Note on the COUNT fields (image_count, external_citation_count): the WebAdapter
records a method for ``images`` / ``external_citations`` only when the list is
NON-empty (web_extract.py), so these metrics carry a value ≥ 1 — a publisher
that declares no structured images is recorded as method-``null`` (absent), not
as ``0``. "Zero structured images" is therefore indistinguishable from "the
publisher does not use the field", so it is correctly treated as absence, not a
coerced ``0``. The metric reads as "count among articles that declare the
field". (The value_fn still computes ``len(...)`` faithfully — it just never
sees an extracted-empty list in production.)

The set is generic: every numeric metadata field is declared here, whether or
not the current corpus populates it. An unpopulated field simply never produces
a row (and is therefore never offered in the picker by the availability gate),
and lights up automatically once a later phase (custom_extractors) makes it
extractable — with no code change here.
"""

from __future__ import annotations

from typing import Callable, Optional

from internal.adapters.web_meta import WebMeta
from internal.metadata_coverage import is_extraction_present

# metric_name → (coverage_field, value_fn).
#   coverage_field — the WebMeta field name as recorded in `extraction_methods`
#                    (the same key `metadata_coverage` uses), so promotion and
#                    coverage always agree on whether a field is present.
#   value_fn       — reads the typed WebMeta field into a float, or returns None
#                    when the field carries no value.
# Count fields take a `_count` metric_name (their analytical quantity is the
# cardinality); boolean/scalar fields keep their field name.
_SCALAR_METADATA_FIELDS: dict[str, tuple[str, Callable[[WebMeta], Optional[float]]]] = {
    "paywall_status": (
        "paywall_status",
        lambda m: None if m.paywall_status is None else (1.0 if m.paywall_status else 0.0),
    ),
    "reading_time_minutes": (
        "reading_time_minutes",
        lambda m: None if m.reading_time_minutes is None else float(m.reading_time_minutes),
    ),
    "comment_count": (
        "comment_count",
        lambda m: None if m.comment_count is None else float(m.comment_count),
    ),
    "image_count": ("images", lambda m: float(len(m.images))),
    "external_citation_count": (
        "external_citations",
        lambda m: float(len(m.external_citations)),
    ),
}

# Exported for tests + the content/provenance authoring checklist.
SCALAR_METADATA_METRIC_NAMES: tuple[str, ...] = tuple(_SCALAR_METADATA_FIELDS.keys())


def _was_extracted(meta: WebMeta, coverage_field: str) -> bool:
    """True iff the WebAdapter populated this field — delegated to the shared
    presence test so promotion and the coverage matrix can never disagree
    (including the out-of-vocabulary-method case)."""
    return is_extraction_present(getattr(meta, "extraction_methods", {}) or {}, coverage_field)


def derive_metadata_metrics(meta) -> list[tuple[str, float]]:
    """Return ``(metric_name, value)`` pairs for the scalar metadata fields the
    WebAdapter extracted for this article.

    Non-``WebMeta`` (RSS / legacy) → ``[]`` (those carry no `extraction_methods`).
    A field that was not extracted, or whose value is ``None``, is omitted —
    never coerced to ``0``.
    """
    if not isinstance(meta, WebMeta):
        return []
    out: list[tuple[str, float]] = []
    for metric_name, (coverage_field, value_fn) in _SCALAR_METADATA_FIELDS.items():
        if not _was_extracted(meta, coverage_field):
            continue
        value = value_fn(meta)
        if value is None:
            continue
        out.append((metric_name, value))
    return out
