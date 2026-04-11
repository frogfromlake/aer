#!/usr/bin/env python3
"""Compute metric baselines for z-score normalization.

Standalone script (not part of the real-time pipeline) that queries
aer_gold.metrics joined with aer_gold.language_detections for a specified
time window, computes mean and standard deviation per (metric_name, source,
language), and inserts results into aer_gold.metric_baselines.

Intended to be run periodically (weekly/monthly) by a researcher or cron job.

Usage:
    python scripts/compute_baselines.py \
        --start 2026-01-01 --end 2026-04-01 \
        --clickhouse-host localhost --clickhouse-port 8123
"""

import argparse
import sys
from datetime import datetime, timezone

import clickhouse_connect


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


BASELINE_QUERY = """
SELECT
    m.metric_name   AS metric_name,
    m.source         AS source,
    ld.detected_language AS language,
    avg(m.value)     AS baseline_value,
    stddevPop(m.value) AS baseline_std,
    count()          AS n_documents
FROM aer_gold.metrics AS m
INNER JOIN aer_gold.language_detections AS ld
    ON m.article_id = ld.article_id AND ld.rank = 1
WHERE m.timestamp >= {start:DateTime}
  AND m.timestamp <  {end:DateTime}
GROUP BY metric_name, source, language
HAVING n_documents >= 2
ORDER BY metric_name, source, language
"""


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

    result = client.query(
        BASELINE_QUERY,
        parameters={"start": window_start, "end": window_end},
    )

    if not result.result_rows:
        print("No data found for the specified window. Nothing to insert.")
        return

    compute_date = datetime.now(timezone.utc)
    rows = []
    for metric_name, source, language, baseline_value, baseline_std, n_docs in result.result_rows:
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
        column_names=[
            "metric_name", "source", "language",
            "baseline_value", "baseline_std",
            "window_start", "window_end",
            "n_documents", "compute_date",
        ],
    )
    print(f"\nInserted {len(rows)} baseline(s) into aer_gold.metric_baselines.")


if __name__ == "__main__":
    main()
