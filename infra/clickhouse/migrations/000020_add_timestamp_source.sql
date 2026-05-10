-- Migration 020: Add `timestamp_source` provenance column to Silver and Gold.
-- Phase 122e A18 / F-A18.
--
-- Iter-3 forensics surfaced a base-dataset falsity: when extraction failed
-- to resolve a publication date, the WebAdapter correctly recorded
-- `core.published_date = None`, `meta.timestamp_source = "fetch_at_fallback"`,
-- and `meta.extraction_methods.published_date = None` in the Silver MinIO
-- envelope — but the Silver→Gold loader (`silver_projection.py`) wrote
-- `core.timestamp` (which had been set to the fetch_at write-time during
-- construction) into `aer_silver.documents.timestamp` and
-- `aer_gold.metrics.timestamp`. Downstream Gold queries could not
-- distinguish "real publication date" from "we have no idea when this was
-- published" — making fetch-time-stamped rows look like 2026-05-09
-- publications even when the source page was a static archive from 2019.
--
-- This migration adds a `timestamp_source` column to both tables so the
-- provenance signal that already lives in the Silver MinIO envelope
-- becomes queryable in Gold. BFF temporal aggregations gain a
-- `WHERE timestamp_source != 'fetch_at_fallback'` filter; Phase 122f's
-- metadata-coverage endpoint reads this column as the substrate for its
-- per-source Negative-Space rendering (Brief §7.7).
--
-- The MV definitions for `metrics_hourly` / `metrics_daily` / `metrics_monthly`
-- are NOT changed — they aggregate on (bucket, source, metric_name) and do
-- not reference `timestamp_source`. Adding the column to the source table
-- is forward-compatible with the existing MV pipeline.
--
-- Allowed values (mirrors `services/analysis-worker/internal/adapters/web_extract.py`'s
-- `ALLOWED_TIMESTAMP_SOURCES` set):
--   * 'json_ld_published'
--   * 'open_graph_published'
--   * 'html_meta_published'
--   * 'sitemap_lastmod'
--   * 'http_last_modified'
--   * 'fetch_at_fallback'      ← the methodologically-honest "no idea" marker
--   * '' (empty)               ← non-web sources (RSS / legacy) that pre-date this provenance dimension

ALTER TABLE aer_silver.documents
    ADD COLUMN IF NOT EXISTS timestamp_source String DEFAULT '';

ALTER TABLE aer_gold.metrics
    ADD COLUMN IF NOT EXISTS timestamp_source String DEFAULT '';
