-- Migration 023: First metric-equivalence review — temporal Level-1 grant.
-- Phase 124: Cross-Probe & Cross-Cultural Operations (WP-004 §6.3, Appendix B).
--
-- The full system-of-record for the temporal Level-1 grant whose concise
-- form lives in ClickHouse aer_gold.metric_equivalence (migration 000028).
-- This mirrors the WP-006 §5.2 out-of-band review pattern: the methodological
-- decision is documented here in prose; the BFF read-path reads the ClickHouse
-- summary. id is referenced by metric_equivalence.validated_by.
--
-- One review row per (metric, language) so the granularity matches the
-- ClickHouse registry rows (the cross-frame gate needs a granted row per
-- language). The rationale is the same methodological decision for all four.
--
-- ids are set explicitly (1..4) so metric_equivalence.validated_by is a stable
-- cross-database reference independent of SERIAL ordering; the sequence is then
-- advanced past them. equivalence_reviews is empty before this migration (it
-- is created empty in migration 000014 and written only by grant migrations),
-- so these ids do not collide.

INSERT INTO equivalence_reviews
    (id, etic_construct, metric_name, language, source_type, equivalence_level,
     reviewer, review_date, rationale, working_paper_anchor, notes_summary, confidence)
VALUES
    (1, 'temporal_rhythm', 'publication_hour', 'de', 'web', 'temporal',
     'AĒR methodology review (Phase 124)', '2026-06-03',
     'Temporal Level-1 equivalence grant for the Probe-0 (DE) x Probe-1 (FR) institutional-web corpora. Publication timing (publication_hour, publication_weekday) is measured on clock and calendar time — a culture-independent axis — so cross-cultural comparison of temporal rhythm, and z-score normalization read as a rhythm/shape comparison, is valid by construction (WP-004 §6.3, Appendix B). The grant is conditional on calendar parity: the DE and FR cultural calendars are structurally comparable (public holidays, parliamentary recess, media events) per configs/cultural_calendars/{de,fr}.yaml. This grant authorises temporal-pattern comparison and rhythm z-score for these two metrics across DE+FR only; it does NOT extend to intensive/scaled metrics (e.g. sentiment), whose measurement axis is culture-laden and which require a separate deviation-level (Level-2) review.',
     'WP-004 Appendix B',
     'Temporal Level-1: clock/calendar time is culture-independent; DE/FR calendar parity verified. Authorises temporal-pattern comparison + rhythm z-score for publication_hour/weekday across DE+FR. Not for intensive metrics (sentiment). WP-004 §6.3, App. B.',
     1.0),
    (2, 'temporal_rhythm', 'publication_hour', 'fr', 'web', 'temporal',
     'AĒR methodology review (Phase 124)', '2026-06-03',
     'Temporal Level-1 equivalence grant for the Probe-0 (DE) x Probe-1 (FR) institutional-web corpora. Publication timing (publication_hour, publication_weekday) is measured on clock and calendar time — a culture-independent axis — so cross-cultural comparison of temporal rhythm, and z-score normalization read as a rhythm/shape comparison, is valid by construction (WP-004 §6.3, Appendix B). The grant is conditional on calendar parity: the DE and FR cultural calendars are structurally comparable (public holidays, parliamentary recess, media events) per configs/cultural_calendars/{de,fr}.yaml. This grant authorises temporal-pattern comparison and rhythm z-score for these two metrics across DE+FR only; it does NOT extend to intensive/scaled metrics (e.g. sentiment), whose measurement axis is culture-laden and which require a separate deviation-level (Level-2) review.',
     'WP-004 Appendix B',
     'Temporal Level-1: clock/calendar time is culture-independent; DE/FR calendar parity verified. Authorises temporal-pattern comparison + rhythm z-score for publication_hour/weekday across DE+FR. Not for intensive metrics (sentiment). WP-004 §6.3, App. B.',
     1.0),
    (3, 'temporal_rhythm', 'publication_weekday', 'de', 'web', 'temporal',
     'AĒR methodology review (Phase 124)', '2026-06-03',
     'Temporal Level-1 equivalence grant for the Probe-0 (DE) x Probe-1 (FR) institutional-web corpora. Publication timing (publication_hour, publication_weekday) is measured on clock and calendar time — a culture-independent axis — so cross-cultural comparison of temporal rhythm, and z-score normalization read as a rhythm/shape comparison, is valid by construction (WP-004 §6.3, Appendix B). The grant is conditional on calendar parity: the DE and FR cultural calendars are structurally comparable (public holidays, parliamentary recess, media events) per configs/cultural_calendars/{de,fr}.yaml. This grant authorises temporal-pattern comparison and rhythm z-score for these two metrics across DE+FR only; it does NOT extend to intensive/scaled metrics (e.g. sentiment), whose measurement axis is culture-laden and which require a separate deviation-level (Level-2) review.',
     'WP-004 Appendix B',
     'Temporal Level-1: clock/calendar time is culture-independent; DE/FR calendar parity verified. Authorises temporal-pattern comparison + rhythm z-score for publication_hour/weekday across DE+FR. Not for intensive metrics (sentiment). WP-004 §6.3, App. B.',
     1.0),
    (4, 'temporal_rhythm', 'publication_weekday', 'fr', 'web', 'temporal',
     'AĒR methodology review (Phase 124)', '2026-06-03',
     'Temporal Level-1 equivalence grant for the Probe-0 (DE) x Probe-1 (FR) institutional-web corpora. Publication timing (publication_hour, publication_weekday) is measured on clock and calendar time — a culture-independent axis — so cross-cultural comparison of temporal rhythm, and z-score normalization read as a rhythm/shape comparison, is valid by construction (WP-004 §6.3, Appendix B). The grant is conditional on calendar parity: the DE and FR cultural calendars are structurally comparable (public holidays, parliamentary recess, media events) per configs/cultural_calendars/{de,fr}.yaml. This grant authorises temporal-pattern comparison and rhythm z-score for these two metrics across DE+FR only; it does NOT extend to intensive/scaled metrics (e.g. sentiment), whose measurement axis is culture-laden and which require a separate deviation-level (Level-2) review.',
     'WP-004 Appendix B',
     'Temporal Level-1: clock/calendar time is culture-independent; DE/FR calendar parity verified. Authorises temporal-pattern comparison + rhythm z-score for publication_hour/weekday across DE+FR. Not for intensive metrics (sentiment). WP-004 §6.3, App. B.',
     1.0);

-- Advance the SERIAL sequence past the explicit ids so future inserts do not collide.
SELECT setval(pg_get_serial_sequence('equivalence_reviews', 'id'), (SELECT MAX(id) FROM equivalence_reviews));
