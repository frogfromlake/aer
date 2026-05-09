-- Migration 016 (down): restore the pre-rename Probe 0 documentation URL.

UPDATE sources
   SET documentation_url = 'http://localhost:8000/probes/probe-0-de-institutional-rss/'
 WHERE name IN ('bundesregierung', 'tagesschau');
