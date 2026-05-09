-- Migration 017 (down): drop the web-crawler dedup state table.

DROP INDEX IF EXISTS crawler_state_source_last_fetched_idx;
DROP TABLE IF EXISTS crawler_state;
