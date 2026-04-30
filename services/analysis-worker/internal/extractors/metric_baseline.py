"""
MetricBaselineExtractor — corpus-level baseline computation (Phase 115).

Promotes ``scripts/compute_baselines.py`` (Phase 71) into a NATS-triggered
automated extractor on the same cadence as ``EntityCoOccurrenceExtractor``
(Phase 102). The standalone script is retained for ad-hoc operations
(first-run on a new probe, manual recompute, Operations-Playbook
walkthroughs); both call paths share the same underlying computation
function exported here so a regression test can guarantee byte-identical
output.

This module does NOT touch ``aer_gold.metric_equivalence``. Equivalence is
granted out-of-band via the Postgres ``equivalence_reviews`` workflow, not
derived statistically — the architectural expression of WP-004 §2.2:
equivalence is a research question, not a computation.
"""

from __future__ import annotations

import math
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Iterable, Sequence

import structlog

from internal.extractors.base import TimeWindow

logger = structlog.get_logger()


BASELINE_COLUMNS = [
    "metric_name",
    "source",
    "language",
    "baseline_value",
    "baseline_std",
    "window_start",
    "window_end",
    "n_documents",
    "compute_date",
]


# Source-of-truth SQL shared between the auto-extractor and the manual
# CLI script. Group by (metric_name, source, language); the window is
# parameterised so callers can drive both periodic and ad-hoc runs.
BASELINE_QUERY = """
SELECT
    m.metric_name        AS metric_name,
    m.source             AS source,
    ld.detected_language AS language,
    avg(m.value)         AS baseline_value,
    stddevPop(m.value)   AS baseline_std,
    count()              AS n_documents
FROM aer_gold.metrics AS m
INNER JOIN aer_gold.language_detections AS ld
    ON m.article_id = ld.article_id AND ld.rank = 1
WHERE m.timestamp >= {start:DateTime}
  AND m.timestamp <  {end:DateTime}
GROUP BY metric_name, source, language
HAVING n_documents >= 2
ORDER BY metric_name, source, language
"""


def compute_mean_std(values: Sequence[float]) -> tuple[float, float]:
    """Population mean and standard deviation for a set of metric values.

    Mirrors ClickHouse ``avg`` / ``stddevPop`` semantics so unit tests can
    verify baseline arithmetic without a live ClickHouse instance.

    Empty input → ``(0.0, 0.0)``; callers must filter empty groups.
    A single value has no dispersion, so std = 0.
    """
    n = len(values)
    if n == 0:
        return 0.0, 0.0
    mean = sum(values) / n
    variance = sum((v - mean) ** 2 for v in values) / n
    return mean, math.sqrt(variance)


def build_baseline_rows(
    query_rows: Iterable[tuple],
    window_start: datetime,
    window_end: datetime,
    compute_date: datetime,
) -> list[list]:
    """Shape pre-aggregated ClickHouse rows into ``metric_baselines`` inserts.

    Each query row is a ``(metric_name, source, language, baseline_value,
    baseline_std, n_documents)`` tuple produced by :data:`BASELINE_QUERY`.
    The resulting list is ready to pass to ``client.insert(..., rows,
    column_names=BASELINE_COLUMNS)``.

    Empty ``query_rows`` → empty list; the caller must skip the insert.
    """
    rows: list[list] = []
    for metric_name, source, language, baseline_value, baseline_std, n_docs in query_rows:
        rows.append([
            metric_name,
            source,
            language,
            baseline_value,
            baseline_std,
            window_start,
            window_end,
            n_docs,
            compute_date,
        ])
    return rows


def compute_baseline_rows(
    ch_client,
    window_start: datetime,
    window_end: datetime,
    compute_date: datetime | None = None,
) -> list[list]:
    """Run the canonical baseline aggregation and return ready-to-insert rows.

    Both the manual CLI script and the auto-extractor go through this
    function; that single dispatch point is the architectural anchor for
    the Phase-115 byte-identical regression guarantee.
    """
    if compute_date is None:
        compute_date = datetime.now(timezone.utc)

    result = ch_client.query(
        BASELINE_QUERY,
        parameters={"start": window_start, "end": window_end},
    )
    return build_baseline_rows(
        result.result_rows, window_start, window_end, compute_date
    )


@dataclass(frozen=True, slots=True)
class BaselineSweepResult:
    rows_written: int
    n_groups: int


class MetricBaselineExtractor:
    """
    Corpus-level extractor (Phase 115) — second ``CorpusExtractor``
    implementation alongside ``EntityCoOccurrenceExtractor``.

    Reads ``aer_gold.metrics`` joined with ``aer_gold.language_detections``
    over a configurable rolling window, computes per ``(metric_name,
    source, language)`` mean and standard deviation via
    :func:`compute_baseline_rows`, and writes the result to
    ``aer_gold.metric_baselines``. Idempotency is preserved by the
    ReplacingMergeTree(``compute_date``) ordering.
    """

    @property
    def name(self) -> str:
        return "metric_baseline"

    def run(
        self,
        ch_client,
        window: TimeWindow,
        compute_date: datetime | None = None,
    ) -> BaselineSweepResult:
        """Compute baselines for ``window`` and insert them into ClickHouse."""
        if compute_date is None:
            compute_date = datetime.now(timezone.utc)

        rows = compute_baseline_rows(ch_client, window.start, window.end, compute_date)
        if not rows:
            logger.info(
                "baseline.sweep.empty",
                window_start=str(window.start),
                window_end=str(window.end),
            )
            return BaselineSweepResult(rows_written=0, n_groups=0)

        ch_client.insert(
            "aer_gold.metric_baselines",
            rows,
            column_names=BASELINE_COLUMNS,
        )
        logger.info(
            "baseline.sweep.complete",
            window_start=str(window.start),
            window_end=str(window.end),
            n_groups=len(rows),
        )
        return BaselineSweepResult(rows_written=len(rows), n_groups=len(rows))
