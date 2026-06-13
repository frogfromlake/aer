-- Reverse migration 026: drop the Phase 135 saved-analyses tables.
-- shares FK to saved_analyses, so it goes first.

DROP INDEX IF EXISTS saved_analysis_shares_grantee_idx;
DROP TABLE IF EXISTS saved_analysis_shares;

DROP INDEX IF EXISTS saved_analyses_owner_idx;
DROP TABLE IF EXISTS saved_analyses;
