-- Reverse migration 024: drop the Phase 134 auth schema (ADR-040).
-- Dropped in reverse dependency order; auth_tokens and sessions both FK
-- to users, so they go first.

DROP INDEX IF EXISTS auth_tokens_user_purpose_idx;
DROP INDEX IF EXISTS auth_tokens_hash_idx;
DROP TABLE IF EXISTS auth_tokens;

DROP INDEX IF EXISTS sessions_absolute_expiry_idx;
DROP INDEX IF EXISTS sessions_user_idx;
DROP TABLE IF EXISTS sessions;

DROP INDEX IF EXISTS users_external_identity_idx;
DROP INDEX IF EXISTS users_email_lower_idx;
DROP TABLE IF EXISTS users;
