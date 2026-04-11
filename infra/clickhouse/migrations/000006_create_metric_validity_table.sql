-- Migration 006: Create metric_validity table.
-- Phase 63: Metric Validity Infrastructure (WP-002).
--
-- Stores validation metadata for metrics. Initially empty — populated
-- when interdisciplinary validation studies are conducted.
-- Uses ReplacingMergeTree to keep only the latest validation per metric+context.

CREATE TABLE IF NOT EXISTS aer_gold.metric_validity
(
    metric_name      String,
    context_key      String,
    validation_date  DateTime,
    alpha_score      Float32,
    correlation      Float32,
    n_annotated      UInt32,
    error_taxonomy   String,
    valid_until      DateTime
)
ENGINE = ReplacingMergeTree(validation_date)
ORDER BY (metric_name, context_key);
