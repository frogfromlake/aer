-- Revert Phase 148e identity names.
ALTER TABLE users
    DROP COLUMN IF EXISTS first_name,
    DROP COLUMN IF EXISTS last_name;
