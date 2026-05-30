-- Reverse migration 022: revoke Probe 1 Silver eligibility (columns persist; they
-- belong to migration 011).

UPDATE sources
   SET silver_eligible = false,
       silver_review_reviewer = NULL,
       silver_review_date = NULL,
       silver_review_rationale = NULL,
       silver_review_reference = NULL
 WHERE name IN ('franceinfo', 'elysee');
