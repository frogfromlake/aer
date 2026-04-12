# Operations Playbook

> **Purpose:** Practical reference for accessing, inspecting, and debugging every component in the AĒR infrastructure stack.
> For *architectural rationale*, see the [arc42 documentation](arc42/01_introduction_and_goals.md). For *scientific workflows* (when and why to populate tables), see the [Scientific Operations Guide](scientific_operations_guide.md). This document covers *how to operate*.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Stack Lifecycle](#stack-lifecycle)
3. [PostgreSQL (Metadata Index)](#postgresql-metadata-index)
    - [Source Classifications (WP-001)](#source-classifications-wp-001) — *touchpoint*
4. [ClickHouse (Gold Layer — Analytics)](#clickhouse-gold-layer-analytics)
    - [Metric Validity (WP-002)](#metric-validity-wp-002) — *touchpoint*
    - [Metric Baselines & Equivalence (WP-004)](#metric-baselines-equivalence-wp-004) — *touchpoint*
5. [MinIO (Data Lake — Object Storage)](#minio-data-lake-object-storage)
6. [NATS JetStream (Event Broker)](#nats-jetstream-event-broker)
7. [Observability Stack](#observability-stack)
    - [Grafana](#grafana-dashboards-traces) · [Prometheus](#prometheus-metrics-alerting) · [Tempo](#tempo-distributed-tracing) · [OpenTelemetry Collector](#opentelemetry-collector)
8. [Application Services](#application-services)
    - [Ingestion API (Go)](#ingestion-api-go)
    - [BFF API (Go)](#bff-api-go) — incl. [Metric Provenance Config](#metric-provenance-config) *touchpoint*
    - [Analysis Worker (Python)](#analysis-worker-python) — incl. `BiasContext` *touchpoint*
    - [RSS Crawler](#rss-crawler)
9. [Configuration & Documentation Files](#configuration-documentation-files)
    - [Cultural Calendar Files](#cultural-calendar-files) — *touchpoint*
    - [Probe Dossier](#probe-dossier) — *touchpoint*
10. [Arc42 Documentation Server](#arc42-documentation-server)
11. [Testing](#testing)
12. [Volume Management](#volume-management)
13. [Network Architecture](#network-architecture)
14. [Scientific Touchpoints Index](#scientific-touchpoints-index)
15. [Quick Reference Card](#quick-reference-card)

Sections marked *touchpoint* are points at which scientific judgment enters the pipeline. Each touchpoint links to the corresponding workflow in the [Scientific Operations Guide](scientific_operations_guide.md), and that guide links back here for the exact commands. The mapping is consolidated in the [Scientific Touchpoints Index](#scientific-touchpoints-index) below.

---

## Prerequisites

Before using any service, ensure the stack is running and the debug ports are exposed:

```bash
# Start infrastructure + application services
make up

# Expose all backend ports to localhost (required for direct access)
make debug-up
```

All credentials are defined in `.env` (copy from `.env.example` on first setup). The variable names referenced below correspond to the keys in that file.

---

## Stack Lifecycle

```bash
make up                  # Start everything (infra + services)
make down                # Stop everything
make restart             # Full restart

make infra-up            # Infrastructure only (Traefik, databases, NATS, observability)
make services-up         # Application services only (requires infra running)

make debug-up            # Expose backend ports to localhost
make debug-down          # Close debug ports (services keep running)

make logs                # Tail combined service logs (Ctrl+C safe)
```

---

## PostgreSQL (Metadata Index)

Stores ingestion job metadata, document lifecycle states, source registry, source classifications, and trace IDs.

**Connection (debug profile required):**

| Property      | Value                              |
| :------------ | :--------------------------------- |
| Host          | `localhost`                        |
| Port          | `5432`                             |
| User          | `$POSTGRES_USER`                   |
| Password      | `$POSTGRES_PASSWORD`               |
| Database      | `$POSTGRES_DB`                     |
| Internal host | `postgres:5432` (from containers)  |

```bash
# Connect via psql
psql -h localhost -p 5432 -U $POSTGRES_USER -d $POSTGRES_DB

# Quick inspection queries
psql -h localhost -p 5432 -U $POSTGRES_USER -d $POSTGRES_DB -c "\dt"              # List tables
psql -h localhost -p 5432 -U $POSTGRES_USER -d $POSTGRES_DB -c "SELECT * FROM sources;"
psql -h localhost -p 5432 -U $POSTGRES_USER -d $POSTGRES_DB -c "SELECT id, source_id, status, created_at FROM documents ORDER BY created_at DESC LIMIT 10;"
```

**Key tables:**

- `sources` — Registered data sources (e.g., `bundesregierung`, `tagesschau`). Crawlers resolve their `source_id` via `GET /api/v1/sources?name=<n>`. Includes `documentation_url` pointing to the Probe Dossier for each source.
- `ingestion_jobs` — Batch job tracking with status (`running`, `completed`, `completed_with_errors`, `failed`).
- `documents` — Per-document lifecycle (`pending` → `uploaded` → `processed`), including `bronze_object_key` and `trace_id`.
- `source_classifications` — Etic/emic discourse-function classification per source (WP-001). See [Source Classifications](#source-classifications-wp-001) below.

**Migrations** are in `infra/postgres/migrations/` and run automatically on `ingestion-api` startup via `golang-migrate`.

**Retention policy (Phase 52):**

Records older than 90 days are automatically removed by the `ingestion-api` background goroutine (`startRetentionCleanup`). Documents are deleted first (FK constraint), then orphaned completed/failed ingestion jobs. The 90-day window matches the MinIO bronze ILM policy — after 90 days the underlying object is gone, making the PostgreSQL record an orphan regardless.

```bash
# Inspect current table sizes
psql -h localhost -p 5432 -U $POSTGRES_USER -d $POSTGRES_DB \
  -c "SELECT relname, n_live_tup FROM pg_stat_user_tables ORDER BY n_live_tup DESC;"

# Manually check oldest document record
psql -h localhost -p 5432 -U $POSTGRES_USER -d $POSTGRES_DB \
  -c "SELECT MIN(ingested_at), MAX(ingested_at), COUNT(*) FROM documents;"

# Retention cleanup is logged by the ingestion-api at INFO level:
#   "PostgreSQL retention: deleted old documents" count=N cutoff=...
#   "PostgreSQL retention: deleted old ingestion jobs" count=N cutoff=...
make logs | grep "PostgreSQL retention"
```

**Volume management:**

```bash
make infra-clean-postgres   # Wipe PostgreSQL volume (interactive confirmation)
```

### Source Classifications (WP-001)

The `source_classifications` table records the etic/emic discourse-function classification of each data source. It is populated by seed migrations under `infra/postgres/migrations/` and updated as the WP-001 §4.4 review process advances a row through `provisional_engineering` → `pending` → `reviewed` (or `contested`). The composite primary key `(source_id, classification_date)` is designed for temporal tracking — insert new rows instead of updating existing ones.

> Scientific rationale: [Scientific Operations Guide → Workflow 1: Classifying a New Probe](scientific_operations_guide.md#workflow-1-classifying-a-new-probe).

**Inspect:**

```bash
psql -h localhost -p 5432 -U $POSTGRES_USER -d $POSTGRES_DB \
  -c "SELECT source_id, primary_function, secondary_function, function_weights, review_status, classification_date FROM source_classifications ORDER BY classification_date DESC;"
```

**Generic template (insert a new classification row):**

```sql
-- Replace placeholders. function_weights stays NULL until WP-001 §4.4 Steps 1-2 complete.
-- review_status starts at 'provisional_engineering' for engineering-team-only classifications.
INSERT INTO source_classifications (
    source_id, primary_function, secondary_function, function_weights,
    emic_designation, emic_context, emic_language,
    classified_by, classification_date, review_status
)
SELECT
    id,
    '<primary>',           -- one of: epistemic_authority | power_legitimation | cohesion_identity | subversion_friction
    '<secondary>',         -- same enum, or NULL
    NULL,                  -- function_weights JSONB, e.g. '{"primary": 0.7, "secondary": 0.3}'::jsonb after WP-001 §4.4
    '<emic_designation>',  -- local participant-perspective name
    '<emic_context>',      -- one-paragraph contextual self-understanding
    '<lang>',              -- ISO 639-1
    '<classified_by>',     -- e.g. 'WP-001/<probe-id>' or reviewer name
    CURRENT_DATE,
    'provisional_engineering'
FROM sources WHERE name = '<source-name>'
ON CONFLICT DO NOTHING;
```

**Probe 0 example** (matches migration `000006`):

```sql
INSERT INTO source_classifications (
    source_id, primary_function, secondary_function, function_weights,
    emic_designation, emic_context, emic_language,
    classified_by, classification_date, review_status
)
SELECT
    id,
    'epistemic_authority',
    'power_legitimation',
    NULL,
    'Tagesschau',
    'State-funded public broadcaster (ARD). Norm-setting through informational baseline. Editorial independence structurally influenced by inter-party proportional governance.',
    'de',
    'WP-001/Probe-0',
    '2026-04-11',
    'provisional_engineering'
FROM sources WHERE name = 'tagesschau'
ON CONFLICT DO NOTHING;
```

**Advancing the review status** (after WP-001 §4.4 Steps 1–4 complete and `function_weights` are quantified, insert a *new* row — do **not** UPDATE; the composite PK is designed to track temporal transitions):

```sql
INSERT INTO source_classifications (
    source_id, primary_function, secondary_function, function_weights,
    emic_designation, emic_context, emic_language,
    classified_by, classification_date, review_status
)
SELECT id, 'epistemic_authority', 'power_legitimation',
       '{"primary": 0.7, "secondary": 0.3}'::jsonb,
       'Tagesschau', '<updated context after review>', 'de',
       '<reviewer-name>', CURRENT_DATE, 'reviewed'
FROM sources WHERE name = 'tagesschau';
```

---

## ClickHouse (Gold Layer — Analytics)

Column-oriented OLAP database storing derived time-series metrics, entities, language detections, and scientific infrastructure tables (validation, baselines, equivalence).

**Connection (debug profile required):**

| Property      | Value                                          |
| :------------ | :--------------------------------------------- |
| HTTP UI       | `http://localhost:8123/play`                    |
| Native port   | `localhost:9002`                                |
| User          | `$CLICKHOUSE_USER`                             |
| Password      | `$CLICKHOUSE_PASSWORD`                         |
| Database      | `$CLICKHOUSE_DB` (default: `aer_gold`)         |
| Internal host | `clickhouse:8123` (HTTP) / `clickhouse:9000` (native) |

```bash
# Browser-based query UI
open http://localhost:8123/play

# CLI via Docker (no local install needed)
docker exec -it aer_clickhouse clickhouse-client

# Example queries inside the client
SELECT count() FROM aer_gold.metrics;
SELECT source, metric_name, count() FROM aer_gold.metrics GROUP BY source, metric_name;
SELECT * FROM aer_gold.metrics ORDER BY timestamp DESC LIMIT 20;

-- Entities queries
SELECT count() FROM aer_gold.entities;
SELECT entity_label, count() FROM aer_gold.entities GROUP BY entity_label ORDER BY count() DESC;
SELECT entity_text, entity_label, count() AS mentions FROM aer_gold.entities GROUP BY entity_text, entity_label ORDER BY mentions DESC LIMIT 20;

SELECT * FROM aer_gold.schema_migrations ORDER BY version;
```

**Schema — Pipeline tables (populated by the analysis worker):**

- `aer_gold.metrics` — columns: `timestamp`, `value`, `source`, `metric_name`, `article_id`, `discourse_function`. MergeTree engine, ordered by `timestamp`, 365-day TTL.
- `aer_gold.entities` — columns: `timestamp`, `source`, `article_id`, `entity_text`, `entity_label`, `start_char`, `end_char`, `discourse_function`. MergeTree engine, ordered by `timestamp`, 365-day TTL. Entity labels follow spaCy German NER taxonomy: `PER`, `ORG`, `LOC`, `MISC`.
- `aer_gold.language_detections` — columns: `timestamp`, `source`, `article_id`, `detected_language`, `confidence`, `rank`. MergeTree engine, ordered by `(timestamp, source)`, 365-day TTL.

**Schema — Scientific infrastructure tables (populated by researchers or scripts):**

- `aer_gold.metric_validity` — Validation study outcomes per `(metric_name, context_key)`. See [Metric Validity](#metric-validity-wp-002) below.
- `aer_gold.metric_baselines` — Per-`(metric_name, source, language)` mean/stddev for z-score normalization. See [Metric Baselines & Equivalence](#metric-baselines-equivalence-wp-004) below.
- `aer_gold.metric_equivalence` — Cross-cultural comparability claims. See [Metric Baselines & Equivalence](#metric-baselines-equivalence-wp-004) below.

**Migrations** are in `infra/clickhouse/migrations/` and run via the `clickhouse-init` container (`infra/clickhouse/migrate.sh`) on startup. Applied versions are tracked in `aer_gold.schema_migrations`.

**Volume management:**

```bash
make infra-clean-clickhouse   # Wipe ClickHouse volume (interactive confirmation)
```

### Metric Validity (WP-002)

The `aer_gold.metric_validity` table (`ReplacingMergeTree`, Migration 006) records the outcome of a validation study for one `(metric_name, context_key)` pair. The BFF API joins this table at query time to compute the `validationStatus` field on `GET /api/v1/metrics/available` and `GET /api/v1/metrics/{metricName}/provenance`.

> Scientific rationale: [Scientific Operations Guide → Workflow 2: Validating a Metric](scientific_operations_guide.md#workflow-2-validating-a-metric). Template: `docs/templates/validation_study_template.yaml`.

**Inspect:**

```sql
-- Connect: docker exec -it aer_clickhouse clickhouse-client
SELECT metric_name, context_key, alpha_score, correlation, n_annotated, valid_until
  FROM aer_gold.metric_validity FINAL
 ORDER BY metric_name, context_key;
```

**Generic template:**

```sql
-- Required fields mirror docs/templates/validation_study_template.yaml.
-- Methodological minima: n_annotated >= 3 annotators, alpha_score >= 0.667 (Krippendorff).
INSERT INTO aer_gold.metric_validity
    (metric_name, context_key, validation_date, alpha_score, correlation,
     n_annotated, error_taxonomy, valid_until)
VALUES
    ('<metric_name>',
     '<lang>:<source_type>:<discourse_function>',  -- e.g. 'de:rss:epistemic_authority'
     now(),
     0.000,    -- Krippendorff alpha from the annotation study
     0.000,    -- Pearson/Spearman correlation with reference annotations
     0,        -- sample size (number of annotated documents)
     '<error_taxonomy>',  -- semicolon-separated error categories or JSON
     toDate('YYYY-MM-DD'));  -- expiry date for the validation claim
```

**Probe 0 example** (hypothetical — no actual study has been performed; `metric_validity` is empty in production):

```sql
-- Hypothetical sentiment_score validation study for Probe 0.
-- DO NOT run this against production. It illustrates the shape of a row
-- that Workflow 2 would produce after a completed annotation study.
INSERT INTO aer_gold.metric_validity
    (metric_name, context_key, validation_date, alpha_score, correlation,
     n_annotated, error_taxonomy, valid_until)
VALUES
    ('sentiment_score',
     'de:rss:epistemic_authority',
     now(),
     0.71,     -- acceptable inter-annotator agreement
     0.62,     -- moderate correlation with human judgment
     250,      -- 250 documents annotated by 3 annotators
     'negation_failure;compound_failure;genre_drift',
     toDate('2027-04-12'));  -- valid for 12 months
```

When `valid_until` passes, the BFF API automatically reverts the metric to `validationStatus = 'unvalidated'` — no manual cleanup is required.

### Metric Baselines & Equivalence (WP-004)

`aer_gold.metric_baselines` stores the per-`(metric_name, source, language)` mean and standard deviation used as the denominator for z-score normalization. `aer_gold.metric_equivalence` records validated cross-instrument comparability claims at one of three levels (`temporal`, `deviation`, `absolute`) and gates the `?normalization=zscore` query parameter on the BFF API.

> Scientific rationale: [Scientific Operations Guide → Workflow 3: Establishing Metric Equivalence](scientific_operations_guide.md#workflow-3-establishing-metric-equivalence) and [Workflow 4: Computing and Updating Baselines](scientific_operations_guide.md#workflow-4-computing-and-updating-baselines).

**Inspect:**

```sql
SELECT * FROM aer_gold.metric_baselines FINAL ORDER BY metric_name, source;
SELECT * FROM aer_gold.metric_equivalence FINAL ORDER BY etic_construct, metric_name;
```

**Computing baselines.** `scripts/compute_baselines.py` runs offline against the ClickHouse instance and writes one row per `(metric_name, source, language)` it finds.

```bash
# Required env (read from .env if present):
#   CLICKHOUSE_HOST, CLICKHOUSE_PORT, CLICKHOUSE_USER, CLICKHOUSE_PASSWORD, CLICKHOUSE_DB
# Common flags:
#   --metric <name>     restrict to one metric (default: all)
#   --window <days>     reference window in days (default: 90)
#   --dry-run           compute and print, do not insert

cd services/analysis-worker
python scripts/compute_baselines.py --metric word_count --window 90 --dry-run
python scripts/compute_baselines.py --metric word_count --window 90
```

**Probe 0 example** (compute the `word_count` baseline for tagesschau.de):

```bash
python scripts/compute_baselines.py --metric word_count --window 90
# Expected output (one line per (metric, source, language)):
#   word_count | tagesschau | de | mean=312.4 | std=158.7 | n=4500
#   word_count | bundesregierung | de | mean=487.2 | std=203.1 | n=450
# Each line corresponds to an INSERT into aer_gold.metric_baselines.
```

**Generic template (manual equivalence INSERT):**

```sql
INSERT INTO aer_gold.metric_equivalence
    (etic_construct, metric_name, language, source_type,
     equivalence_level, validated_by, validation_date, confidence)
VALUES
    ('<etic_construct>',           -- e.g. 'evaluative_polarity'
     '<metric_name>',
     '<lang>',                     -- ISO 639-1
     '<source_type>',              -- e.g. 'rss'
     '<temporal|deviation|absolute>',
     '<doi-or-validation-study-id>',
     now(),
     0.00);                        -- confidence score from the equivalence study
```

**Probe 0 note:** A single probe cannot establish cross-cultural equivalence — it requires at least two probes from different cultural contexts. `?normalization=zscore` against Probe 0 sources returns HTTP 400 by design until a second probe enables meaningful comparison. This is not a bug — it is the validation gate (WP-004 §7.3) functioning as intended.

---

## MinIO (Data Lake — Object Storage)

S3-compatible storage for the Medallion Architecture layers (Bronze, Silver, DLQ).

**Connection (debug profile required):**

| Property      | Value                                |
| :------------ | :----------------------------------- |
| API           | `http://localhost:9000`              |
| Console (UI)  | `http://localhost:9001`              |
| Console (TLS) | `https://$MINIO_CONSOLE_HOST` (via Traefik) |
| Access Key    | `$MINIO_ROOT_USER`                  |
| Secret Key    | `$MINIO_ROOT_PASSWORD`              |
| Internal host | `minio:9000` (from containers)      |

```bash
# Web console
open http://localhost:9001

# CLI via mc (MinIO Client) — install: https://min.io/docs/minio/linux/reference/minio-mc.html
mc alias set aer http://localhost:9000 $MINIO_ROOT_USER $MINIO_ROOT_PASSWORD

mc ls aer/                    # List buckets
mc ls aer/bronze/             # List Bronze layer objects
mc ls aer/silver/             # List Silver layer objects
mc ls aer/bronze-quarantine/  # List DLQ (malformed data)

mc cat aer/bronze/tagesschau/some-article/2026-04-01.json   # Read a raw document
mc cat aer/silver/tagesschau/some-article/2026-04-01.json   # Read harmonized version
```

**Buckets** (provisioned by `minio-init` container on startup):

- `bronze` — Raw, immutable source data (write-once by Ingestion API).
- `silver` — Harmonized data validated against the Silver Contract (written by Analysis Worker).
- `bronze-quarantine` — Dead Letter Queue for malformed documents.

**Event notifications:** MinIO publishes `PUT` events from the `bronze` bucket to NATS subject `aer.lake.bronze` via the built-in NATS notification target. This is configured via `MINIO_NOTIFY_NATS_*` environment variables in `compose.yaml`.

**Volume management:**

```bash
make infra-clean-minio   # Wipe MinIO volume (interactive confirmation)
```

---

## NATS JetStream (Event Broker)

Durable message broker mediating all inter-service communication.

**Connection (debug profile required):**

| Property       | Value                               |
| :------------- | :---------------------------------- |
| Client         | `localhost:4222`                    |
| Monitoring UI  | `http://localhost:8222`             |
| Internal host  | `nats:4222` (from containers)       |

```bash
# Monitoring dashboard
open http://localhost:8222

# Inspect via nats CLI (install: https://github.com/nats-io/natscli)
nats -s nats://localhost:4222 stream ls
nats -s nats://localhost:4222 stream info AER_LAKE
nats -s nats://localhost:4222 consumer ls AER_LAKE
nats -s nats://localhost:4222 stream view AER_LAKE   # View recent messages
```

**Stream configuration** (provisioned by `nats-init` container):

- Stream: `AER_LAKE`
- Subjects: `aer.lake.>`
- Storage: File-backed (durable across restarts)
- Delivery: At-least-once (manual `msg.ack()` by Analysis Worker)

---

## Observability Stack

### Grafana (Dashboards & Traces)

| Property       | Value                                 |
| :------------- | :------------------------------------ |
| URL (debug)    | `http://localhost:3000`               |
| URL (TLS)      | `https://$GRAFANA_HOST` (via Traefik) |
| User           | `$GF_SECURITY_ADMIN_USER`            |
| Password       | `$GF_SECURITY_ADMIN_PASSWORD`        |

Pre-provisioned datasources: Tempo (traces) and Prometheus (metrics). Dashboards auto-load from `infra/observability/grafana/provisioning/dashboards/`.

**Finding a trace:** Navigate to Explore → select Tempo datasource → paste a `trace_id` from PostgreSQL's `documents` table. The trace spans the entire pipeline: Ingestion → NATS → Analysis Worker → ClickHouse.

### Prometheus (Metrics & Alerting)

| Property       | Value                                        |
| :------------- | :------------------------------------------- |
| Internal only  | `prometheus:9090` (not exposed to host)       |

Accessed via Grafana. Scrapes the OTel Collector (`:8889`) and the Analysis Worker (`:8001`) every 5 seconds. Alert rules are defined in `infra/observability/prometheus/alert.rules.yml`.

### Tempo (Distributed Tracing)

| Property       | Value                                        |
| :------------- | :------------------------------------------- |
| Internal only  | `tempo:3200` (not exposed to host)            |

Accessed via Grafana. Stores traces with configurable block retention (72h dev / 720h prod). Trace context propagates across the NATS boundary via message headers.

### OpenTelemetry Collector

| Property       | Value                                          |
| :------------- | :--------------------------------------------- |
| gRPC receiver  | `otel-collector:4317` (debug: `localhost:4317`) |
| HTTP receiver  | `otel-collector:4318` (debug: `localhost:4318`) |
| Prom exporter  | `otel-collector:8889`                           |

Configuration: `infra/observability/otel-collector.yaml`. Routes traces to Tempo and metrics to Prometheus.

**Trace sampling** is configurable via `OTEL_TRACE_SAMPLE_RATE` (default: `1.0` = 100%). Production recommendation: `0.1` (10%).

---

## Application Services

### Ingestion API (Go)

| Property       | Value                                       |
| :------------- | :------------------------------------------ |
| Port (debug)   | `http://localhost:8081`                      |
| Internal host  | `ingestion-api:8081`                         |
| Auth           | `X-API-Key` header or `Bearer` token        |
| API Key        | `$INGESTION_API_KEY`                         |

```bash
# Health check (no auth required)
curl http://localhost:8081/api/v1/healthz
curl http://localhost:8081/api/v1/readyz

# Resolve a source ID
curl -H "X-API-Key: $INGESTION_API_KEY" "http://localhost:8081/api/v1/sources?name=tagesschau"

# Ingest a test document
curl -X POST http://localhost:8081/api/v1/ingest \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $INGESTION_API_KEY" \
  -d '{
    "source_id": 1,
    "documents": [{
      "key": "wikipedia/test-article/2026-04-04.json",
      "data": {
        "source": "wikipedia",
        "title": "Test Article",
        "raw_text": "This is a test document for pipeline validation.",
        "url": "https://en.wikipedia.org/wiki/Test",
        "timestamp": "2026-04-04T12:00:00Z"
      }
    }]
  }'
```

### BFF API (Go)

| Property       | Value                                       |
| :------------- | :------------------------------------------ |
| Port           | `http://localhost:8080`                      |
| TLS (Traefik)  | `https://$BFF_HOST/api/...`                 |
| Auth           | `X-API-Key` header or `Bearer` token        |
| API Key        | `$BFF_API_KEY`                              |

```bash
# Health check (no auth required)
curl http://localhost:8080/api/v1/healthz
curl http://localhost:8080/api/v1/readyz

# Query Gold layer metrics (raw values, 5-minute resolution — defaults)
curl -H "X-API-Key: $BFF_API_KEY" \
  "http://localhost:8080/api/v1/metrics?startDate=2026-01-01T00:00:00Z&endDate=2026-12-31T23:59:59Z"

# Filter by source and metric name
curl -H "X-API-Key: $BFF_API_KEY" \
  "http://localhost:8080/api/v1/metrics?startDate=2026-01-01T00:00:00Z&endDate=2026-12-31T23:59:59Z&source=tagesschau&metricName=word_count"

# Multi-resolution queries (hourly, daily, weekly, monthly)
curl -H "X-API-Key: $BFF_API_KEY" \
  "http://localhost:8080/api/v1/metrics?startDate=2026-01-01T00:00:00Z&endDate=2026-12-31T23:59:59Z&metricName=word_count&resolution=daily"

# Z-score normalization (requires baseline + equivalence entry — returns 400 for Probe 0)
curl -H "X-API-Key: $BFF_API_KEY" \
  "http://localhost:8080/api/v1/metrics?startDate=2026-01-01T00:00:00Z&endDate=2026-12-31T23:59:59Z&metricName=word_count&normalization=zscore"

# Discover available metric names (with validation status and equivalence metadata)
curl -H "X-API-Key: $BFF_API_KEY" \
  "http://localhost:8080/api/v1/metrics/available?startDate=2026-01-01T00:00:00Z&endDate=2026-12-31T23:59:59Z"

# Query named entities (aggregated by text and label)
curl -H "X-API-Key: $BFF_API_KEY" \
  "http://localhost:8080/api/v1/entities?startDate=2026-01-01T00:00:00Z&endDate=2026-12-31T23:59:59Z"

# Filter entities by source and NER label (PER, ORG, LOC, MISC)
curl -H "X-API-Key: $BFF_API_KEY" \
  "http://localhost:8080/api/v1/entities?startDate=2026-01-01T00:00:00Z&endDate=2026-12-31T23:59:59Z&source=bundesregierung&label=ORG&limit=50"

# Query language detections
curl -H "X-API-Key: $BFF_API_KEY" \
  "http://localhost:8080/api/v1/languages?startDate=2026-01-01T00:00:00Z&endDate=2026-12-31T23:59:59Z"

# List known data sources (with documentation URLs)
curl -H "X-API-Key: $BFF_API_KEY" \
  "http://localhost:8080/api/v1/sources"

# Metric provenance (tier, algorithm, known limitations, validation status)
curl -H "X-API-Key: $BFF_API_KEY" \
  "http://localhost:8080/api/v1/metrics/word_count/provenance"
```

**OpenAPI spec:** `services/bff-api/api/openapi.yaml`. Regenerate server stubs: `make codegen`.

#### Metric Provenance Config

`services/bff-api/configs/metric_provenance.yaml` is the SSoT for the static fields returned by `GET /api/v1/metrics/{metricName}/provenance`. The BFF loads it at startup; dynamic fields (`validationStatus`, `culturalContextNotes`) are resolved per-request against the ClickHouse `metric_validity` and `metric_equivalence` tables.

> Scientific rationale: [Scientific Operations Guide → Workflow 1, Step 5](scientific_operations_guide.md#workflow-1-classifying-a-new-probe) (registration) and [Workflow 2](scientific_operations_guide.md#workflow-2-validating-a-metric) (validation).

**Trigger for updates:** every time a new extractor is registered in `services/analysis-worker/internal/extractors/`. The registered-metric invariant is verified by the BFF handler tests — the build will fail if a metric exists in the worker but not in this YAML.

**Required fields per metric entry:**

| Field | Required | Notes |
| :--- | :--- | :--- |
| `tier_classification` | yes | 1, 2, or 3 (see ADR-016 / §8.12) |
| `algorithm_description` | yes | Plain-language summary |
| `known_limitations` | yes (may be empty) | Full sentences explaining problem and consequence |
| `extractor_version_hash` | yes | Pinned model/lexicon version or `v1` for deterministic |
| `wp_reference` | yes | YAML-only — Go parser ignores; for developer orientation |

**Probe 0 example** — the file currently registers six metrics from the Phase 42–45 extractors: `word_count`, `sentiment_score`, `language_confidence`, `publication_hour`, `publication_weekday`, `entity_count`. Each includes `known_limitations` as full sentences with WP-002 §3 cross-references where applicable.

### Analysis Worker (Python)

| Property         | Value                                    |
| :--------------- | :--------------------------------------- |
| Prometheus       | `http://localhost:8001/metrics` (internal)|
| Internal only    | No HTTP API — event-driven via NATS      |

The worker subscribes to NATS subject `aer.lake.bronze`, downloads raw documents from MinIO, validates against the Silver Contract (Pydantic), runs the extractor pipeline, writes harmonized data to the Silver bucket, and inserts metrics/entities into ClickHouse Gold layer.

**Concurrency:** Controlled by `WORKER_COUNT` in `.env` (default: `5`). Sets the number of concurrent event processing coroutines.

**Metric extractors** (registered in `main.py`, all provisional except Temporal):

| Extractor | Metric Name(s) | Description |
| :--- | :--- | :--- |
| `WordCountExtractor` | `word_count` | Token count of cleaned text |
| `TemporalDistributionExtractor` | `publication_hour`, `publication_weekday` | Publication time metadata |
| `LanguageDetectionExtractor` | `language_confidence` | Confidence score via `langdetect` |
| `SentimentExtractor` | `sentiment_score` | German lexicon polarity via SentiWS |
| `NamedEntityExtractor` | `entity_count` | spaCy `de_core_news_lg` NER; raw spans stored in `aer_gold.entities` |

Extractors are independent — one failing extractor does not crash others or route the document to the DLQ. Extractor source code is in `services/analysis-worker/internal/extractors/`.

**Source adapters** select the harmonization strategy based on `source_type` in the Bronze document:

| Source Type | Adapter | Silver Output |
| :--- | :--- | :--- |
| `rss` | `RSSAdapter` | `SilverCore` + `RssMeta` (feed_url, categories, author, feed_title, `DiscourseContext`, `BiasContext`) |
| *(missing/unknown)* | `LegacyAdapter` | Backward-compatible mapping for pre-Phase 39 documents |

The `RSSAdapter` reads the `source_classifications` table (via `get_source_classification(source_id)`) and populates `DiscourseContext` in `RssMeta`. If no classification exists, the field is `None` and the pipeline continues without failure. `BiasContext` is populated with static RSS-specific values (`platform_type='rss'`, `visibility_mechanism='chronological'`, etc.).

```bash
# Check worker logs
make logs   # Combined logs of all services
```

### RSS Crawler

The RSS crawler is a standalone Go binary under `crawlers/rss-crawler/`. It fetches German institutional RSS feeds and submits documents to the Ingestion API. It is **not** part of the Docker Compose stack — you run it manually or via a cron job.

```bash
# Quick start (builds and runs with defaults from .env)
make crawl

# Or manually: build + run with explicit flags
go build -o bin/rss-crawler ./crawlers/rss-crawler
./bin/rss-crawler \
  -config crawlers/rss-crawler/feeds.yaml \
  -api-url http://localhost:8081/api/v1/ingest \
  -sources-url http://localhost:8081/api/v1/sources \
  -api-key $INGESTION_API_KEY
```

`make crawl` requires `make debug-up` (so the Ingestion API is reachable on `localhost:8081`). It automatically sources `.env` to load `INGESTION_API_KEY` and any other configured values — if `.env` is missing, the target exits with an error before running the binary.

**CLI flags:**

| Flag | Description | Default |
| :--- | :--- | :--- |
| `-config` | Path to `feeds.yaml` | *(required)* |
| `-api-url` | Ingestion API ingest endpoint | *(required)* |
| `-sources-url` | Ingestion API sources endpoint | *(required)* |
| `-api-key` | `INGESTION_API_KEY` value | *(required)* |

**Feed configuration** (`crawlers/rss-crawler/feeds.yaml`):

```yaml
feeds:
  - name: bundesregierung   # Must match a source registered in PostgreSQL
    url: https://www.bundesregierung.de/breg-de/feed
  - name: tagesschau
    url: https://www.tagesschau.de/index~rss2.xml
```

Adding a new RSS feed requires:
1. A new entry in `feeds.yaml`.
2. A PostgreSQL seed migration in `infra/postgres/migrations/` registering the source name.
3. A Probe Dossier and Probe Classification workflow (see [Scientific Operations Guide → Workflow 1](scientific_operations_guide.md#workflow-1-classifying-a-new-probe)).

**Deduplication:** The crawler maintains a local state file (`.rss-crawler-state.json`) to prevent re-ingestion of previously submitted items across runs. State is written to disk immediately after each feed's batch is successfully submitted — not once at the end — so a crash or interruption mid-run does not cause re-ingestion of already-processed feeds.

**HTTP timeouts:** All outbound HTTP clients enforce strict timeouts — 10 s for source ID lookup, 30 s for ingestion API posts, and 30 s for RSS feed fetches — to prevent the crawler from hanging indefinitely on unresponsive upstreams.

**Graceful shutdown:** The crawler respects `SIGINT`/`SIGTERM` via `signal.NotifyContext`. Cancelling mid-run will abort in-flight HTTP requests cleanly; any feeds processed before the signal will have their state already persisted.

---

## Configuration & Documentation Files

### Cultural Calendar Files

Per-region cultural calendars live under `configs/cultural_calendars/<region>.yaml`. They enumerate culturally significant dates (public holidays, elections, religious observances, recurring major media events) whose presence is expected to perturb discourse metrics. The calendars are static lookups for manual interpretation — they are not yet wired into the query layer.

> Scientific rationale: [Scientific Operations Guide → Workflow 6: Updating the Cultural Calendar](scientific_operations_guide.md#workflow-6-updating-the-cultural-calendar).

**File format** (per entry):

```yaml
- date: "YYYY-MM-DD"        # or "MM-DD" with recurrence: annual
  recurrence: annual         # optional: annual | movable
  name: "<local name>"
  type: public_holiday       # public_holiday | election | media_event | commemoration
  expected_discourse_effect: "<qualitative note>"
```

**Adding a new region:**

1. Create `configs/cultural_calendars/<iso-region>.yaml`.
2. Set `region`, `language`, and `last_updated` at the top of the file.
3. Add entries; group by category with comment headers (`# --- Public Holidays ---`).
4. Reference the file from the corresponding Probe Dossier `temporal_profile.md`.

**Probe 0 example** — `configs/cultural_calendars/de.yaml` contains German federal public holidays, Bundestag election dates, Easter-based movable feasts, and recurring media events (Berlinale, Buchmesse). See the file for the full list.

### Probe Dossier

The Probe Dossier Pattern groups all per-probe scientific documentation under a single directory `docs/probes/<probe-id>/`. Per-metric provenance (`metric_provenance.yaml`) and per-validation context (`metric_validity`) are system-wide and intentionally **not** part of the dossier — they are referenced from the dossier's `README.md` WP Coverage Matrix.

> Scientific rationale: Arc42 §8.15 — Probe Dossier Pattern. [Scientific Operations Guide → Workflow 1, Step 5](scientific_operations_guide.md#workflow-1-classifying-a-new-probe).

**Mandatory dossier files:**

| File | WP | Purpose |
| :--- | :--- | :--- |
| `README.md` | (overview) | Probe overview, source list, calibration status, exit criteria, WP coverage matrix |
| `classification.md` | WP-001 | Etic/emic classification, mirror of the `source_classifications` row |
| `bias_assessment.md` | WP-003 | `BiasContext` values and structural biases per source |
| `temporal_profile.md` | WP-005 | Publication-rate heuristics, `min_meaningful_resolution` derivation, cultural-calendar pointer |
| `observer_effect.md` | WP-006 | Completed `observer_effect_assessment.yaml` for the probe |

**Authoring a new dossier:**

1. `mkdir docs/probes/<probe-id>/` (use a kebab-case slug, e.g. `probe-1-fr-civic-forum`).
2. Copy the five files from `docs/probes/probe-0-de-institutional-rss/` and replace the content. Keep the headings stable so the structure stays uniform across probes.
3. Add the dossier to `mkdocs.yml` under the `Probes` nav entry.
4. Create a PostgreSQL migration that updates `sources.documentation_url` to point at the new dossier directory:
   ```sql
   UPDATE sources SET documentation_url = 'docs/probes/<probe-id>/' WHERE name = '<source-name>';
   ```

**Probe 0 example** — `docs/probes/probe-0-de-institutional-rss/` contains the five files above, populated for tagesschau.de and bundesregierung.de. Migration `000008_update_documentation_url.up.sql` points the `sources.documentation_url` column at this directory.

---

## Arc42 Documentation Server

| Property       | Value                                |
| :------------- | :----------------------------------- |
| URL            | `http://localhost:8000`              |
| Live reload    | Yes (auto-refreshes on `.md` edits)  |

```bash
open http://localhost:8000
```

The MkDocs Material container mounts the repository root and serves `docs/` with live reload. Editing any `.md` file triggers an immediate refresh.

---

## Testing

```bash
make test            # Full suite: Go integration + Python unit tests
make test-go         # Go integration tests (requires Docker for Testcontainers)
make test-go-pkg     # Shared pkg/ module tests
make test-e2e        # Docker Compose end-to-end smoke test
make lint            # golangci-lint (Go) + ruff (Python, with .venv fallback)
make codegen         # Regenerate OpenAPI stubs, then check for drift
```

**Testcontainers:** Go and Python tests spin up real database containers using image tags parsed from `compose.yaml` (SSoT enforcement). No hardcoded tags in test files.

**E2E smoke test** (`scripts/e2e_smoke_test.sh`): Starts a fixture HTTP server → runs the RSS crawler against test fixtures → waits for pipeline processing → queries BFF API endpoints (metrics, entities, available metrics, provenance) → validates end-to-end data flow including `discourse_function` propagation and multi-resolution queries → teardown.

---

## Volume Management

```bash
make infra-clean             # Wipe ALL persistent volumes (with confirmation)
make infra-clean-postgres    # Wipe PostgreSQL only
make infra-clean-minio       # Wipe MinIO only (Bronze, Silver, DLQ data)
make infra-clean-clickhouse  # Wipe ClickHouse only (Gold metrics)
```

All wipe commands require interactive confirmation. After wiping, the next `make up` will re-run init containers and migrations automatically.

---

## Network Architecture

AĒR follows a Zero-Trust posture (ADR-013). Two isolated Docker networks:

- **`aer-frontend`** — Traefik only. Internet-facing.
- **`aer-backend`** — All databases, NATS, observability, application services.

Only `bff-api` and `grafana` bridge both networks. No backend service is reachable from the host unless `make debug-up` is explicitly run.

---

## Scientific Touchpoints Index

The following table indexes every point where scientific judgment enters the AĒR pipeline — tables populated by researchers, config files set by domain experts, and documentation authored by interdisciplinary collaborators. Each row links to the relevant Playbook section (for "what to type") and the Scientific Operations Guide workflow (for "when and why").

| Touchpoint | Technology | Playbook Section | Scientific Operations Guide |
| :--- | :--- | :--- | :--- |
| `source_classifications` | PostgreSQL | [Source Classifications](#source-classifications-wp-001) | [Workflow 1: Classifying a New Probe](scientific_operations_guide.md#workflow-1-classifying-a-new-probe) |
| `aer_gold.metric_validity` | ClickHouse | [Metric Validity](#metric-validity-wp-002) | [Workflow 2: Validating a Metric](scientific_operations_guide.md#workflow-2-validating-a-metric) |
| `aer_gold.metric_equivalence` | ClickHouse | [Metric Baselines & Equivalence](#metric-baselines-equivalence-wp-004) | [Workflow 3: Establishing Metric Equivalence](scientific_operations_guide.md#workflow-3-establishing-metric-equivalence) |
| `aer_gold.metric_baselines` | ClickHouse | [Metric Baselines & Equivalence](#metric-baselines-equivalence-wp-004) | [Workflow 4: Computing and Updating Baselines](scientific_operations_guide.md#workflow-4-computing-and-updating-baselines) |
| `metric_provenance.yaml` | BFF Config | [Metric Provenance Config](#metric-provenance-config) | [Workflow 1 (Step 5)](scientific_operations_guide.md#workflow-1-classifying-a-new-probe) / [Workflow 2](scientific_operations_guide.md#workflow-2-validating-a-metric) |
| Probe Dossier (`docs/probes/`) | Filesystem | [Probe Dossier](#probe-dossier) | [Workflow 1 (Step 5)](scientific_operations_guide.md#workflow-1-classifying-a-new-probe) / [Workflow 5](scientific_operations_guide.md#workflow-5-assessing-bias-for-a-data-source) |
| `BiasContext` (in adapter code) | Python | [Analysis Worker](#analysis-worker-python) | [Workflow 5: Assessing Bias](scientific_operations_guide.md#workflow-5-assessing-bias-for-a-data-source) |
| Cultural Calendar (`configs/cultural_calendars/`) | Filesystem | [Cultural Calendar Files](#cultural-calendar-files) | [Workflow 6: Updating the Cultural Calendar](scientific_operations_guide.md#workflow-6-updating-the-cultural-calendar) |

---

## Quick Reference Card

| Service              | Debug URL / Port                  | Credentials                                    |
| :------------------- | :-------------------------------- | :--------------------------------------------- |
| PostgreSQL           | `localhost:5432`                  | `$POSTGRES_USER` / `$POSTGRES_PASSWORD`        |
| ClickHouse (UI)      | `http://localhost:8123/play`      | `$CLICKHOUSE_USER` / `$CLICKHOUSE_PASSWORD`    |
| ClickHouse (native)  | `localhost:9002`                  | same                                           |
| MinIO Console        | `http://localhost:9001`           | `$MINIO_ROOT_USER` / `$MINIO_ROOT_PASSWORD`   |
| MinIO API            | `http://localhost:9000`           | same                                           |
| NATS                 | `localhost:4222`                  | none                                           |
| NATS Monitor         | `http://localhost:8222`           | none                                           |
| Grafana              | `http://localhost:3000`           | `$GF_SECURITY_ADMIN_USER` / `$GF_SECURITY_ADMIN_PASSWORD` |
| Ingestion API        | `http://localhost:8081`           | `$INGESTION_API_KEY`                           |
| BFF API              | `http://localhost:8080`           | `$BFF_API_KEY`                                 |
| OTel Collector       | `localhost:4317` (gRPC) / `4318`  | none                                           |
| Arc42 Docs           | `http://localhost:8000`           | none                                           |