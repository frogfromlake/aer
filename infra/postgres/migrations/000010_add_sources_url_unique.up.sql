-- Migration 010: Add unique constraint on sources.url
--
-- Without a unique constraint, two crawlers registering the same source URL
-- create silent duplicates that fragment metrics by source_id.
--
-- Before applying to a production database with existing data, verify no
-- duplicates exist:
--   SELECT url, COUNT(*) FROM sources GROUP BY url HAVING COUNT(*) > 1;

CREATE UNIQUE INDEX IF NOT EXISTS idx_sources_url_unique ON sources (url);
