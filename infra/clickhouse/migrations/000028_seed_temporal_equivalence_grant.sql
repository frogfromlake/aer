-- Migration 028: First metric-equivalence grant — temporal Level-1.
-- Phase 124: Cross-Probe & Cross-Cultural Operations (WP-004 §6.3, Appendix B).
--
-- The metric_equivalence registry has been empty since Phase 65 (migration
-- 008) by design — equivalence is a research question granted out of band,
-- not a computation (WP-004 §2.2). This is the FIRST non-empty entry: the
-- temporal Level-1 grant for the Probe-0 (DE) × Probe-1 (FR) institutional-web
-- corpora.
--
-- Why temporal Level-1 is valid by construction (and needs no validation
-- study): publication_hour / publication_weekday are measured on clock and
-- calendar time — a culture-INDEPENDENT axis. Comparing the temporal rhythm
-- of two cultures (and z-scoring it as a rhythm/shape comparison) asserts no
-- cross-cultural intensity claim, so it is admissible the moment the two
-- calendars are structurally comparable. That calendar-parity condition is
-- documented in the matching Postgres equivalence_reviews record (migration
-- 000023) and rests on configs/cultural_calendars/{de,fr}.yaml.
--
-- Scope of the grant: temporal-pattern comparison + rhythm z-score for
-- publication_hour and publication_weekday across DE+FR. It deliberately does
-- NOT extend to intensive/scaled metrics (e.g. sentiment), whose measurement
-- axis is culture-laden and which require a separate deviation-level (Level-2)
-- review. The BFF normalization gate enforces this metric-class split.
--
-- One row per (metric, language): the cross-frame gate counts distinct
-- granted languages, so DE+FR each need a row. validated_by points to the
-- equivalence_reviews.id of the matching full review record (Postgres
-- migration 000023). validation_date is fixed so the ReplacingMergeTree
-- de-duplicates on re-application (idempotent with the migrate.sh tracker).

INSERT INTO aer_gold.metric_equivalence
    (etic_construct, metric_name, language, source_type, equivalence_level, validated_by, validation_date, confidence, notes)
VALUES
    ('temporal_rhythm', 'publication_hour',    'de', 'web', 'temporal', '1', '2026-06-03 12:00:00', 1.0, 'Temporal Level-1 (WP-004 App. B): clock/calendar time is culture-independent; DE/FR calendar parity verified. Authorises temporal-pattern comparison + rhythm z-score across DE+FR. Not for intensive metrics.'),
    ('temporal_rhythm', 'publication_hour',    'fr', 'web', 'temporal', '2', '2026-06-03 12:00:00', 1.0, 'Temporal Level-1 (WP-004 App. B): clock/calendar time is culture-independent; DE/FR calendar parity verified. Authorises temporal-pattern comparison + rhythm z-score across DE+FR. Not for intensive metrics.'),
    ('temporal_rhythm', 'publication_weekday', 'de', 'web', 'temporal', '3', '2026-06-03 12:00:00', 1.0, 'Temporal Level-1 (WP-004 App. B): clock/calendar time is culture-independent; DE/FR calendar parity verified. Authorises temporal-pattern comparison + rhythm z-score across DE+FR. Not for intensive metrics.'),
    ('temporal_rhythm', 'publication_weekday', 'fr', 'web', 'temporal', '4', '2026-06-03 12:00:00', 1.0, 'Temporal Level-1 (WP-004 App. B): clock/calendar time is culture-independent; DE/FR calendar parity verified. Authorises temporal-pattern comparison + rhythm z-score across DE+FR. Not for intensive metrics.');
