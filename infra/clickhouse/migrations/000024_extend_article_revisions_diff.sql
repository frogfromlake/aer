-- Migration 024 — Silent-Edit Diff Substance: extend article_revisions.
-- Phase 122d.1 / ADR-032 amendment.
--
-- Phase 122d.0 (migration 000023) made the EXISTENCE of revisions
-- queryable. This migration makes the SUBSTANCE of each revision pair
-- queryable — paragraph-level diffs between consecutive snapshots, plus
-- the headline-change signal extracted from the title chain.
--
-- Additive only — no ORDER BY change, no engine change, no breaking
-- semantics. The Phase-122d.0 row shape is fully preserved; rows
-- written before this migration ran simply have empty defaults for the
-- new columns (the worker stores `''` / `false` / `[]` until the
-- snapshot fetcher backfills them on a subsequent re-extraction pass).
--
-- Column choices
-- --------------
-- * `diff_paragraphs Array(String)` — per-snapshot-pair list of JSON-
--   encoded ops `{op, before, after}`. JSON inside Array(String) trades
--   schema rigidity for evolution flexibility (the op vocabulary may
--   grow: add/del/modify today; potentially split/merge later); the
--   ReplacingMergeTree key tuple is unaffected. LZ4 column compression
--   (ClickHouse default) keeps storage bounded at ~~30% of raw JSON.
--   Per ADR-028, Bronze is the immutable archive — the diff is a
--   derived projection, regenerable from the Wayback HTML.
--
-- * `headline_changed Bool` — boolean signal extracted from the
--   article's title chain (`<title>` / `og:title` / `<h1>`) between
--   the two snapshots. WP-003 §5 frames "coordinated cross-source
--   rephrasings and silent post-hoc edits" as the load-bearing
--   silent-edit signal; the headline element is engineering-
--   selected as the highest-cardinality semantic position within
--   that vector (NOT a methodological canon — see ADR-037 forthcoming
--   in Phase 122d.2 for the disclosure prose).
--
-- * `headline_before` / `headline_after` — the raw title strings,
--   so the dashboard can render the exact change without a second
--   round-trip to MinIO/Wayback. Empty when `headline_changed=false`
--   (the worker writes the current title to both fields only when
--   they actually differ).
--
-- TTL anchors
-- -----------
-- Inherited from migration 000023 — the new columns share the row's
-- `snapshot_at + INTERVAL 365 DAY` TTL.

-- ``archive_url`` was an oversight in migration 000023: Phase 122d.0
-- captured ``archive_url`` on the WebMeta envelope (Silver) but did NOT
-- promote it to Gold, so the diff sweep loop (which reads only Gold)
-- has no way to fetch the snapshot HTML. Add it here additively; the
-- worker writes it forward on every new harmonisation, and existing
-- 122d.0 rows simply have an empty default until the next reprocess
-- (graceful — sweep skips rows whose archive_url is empty).
ALTER TABLE aer_gold.article_revisions
    ADD COLUMN IF NOT EXISTS archive_url String
        DEFAULT '';

ALTER TABLE aer_gold.article_revisions
    ADD COLUMN IF NOT EXISTS diff_paragraphs Array(String)
        DEFAULT [];

ALTER TABLE aer_gold.article_revisions
    ADD COLUMN IF NOT EXISTS headline_changed Bool
        DEFAULT false;

ALTER TABLE aer_gold.article_revisions
    ADD COLUMN IF NOT EXISTS headline_before String
        DEFAULT '';

ALTER TABLE aer_gold.article_revisions
    ADD COLUMN IF NOT EXISTS headline_after String
        DEFAULT '';
