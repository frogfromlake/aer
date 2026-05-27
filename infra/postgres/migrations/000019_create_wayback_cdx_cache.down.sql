-- Down migration for Phase 122d.0 Wayback CDX cache.
BEGIN;

DROP INDEX IF EXISTS idx_wayback_cdx_cache_fetched_at;
DROP TABLE IF EXISTS wayback_cdx_cache;

COMMIT;
