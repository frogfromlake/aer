-- Rollback Migration 006: Remove Probe 0 seed classifications

DELETE FROM source_classifications WHERE classified_by = 'WP-001/Probe-0';
