-- Migration 018: Create aer_gold.topic_assignments for Topic Modeling view modes.
-- Phase 120: Iteration 6 — Corpus-Level Extractors — Topic Modeling
-- (BERTopic, the second CorpusExtractor implementation per ADR-020).
--
-- Backs the GET /api/v1/topics/distribution endpoint (Episteme pillar). Rows
-- are emitted per (article_id, topic_id) by the TopicModelingExtractor inside
-- the analysis-worker corpus loop. Per WP-004 §3.4, BERTopic is fit per
-- detected_language: every row is a member of exactly one language partition,
-- and topic_id is unique only within (window_start, language). Cross-language
-- alignment is explicitly out of scope.
--
-- topic_id = -1 is BERTopic's outlier class. Rows with topic_id = -1 are
-- stored unchanged so the distribution endpoint can surface "uncategorised"
-- as a first-class observation rather than silently dropping outliers.
--
-- model_hash carries the BERTopic / sentence-transformer / UMAP / HDBSCAN
-- provenance so downstream consumers (Phase 121 methodology tray, future
-- reproducibility audits) can detect when topic identifiers no longer
-- correspond to a fixed model configuration. See
-- internal/extractors/topic_modeling.py for the hash composition.
--
-- ingestion_version = nanoseconds at corpus-sweep start. Re-runs over the
-- same window emit identical (window_start, source, article_id, topic_id)
-- tuples with a higher ingestion_version, and ReplacingMergeTree collapses
-- to the latest row on merge / FINAL.

CREATE TABLE IF NOT EXISTS aer_gold.topic_assignments (
    window_start        DateTime,
    window_end          DateTime,
    source              String,
    article_id          String,
    language            String,
    topic_id            Int32,
    topic_label         String,
    topic_confidence    Float32,
    model_hash          String,
    ingestion_version   UInt64 DEFAULT 0
)
ENGINE = ReplacingMergeTree(ingestion_version)
ORDER BY (window_start, source, article_id, language, topic_id)
TTL window_start + INTERVAL 365 DAY;
