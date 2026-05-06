#!/usr/bin/env python3
"""Compute metric baselines for z-score normalization (manual / ad-hoc path).

Phase 115 retains this script for ad-hoc operations: first-run on a new
probe (Phase 126 uses it explicitly), manual recompute after a schema
change, and Operations-Playbook walkthroughs. Periodic baseline
maintenance is handled by ``MetricBaselineExtractor`` running inside the
analysis-worker (see ``internal/corpus.py::baseline_extraction_loop``).

Both call paths go through the same canonical computation in
``internal.extractors.metric_baseline.compute_baseline_rows`` so the auto
extractor and this script produce byte-identical baselines for the same
input window — the regression guard documented in Phase 115.

Public re-exports (``BASELINE_QUERY``, ``compute_mean_std``,
``build_baseline_rows``) remain available from this module so existing
test imports continue to work.

Usage:
    python scripts/operations/compute_baselines.py \
        --start 2026-01-01 --end 2026-04-01 \
        --clickhouse-host localhost --clickhouse-port 8123
"""

import argparse
import sys
from datetime import datetime, timezone
from pathlib import Path

# The shared computation lives inside the analysis-worker package.
# Insert the worker root onto sys.path so this standalone script can
# import it without packaging gymnastics.
# Phase 120c moved this script under `scripts/operations/`, so the repo root
# is two parents up (`scripts/operations/X.py → scripts → repo`).
_WORKER_ROOT = Path(__file__).resolve().parents[2] / "services" / "analysis-worker"
if str(_WORKER_ROOT) not in sys.path:
    sys.path.insert(0, str(_WORKER_ROOT))

import clickhouse_connect

from internal.extractors.metric_baseline import (  # noqa: E402  — sys.path insert above
    BASELINE_COLUMNS,
    BASELINE_QUERY,
    build_baseline_rows,
    compute_baseline_rows,
    compute_mean_std,
)

__all__ = [
    "BASELINE_COLUMNS",
    "BASELINE_QUERY",
    "build_baseline_rows",
    "compute_baseline_rows",
    "compute_mean_std",
    "main",
]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Compute metric baselines for z-score normalization."
    )
    parser.add_argument(
        "--start", required=True,
        help="Window start date (ISO 8601, e.g. 2026-01-01)",
    )
    parser.add_argument(
        "--end", required=True,
        help="Window end date (ISO 8601, e.g. 2026-04-01)",
    )
    parser.add_argument("--clickhouse-host", default="localhost")
    parser.add_argument("--clickhouse-port", type=int, default=8123)
    parser.add_argument("--clickhouse-user", default="default")
    parser.add_argument("--clickhouse-password", default="")
    parser.add_argument("--clickhouse-db", default="aer_gold")
    parser.add_argument(
        "--dry-run", action="store_true",
        help="Print computed baselines without inserting.",
    )
    return parser.parse_args()


def main() -> None:
    args = parse_args()

    window_start = datetime.fromisoformat(args.start).replace(tzinfo=timezone.utc)
    window_end = datetime.fromisoformat(args.end).replace(tzinfo=timezone.utc)

    if window_start >= window_end:
        print("Error: --start must be before --end", file=sys.stderr)
        sys.exit(1)

    client = clickhouse_connect.get_client(
        host=args.clickhouse_host,
        port=args.clickhouse_port,
        username=args.clickhouse_user,
        password=args.clickhouse_password,
        database=args.clickhouse_db,
    )

    print(
        f"Computing baselines for window [{window_start.date()} .. {window_end.date()})..."
    )

    compute_date = datetime.now(timezone.utc)
    rows = compute_baseline_rows(client, window_start, window_end, compute_date)

    if not rows:
        print("No data found for the specified window. Nothing to insert.")
        return

    for row in rows:
        metric_name, source, language, baseline_value, baseline_std = row[0:5]
        n_docs = row[7]
        print(
            f"  {metric_name:30s} | {source:20s} | {language:5s} | "
            f"mean={baseline_value:8.4f}  std={baseline_std:8.4f}  n={n_docs}"
        )

    if args.dry_run:
        print(f"\nDry run: {len(rows)} baseline(s) computed, not inserted.")
        return

    client.insert(
        "aer_gold.metric_baselines",
        rows,
        column_names=BASELINE_COLUMNS,
    )
    print(f"\nInserted {len(rows)} baseline(s) into aer_gold.metric_baselines.")


if __name__ == "__main__":
    main()
