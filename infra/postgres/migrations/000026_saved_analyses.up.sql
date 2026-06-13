-- Migration 026: Phase 135 — Saved Analyses & Sharing (Auth-2).
--
-- Persists a configured Workbench analysis (its serialized URL-state) so a user
-- does not lose work on browser-back, and lets the owner share it with specific
-- users by identity. Written by the BFF under the `bff_auth` role (the same
-- write role as the Phase-134 auth tables; the grant is added in
-- infra/postgres/init-roles.sh).
--
-- Sharing is IDENTITY-BASED ONLY (ADR-040 posture): access is checked
-- server-side against the session user. There is no "share with all" and no
-- unguessable capability/bearer link — a forwarded URL never grants access.
-- `readable` vs `editable` is NOT stored: it is derived per viewer (owner ⇒
-- editable; grantee ⇒ editable iff `can_edit`, else readable).

CREATE TABLE IF NOT EXISTS saved_analyses (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    -- The serialized Workbench query string (the canonical URL grammar
    -- `?activePillar=&{aleph,episteme,rhizome}=…`). Opaque to the BFF; the
    -- dashboard round-trips it verbatim to restore the analysis.
    state       TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS saved_analyses_owner_idx
    ON saved_analyses (owner_id, created_at DESC);

COMMENT ON TABLE saved_analyses IS
    'Saved Workbench analyses (Phase 135 / ADR-040). `state` is the serialized Workbench URL query, round-tripped verbatim by the dashboard. Written by the bff_auth role.';

-- Per-grantee access grants. `can_edit=false` ⇒ read-only. One row per
-- (analysis, grantee). No "share with all" row exists by construction.
CREATE TABLE IF NOT EXISTS saved_analysis_shares (
    analysis_id      UUID NOT NULL REFERENCES saved_analyses(id) ON DELETE CASCADE,
    grantee_user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    can_edit         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (analysis_id, grantee_user_id)
);

CREATE INDEX IF NOT EXISTS saved_analysis_shares_grantee_idx
    ON saved_analysis_shares (grantee_user_id);

COMMENT ON TABLE saved_analysis_shares IS
    'Identity-based access grants for saved analyses (Phase 135). can_edit=false is read-only. No capability links, no share-with-all.';
