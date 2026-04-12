-- Revert Probe 0 documentation_url to the pre-Phase-70 single-file pointer.
-- The referenced file no longer exists on disk after Phase 70; this rollback
-- only restores the column value, not the file.

UPDATE sources
   SET documentation_url = 'docs/methodology/probe0_bias_profile.md'
 WHERE name IN ('bundesregierung', 'tagesschau');
