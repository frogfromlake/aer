-- Migration 007: Create metric_baselines table.
-- Phase 65: Cross-Cultural Comparability Infrastructure (WP-004).
--
-- Stores per-(metric, source, language) baseline statistics for z-score
-- normalization. Populated offline by scripts/compute_baselines.py.
-- Uses ReplacingMergeTree to keep only the latest computation per key.

CREATE TABLE IF NOT EXISTS aer_gold.metric_baselines
(
    metric_name    String,
    source         String,
    language       String,
    baseline_value Float64,
    baseline_std   Float64,
    window_start   DateTime,
    window_end     DateTime,
    n_documents    UInt32,
    compute_date   DateTime
)
ENGINE = ReplacingMergeTree(compute_date)
ORDER BY (metric_name, source, language);
