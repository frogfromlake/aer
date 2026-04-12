-- Migration 007: Add documentation_url column to sources table.
-- Phase 67: Reflexive Architecture — Methodological Transparency (WP-006).
--
-- Links each data source to its methodology documentation (bias profile,
-- classification rationale) so downstream consumers can resolve provenance
-- without out-of-band context. The BFF API surfaces this field via
-- GET /api/v1/sources.

ALTER TABLE sources
    ADD COLUMN IF NOT EXISTS documentation_url VARCHAR(255);

-- Seed documentation URLs for the Probe 0 RSS sources. The referenced file
-- is created in Phase 64 (probe0_bias_profile.md).
UPDATE sources
   SET documentation_url = 'docs/methodology/probe0_bias_profile.md'
 WHERE name IN ('bundesregierung', 'tagesschau');
