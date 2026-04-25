-- Migration 012: Create aer_silver.documents projection table.
-- Phase 103b: Iteration 5 — Silver Aggregation Endpoints with Projection Table.
--
-- Backs the GET /api/v1/silver/aggregations/{aggregationType} endpoints
-- (ADR-020 §"Silver-layer access"). Silver itself lives as JSON envelopes in
-- MinIO, which has no queryable index. To support distributional / heatmap /
-- correlation queries analogous to the Phase 102 Gold view-mode endpoints
-- without paying a per-request MinIO scan, the analysis worker writes a
-- one-row-per-document projection of the Silver fields needed for those
-- aggregations into this ClickHouse table at the same point Silver is
-- uploaded to MinIO.
--
-- Eligibility is still enforced at the BFF, so a row in this table for a
-- non-eligible source is harmless: the aggregation endpoint refuses with the
-- same 403 + RefusalPayload as Phase 103's per-document endpoints.
--
-- ingestion_version = nanoseconds at MinIO event time (mirrors aer_gold.metrics).
-- ReplacingMergeTree collapses duplicate rows from NATS redelivery.

CREATE DATABASE IF NOT EXISTS aer_silver;

CREATE TABLE IF NOT EXISTS aer_silver.documents (
    timestamp            DateTime,
    source               String,
    article_id           String,
    language             String,
    cleaned_text_length  UInt32,
    word_count           UInt32,
    raw_entity_count     UInt32,
    ingestion_version    UInt64 DEFAULT 0
)
ENGINE = ReplacingMergeTree(ingestion_version)
ORDER BY (timestamp, source, article_id)
TTL timestamp + INTERVAL 365 DAY;
