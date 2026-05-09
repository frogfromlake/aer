-- Migration 017: Phase 122 — `crawler_state` table for the new Python
-- web-crawler's deduplication and conditional-GET state.
--
-- Replaces the local JSON dedup file the legacy Go RSS crawler used.
-- Postgres-backed for cross-container persistence and crash recovery —
-- a re-run of `make crawl-probe0` after a container restart honours the
-- previously-recorded last_fetched / etag / http_last_modified values.
--
-- Schema:
--   source_id           — FK-style reference to sources.id (no FK
--                         constraint: the crawler resolves source_id
--                         dynamically via the ingestion API and we keep
--                         the table independent of migration order).
--   canonical_url       — courlan-normalised URL of the article.
--                         Composite PK with source_id ensures one row
--                         per (source, canonical URL).
--   last_fetched        — wall-clock timestamp of the most recent
--                         successful fetch. Used as the visibility
--                         signal for retry/refresh logic.
--   etag                — value of the response's ETag header, fed
--                         back as `If-None-Match` on the next request.
--   http_last_modified  — value of the response's Last-Modified header,
--                         fed back as `If-Modified-Since` on the next
--                         request.
--   content_hash        — sha256 of the raw HTML body, used as a
--                         secondary dedup signal when a server's
--                         conditional-GET headers are unreliable.
--   sitemap_lastmod     — `<lastmod>` value from the sitemap entry that
--                         surfaced this URL. Kept on-row so the crawler
--                         can compare a freshly-discovered sitemap
--                         lastmod against the stored value without a
--                         join.

CREATE TABLE IF NOT EXISTS crawler_state (
    source_id          INTEGER NOT NULL,
    canonical_url      TEXT    NOT NULL,
    last_fetched       TIMESTAMPTZ NOT NULL,
    etag               TEXT,
    http_last_modified TIMESTAMPTZ,
    content_hash       TEXT,
    sitemap_lastmod    TIMESTAMPTZ,
    PRIMARY KEY (source_id, canonical_url)
);

CREATE INDEX IF NOT EXISTS crawler_state_source_last_fetched_idx
    ON crawler_state (source_id, last_fetched DESC);
