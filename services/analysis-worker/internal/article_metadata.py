"""
Categorical article-metadata projection (Phase 133 / WP-003 §3.2).

Scalar metadata (paywall_status, image_count, …) ride the existing
``aer_gold.metrics`` rails as new metric_names (see :mod:`metadata_metrics`).
CATEGORICAL metadata (section, author, tags, categories, article_type,
editorial_labels, dateline_location, editor) cannot — ``metrics.value`` is a
Float64 and "Politik" is not a number. This module promotes those categorical
VALUES into ``aer_gold.article_metadata`` so they are analytically queryable as
grouping/measured dimensions (article count per Ressort, mean sentiment by
section, …).

Like :mod:`metadata_coverage`, this is web-only: only ``WebMeta`` carries these
fields, so the function silently no-ops for RSS / legacy meta. One row per
(article, field, value); list fields expand to one row per element, mirroring
``aer_gold.entities`` (one row per occurrence).

Disclose-never-coerce (WP-003 §3.2): a row is written ONLY for a non-empty
value. An absent / empty categorical field produces NO row — its absence is
surfaced as Negative Space via ``aer_gold.metadata_coverage`` (Phase 122f),
never as an empty-string value. The gate is the value itself (not the
``extraction_methods`` vocabulary), so a value recovered by a future
custom-extractor rule (Phase 133 Slice 6) flows here immediately without a
vocabulary change.

The field set is generic: every categorical WebMeta field is declared here
whether or not the current corpus populates it. An unpopulated field simply
produces no rows (and is therefore never offered in the picker by the
availability gate).

Failure mode mirrors :mod:`metadata_coverage`: a failed insert only impacts the
categorical-metadata cells; the canonical Silver record stays in MinIO and the
rest of the Gold pipeline is unaffected.
"""

from __future__ import annotations

from datetime import datetime

import structlog

from internal.adapters.web_meta import WebMeta

logger = structlog.get_logger()

ARTICLE_METADATA_TABLE = "aer_gold.article_metadata"

# Categorical WebMeta fields → is_list. Scalar-string fields emit one row (when
# non-empty); list fields expand to one row per non-empty element. Frozen here
# so the analysable categorical dimension set is explicit and stable.
CATEGORICAL_METADATA_FIELDS: dict[str, bool] = {
    "section": False,
    "author": False,
    "article_type": False,
    "editor": False,
    "dateline_location": False,
    "categories": True,
    "tags": True,
    "editorial_labels": True,
}


def _values_for_field(meta: WebMeta, field: str, is_list: bool) -> list[str]:
    """Return the non-empty string value(s) for one categorical field. Scalar
    fields yield 0 or 1 value; list fields yield one per non-empty element.
    Empty strings / empty lists yield nothing (disclose-never-coerce)."""
    raw = getattr(meta, field, None)
    if is_list:
        if not raw:
            return []
        out: list[str] = []
        for item in raw:
            if item is None:
                continue
            s = str(item).strip()
            if s:
                out.append(s)
        return out
    if raw is None:
        return []
    s = str(raw).strip()
    return [s] if s else []


def build_article_metadata_rows(
    source: str,
    article_id: str,
    meta: WebMeta,
    timestamp: datetime,
    discourse_function: str,
    timestamp_source: str,
    ingestion_version: int,
) -> list[list[object]]:
    """Construct the per-(article, field) rows for one Silver write.

    ONE row per field, with ``value`` an Array(String) of that field's non-empty
    values (a one-element array for scalar fields; the full list for tags /
    categories / editorial_labels). A field with no non-empty value yields NO
    row (disclose-never-coerce). The Array layout lets ReplacingMergeTree
    overwrite the whole field on re-ingest, so a corrected scalar value or a
    removed list element leaves no stale row.

    The column order mirrors ``aer_gold.article_metadata`` in
    ``infra/clickhouse/migrations/000030_create_article_metadata.sql``.
    """
    rows: list[list[object]] = []
    for field, is_list in CATEGORICAL_METADATA_FIELDS.items():
        values = _values_for_field(meta, field, is_list)
        if not values:
            continue
        rows.append(
            [
                timestamp,
                source,
                article_id,
                field,
                values,
                discourse_function,
                timestamp_source,
                ingestion_version,
            ]
        )
    return rows


def upload_article_metadata(
    ch_client,
    core,
    meta,
    ingestion_version: int,
    discourse_function: str = "",
    timestamp_source: str = "",
) -> None:
    """Insert one row per non-empty categorical (field, value) into
    ``aer_gold.article_metadata``. No-op for non-``WebMeta`` and for articles
    with no categorical values (disclose-never-coerce — no rows, never empties).
    """
    if not isinstance(meta, WebMeta):
        return
    try:
        rows = build_article_metadata_rows(
            source=core.source,
            article_id=core.document_id,
            meta=meta,
            timestamp=core.timestamp,
            discourse_function=discourse_function,
            timestamp_source=timestamp_source,
            ingestion_version=ingestion_version,
        )
        if not rows:
            return
        ch_client.insert(
            ARTICLE_METADATA_TABLE,
            rows,
            column_names=[
                "timestamp",
                "source",
                "article_id",
                "field",
                "value",
                "discourse_function",
                "timestamp_source",
                "ingestion_version",
            ],
            deduplication_token=f"{ARTICLE_METADATA_TABLE}:{core.document_id}:{ingestion_version}",
        )
        logger.info(
            "Article metadata updated",
            source=core.source,
            article_id=core.document_id,
            row_count=len(rows),
        )
    except Exception as exc:
        logger.error(
            "Article metadata insert failed; categorical metadata cells will miss this article.",
            source=core.source,
            article_id=core.document_id,
            error=str(exc),
        )
