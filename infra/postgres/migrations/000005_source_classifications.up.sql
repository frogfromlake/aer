-- Migration 005: Source classifications table for Functional Probe Taxonomy (WP-001)
--
-- Stores etic/emic discourse function classifications per source.
-- Multiple classification records per source enable temporal tracking
-- of functional transitions. The table is additive — it does not
-- modify existing sources entries.

CREATE TABLE IF NOT EXISTS source_classifications (
    source_id INTEGER REFERENCES sources(id),
    primary_function VARCHAR(30) NOT NULL,
    secondary_function VARCHAR(30),
    function_weights JSONB,
    emic_designation TEXT NOT NULL,
    emic_context TEXT NOT NULL,
    emic_language VARCHAR(10),
    classified_by VARCHAR(100) NOT NULL,
    classification_date DATE NOT NULL,
    review_status VARCHAR(30) DEFAULT 'pending',
    PRIMARY KEY (source_id, classification_date),

    CONSTRAINT chk_primary_function CHECK (
        primary_function IN ('epistemic_authority', 'power_legitimation', 'cohesion_identity', 'subversion_friction')
    ),
    CONSTRAINT chk_secondary_function CHECK (
        secondary_function IS NULL OR secondary_function IN ('epistemic_authority', 'power_legitimation', 'cohesion_identity', 'subversion_friction')
    ),
    CONSTRAINT chk_review_status CHECK (
        review_status IN ('provisional_engineering', 'pending', 'reviewed', 'contested')
    )
);
