-- Down migration for Phase 122g discovery-telemetry tables.
BEGIN;

DROP INDEX IF EXISTS idx_crawler_discovery_alerts_last;
DROP TABLE IF EXISTS crawler_discovery_alerts;

DROP INDEX IF EXISTS idx_crawler_discovery_runs_source_channel;
DROP INDEX IF EXISTS idx_crawler_discovery_runs_source_run;
DROP TABLE IF EXISTS crawler_discovery_runs;

COMMIT;
