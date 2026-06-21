-- Migration 028: SEC-016 — bound saved-analysis field sizes at the DB layer.
--
-- Defense-in-depth for the per-field length validation the BFF enforces in the
-- handler (services/bff-api/internal/handler/analyses_handlers.go). The columns
-- were unbounded TEXT (~1 GB/field), a write-amplification vector into the
-- shared auth Postgres (the same instance that backs sessions). octet_length
-- (byte length) matches Go's len(), so the handler guard and this CHECK agree
-- exactly. Idempotent (DO block) so a restore/replay never errors on re-apply.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'saved_analyses_name_len') THEN
        ALTER TABLE saved_analyses
            ADD CONSTRAINT saved_analyses_name_len CHECK (octet_length(name) <= 200);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'saved_analyses_description_len') THEN
        ALTER TABLE saved_analyses
            ADD CONSTRAINT saved_analyses_description_len CHECK (octet_length(description) <= 2048);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'saved_analyses_state_len') THEN
        ALTER TABLE saved_analyses
            ADD CONSTRAINT saved_analyses_state_len CHECK (octet_length(state) <= 262144);
    END IF;
END $$;
