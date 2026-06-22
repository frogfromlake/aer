-- Phase 148e — identity names on the auth user.
--
-- first_name / last_name are the display identity: first set at invite
-- acceptance (the /auth/accept-invite handler), then self-service editable by
-- the user (PATCH /auth/me). They power the account avatar initials, the
-- identity card, and the saved-analyses owner column — all live-joined on
-- owner_id, so a name change propagates everywhere and is never stale.
--
-- NOT NULL with a '' default so the columns add cleanly even to a pre-existing
-- row (e.g. a locally-bootstrapped admin). The application layer requires a
-- non-empty name for every NEW account; any pre-names row that still carries ''
-- renders via the email-derived initials fallback, never as a blank.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS first_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS last_name  TEXT NOT NULL DEFAULT '';
