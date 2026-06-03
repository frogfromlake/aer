-- Rollback Migration 023: remove the temporal Level-1 equivalence review records.
-- The ClickHouse metric_equivalence rows are removed by the ClickHouse side
-- (re-seed from migrations on a clean rebuild); this only reverts Postgres.

DELETE FROM equivalence_reviews
 WHERE etic_construct = 'temporal_rhythm'
   AND metric_name IN ('publication_hour', 'publication_weekday')
   AND equivalence_level = 'temporal';
