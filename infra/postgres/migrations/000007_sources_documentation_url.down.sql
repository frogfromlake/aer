-- Rollback Migration 007: Drop documentation_url column from sources.

ALTER TABLE sources
    DROP COLUMN IF EXISTS documentation_url;
