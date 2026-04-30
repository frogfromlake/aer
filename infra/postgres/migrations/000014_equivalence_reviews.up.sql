-- Migration 014: equivalence_reviews workflow table.
-- Phase 115: Iteration 5 — Cross-Cultural Analysis Foundations (WP-004).
--
-- Mirrors the WP-006 §5.2 Silver-eligibility review pattern (Phase 103,
-- migration 011): each row in aer_gold.metric_equivalence is granted out of
-- band by a methodological review. The full record — reviewer, date,
-- working-paper anchor, full prose rationale — is held here, in Postgres.
-- The ClickHouse metric_equivalence row carries a concise summary in its
-- `notes` column (Phase 115 ClickHouse migration 000014) so the BFF can
-- serve the methodology-tray rationale without a cross-database join; the
-- `id` here is referenced by metric_equivalence.validated_by for
-- traceability back to the full record.
--
-- Out-of-band review only: there is no in-band UI for granting equivalence.
-- The Operations Playbook section "Granting metric equivalence (WP-004
-- §5.2)" documents the manual workflow: insert here, insert into
-- ClickHouse, document in WP-004 Appendix B.

CREATE TABLE IF NOT EXISTS equivalence_reviews (
    id                     SERIAL PRIMARY KEY,
    etic_construct         VARCHAR(255) NOT NULL,
    metric_name            VARCHAR(255) NOT NULL,
    language               VARCHAR(16)  NOT NULL,
    source_type            VARCHAR(64)  NOT NULL,
    equivalence_level      VARCHAR(32)  NOT NULL CHECK (equivalence_level IN ('temporal', 'deviation', 'absolute')),
    reviewer               VARCHAR(255) NOT NULL,
    review_date            DATE         NOT NULL,
    rationale              TEXT         NOT NULL,
    working_paper_anchor   VARCHAR(500) NOT NULL,
    notes_summary          VARCHAR(280) NOT NULL DEFAULT '',
    confidence             REAL         NOT NULL DEFAULT 0.0,
    created_at             TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS equivalence_reviews_metric_idx
    ON equivalence_reviews (metric_name, language);
CREATE INDEX IF NOT EXISTS equivalence_reviews_etic_idx
    ON equivalence_reviews (etic_construct, equivalence_level);

COMMENT ON TABLE equivalence_reviews IS
    'WP-004 §5.2 cross-cultural metric-equivalence review records. The ClickHouse aer_gold.metric_equivalence row carries a notes summary; this table holds the full review prose. Phase 115.';
COMMENT ON COLUMN equivalence_reviews.notes_summary IS
    'Concise rationale (≤280 chars) mirrored to aer_gold.metric_equivalence.notes for read-path display. Phase 115.';
