"""Unit tests for MetricBaselineExtractor (Phase 115).

Promotes the manual ``scripts/compute_baselines.py`` to a NATS-triggered
``CorpusExtractor``. Both call paths share
:func:`internal.extractors.metric_baseline.compute_baseline_rows`; the
regression test below checks that the auto-extractor and the script
expose the same canonical SQL and helpers, so they cannot drift
silently.
"""

from __future__ import annotations

import math
import sys
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path

from internal.extractors import (
    BASELINE_COLUMNS,
    BASELINE_QUERY,
    MetricBaselineExtractor,
    TimeWindow,
    build_baseline_rows,
    compute_baseline_rows,
    compute_mean_std,
)

REPO_ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(REPO_ROOT / "scripts"))

import compute_baselines as cli  # noqa: E402  — path insertion is intentional


def _dt(iso: str) -> datetime:
    return datetime.fromisoformat(iso).replace(tzinfo=timezone.utc)


# ---------------------------------------------------------------------------
# Regression guard — manual script and auto-extractor must share computation
# ---------------------------------------------------------------------------


def test_cli_and_extractor_share_canonical_query():
    """Single source-of-truth SQL: any drift is a deployment hazard."""
    assert cli.BASELINE_QUERY is BASELINE_QUERY


def test_cli_and_extractor_share_canonical_helpers():
    """The CLI re-exports the canonical helpers — no parallel implementations."""
    assert cli.compute_mean_std is compute_mean_std
    assert cli.build_baseline_rows is build_baseline_rows
    assert cli.compute_baseline_rows is compute_baseline_rows


# ---------------------------------------------------------------------------
# Pure helpers — preserved Phase-65 coverage
# ---------------------------------------------------------------------------


def test_compute_mean_std_known_values():
    mean, std = compute_mean_std([10.0, 20.0, 30.0, 40.0, 50.0])
    assert mean == 30.0
    assert math.isclose(std, math.sqrt(200), rel_tol=1e-9)


def test_compute_mean_std_empty_set_is_zero_zero():
    assert compute_mean_std([]) == (0.0, 0.0)


def test_compute_mean_std_single_value_has_zero_std():
    mean, std = compute_mean_std([42.5])
    assert mean == 42.5
    assert std == 0.0


def test_build_baseline_rows_shape():
    rows = build_baseline_rows(
        [("word_count", "tagesschau", "de", 150.0, 30.0, 100)],
        _dt("2026-01-01"),
        _dt("2026-04-01"),
        _dt("2026-04-12"),
    )
    assert rows == [[
        "word_count", "tagesschau", "de", 150.0, 30.0,
        _dt("2026-01-01"), _dt("2026-04-01"), 100, _dt("2026-04-12"),
    ]]


# ---------------------------------------------------------------------------
# MetricBaselineExtractor — fakes the ClickHouse client to exercise dispatch
# ---------------------------------------------------------------------------


@dataclass
class _QueryResult:
    result_rows: list[tuple]


class _FakeClickHouse:
    """Minimal stand-in capturing the calls the extractor makes."""

    def __init__(self, rows: list[tuple]):
        self._rows = rows
        self.query_calls: list[tuple[str, dict]] = []
        self.insert_calls: list[tuple[str, list[list], list[str]]] = []

    def query(self, sql: str, parameters: dict):
        self.query_calls.append((sql, parameters))
        return _QueryResult(result_rows=list(self._rows))

    def insert(self, table: str, rows: list[list], column_names: list[str]):
        self.insert_calls.append((table, rows, column_names))


def test_extractor_name_is_stable():
    assert MetricBaselineExtractor().name == "metric_baseline"


def test_extractor_runs_canonical_query_and_writes_rows():
    fake = _FakeClickHouse(rows=[
        ("word_count", "tagesschau", "de", 150.0, 30.0, 100),
        ("sentiment_score_sentiws", "tagesschau", "de", 0.25, 0.10, 100),
    ])
    extractor = MetricBaselineExtractor()
    window = TimeWindow(start=_dt("2026-01-01"), end=_dt("2026-04-01"))
    compute_date = _dt("2026-04-12")

    result = extractor.run(fake, window, compute_date=compute_date)

    assert result.rows_written == 2
    assert len(fake.query_calls) == 1
    assert fake.query_calls[0][0] is BASELINE_QUERY
    assert fake.query_calls[0][1] == {"start": window.start, "end": window.end}

    assert len(fake.insert_calls) == 1
    table, rows, cols = fake.insert_calls[0]
    assert table == "aer_gold.metric_baselines"
    assert cols == BASELINE_COLUMNS
    assert rows[0][0] == "word_count"
    assert rows[0][-1] == compute_date


def test_extractor_skips_insert_when_no_rows():
    """Empty corpus → no insert call (and no crash)."""
    fake = _FakeClickHouse(rows=[])
    extractor = MetricBaselineExtractor()
    window = TimeWindow(start=_dt("2026-01-01"), end=_dt("2026-04-01"))

    result = extractor.run(fake, window, compute_date=_dt("2026-04-12"))

    assert result.rows_written == 0
    assert fake.insert_calls == []


def test_extractor_and_compute_baseline_rows_produce_identical_output():
    """Byte-identical output is the Phase 115 regression guarantee."""
    fixture = [
        ("word_count", "tagesschau", "de", 150.0, 30.0, 100),
        ("sentiment_score_sentiws", "tagesschau", "de", 0.25, 0.10, 100),
    ]
    window = TimeWindow(start=_dt("2026-01-01"), end=_dt("2026-04-01"))
    compute_date = _dt("2026-04-12")

    via_helper = compute_baseline_rows(
        _FakeClickHouse(rows=fixture), window.start, window.end, compute_date,
    )

    via_extractor_fake = _FakeClickHouse(rows=fixture)
    MetricBaselineExtractor().run(via_extractor_fake, window, compute_date=compute_date)
    via_extractor_rows = via_extractor_fake.insert_calls[0][1]

    assert via_helper == via_extractor_rows
