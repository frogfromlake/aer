-- Migration 026: Create aer_gold.wikidata_labels — QID → display label per language.
-- Phase 123b: Cross-Lingual Readability of Relational Artefacts.
--
-- Backs the per-Panel "viewer language" relabelling toggle on the
-- cooccurrence_network presentation (and entity surfaces). The BFF LEFT JOINs
-- this table at query time on the QID already resolved from aer_gold.entity_links
-- (queryNodeWikidataQids) — the same join pattern that surfaces wikidataQid
-- today. A node with a resolved QID and a label in the viewer's language is
-- relabelled (Russie → Russland); a node without either keeps its source
-- surface form. No machine translation is involved — `label` is the original-
-- case Wikidata rdfs:label, not a derived string.
--
-- Why a dedicated reference table rather than the baked alias index:
--   * The alias index (wikidata_aliases.db) stores LOWERCASED match forms
--     ("russland") for entity linking, not display-cased labels — useless for
--     a readability surface, and especially wrong for German nouns.
--   * The BFF reads only from ClickHouse; mounting the 128 MB SQLite + a SQLite
--     driver onto the sole internet-facing service has no precedent there.
--   This table is populated by the wikidata-labels-load init from the display-
--   cased TSV emitted by scripts/build/build_wikidata_index.py (rides the same
--   quarterly index rebuild) — see compose.yaml.
--
-- No TTL: this is stable reference data keyed on the QID, not article-derived
-- Gold that ages out on published_date. ReplacingMergeTree(updated_at) so a
-- newer index rebuild's labels win on merge. The loader inserts updated_at via
-- DEFAULT now(), so a re-run produces a distinct block (NOT collapsed by
-- insert-dedup) — duplicate (qid,language) rows are reconciled on merge and the
-- BFF reads FINAL, which is what keeps reads single-valued. Do not drop FINAL.

CREATE TABLE IF NOT EXISTS aer_gold.wikidata_labels (
    wikidata_qid    String,
    language        LowCardinality(String),
    label           String,
    updated_at      DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (wikidata_qid, language);
