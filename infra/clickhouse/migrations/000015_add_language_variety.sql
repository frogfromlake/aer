-- Migration 015: Phase 116 — add language_variety to aer_gold.language_detections.
--
-- Adds a coarse source-level metadata signal distinguishing German varieties
-- (de-DE / de-AT / de-CH) for the multilingual foundation. Population is
-- driven by a TLD heuristic on RssMeta.feed_url (`.at` → de-AT, `.ch` → de-CH,
-- otherwise de-DE for de texts; empty string for non-de). This is a metadata
-- tag, NOT a dialect classifier.
--
-- The column defaults to '' so historical rows remain valid; the worker
-- populates it on new inserts (Phase 116 processor enrichment).

ALTER TABLE aer_gold.language_detections
    ADD COLUMN IF NOT EXISTS language_variety String DEFAULT '';
