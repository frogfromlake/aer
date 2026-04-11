-- Migration 005: Add discourse_function column to metrics and entities tables.
-- Phase 62: Functional Probe Taxonomy (WP-001).
--
-- Stores the primary discourse function from source_classifications,
-- enabling aggregation by discourse function in the Gold layer.

ALTER TABLE aer_gold.metrics ADD COLUMN IF NOT EXISTS discourse_function String DEFAULT '';
ALTER TABLE aer_gold.entities ADD COLUMN IF NOT EXISTS discourse_function String DEFAULT '';
