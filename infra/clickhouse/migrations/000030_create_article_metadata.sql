-- Migration 030: Categorical article-metadata values promoted to Gold.
-- Phase 133 (Slice 2) / WP-003 §3.2.
--
-- Phase 133 makes the already-extracted WebMeta steckbrief VALUES analytically
-- queryable. Scalar metadata (paywall_status, image_count, …) ride the existing
-- `aer_gold.metrics` rails as new metric_names (Slice 1). CATEGORICAL metadata
-- (section, author, tags, categories, article_type, editorial_labels,
-- dateline_location, editor) cannot — `metrics.value` is Float64 and "Politik"
-- is not a number. This table holds those categorical VALUES so they can be
-- analysed as grouping/measured dimensions (e.g. article count per Ressort,
-- mean sentiment by section).
--
-- Shape: ONE row per (article, field), with `value` an Array(String) carrying
-- every value for that field (a one-element array for scalar fields like
-- section/author; the full list for tags/categories/editorial_labels). The
-- categorical-distribution read (Slice 3) does
-- `arrayJoin(value) AS v … GROUP BY v, uniqExact(article_id)` so each value is
-- counted by distinct article and a duplicate element within one article never
-- double-counts.
--
-- Why an Array column (not one row per value): `value` is therefore NOT in the
-- ORDER BY key, so a re-ingest of the same (source, field, article_id) at a
-- higher ingestion_version OVERWRITES the prior array wholesale under
-- ReplacingMergeTree. A re-ingest that CHANGES the field — a corrected scalar
-- (section 'Politik' → 'Inland') or a removed list element while ≥1 value
-- remains — cleanly replaces the old row: no stale value rows, no
-- double-attribution of one article to two mutually-exclusive scalar values.
-- (A one-row-per-value layout keyed on `value` would leave the old value's row
-- behind on change, the way `aer_gold.entities` does; categorical scalars are
-- single-valued, so that staleness would corrupt their distribution. The Array
-- layout avoids it.)
--
-- One residual case is NOT self-healing: if a re-ingest drops the field
-- ENTIRELY (no values at all), disclose-never-coerce writes NO row, so there is
-- nothing to supersede the prior version's row and the old value lingers until
-- TTL or `make reset`. This is the same bounded staleness every Gold projection
-- carries under graceful degradation (an extractor that stops producing leaves
-- its last rows in `metrics`/`entities` too) — the worker processes one article
-- in isolation and cannot cheaply know a field WAS present before. Deterministic
-- replay from Bronze + the periodic reset bound it; it is not a fresh defect of
-- this table, and full-field removal on re-archival is rare in practice.
--
-- Disclose-never-coerce (WP-003 §3.2): the worker writes a row ONLY when the
-- field has at least one non-empty value. An absent/empty categorical field
-- produces NO row — its absence is surfaced as Negative Space via
-- `aer_gold.metadata_coverage` (Phase 122f), never as an empty-string value.
-- The availability gate (Slice 3) reads that coverage signal, so a field with
-- no rows here is simply never offered in the picker for that scope.
--
-- No materialized view: categorical distribution is a top-N over the active
-- window, not a time-bucketed aggregate, so the resolution-MV pattern
-- (`metrics_hourly/daily/monthly`) does not apply.
--
-- `non_replicated_deduplication_window` matches the sibling Gold projection
-- tables (metadata_coverage 000022, article_revisions 000023, wayback_lookups
-- 000025) so the worker's `deduplication_token` actually suppresses NATS
-- redelivery / processor-retry double inserts (the default window is 0 =
-- disabled, which would make the token a silent no-op).
--
-- ORDER BY (source, field, article_id): the load-bearing read filters by source
-- + field, and panels always scope by source — the (source, field) prefix makes
-- it a primary-key range scan; article_id completes the per-article dedup key.
-- TTL matches `aer_gold.metrics` (365 d on the article's published timestamp).

CREATE TABLE IF NOT EXISTS aer_gold.article_metadata
(
    timestamp          DateTime,
    source             LowCardinality(String),
    article_id         String,
    field              LowCardinality(String),
    value              Array(String),
    discourse_function String DEFAULT '',
    timestamp_source   String DEFAULT '',
    ingestion_version  UInt64 DEFAULT 0
)
ENGINE = ReplacingMergeTree(ingestion_version)
ORDER BY (source, field, article_id)
TTL timestamp + INTERVAL 365 DAY
SETTINGS non_replicated_deduplication_window = 1000;
