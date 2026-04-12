"""Phase 65 test coverage — compute_baselines script logic.

Tests the pure Python helpers extracted from ``scripts/compute_baselines.py``:
``compute_mean_std`` (reference implementation of ClickHouse ``avg`` /
``stddevPop``) and ``build_baseline_rows`` (shaping pre-aggregated rows
into insert tuples for ``aer_gold.metric_baselines``).

The script itself runs against a live ClickHouse; these tests exercise the
edge cases noted in the Phase 72 roadmap (empty input, single-value set,
known-values set) without the infrastructure dependency.
"""

import math
import sys
from datetime import datetime, timezone
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(REPO_ROOT / "scripts"))

import compute_baselines  # noqa: E402  — path insertion is intentional


# ---------------------------------------------------------------------------
# compute_mean_std
# ---------------------------------------------------------------------------


def test_compute_mean_std_known_values():
    """Mean and population std for a deterministic input set."""
    values = [10.0, 20.0, 30.0, 40.0, 50.0]
    mean, std = compute_baselines.compute_mean_std(values)
    assert mean == 30.0
    # Population stddev of [10..50] step 10 is sqrt(200) ≈ 14.1421356
    assert math.isclose(std, math.sqrt(200), rel_tol=1e-9)


def test_compute_mean_std_empty_set_is_zero_zero():
    """Empty input → (0.0, 0.0); callers must filter empty groups separately."""
    mean, std = compute_baselines.compute_mean_std([])
    assert mean == 0.0
    assert std == 0.0


def test_compute_mean_std_single_value_has_zero_std():
    """A single measurement has no dispersion — std must be 0.0, not NaN."""
    mean, std = compute_baselines.compute_mean_std([42.5])
    assert mean == 42.5
    assert std == 0.0
    assert not math.isnan(std)


def test_compute_mean_std_identical_values_have_zero_std():
    mean, std = compute_baselines.compute_mean_std([7.0, 7.0, 7.0, 7.0])
    assert mean == 7.0
    assert std == 0.0


# ---------------------------------------------------------------------------
# build_baseline_rows
# ---------------------------------------------------------------------------


def _dt(iso: str) -> datetime:
    return datetime.fromisoformat(iso).replace(tzinfo=timezone.utc)


def test_build_baseline_rows_maps_query_rows_to_inserts():
    window_start = _dt("2026-01-01")
    window_end = _dt("2026-04-01")
    compute_date = _dt("2026-04-12")

    query_rows = [
        ("word_count", "tagesschau", "de", 150.0, 30.0, 100),
        ("sentiment_score", "tagesschau", "de", 0.25, 0.10, 100),
    ]
    rows = compute_baselines.build_baseline_rows(
        query_rows, window_start, window_end, compute_date
    )

    assert len(rows) == 2
    assert rows[0] == [
        "word_count", "tagesschau", "de", 150.0, 30.0,
        window_start, window_end, 100, compute_date,
    ]
    assert rows[1][0] == "sentiment_score"
    assert rows[1][-1] == compute_date


def test_build_baseline_rows_empty_input_yields_empty_list():
    """No query rows → empty insert list (no crash, no insert call)."""
    rows = compute_baselines.build_baseline_rows(
        [], _dt("2026-01-01"), _dt("2026-04-01"), _dt("2026-04-12")
    )
    assert rows == []


def test_build_baseline_rows_single_row_single_value_zero_std():
    """Single-group baseline with n=1 → std forwarded unchanged (CH returns 0)."""
    rows = compute_baselines.build_baseline_rows(
        [("word_count", "tagesschau", "de", 42.0, 0.0, 1)],
        _dt("2026-01-01"),
        _dt("2026-04-01"),
        _dt("2026-04-12"),
    )
    assert len(rows) == 1
    assert rows[0][3] == 42.0  # baseline_value
    assert rows[0][4] == 0.0   # baseline_std
    assert rows[0][7] == 1     # n_documents
