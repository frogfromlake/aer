-- Migration 015: Phase 122 — Probe 0 RSS → web crawl migration.
--
-- The two Probe 0 sources (`bundesregierung`, `tagesschau`) keep their
-- name and identity; only the collection method changes. Source rows
-- transition from `type = 'rss'` to `type = 'web'` so the analysis-worker
-- AdapterRegistry routes their Bronze documents through `WebAdapter`
-- (source_type="web") instead of `RssAdapter`.
--
-- The Bronze key prefix changes from `rss/...` to `web/...` at cutover;
-- the existing 90-day Bronze TTL window will age out the residual rss/
-- objects without further intervention. See ROADMAP.md Phase 122.

UPDATE sources
   SET type = 'web'
 WHERE name IN ('bundesregierung', 'tagesschau')
   AND type = 'rss';
