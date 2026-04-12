-- Migration 008: Update documentation_url for Probe 0 sources to point at
-- the probe dossier directory instead of a single bias-profile file.
-- Phase 70: Probe Dossier Pattern.
--
-- The dossier groups WP-001 classification, WP-003 bias assessment,
-- WP-005 temporal profile, and WP-006 observer-effect assessment for the
-- probe. Per-metric methodological provenance lives system-wide in
-- services/bff-api/configs/metric_provenance.yaml; per-validation context
-- lives in aer_gold.metric_validity. Neither belongs in the dossier.

UPDATE sources
   SET documentation_url = 'docs/probes/probe-0-de-institutional-rss/'
 WHERE name IN ('bundesregierung', 'tagesschau');
