-- Migration 002: Extend aer_gold.metrics with dimension columns (D-7)
--
-- Adds source, metric_name, and article_id to support filtering
-- by data origin, metric type, and individual articles.

ALTER TABLE aer_gold.metrics ADD COLUMN IF NOT EXISTS source String DEFAULT '';
ALTER TABLE aer_gold.metrics ADD COLUMN IF NOT EXISTS metric_name String DEFAULT '';
ALTER TABLE aer_gold.metrics ADD COLUMN IF NOT EXISTS article_id Nullable(String);
