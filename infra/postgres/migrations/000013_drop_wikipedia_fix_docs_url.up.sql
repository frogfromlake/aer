-- Migration 013: Phase 113 cleanup.
--
-- (1) Drop the legacy `wikipedia` source seed (Phase 113 / Bug 7) — it is
--     a leftover from the first PoC; no crawler is registered for it and
--     it surfaces as an empty source on Surface I.
--
-- (2) Repair the `documentation_url` values seeded by migration 008, which
--     stored a relative `docs/probes/...` path. The dashboard renders the
--     URL verbatim, so a relative path resolves against the dashboard
--     origin and 404s. The dev-default MkDocs container serves the docs
--     site at `http://localhost:8000`, so we store the absolute URL here.
--     Production overlays may rewrite this value out-of-band.

DELETE FROM source_classifications
 WHERE source_id IN (SELECT id FROM sources WHERE name = 'wikipedia');

DELETE FROM sources WHERE name = 'wikipedia';

UPDATE sources
   SET documentation_url = 'http://localhost:8000/probes/probe-0-de-institutional-rss/'
 WHERE name IN ('bundesregierung', 'tagesschau');
