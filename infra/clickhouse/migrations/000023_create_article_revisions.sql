-- Migration 023 — Silent-Edit Observability analytical table.
-- Phase 122d.0 / ADR-032.
--
-- The Wayback Machine CDX API tells us when an article was archived
-- and at what content digest. The publisher's own sitemap-lastmod tells
-- us when the article was re-listed even if the body did not change.
-- Both signals — independent third-party (CDX) and first-party
-- (sitemap) — feed this analytical table so the dashboard can answer
-- "which source edits how often, when, and by what mechanism" without
-- participating in the AI-text-detection arms race (WP-003 §5).
--
-- Spec vs. reality (Implementation Protocol Rule 2)
-- -------------------------------------------------
-- The ROADMAP entry for Phase 122d.0 lists `probe` as a column. The
-- established Gold-layer pattern in `aer_gold.metrics`, `aer_gold.entities`,
-- and `aer_gold.metadata_coverage` does not carry a `probe` column —
-- the probe→source mapping is BFF-configured (`configs/probes/*.yaml`)
-- and resolved at query time. We follow that pattern here to avoid
-- a parallel resolution path: the BFF aggregates over the probe's
-- source set via the same join that drives every other Gold endpoint.
--
-- `revision_trigger` (Phase 131a artefact reconciliation)
-- -------------------------------------------------------
-- ROADMAP 122d.0 calls out a Phase-131a UX/conceptual artefact: the
-- crawler's `time_window_days` filters discovery (sitemap-lastmod /
-- RSS pubDate), but an article's stored `published_date` can be far
-- older when the publisher re-listed it. A re-listed old article is
-- itself a silent-edit signal — not a stale article slipping past the
-- 7-day window. The `revision_trigger` column makes the mechanism
-- queryable:
--
--   * 'cdx_snapshot'           — third-party Wayback CDX captured a
--                                content-hash transition for this URL.
--   * 'republication_trigger'  — publisher's sitemap-lastmod was bumped
--                                significantly after the article's
--                                original publication date (≥ 7 days).
--                                The worker emits one synthetic row
--                                per detected republication event so
--                                the dashboard renders re-list activity
--                                as a first-class revision signal.
--   * 'unknown'                — neither classifier fired (defensive
--                                fallback; should not occur in steady
--                                state).
--
-- Engine + idempotency
-- --------------------
-- ReplacingMergeTree(ingestion_version) collapses NATS redeliveries
-- by article_id × snapshot_at × content_hash. The (article_id,
-- snapshot_at, content_hash) ORDER BY tuple is monotone for a stable
-- corpus: re-processing the same article emits the same digest set,
-- so duplicates de-merge correctly.
--
-- TTL
-- ---
-- 365 days on `snapshot_at` matches the Gold-layer analytical horizon
-- (raw `aer_gold.metrics` TTL). Resolution MVs may carry longer
-- history in a future phase; the raw table is bounded so the
-- "revisions per source per month" panel reflects a comparable
-- window across every source.

CREATE TABLE IF NOT EXISTS aer_gold.article_revisions
(
    article_id              String,
    source                  LowCardinality(String),
    discourse_function      LowCardinality(String) DEFAULT '',
    snapshot_at             DateTime,
    content_hash            String,
    prev_content_hash       String DEFAULT '',
    revision_index          UInt32 DEFAULT 0,
    time_since_prev_hours   Float64 DEFAULT 0,
    revision_trigger        LowCardinality(String) DEFAULT 'unknown',
    ingestion_version       UInt64
)
ENGINE = ReplacingMergeTree(ingestion_version)
ORDER BY (article_id, snapshot_at, content_hash)
TTL snapshot_at + INTERVAL 365 DAY
SETTINGS non_replicated_deduplication_window = 1000;
