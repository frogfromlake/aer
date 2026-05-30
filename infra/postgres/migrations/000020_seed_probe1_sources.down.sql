-- Rollback Migration 020: remove Probe 1 web source seeds.

DELETE FROM sources WHERE name = 'franceinfo' AND type = 'web';
DELETE FROM sources WHERE name = 'elysee' AND type = 'web';
