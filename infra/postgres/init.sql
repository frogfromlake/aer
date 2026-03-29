-- Schema for AĒR Metadata Index

CREATE TABLE IF NOT EXISTS sources (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL, -- e.g., 'rss', 'api', 'scraper'
    url VARCHAR(500) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ingestion_jobs (
    id SERIAL PRIMARY KEY,
    source_id INTEGER REFERENCES sources(id),
    status VARCHAR(50) NOT NULL, -- e.g., 'running', 'completed', 'failed'
    started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    finished_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE IF NOT EXISTS documents (
    id SERIAL PRIMARY KEY,
    job_id INTEGER REFERENCES ingestion_jobs(id),
    bronze_object_key VARCHAR(500) UNIQUE NOT NULL, -- The MinIO Path
    trace_id VARCHAR(255), -- OpenTelemetry Trace ID for full observability
    status VARCHAR(50) DEFAULT 'pending', -- Tracks document lifecycle
    ingested_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Insert a dummy source for PoC
INSERT INTO sources (name, type, url) 
VALUES ('AER Dummy Generator', 'internal_test', 'http://localhost')
ON CONFLICT DO NOTHING;