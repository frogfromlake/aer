-- Migration 003: Seed German institutional RSS sources (Probe 0 — pipeline calibration)

INSERT INTO sources (name, type, url)
VALUES ('bundesregierung', 'rss', 'https://www.bundesregierung.de/breg-de/aktuelles.rss')
ON CONFLICT DO NOTHING;

INSERT INTO sources (name, type, url)
VALUES ('tagesschau', 'rss', 'https://www.tagesschau.de/index~rss2.xml')
ON CONFLICT DO NOTHING;
