CREATE DATABASE IF NOT EXISTS aer_gold;

-- Create the metrics table with a 365-day Time-To-Live (TTL)
CREATE TABLE IF NOT EXISTS aer_gold.metrics (
    timestamp DateTime,
    value Float64
) ENGINE = MergeTree()
ORDER BY timestamp
TTL timestamp + INTERVAL 365 DAY;