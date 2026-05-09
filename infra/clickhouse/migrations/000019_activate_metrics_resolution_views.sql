-- Migration 019: Activate the deferred multi-resolution materialized views.
-- Phase 122c — WP-005 §5.4 Tiered Retention activation.
--
-- Phase 66 prepared the MV scaffolding in
-- `000009_metrics_resolution_views.sql` as commented-out CREATE statements
-- with three explicit activation criteria:
--   (a) p95 GetMetrics latency exceeds the 1.5 s SLO; OR
--   (b) the BFF row-cap multiplier truncates result sets; OR
--   (c) raw scans exceed 10⁸ rows per typical request.
-- All three are met by the Phase 122 web-crawl horizon (Phase 122b's
-- `time_window_days = 1825`): a per-document scan over 5 years of
-- aer_gold.metrics at any aggregate-source query approaches the 10⁸-row
-- threshold; the existing 365-day TTL on the raw table silently
-- truncates daily/monthly queries beyond a year; and the row-cap
-- multiplier on monthly resolution (8640× the 5-min baseline) already
-- returns truncated series under realistic Probe 0 volumes.
--
-- This migration:
--   1. Creates the three MVs (hourly, daily, monthly) with the exact
--      shape the Phase-66 scaffold prescribed — same engine
--      (AggregatingMergeTree), same ORDER BY tuple, same TTL anchors.
--      The 000009 file is preserved verbatim as the audit-trail record
--      of the deferred design.
--   2. Backfills any rows already in aer_gold.metrics into the MVs via
--      INSERT INTO ... SELECT. After a Phase-120b reset both source and
--      MVs are empty so the backfills are no-ops; on a non-reset upgrade
--      path they populate the MVs from existing source data. The
--      `infra/clickhouse/migrate.sh` tracker ensures this migration runs
--      at most once per deployment, so the backfills are not gated on
--      content beyond what the tracker provides.
--
-- After activation, the BFF GetMetrics handler routes per resolution:
--   5min    → aer_gold.metrics            (raw, 365 d TTL)
--   hourly  → aer_gold.metrics_hourly     (365 d TTL on bucket)
--   daily   → aer_gold.metrics_daily      (1825 d TTL on bucket)
--   weekly  → aer_gold.metrics_daily      (toStartOfWeek rebucket)
--   monthly → aer_gold.metrics_monthly    (no TTL — multi-decade Episteme)
--
-- The raw-table (aer_gold.metrics) TTL is unchanged at 365 days. WP-005
-- §5.4 prescribes 0–30 d at full resolution and 30–365 d at hourly; the
-- current 365 d on raw is a deliberate over-provisioning relative to the
-- WP, providing finer granularity for the recent year than strictly
-- required. Storage is bounded (the worker emits ~10 metric rows per
-- article and Phase 122b's 5-year horizon caps the corpus growth) — see
-- Arc42 §8.8 for the per-layer footprint estimate.
--
-- Note on MV semantics under ReplacingMergeTree: aer_gold.metrics is
-- ReplacingMergeTree(ingestion_version), but a MV triggers on every
-- INSERT (pre-merge), so re-ingestion of the same row with a higher
-- ingestion_version accumulates `countMerge()` inflation. `avgMerge()`
-- over identical values is unaffected because adding the same value
-- twice does not shift the mean. Re-ingestion is exceptional in AĒR
-- (the worker uses ingestion_version for rare reprocessing, not routine
-- updates); the bounded count inflation is documented in Arc42 §8.13.

-- ---------------------------------------------------------------------
-- Hourly bucket — 365-day TTL.
-- Source for queries with `?resolution=hourly`.
-- ---------------------------------------------------------------------
CREATE MATERIALIZED VIEW IF NOT EXISTS aer_gold.metrics_hourly
ENGINE = AggregatingMergeTree
ORDER BY (bucket, source, metric_name)
TTL bucket + INTERVAL 365 DAY
AS SELECT
    toStartOfHour(timestamp) AS bucket,
    source,
    metric_name,
    avgState(value)          AS value_avg_state,
    countState()             AS sample_count_state
FROM aer_gold.metrics
GROUP BY bucket, source, metric_name;

-- ---------------------------------------------------------------------
-- Daily bucket — 1825-day TTL (5 years, WP-005 Scale-5 cultural drift).
-- Source for queries with `?resolution=daily` AND `?resolution=weekly`
-- (weekly rebuckets the daily MV via toStartOfWeek at query time).
-- ---------------------------------------------------------------------
CREATE MATERIALIZED VIEW IF NOT EXISTS aer_gold.metrics_daily
ENGINE = AggregatingMergeTree
ORDER BY (bucket, source, metric_name)
TTL bucket + INTERVAL 1825 DAY
AS SELECT
    toStartOfDay(timestamp) AS bucket,
    source,
    metric_name,
    avgState(value)         AS value_avg_state,
    countState()            AS sample_count_state
FROM aer_gold.metrics
GROUP BY bucket, source, metric_name;

-- ---------------------------------------------------------------------
-- Monthly bucket — no TTL. Multi-decade Episteme retention; WP-005 row 3.
-- Source for queries with `?resolution=monthly`.
-- ---------------------------------------------------------------------
CREATE MATERIALIZED VIEW IF NOT EXISTS aer_gold.metrics_monthly
ENGINE = AggregatingMergeTree
ORDER BY (bucket, source, metric_name)
AS SELECT
    toStartOfMonth(timestamp) AS bucket,
    source,
    metric_name,
    avgState(value)           AS value_avg_state,
    countState()              AS sample_count_state
FROM aer_gold.metrics
GROUP BY bucket, source, metric_name;

-- ---------------------------------------------------------------------
-- Backfills. After a Phase-120b reset these are no-ops; on a non-reset
-- upgrade path they populate the MVs from existing source data. The
-- migrate.sh tracker prevents re-application.
-- ---------------------------------------------------------------------
INSERT INTO aer_gold.metrics_hourly
SELECT
    toStartOfHour(timestamp) AS bucket,
    source,
    metric_name,
    avgState(value),
    countState()
FROM aer_gold.metrics
GROUP BY bucket, source, metric_name;

INSERT INTO aer_gold.metrics_daily
SELECT
    toStartOfDay(timestamp) AS bucket,
    source,
    metric_name,
    avgState(value),
    countState()
FROM aer_gold.metrics
GROUP BY bucket, source, metric_name;

INSERT INTO aer_gold.metrics_monthly
SELECT
    toStartOfMonth(timestamp) AS bucket,
    source,
    metric_name,
    avgState(value),
    countState()
FROM aer_gold.metrics
GROUP BY bucket, source, metric_name;
