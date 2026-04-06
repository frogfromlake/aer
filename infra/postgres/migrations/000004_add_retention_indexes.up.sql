-- Migration 004: Add indexes for PostgreSQL retention policy
--
-- AĒR PostgreSQL retention policy (implemented in Phase 52):
--   documents      — 90 days, matching MinIO bronze ILM (orphan prevention)
--   ingestion_jobs — 90 days, completed/failed jobs with no remaining documents
--
-- Retention cleanup is performed by the ingestion-api background goroutine
-- (startRetentionCleanup in cmd/api/main.go). Indexes support efficient
-- time-based DELETE queries without full-table scans.

CREATE INDEX IF NOT EXISTS idx_documents_ingested_at ON documents (ingested_at);
CREATE INDEX IF NOT EXISTS idx_ingestion_jobs_started_at ON ingestion_jobs (started_at);
