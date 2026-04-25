-- Migration 011: Add Silver-layer eligibility flag and review metadata to sources.
-- Phase 101: Iteration 5 — Probe Dossier & Article Browsing Endpoints (ADR-020 §Backend-Work).
--
-- Silver exposes raw cleaned text. Unlike Gold, which is already aggregated and
-- k-anonymity-safe by construction, Silver-level access for a source requires an
-- explicit eligibility flag with WP-006 §5.2 review metadata recorded alongside.
-- Probe 0's two sources are seeded as auto-eligible per Manifesto §VI and
-- WP-006 §7 — institutional public data, government/public-broadcaster RSS,
-- no re-identification risk. All other sources default to false; review
-- mutations happen via dedicated migrations (one-off per review).

ALTER TABLE sources
    ADD COLUMN IF NOT EXISTS silver_eligible BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS silver_review_reviewer VARCHAR(255),
    ADD COLUMN IF NOT EXISTS silver_review_date DATE,
    ADD COLUMN IF NOT EXISTS silver_review_rationale TEXT,
    ADD COLUMN IF NOT EXISTS silver_review_reference VARCHAR(500);

UPDATE sources
   SET silver_eligible = true,
       silver_review_reviewer = 'auto-eligible (Probe 0 baseline)',
       silver_review_date = DATE '2026-04-25',
       silver_review_rationale = 'Probe 0 — institutional public data, government/public-broadcaster RSS, no re-identification risk per Manifesto §VI and WP-006 §7. Auto-eligible.',
       silver_review_reference = 'docs/arc42/09_architecture_decisions.md#adr-020'
 WHERE name IN ('tagesschau', 'bundesregierung');
