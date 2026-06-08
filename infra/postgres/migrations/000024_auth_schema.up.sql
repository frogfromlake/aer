-- Migration 024: Phase 134 — Access Control & User Management (Auth-1).
-- ADR-040: BFF-driven sessions, no tokens in the client.
--
-- Creates the three core auth tables. The BFF is the confidential auth
-- authority; these tables are written by the BFF under the dedicated
-- `bff_auth` role (granted in infra/postgres/init-roles.sh, Slice 2) and
-- are owned by the migration runner (aer_admin) like every other table.
--
-- Security shape (ADR-040):
--   * Secrets are stored HASHED, never raw. `sessions.id` holds the
--     sha256 of the opaque 256-bit session id that the browser carries
--     in the __Host- cookie; `auth_tokens.token_hash` holds the sha256
--     of the single-use invite / password-reset token whose raw value
--     only ever appears in the emailed (or admin-surfaced) link. A
--     read-only leak of this database therefore yields no live session
--     and no usable link. The ids are high-entropy, so a fast hash
--     (sha256, computed in the BFF) is correct here — argon2id is for
--     the low-entropy `users.password_hash` only.
--   * Revocation is immediate (LICENSE §3.3): set `sessions.revoked_at`
--     or delete the row and the next request fails server-side.
--   * Privacy-minimal (Manifesto §VI / WP-006 §7): we store email, the
--     argon2id password hash, role, status and the responsible-use
--     consent timestamp — and nothing about what a user analyses. No raw
--     IP is retained; `user_agent` is optional and only powers the
--     user's own "active sessions" view.
--
-- UUID primary keys for `users` and `auth_tokens` avoid leaking row
-- counts and are unguessable. `gen_random_uuid()` is in Postgres core
-- (>=13), so no extension is required. Email uniqueness is enforced
-- case-insensitively via a functional unique index on lower(email),
-- avoiding a citext-extension dependency; the BFF normalises on write.

CREATE TABLE IF NOT EXISTS users (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email                       TEXT NOT NULL,
    -- argon2id PHC-encoded string (e.g. $argon2id$v=19$m=...$...). NULL
    -- while a user is invited-but-not-yet-activated, and reserved NULL
    -- for a future SSO-only account (external_idp set, no local password).
    password_hash               TEXT,
    role                        VARCHAR(16) NOT NULL DEFAULT 'researcher'
                                    CHECK (role IN ('admin', 'researcher')),
    status                      VARCHAR(16) NOT NULL DEFAULT 'invited'
                                    CHECK (status IN ('invited', 'active', 'suspended')),
    -- LICENSE §3.2.b: the responsible-use agreement is recorded at
    -- activation. NULL until the invitee accepts; non-NULL == consented.
    responsible_use_accepted_at TIMESTAMPTZ,
    -- SSO seam (ADR-040 deferred): a future ORCID / eduGAIN / OIDC login
    -- binds a verified external identity to an already-approved account.
    -- Both NULL until SSO lands. SSO is authentication only — never a
    -- self-registration path around the invite/consent gate.
    external_idp                VARCHAR(32),
    external_subject            VARCHAR(255),
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Case-insensitive email uniqueness without the citext extension.
CREATE UNIQUE INDEX IF NOT EXISTS users_email_lower_idx
    ON users (lower(email));

-- One external identity maps to at most one account (partial: only rows
-- that actually carry an SSO binding participate in the constraint).
CREATE UNIQUE INDEX IF NOT EXISTS users_external_identity_idx
    ON users (external_idp, external_subject)
    WHERE external_idp IS NOT NULL AND external_subject IS NOT NULL;

COMMENT ON TABLE users IS
    'AĒR user accounts (ADR-040, Phase 134). Privacy-minimal: email + argon2id hash + role + status + responsible-use consent. No analytical-activity tracking. Written by the BFF bff_auth role.';
COMMENT ON COLUMN users.password_hash IS
    'argon2id PHC-encoded string; NULL while invited-but-not-activated, and reserved NULL for future SSO-only accounts.';
COMMENT ON COLUMN users.responsible_use_accepted_at IS
    'LICENSE §3.2.b responsible-use consent timestamp, captured at invite acceptance. NULL == not yet consented.';


-- Server-side session store. The cookie carries only the opaque random
-- session id; `id` here is its sha256. Sliding idle expiry + a hard
-- absolute cap; immediate revocation via revoked_at.
CREATE TABLE IF NOT EXISTS sessions (
    id                  TEXT PRIMARY KEY,                 -- sha256(opaque session id)
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    idle_expires_at     TIMESTAMPTZ NOT NULL,             -- slides forward on activity
    absolute_expires_at TIMESTAMPTZ NOT NULL,             -- hard cap, never extended
    revoked_at          TIMESTAMPTZ,                      -- NULL == active; set for immediate revocation
    user_agent          TEXT                              -- optional, minimal; powers the user's own active-sessions view
);

CREATE INDEX IF NOT EXISTS sessions_user_idx
    ON sessions (user_id);
-- Supports the periodic expired-session cleanup sweep.
CREATE INDEX IF NOT EXISTS sessions_absolute_expiry_idx
    ON sessions (absolute_expires_at);

COMMENT ON TABLE sessions IS
    'Server-side opaque sessions (ADR-040, Phase 134). `id` is sha256 of the cookie session id. Sliding idle expiry + absolute cap; revoked_at gives immediate revocation (LICENSE §3.3).';


-- Single-use, hashed, short-lived tokens shared by the invite and
-- password-reset flows (ADR-040). The raw token only ever lives in the
-- emailed/admin-surfaced link; `token_hash` is its sha256.
CREATE TABLE IF NOT EXISTS auth_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose     VARCHAR(24) NOT NULL
                    CHECK (purpose IN ('invite', 'password_reset')),
    token_hash  TEXT NOT NULL,                            -- sha256(single-use raw token)
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ                               -- NULL == unused; set on first use (single-use)
);

-- Lookups are by token hash; uniqueness also prevents collision reuse.
CREATE UNIQUE INDEX IF NOT EXISTS auth_tokens_hash_idx
    ON auth_tokens (token_hash);
CREATE INDEX IF NOT EXISTS auth_tokens_user_purpose_idx
    ON auth_tokens (user_id, purpose);

COMMENT ON TABLE auth_tokens IS
    'Single-use hashed invite / password-reset tokens (ADR-040, Phase 134). Raw token only in the link; token_hash is sha256. Filtered on consumed_at IS NULL AND expires_at > now().';
