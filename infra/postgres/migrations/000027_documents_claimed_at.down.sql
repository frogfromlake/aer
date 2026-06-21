-- Reverse 000027 — drop the processing-claim timestamp + its partial index.
DROP INDEX IF EXISTS idx_documents_processing_claimed_at;
ALTER TABLE documents DROP COLUMN IF EXISTS claimed_at;
