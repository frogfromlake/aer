-- Migration 015 (down): restore Probe 0 sources to `type = 'rss'`.

UPDATE sources
   SET type = 'rss'
 WHERE name IN ('bundesregierung', 'tagesschau')
   AND type = 'web';
