-- Rollback Migration 002: Remove Wikipedia source seed

DELETE FROM sources WHERE name = 'wikipedia' AND type = 'scraper';
