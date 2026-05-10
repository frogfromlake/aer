-- Migration 021: Enable ClickHouse block-level insert deduplication on
-- every Silver / Gold ingestion target.
-- Phase 122e A19 / F-A19.
--
-- Iter-3 forensics surfaced a 1-off systematic over-count in
-- `aer_gold.metrics_hourly` (and by extension `metrics_daily` /
-- `metrics_monthly`) for every cross-language metric: 255 raw rows yielded
-- 256 hourly samples; 254 raw bert-multilingual rows yielded 255. The
-- German-only metrics (`entity_count`, `sentiment_score_bert_de_news`,
-- `sentiment_score_sentiws`) were exact at 115 / 115.
--
-- Root cause:
--   * `aer_gold.metrics` is `ReplacingMergeTree(ingestion_version)
--     ORDER BY (article_id, metric_name)`. Duplicate inserts on the same
--     sorting key collapse to a single row at *merge time*.
--   * `aer_gold.metrics_hourly|daily|monthly` are `AggregatingMergeTree`
--     materialised views fired *on insert*. They aggregate every block
--     ClickHouse accepts — including duplicates that ReplacingMergeTree
--     will later collapse on the raw side. The MV has no way to "undo"
--     an aggregate the duplicate already contributed.
--   * At-least-once delivery (NATS redeliver, processor retry) caused at
--     least one duplicate `(article_id, metric_name, ingestion_version)`
--     insert in iter-3. The raw table eventually deduplicates; the MV
--     stays 1-too-many.
--
-- This is a known ClickHouse footgun for AggregatingMergeTree-over-
-- ReplacingMergeTree chains. The canonical fix is *block-level insert
-- deduplication*: ClickHouse refuses a duplicate INSERT block (matched
-- by `insert_deduplication_token`) BEFORE the MV fires, eliminating the
-- over-count at the canonical layer.
--
-- This migration enables a generous window (1000 distinct insert blocks)
-- for the seven gold-side ingestion targets the worker writes to. The
-- worker (Phase 122e A19 part 2) starts emitting a stable
-- `insert_deduplication_token` per `(article_id, table, ingestion_version)`
-- on every callsite — a NATS redeliver of the same article reuses the
-- same token, and ClickHouse silently no-ops the duplicate.
--
-- Symptom-hiding via "truncate MV target tables in `make reset`" was
-- considered and rejected: it would only mask the over-count immediately
-- after a wipe; the production runtime case (any retry on a healthy stack)
-- would stay broken. The fix lives at the insert layer where the duplicate
-- arises, not in the operational reset path.

ALTER TABLE aer_silver.documents
    MODIFY SETTING non_replicated_deduplication_window = 1000;

ALTER TABLE aer_gold.metrics
    MODIFY SETTING non_replicated_deduplication_window = 1000;

ALTER TABLE aer_gold.entities
    MODIFY SETTING non_replicated_deduplication_window = 1000;

ALTER TABLE aer_gold.entity_links
    MODIFY SETTING non_replicated_deduplication_window = 1000;

ALTER TABLE aer_gold.language_detections
    MODIFY SETTING non_replicated_deduplication_window = 1000;

ALTER TABLE aer_gold.entity_cooccurrences
    MODIFY SETTING non_replicated_deduplication_window = 1000;

ALTER TABLE aer_gold.topic_assignments
    MODIFY SETTING non_replicated_deduplication_window = 1000;

ALTER TABLE aer_gold.metric_baselines
    MODIFY SETTING non_replicated_deduplication_window = 1000;
