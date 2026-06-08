-- Migration 025: Phase 134 — WebAuthn / passkey second factor (ADR-040).
--
-- Phishing-resistant FIDO2 credentials layered on the argon2id password
-- baseline (NIST SP 800-63-4 AAL2). The cryptography is handled by the
-- go-webauthn library; these tables persist the registered credentials and the
-- short-lived ceremony state (the challenge) between the begin and finish steps
-- of a registration/assertion ceremony.

-- Registered authenticators. The full go-webauthn `Credential` value is stored
-- as JSONB (it round-trips losslessly); `credential_id` is duplicated out as a
-- BYTEA column for fast lookup on assertion. `sign_count` is mirrored for
-- clone-detection visibility but the authoritative copy lives inside the JSON.
CREATE TABLE IF NOT EXISTS webauthn_credentials (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id BYTEA NOT NULL,
    credential    JSONB NOT NULL,
    name          TEXT,
    sign_count    BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at  TIMESTAMPTZ
);

-- A credential id is globally unique to one authenticator.
CREATE UNIQUE INDEX IF NOT EXISTS webauthn_credentials_credid_idx
    ON webauthn_credentials (credential_id);
CREATE INDEX IF NOT EXISTS webauthn_credentials_user_idx
    ON webauthn_credentials (user_id);

COMMENT ON TABLE webauthn_credentials IS
    'Registered WebAuthn/passkey authenticators (ADR-040, Phase 134). Second factor on top of the argon2id password.';

-- Ephemeral ceremony state (the challenge / SessionData) held server-side
-- between begin and finish. One active ceremony per (user, purpose); a new
-- begin upserts over the previous. Consumed on finish; swept on expiry.
CREATE TABLE IF NOT EXISTS webauthn_ceremonies (
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose      VARCHAR(16) NOT NULL CHECK (purpose IN ('register', 'login')),
    session_data JSONB NOT NULL,
    expires_at   TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, purpose)
);

CREATE INDEX IF NOT EXISTS webauthn_ceremonies_expiry_idx
    ON webauthn_ceremonies (expires_at);

COMMENT ON TABLE webauthn_ceremonies IS
    'Short-lived WebAuthn ceremony challenge state (ADR-040). Server-side SessionData between begin and finish; never reaches the client beyond the challenge in the options.';
