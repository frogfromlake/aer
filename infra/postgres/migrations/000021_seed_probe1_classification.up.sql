-- Migration 021: Seed Probe 1 etic/emic classifications (WP-001 §3, §6) — Phase 123.
--
-- Discourse-function classification is DB-driven (this table) — the established
-- pattern from Probe-0 migration 006. (There is no `discourse_function_rules.yaml`;
-- per-article URL-rule discourse-function classification is Phase 122a, deferred
-- per ADR-030.) Mirrors Probe-0's EA+PL coverage so the two probes compose as
-- parallel streams with Cohesion/Identity and Subversion/Friction structurally
-- unobserved on both — the symmetry the Phase-124 equivalence grant requires.
--
-- Qualitative justification:
--   - franceinfo: Epistemic Authority (primary) — France Télévisions public
--     broadcaster setting the informational baseline. Power Legitimation
--     (secondary) — public-service governance structurally shapes editorial
--     framing. Mirrors tagesschau's classification.
--   - elysee: Power Legitimation (primary) — official head-of-state
--     communication with structural agenda-setting. Epistemic Authority
--     (secondary) — authoritative by institutional position. Mirrors
--     bundesregierung's classification. NOTE: the PL *locus* differs across
--     the polities — head of STATE (Élysée) in the French semi-presidential
--     system vs. head of GOVERNMENT (Bundesregierung / BPA) in Germany. This
--     asymmetry is a documented cross-cultural parameter (WP-004), surfaced in
--     the dossier and the per-source content cards, not a defect.
--
-- function_weights intentionally NULL — quantification requires the WP-001
-- §4.4 classification process (area expert nomination + peer review), not yet
-- executed. review_status = 'provisional_engineering'. Every Probe-1 metric
-- reports validation_status = unvalidated.

INSERT INTO source_classifications (
    source_id, primary_function, secondary_function, function_weights,
    emic_designation, emic_context, emic_language,
    classified_by, classification_date, review_status
)
SELECT
    id,
    'epistemic_authority',
    'power_legitimation',
    NULL,  -- function_weights intentionally NULL. Quantification requires WP-001 §4.4.
    'franceinfo',
    'Public broadcaster (France Télévisions / franceinfo). Norm-setting through the informational baseline; editorial independence under a public-service mandate.',
    'fr',
    'WP-001/Probe-1',
    '2026-05-29',
    'provisional_engineering'
FROM sources WHERE name = 'franceinfo'
ON CONFLICT DO NOTHING;

INSERT INTO source_classifications (
    source_id, primary_function, secondary_function, function_weights,
    emic_designation, emic_context, emic_language,
    classified_by, classification_date, review_status
)
SELECT
    id,
    'power_legitimation',
    'epistemic_authority',
    NULL,  -- function_weights intentionally NULL. Quantification requires WP-001 §4.4.
    'Élysée (Présidence de la République)',
    'Official communication channel of the President of the Republic. Structural power legitimation through head-of-state agenda-setting and framing in the French semi-presidential system.',
    'fr',
    'WP-001/Probe-1',
    '2026-05-29',
    'provisional_engineering'
FROM sources WHERE name = 'elysee'
ON CONFLICT DO NOTHING;
