-- Migration 016: Phase 122 — refresh Probe 0 `documentation_url` to point
-- at the renamed dossier directory (probe-0-de-institutional-rss →
-- probe-0-de-institutional-web). The dashboard renders the URL verbatim;
-- the dev-default MkDocs container serves the docs site at
-- http://localhost:8000. Production overlays may rewrite this value
-- out-of-band.

UPDATE sources
   SET documentation_url = 'http://localhost:8000/probes/probe-0-de-institutional-web/'
 WHERE name IN ('bundesregierung', 'tagesschau');
