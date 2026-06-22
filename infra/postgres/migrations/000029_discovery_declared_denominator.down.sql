-- Reverse Phase 148d (WP-007) declared-denominator columns.
BEGIN;

ALTER TABLE crawler_discovery_runs
    DROP COLUMN IF EXISTS declared,
    DROP COLUMN IF EXISTS declared_indeterminate;

COMMIT;
