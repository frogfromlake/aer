-- Migration 022: Metadata coverage surface — per-source-per-field
-- provenance aggregation feeding the Phase 122f BFF endpoints and the
-- dashboard's field-level Negative-Space rendering (Brief §7.7).
-- Phase 122f / WP-003 §3.2.
--
-- WP-003 §3.2 documents "metadata-richness asymmetry between sources" as
-- a structural bias; the Probe 0 dossier's bias_assessment.md records the
-- per-source matrix numerically (Phase 122e A13). Until now the asymmetry
-- was observable only in static analytical artefacts. Phase 122f promotes
-- it to a runtime signal: every Silver write inserts one row per
-- (article, field) tuple into a raw ingestion table, and an
-- AggregatingMergeTree materialised view rolls it up into the per-source
-- coverage matrix that backs `GET /api/v1/probes/{id}/metadata-coverage`.
--
-- Two-table design rationale
-- --------------------------
-- A naive ReplacingMergeTree(source, field, method) would lose the
-- per-article cardinality needed to render "this publisher emitted X for
-- Y of N articles" — counting collapses on merge. The canonical pattern
-- is therefore a raw fact table + AggregatingMergeTree MV:
--
--   raw   : (source, article_id, field, method, ingestion_version,
--            ingestion_at) one row per (article, field) — the worker's
--            INSERT target. ReplacingMergeTree(ingestion_version) so
--            NATS redelivery collapses to the latest version on merge.
--
--   mv    : (source, field, method) → uniqExactState(article_id) +
--           maxState(ingestion_at). AggregateFunction states combine
--           correctly across merge blocks; combined with the
--           insert-block-level deduplication enabled below (mirrors
--           migration 021's pattern) this gives an exact distinct-article
--           count per cell without the
--           AggregatingMergeTree-over-ReplacingMergeTree double-count
--           footgun documented in migration 021.
--
-- Reads use the MV: uniqExactMerge / maxMerge collapse the
-- AggregateFunction states into scalars at query time. The BFF's
-- coverage handler issues a single GROUP BY (source, field, method)
-- against the view per request.
--
-- TTL anchors
-- -----------
-- Raw rows TTL on `ingestion_at + 30 days` — the ROADMAP's
-- structurally-absent threshold ("0 % population on a field across ≥ 50
-- articles in the last 30 days") is the analytical horizon, so older
-- raw rows have no semantic role and only inflate storage. The MV
-- itself is naturally tiny (sources × fields × methods is bounded
-- at ~few-hundred rows for the foreseeable corpus) and carries no TTL —
-- a stale row simply means "this combination has not been observed
-- recently", which is itself a signal.
--
-- Allowed `method` values mirror `web_meta.ALLOWED_EXTRACTION_METHODS`
-- plus the literal `"null"` for unfilled fields. The literal-string
-- "null" is a deliberate choice: structural absence is the signal we
-- are surfacing, so it must be a queryable value, not a SQL NULL that
-- hides in count semantics.

-- ---------------------------------------------------------------------
-- Raw fact table — one row per (article, field) on every Silver write.
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS aer_gold.metadata_coverage_raw
(
    source             LowCardinality(String),
    article_id         String,
    field              LowCardinality(String),
    method             LowCardinality(String),
    ingestion_version  UInt64,
    ingestion_at       DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(ingestion_version)
ORDER BY (source, article_id, field)
TTL ingestion_at + INTERVAL 30 DAY
SETTINGS non_replicated_deduplication_window = 1000;

-- ---------------------------------------------------------------------
-- AggregatingMergeTree MV — per-source-per-field-per-method roll-up.
-- ---------------------------------------------------------------------
CREATE MATERIALIZED VIEW IF NOT EXISTS aer_gold.metadata_coverage
ENGINE = AggregatingMergeTree
ORDER BY (source, field, method)
AS SELECT
    source,
    field,
    method,
    uniqExactState(article_id) AS articles_state,
    maxState(ingestion_at)     AS last_seen_state
FROM aer_gold.metadata_coverage_raw
GROUP BY source, field, method;
