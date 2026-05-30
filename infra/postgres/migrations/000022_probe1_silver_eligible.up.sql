-- Migration 022: Grant Silver-layer eligibility to Probe 1 sources (Phase 123).
--
-- Same WP-006 §7 source-class rationale as Probe 0 (migration 011):
-- institutional public data (public broadcaster + head-of-state
-- communication), no re-identification risk per Manifesto §VI. The
-- silver_eligible columns were added by migration 011; this migration only
-- sets the flag + review metadata for the two Probe-1 sources.

UPDATE sources
   SET silver_eligible = true,
       silver_review_reviewer = 'auto-eligible (Probe 1 baseline)',
       silver_review_date = DATE '2026-05-29',
       silver_review_rationale = 'Probe 1 — institutional public data (France Télévisions public broadcaster + Présidence de la République), no re-identification risk per Manifesto §VI and WP-006 §7. Auto-eligible.',
       silver_review_reference = 'docs/probes/probe-1-fr-institutional-web/observer_effect.md'
 WHERE name IN ('franceinfo', 'elysee');
