-- Migration 006: Seed Probe 0 etic/emic classifications (WP-001 §3, §6)
--
-- Qualitative justification for primary/secondary function assignments:
--   - tagesschau.de: Epistemic Authority (primary) — state-funded public broadcaster
--     that sets the informational baseline. Power Legitimation (secondary) — editorial
--     independence structurally influenced by inter-party proportional governance.
--   - bundesregierung.de: Power Legitimation (primary) — official government
--     communication channel with structural agenda-setting. Epistemic Authority
--     (secondary) — authoritative source by institutional position.
--
-- function_weights intentionally NULL. Quantification requires WP-001 §4.4
-- classification process (Steps 1-2: area expert nomination and peer review).
-- See docs/scientific_operations_guide.md.

INSERT INTO source_classifications (
    source_id, primary_function, secondary_function, function_weights,
    emic_designation, emic_context, emic_language,
    classified_by, classification_date, review_status
)
SELECT
    id,
    'epistemic_authority',
    'power_legitimation',
    NULL,  -- function_weights intentionally NULL. Quantification requires WP-001 §4.4 classification process. See docs/scientific_operations_guide.md.
    'Tagesschau',
    'State-funded public broadcaster (ARD). Norm-setting through informational baseline. Editorial independence structurally influenced by inter-party proportional governance.',
    'de',
    'WP-001/Probe-0',
    '2026-04-11',
    'provisional_engineering'
FROM sources WHERE name = 'tagesschau'
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
    NULL,  -- function_weights intentionally NULL. Quantification requires WP-001 §4.4 classification process. See docs/scientific_operations_guide.md.
    'Bundesregierung',
    'Official government communication channel. Structural power legitimation through agenda-setting and framing.',
    'de',
    'WP-001/Probe-0',
    '2026-04-11',
    'provisional_engineering'
FROM sources WHERE name = 'bundesregierung'
ON CONFLICT DO NOTHING;
