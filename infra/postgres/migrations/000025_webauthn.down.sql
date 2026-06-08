-- Reverse migration 025: drop the WebAuthn tables (ADR-040).

DROP INDEX IF EXISTS webauthn_ceremonies_expiry_idx;
DROP TABLE IF EXISTS webauthn_ceremonies;

DROP INDEX IF EXISTS webauthn_credentials_user_idx;
DROP INDEX IF EXISTS webauthn_credentials_credid_idx;
DROP TABLE IF EXISTS webauthn_credentials;
