-- Migration 013: Add bronze_object_key to aer_silver.documents.
-- Phase 113b / ADR-022: Article Resolution SoT moves to the analytical layer.
--
-- The BFF previously resolved (article_id) → (bronze_object_key, source) via
-- PostgreSQL `documents`, which is pruned at 90 days. Silver/Gold retain for
-- 365 days, so between days 90 and 365 the lookup 404s on articles whose
-- analytical record is still live. Adding bronze_object_key here makes
-- aer_silver.documents self-sufficient for resolution: it is already
-- one-row-per-document, already 365-day TTL, and already written by the
-- worker at the same point Silver is uploaded to MinIO.
--
-- Existing rows backfill to the empty string (column DEFAULT). The worker
-- repopulates on the next reprocess of any redelivered event, and the
-- one-shot reconcile_documents script writes the column for historical
-- articles whose Silver envelope is recoverable from MinIO.

ALTER TABLE aer_silver.documents
    ADD COLUMN IF NOT EXISTS bronze_object_key String DEFAULT '';
