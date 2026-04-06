-- Rollback Migration 004: Drop retention policy indexes

DROP INDEX IF EXISTS idx_documents_ingested_at;
DROP INDEX IF EXISTS idx_ingestion_jobs_started_at;
