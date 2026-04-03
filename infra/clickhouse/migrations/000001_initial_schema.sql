-- Migration 001: Initial AĒR Gold Layer schema

CREATE DATABASE IF NOT EXISTS aer_gold;

CREATE TABLE IF NOT EXISTS aer_gold.metrics (
    timestamp DateTime,
    value Float64
) ENGINE = MergeTree()
ORDER BY timestamp
TTL timestamp + INTERVAL 365 DAY;
