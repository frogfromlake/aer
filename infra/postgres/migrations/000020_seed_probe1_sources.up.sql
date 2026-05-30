-- Migration 020: Seed Probe 1 — French institutional web sources (Phase 123).
--
-- AĒR's first non-German cultural context. Both sources register as
-- `type='web'` from the start (no RSS→web transition, unlike Probe 0's
-- migration 015). Source selection was live-audited and is documented in the
-- Probe-1 Dossier (docs/probes/probe-1-fr-institutional-web/):
--   * franceinfo — France Télévisions / franceinfo public broadcaster.
--                  The ROADMAP-named francetvinfo.fr 301-redirects to the
--                  publisher's canonical domain franceinfo.fr. EA primary.
--   * elysee     — Présidence de la République. PL primary. Chosen over the
--                  ROADMAP-named gouvernement.fr, which 301-redirects to the
--                  Cloudflare-bot-walled info.gouv.fr (uncollectable by the
--                  ADR-028 polite crawler). See bias_assessment.md.
--
-- `url` is the publisher homepage (the registration identity; runtime
-- discovery channels live in crawlers/web-crawler/probes/probe1/sources.yaml).
-- documentation_url points at the dossier directory; the dashboard renders it
-- verbatim and the dev MkDocs container serves it at localhost:8000.

INSERT INTO sources (name, type, url)
VALUES ('franceinfo', 'web', 'https://www.franceinfo.fr')
ON CONFLICT DO NOTHING;

INSERT INTO sources (name, type, url)
VALUES ('elysee', 'web', 'https://www.elysee.fr')
ON CONFLICT DO NOTHING;

UPDATE sources
   SET documentation_url = 'http://localhost:8000/probes/probe-1-fr-institutional-web/'
 WHERE name IN ('franceinfo', 'elysee');
