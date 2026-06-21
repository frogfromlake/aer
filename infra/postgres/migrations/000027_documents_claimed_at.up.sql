-- Phase 148 / SR-8 — timestamp the atomic processing-claim (A27).
--
-- try_claim_document CAS-transitions a document to 'processing', and the only
-- release path runs in the worker's Python except-handler. A hard kill
-- (OOM/SIGKILL/segfault) never executes that except, so the row stays
-- 'processing' forever; the next NATS redelivery sees 'processing', treats it
-- as a duplicate, and ACKs the message — silent permanent analytical data loss.
--
-- `claimed_at` lets a periodic stale-processing reaper detect rows stranded
-- past a threshold (N x ack_wait) and recover them. The partial index keeps
-- that scan cheap by indexing only the in-flight minority, never the
-- processed/quarantined bulk.
ALTER TABLE documents ADD COLUMN IF NOT EXISTS claimed_at TIMESTAMP WITH TIME ZONE;

CREATE INDEX IF NOT EXISTS idx_documents_processing_claimed_at
    ON documents (claimed_at)
    WHERE status = 'processing';
