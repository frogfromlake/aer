-- Reverse 000028 — drop the saved-analysis field-length CHECK constraints.
ALTER TABLE saved_analyses DROP CONSTRAINT IF EXISTS saved_analyses_name_len;
ALTER TABLE saved_analyses DROP CONSTRAINT IF EXISTS saved_analyses_description_len;
ALTER TABLE saved_analyses DROP CONSTRAINT IF EXISTS saved_analyses_state_len;
