-- Migration 008: Create metric_equivalence table.
-- Phase 65: Cross-Cultural Comparability Infrastructure (WP-004).
--
-- Maps etic constructs (e.g., "evaluative_polarity") to concrete metrics
-- (e.g., "sentiment_score_sentiws") with equivalence level and validation
-- provenance. Initially empty — populated when interdisciplinary validation
-- studies establish cross-cultural equivalence.
-- Uses ReplacingMergeTree to keep only the latest validation per key.

CREATE TABLE IF NOT EXISTS aer_gold.metric_equivalence
(
    etic_construct    String,
    metric_name       String,
    language          String,
    source_type       String,
    equivalence_level String,
    validated_by      String,
    validation_date   DateTime,
    confidence        Float32
)
ENGINE = ReplacingMergeTree(validation_date)
ORDER BY (etic_construct, metric_name, language);
