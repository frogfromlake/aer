-- Migration 025 — Wayback CDX lookup observability (silent-failure guard).
--
-- Why this table exists
-- ---------------------
-- Before this migration, the per-article Wayback CDX lookup outcome was
-- written nowhere durable: `aer_gold.article_revisions` only gets rows when
-- revisions actually exist. A lookup that did NOT complete (IA unreachable,
-- circuit breaker open, local rate-limit) produced no row and no signal — so
-- "we could not look" was indistinguishable from "no edits observed". A
-- transient IA outage therefore became permanent, SILENT, source-skewed data
-- loss (institutional/PL sources with few articles lost 100% of their lookups
-- to a flapping circuit breaker while news sources kept partial coverage).
--
-- This table records ONE row per article per ingestion, UNCONDITIONALLY, so
-- the lookup outcome is always observable and queryable per source. "0 rows
-- for source X" becomes impossible: the worker writes a row for every WebMeta
-- article it processes. The dashboard/Negative-Space surface (Phase 122d.2)
-- and operational monitoring read this to distinguish:
--
--   * 'ok'            — CDX returned snapshots.
--   * 'no_snapshots'  — CDX reached, genuinely no captures (a real "we looked,
--                       nothing there" — NOT a failure).
--   * 'failed'        — CDX request was attempted but errored (timeout, DNS,
--                       connection refused, malformed body).
--   * 'circuit_open'  — the breaker was open, so the request was skipped to
--                       protect the corpus throughput (NOT attempted).
--   * 'rate_limited'  — local token bucket denied the call (NOT attempted).
--   * 'skipped'       — no canonical_url to look up.
--   * 'disabled'      — Wayback integration disabled for this deployment.
--   * 'unknown'       — defensive fallback; should not occur in steady state.
--
-- The first two are "we know"; the rest are "we do NOT know" — and that gap
-- must always be visible (WP-003 §5.3.1 "Document, Don't Filter"; the
-- "disclose, never coerce" guardrail). `failed` / `circuit_open` /
-- `rate_limited` are split (the client previously collapsed all three to
-- `failed`) so monitoring can tell an external IA outage from our own
-- self-throttling — different fixes.
--
-- Engine + idempotency
-- --------------------
-- ReplacingMergeTree(ingestion_version) collapses NATS redeliveries by
-- (source, article_id): re-processing the same article replaces its prior
-- row with the latest ingestion's outcome. A future re-crawl (the recovery
-- path — there is no in-place backfill) overwrites stale 'failed' rows with
-- fresh outcomes.
--
-- TTL
-- ---
-- 365 days on `looked_up_at`, matching the Gold analytical horizon
-- (`aer_gold.metrics`, `aer_gold.article_revisions`).

CREATE TABLE IF NOT EXISTS aer_gold.wayback_lookups
(
    article_id          String,
    source              LowCardinality(String),
    canonical_url       String DEFAULT '',
    status              LowCardinality(String) DEFAULT 'unknown',
    looked_up_at        DateTime,
    ingestion_version   UInt64
)
ENGINE = ReplacingMergeTree(ingestion_version)
ORDER BY (source, article_id)
TTL looked_up_at + INTERVAL 365 DAY
SETTINGS non_replicated_deduplication_window = 1000;
