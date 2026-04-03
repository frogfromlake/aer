-- Migration 002: Seed the Wikipedia source (dev/PoC)

INSERT INTO sources (name, type, url)
VALUES ('wikipedia', 'scraper', 'https://en.wikipedia.org/api/rest_v1/page/random/summary')
ON CONFLICT DO NOTHING;
