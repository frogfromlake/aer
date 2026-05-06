-- Migration 014: Add notes column to aer_gold.metric_equivalence.
-- Phase 115: Iteration 5 — Cross-Cultural Analysis Foundations (WP-004).
--
-- Adds a free-form prose field for the methodological-rationale text
-- referenced by the Operations Playbook section "Granting metric equivalence
-- (WP-004 §5.2)" introduced in Phase 115. The full review record (reviewer,
-- date, working-paper anchor, full prose) lives in the Postgres
-- equivalence_reviews table; this column carries a concise summary that
-- travels with the row when it is read by the BFF, so the dashboard's
-- methodology tray can render the rationale without a cross-database join.
--
-- Type-choice rationale: String DEFAULT '', not Nullable(String). An
-- equivalence entry without a notes summary is valid (e.g. the temporal-Level
-- grant for Probe 0 × Probe 1 in Phase 126, whose rationale is fully captured
-- in WP-004 Appendix B and needs no additional paraphrase). The IF NOT EXISTS
-- guard matches the existing migration idempotency pattern.

ALTER TABLE aer_gold.metric_equivalence
    ADD COLUMN IF NOT EXISTS notes String DEFAULT '';
