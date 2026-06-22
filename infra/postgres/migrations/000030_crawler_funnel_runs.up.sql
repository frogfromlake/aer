-- Phase 148d (WP-007 §5) — per-source-per-run collection funnel.
--
-- The completeness funnel spans three services. Migration 000018/000029
-- (`crawler_discovery_runs`) records the channel-attributable head of the
-- funnel — declared → discovered → after_dedup — because at discovery time
-- the channel a URL came from is still known. Once URLs are merged into one
-- crawl list, channel attribution is gone (a 404 or a thin-body drop is a
-- property of the SOURCE, not the channel that surfaced the URL — WP-007
-- Decision A). The spider stages are therefore recorded PER SOURCE here.
--
--   after_dedup (= discovered input to the spider)
--      ├─ url_filtered        (url_filter + IgnoreRequest — fails passed_filters)
--      ├─ already_collected   (conditional-GET avoidance — NOT a loss; we hold it)
--      └─ requests issued
--             └─ fetched       (HTTP response received)
--                  ├─ not_modified        (304 — already held, unchanged)
--                  ├─ content_dropped     (content-type / empty body — fails passed_filters)
--                  ├─ thin_content_dropped (min_word_count — the Layer-3 non-article signal)
--                  ├─ errored             (non-200 / build / submit / transport)
--                  └─ submitted           (reached Bronze → feeds extracted → Gold)
--
-- The tail of the funnel — extracted → Gold — lives in the analysis worker
-- and ClickHouse, reconciled at BFF read-time against the current Gold row
-- count for the source (no run_id is propagated Bronze→Silver→Gold; that
-- precision is not worth threading a correlation id through three services
-- for a per-source figure — WP-007 Decision A).
BEGIN;

CREATE TABLE crawler_funnel_runs (
    run_id                UUID PRIMARY KEY,
    source_id             INTEGER NOT NULL REFERENCES sources (id) ON DELETE CASCADE,
    discovered            INTEGER NOT NULL CHECK (discovered >= 0),
    url_filtered          INTEGER NOT NULL DEFAULT 0 CHECK (url_filtered >= 0),
    already_collected     INTEGER NOT NULL DEFAULT 0 CHECK (already_collected >= 0),
    fetched               INTEGER NOT NULL DEFAULT 0 CHECK (fetched >= 0),
    not_modified          INTEGER NOT NULL DEFAULT 0 CHECK (not_modified >= 0),
    content_dropped       INTEGER NOT NULL DEFAULT 0 CHECK (content_dropped >= 0),
    thin_content_dropped  INTEGER NOT NULL DEFAULT 0 CHECK (thin_content_dropped >= 0),
    submitted             INTEGER NOT NULL DEFAULT 0 CHECK (submitted >= 0),
    errored               INTEGER NOT NULL DEFAULT 0 CHECK (errored >= 0),
    run_started_at        TIMESTAMPTZ NOT NULL,
    run_completed_at      TIMESTAMPTZ NOT NULL CHECK (run_completed_at >= run_started_at)
);

-- Dominant access pattern: "latest funnel run for source X" (BFF
-- discovery-coverage reconciliation + the dashboard panel).
CREATE INDEX idx_crawler_funnel_runs_source_run
    ON crawler_funnel_runs (source_id, run_started_at DESC);

COMMIT;
