-- Migration 012: Add article_id column to documents for L5 Evidence resolution.
-- Phase 101: Iteration 5 — Probe Dossier & Article Browsing Endpoints.
--
-- The analysis worker derives article_id = SHA-256(source + bronze_object_key)
-- when harmonizing each document into Silver (see services/analysis-worker
-- internal/models __init__.py generate_document_id). Gold rows in ClickHouse
-- key off this hash. The BFF article-detail endpoint needs the inverse map —
-- given an article_id, locate its Bronze/Silver object — so the worker now
-- persists the hash alongside the existing documents row when it commits
-- "processed". Nullable because documents in flight ("pending"/"failed")
-- have no article_id yet, and historical rows pre-dating this migration
-- carry NULL until reprocessed.

ALTER TABLE documents
    ADD COLUMN IF NOT EXISTS article_id VARCHAR(64);

CREATE INDEX IF NOT EXISTS idx_documents_article_id ON documents (article_id);
