-- Phase 122d.0 — Wayback Machine CDX lookup cache.
--
-- The analysis worker queries the Internet Archive CDX API at the
-- Bronze→Silver boundary to capture every archived snapshot of an
-- article's canonical URL. The corpus-wide hit rate is the cross-
-- source bottleneck: every harmonised web article issues one lookup.
-- This cache lets us amortise that lookup across NATS redeliveries,
-- re-processings, and adapter restarts, and keeps us well inside the
-- public CDX usage envelope.
--
-- The cache is intentionally THIN: one row per canonical URL, the
-- raw CDX status (ok / no_snapshots / failed / skipped / disabled —
-- see internal.wayback.client), and the decoded snapshot list as
-- jsonb. Freshness is enforced application-side via
-- `WAYBACK_CDX_CACHE_TTL_HOURS` (default 24 h); we do not run a
-- separate retention sweep because at one row per canonical URL the
-- table is naturally bounded by the worker's source set, and stale
-- rows are overwritten on the next lookup.
--
-- Failure-mode discipline (Phase 122d.0 invariant): a cache miss
-- falls through to a network call; a cache write failure logs and
-- continues; neither path ever DLQs the document. See
-- `services/analysis-worker/internal/wayback/cache.py`.
BEGIN;

CREATE TABLE wayback_cdx_cache (
    canonical_url    TEXT PRIMARY KEY,
    fetched_at       TIMESTAMPTZ NOT NULL,
    status           TEXT NOT NULL CHECK (
        status IN ('ok', 'no_snapshots', 'failed', 'skipped', 'disabled')
    ),
    revisions_jsonb  JSONB NOT NULL DEFAULT '[]'::jsonb
);

-- The dominant access pattern is point-get by canonical_url
-- (covered by the PK). The `fetched_at` index supports the optional
-- retention sweep an operator may run manually if the table needs
-- trimming after a corpus-wide reset.
CREATE INDEX idx_wayback_cdx_cache_fetched_at
    ON wayback_cdx_cache (fetched_at);

COMMIT;
