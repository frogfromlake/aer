-- Rollback Migration 003: Remove RSS source seeds

DELETE FROM sources WHERE name = 'bundesregierung' AND type = 'rss';
DELETE FROM sources WHERE name = 'tagesschau' AND type = 'rss';
