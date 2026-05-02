-- Migration 017: Create aer_gold.entity_links for Wikidata QID disambiguation.
-- Phase 118: NLP Hardening — Entity Linking (Wikidata).
--
-- Backs the wikidataQid / linkConfidence fields surfaced via LEFT JOIN by
-- GET /api/v1/entities and GET /api/v1/entities/cooccurrence. Rows are emitted
-- by NamedEntityExtractor after a deterministic alias-index lookup against the
-- Wikidata SQLite index built by scripts/build_wikidata_index.py.
--
-- Linked-only storage policy: only spans that resolve to a QID with
-- confidence >= 0.7 are written here. Spans that produce no match are absent
-- from this table; aer_gold.entities remains the canonical record of every
-- NER span. The link_rate is therefore computed as
--   (count(entity_links) / count(entities)) over the same window
-- — the table size grows with linked entities, not all entities, which keeps
-- it proportional to analytical signal during early Probe 0/1 operation when
-- the linked rate is expected to be low.
--
-- link_method values:
--   exact_match  — surface form matched rdfs:label exactly      (confidence 1.0)
--   alias_lookup — surface form matched skos:altLabel           (confidence 0.85)
--   accent_fold  — match required accent-folding (e.g. fr text) (confidence 0.7)
--
-- ingestion_version = nanoseconds at MinIO event time. Redelivered events
-- share the same event_time → same version → ReplacingMergeTree collapses
-- duplicates on merge / FINAL. The same (article_id, entity_text) tuple
-- always resolves to the same QID for a given index build, so re-runs are
-- byte-identical aside from version.

CREATE TABLE IF NOT EXISTS aer_gold.entity_links (
    timestamp           DateTime,
    article_id          String,
    entity_text         String,
    entity_label        String,
    wikidata_qid        String,
    link_confidence     Float32,
    link_method         LowCardinality(String),
    ingestion_version   UInt64 DEFAULT 0
)
ENGINE = ReplacingMergeTree(ingestion_version)
ORDER BY (article_id, entity_text)
TTL timestamp + INTERVAL 365 DAY;
