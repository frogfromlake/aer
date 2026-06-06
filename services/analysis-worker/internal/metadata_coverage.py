"""
Per-article metadata-coverage projection (Phase 122f / WP-003 §3.2).

Every Silver write of a `source_type == "web"` envelope appends one row
per Tier-B/C field into `aer_gold.metadata_coverage_raw`. The
`extraction_methods` provenance dict on `WebMeta` already carries the
per-field method (`json_ld`, `open_graph`, `microdata`, `html_meta`,
`heuristic_htmldate`, `derived`, or ``None`` for unfilled fields). This
module is the bridge between that in-memory provenance and the
ClickHouse aggregation that backs the BFF's
`/probes/{id}/metadata-coverage` endpoint and the dashboard's
field-level Negative-Space rendering (Brief §7.7).

Failure mode mirrors :mod:`silver_projection`: a missing coverage row
only impacts the metadata-coverage panel, the canonical Silver record
remains in MinIO, and hard-failing here would jeopardise the rest of
the pipeline.
"""

from __future__ import annotations

from datetime import datetime
from typing import Iterable, Optional

import structlog

from internal.adapters.web_meta import ALLOWED_EXTRACTION_METHODS, WebMeta

logger = structlog.get_logger()

METADATA_COVERAGE_TABLE = "aer_gold.metadata_coverage_raw"

# Canonical Tier-B / Tier-C field set written by the WebAdapter
# (`web_extract.py` `_record(meta, "<field>", ...)` callsites). Frozen
# here so the coverage matrix reports a stable column set even when an
# article fails to populate a given field — structural absence is the
# whole point of this surface.
TIER_B_FIELDS: tuple[str, ...] = (
    "published_date",
    "modified_date",
    "author",
    "description",
    "categories",
    "tags",
    "section",
    "image_url",
    "article_type",
    "word_count",
)

TIER_C_FIELDS: tuple[str, ...] = (
    "comment_count",
    "comment_url",
    "editor",
    "reading_time_minutes",
    "dateline_location",
    "paywall_status",
    "correction_notice",
    "editorial_labels",
    "external_citations",
    "images",
    "social_share_counts",
    "revision_date",
)

COVERAGE_FIELDS: tuple[str, ...] = TIER_B_FIELDS + TIER_C_FIELDS

# Literal-string sentinel for unfilled fields. Stored as a value, not a
# SQL NULL — structural absence is the signal we surface, so it must be
# queryable as a normal `method` value.
NULL_METHOD: str = "null"


def _normalise_method(raw: Optional[str]) -> str:
    """Return the storage form of an `extraction_methods` value.

    ``None`` collapses to the literal-string ``"null"``. An out-of-vocabulary
    value (defensive guard — the WebAdapter already filters via
    ``ALLOWED_EXTRACTION_METHODS``) is also treated as null so the coverage
    matrix never carries methods the read side does not expect.
    """
    if raw is None or raw == "":
        return NULL_METHOD
    if raw not in ALLOWED_EXTRACTION_METHODS:
        return NULL_METHOD
    return raw


def is_extraction_present(extraction_methods: dict[str, Optional[str]], field: str) -> bool:
    """True iff ``field`` was populated by a real (in-vocabulary) extraction
    method. This is the single presence definition shared by the coverage matrix
    and the Phase-133 metadata-metric promotion (:mod:`metadata_metrics`), so the
    two surfaces never disagree about whether a field is present: an out-of-
    vocabulary method collapses to ``null`` here exactly as it does in coverage.
    """
    return _normalise_method(extraction_methods.get(field)) != NULL_METHOD


def build_coverage_rows(
    source: str,
    article_id: str,
    extraction_methods: dict[str, Optional[str]],
    ingestion_version: int,
    ingestion_at: datetime,
    fields: Iterable[str] = COVERAGE_FIELDS,
) -> list[list[object]]:
    """Construct the (article, field, method) rows for one Silver write.

    The column order mirrors `metadata_coverage_raw` in
    `infra/clickhouse/migrations/000022_metadata_coverage.sql`.
    """
    rows: list[list[object]] = []
    for field in fields:
        method = _normalise_method(extraction_methods.get(field))
        rows.append([source, article_id, field, method, ingestion_version, ingestion_at])
    return rows


def upload_metadata_coverage(
    ch_client,
    core,
    meta,
    ingestion_version: int,
    ingestion_at: datetime,
) -> None:
    """Insert one row per Tier-B/C field into `metadata_coverage_raw`.

    Only `WebMeta` envelopes carry per-field provenance — RSS / legacy
    `SilverMeta` subclasses have no `extraction_methods` dict, so the
    function silently no-ops for non-web sources rather than guessing a
    coverage shape that does not exist there. Phase 122f is web-only by
    design (WP-003 §3.2 frames the asymmetry as a publisher/web-platform
    structural bias).
    """
    if not isinstance(meta, WebMeta):
        return

    extraction_methods = getattr(meta, "extraction_methods", {}) or {}
    try:
        rows = build_coverage_rows(
            source=core.source,
            article_id=core.document_id,
            extraction_methods=extraction_methods,
            ingestion_version=ingestion_version,
            ingestion_at=ingestion_at,
        )
        ch_client.insert(
            METADATA_COVERAGE_TABLE,
            rows,
            column_names=[
                "source",
                "article_id",
                "field",
                "method",
                "ingestion_version",
                "ingestion_at",
            ],
            deduplication_token=f"{METADATA_COVERAGE_TABLE}:{core.document_id}:{ingestion_version}",
        )
        logger.info(
            "Metadata coverage updated",
            source=core.source,
            article_id=core.document_id,
            field_count=len(rows),
        )
    except Exception as exc:
        logger.error(
            "Metadata coverage insert failed; coverage panel will be missing this article.",
            source=core.source,
            article_id=core.document_id,
            error=str(exc),
        )
