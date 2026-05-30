-- Rollback Migration 021: remove Probe 1 seed classifications.

DELETE FROM source_classifications WHERE classified_by = 'WP-001/Probe-1';
