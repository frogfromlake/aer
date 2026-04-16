-- Migration 009: Add index on documents.job_id
--
-- The job_id foreign key (Migration 001) had no index. Queries joining
-- documents by job_id (e.g. listing all documents for a job in the
-- ingestion service) trigger sequential scans as the table grows.

CREATE INDEX IF NOT EXISTS idx_documents_job_id ON documents (job_id);
