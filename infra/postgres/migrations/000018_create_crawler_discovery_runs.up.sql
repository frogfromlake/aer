-- Phase 122g — per-source-per-channel discovery telemetry.
--
-- Captures how many URLs each declared discovery channel surfaced per
-- crawl run, so cross-source coverage asymmetries are observable
-- instead of silently degrading the corpus. Sibling to the metadata-
-- coverage signal Phase 122f shipped — both are read-only outputs
-- consumed by the BFF + dashboard.
--
-- Universal-core schema: the same row shape applies to future Twitter,
-- Reddit, Mastodon, YouTube crawlers — each crawler writes its own
-- channel names (e.g. `timeline`, `subreddit`, `hashtag_stream`,
-- `channel_uploads`) and the cross-platform telemetry layer treats them
-- symmetrically. Cross-platform comparability of "did source X deliver
-- expected coverage this run" therefore requires no schema work per new
-- platform class.
--
-- Recorded in ADR-031 (DiscoveryProtocol Contract for Multi-Channel
-- Source Discovery, Phase 122g).
BEGIN;

CREATE TABLE crawler_discovery_runs (
    run_id            UUID PRIMARY KEY,
    source_id         INTEGER NOT NULL REFERENCES sources (id) ON DELETE CASCADE,
    channel           TEXT NOT NULL,
    urls_discovered   INTEGER NOT NULL CHECK (urls_discovered >= 0),
    urls_after_dedup  INTEGER NOT NULL CHECK (urls_after_dedup >= 0),
    run_started_at    TIMESTAMPTZ NOT NULL,
    run_completed_at  TIMESTAMPTZ NOT NULL CHECK (run_completed_at >= run_started_at)
);

-- The dominant access pattern is "give me the last N runs for source X
-- by recency" (BFF discovery-coverage endpoint + underflow-alert
-- detection). One descending compound index covers both.
CREATE INDEX idx_crawler_discovery_runs_source_run
    ON crawler_discovery_runs (source_id, run_started_at DESC);

-- `(source_id, channel)` slice is needed for the per-channel
-- aggregation (the dashboard panel renders one row per channel).
CREATE INDEX idx_crawler_discovery_runs_source_channel
    ON crawler_discovery_runs (source_id, channel, run_started_at DESC);

-- Phase 122g — discovery-underflow alerts. Two-consecutive-underflow
-- semantics: a transient publisher hiccup (single failed run) does NOT
-- fire an alert; a sustained degradation (two runs in a row) does. The
-- (source_id, alert_type) uniqueness keeps the table idempotent — the
-- writer upserts on detection, deletes on recovery.
CREATE TABLE crawler_discovery_alerts (
    source_id          INTEGER NOT NULL REFERENCES sources (id) ON DELETE CASCADE,
    alert_type         TEXT NOT NULL,
    first_observed_at  TIMESTAMPTZ NOT NULL,
    last_observed_at   TIMESTAMPTZ NOT NULL,
    consecutive_runs   INTEGER NOT NULL CHECK (consecutive_runs >= 1),
    expected_floor     INTEGER NOT NULL,
    last_urls_observed INTEGER NOT NULL,
    PRIMARY KEY (source_id, alert_type)
);

CREATE INDEX idx_crawler_discovery_alerts_last
    ON crawler_discovery_alerts (last_observed_at DESC);

COMMIT;
