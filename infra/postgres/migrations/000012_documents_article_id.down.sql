-- Reverse migration 012: drop article_id from documents.

DROP INDEX IF EXISTS idx_documents_article_id;
ALTER TABLE documents DROP COLUMN IF EXISTS article_id;
