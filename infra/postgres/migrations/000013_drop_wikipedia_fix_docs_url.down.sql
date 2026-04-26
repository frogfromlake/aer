-- Reverse of 000013: restore the relative documentation_url values and
-- the wikipedia seed row. The `source_classifications` row is not
-- restored — none was seeded for wikipedia in the first place.

UPDATE sources
   SET documentation_url = 'docs/probes/probe-0-de-institutional-rss/'
 WHERE name IN ('bundesregierung', 'tagesschau');

INSERT INTO sources (name, type, url)
SELECT 'wikipedia', 'scraper', 'https://en.wikipedia.org/api/rest_v1/page/random/summary'
 WHERE NOT EXISTS (SELECT 1 FROM sources WHERE name = 'wikipedia');
