-- Reverse migration 014: drop the equivalence_reviews workflow table.

DROP INDEX IF EXISTS equivalence_reviews_etic_idx;
DROP INDEX IF EXISTS equivalence_reviews_metric_idx;
DROP TABLE IF EXISTS equivalence_reviews;
