-- Reverse migration 011: drop the Silver eligibility columns.

ALTER TABLE sources
    DROP COLUMN IF EXISTS silver_review_reference,
    DROP COLUMN IF EXISTS silver_review_rationale,
    DROP COLUMN IF EXISTS silver_review_date,
    DROP COLUMN IF EXISTS silver_review_reviewer,
    DROP COLUMN IF EXISTS silver_eligible;
