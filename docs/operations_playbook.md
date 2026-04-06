# Operations Playbook

> **Purpose:** Practical reference for accessing, inspecting, and debugging every component in the AĒR infrastructure stack.
> For *architectural rationale*, see the [arc42 documentation](arc42/01_introduction_and_goals.md). This document covers *how to operate*.

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

make infra-up            # Infrastructure only (databases, NATS, observability)
make services-up         # Application services only (requires infra running)

make debug-up            # Expose backend ports to localhost
make debug-down          # Close debug ports (services keep running)

make logs                # Tail combined service logs (Ctrl+C safe)
```

---

## PostgreSQL (Metadata Index)

Stores ingestion job metadata, document lifecycle states, source registry, and trace IDs.

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

- `sources` — Registered data sources (e.g., `bundesregierung`, `tagesschau`). Crawlers resolve their `source_id` via `GET /api/v1/sources?name=<n>`.
- `ingestion_jobs` — Batch job tracking with status (`running`, `completed`, `completed_with_errors`, `failed`).
- `documents` — Per-document lifecycle (`pending` → `uploaded` → `processed`), including `bronze_object_key` and `trace_id`.

**Migrations** are in `infra/postgres/migrations/` and run automatically on `ingestion-api` startup via `golang-migrate`.

**Volume management:**

```bash
make infra-clean-postgres   # Wipe PostgreSQL volume (interactive confirmation)
```

---

## ClickHouse (Gold Layer — Analytics)

Column-oriented OLAP database storing derived time-series metrics.

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

**Schema:**

- `aer_gold.metrics` — columns: `timestamp`, `value`, `source`, `metric_name`, `article_id`. MergeTree engine, ordered by `timestamp`, 365-day TTL.
- `aer_gold.entities` — columns: `timestamp`, `source`, `article_id`, `entity_text`, `entity_label`, `start_char`, `end_char`. MergeTree engine, ordered by `timestamp`, 365-day TTL. Entity labels follow spaCy German NER taxonomy: `PER`, `ORG`, `LOC`, `MISC`.

**Migrations** are in `infra/clickhouse/migrations/` and run via the `clickhouse-init` container (`infra/clickhouse/migrate.sh`) on startup. Applied versions are tracked in `aer_gold.schema_migrations`.

**Volume management:**

```bash
make infra-clean-clickhouse   # Wipe ClickHouse volume (interactive confirmation)
```

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

# Query Gold layer metrics
curl -H "X-API-Key: $BFF_API_KEY" \
  "http://localhost:8080/api/v1/metrics?startDate=2026-01-01T00:00:00Z&endDate=2026-12-31T23:59:59Z"

# Filter by source and metric name
curl -H "X-API-Key: $BFF_API_KEY" \
  "http://localhost:8080/api/v1/metrics?startDate=2026-01-01T00:00:00Z&endDate=2026-12-31T23:59:59Z&source=tagesschau&metricName=word_count"

# Discover available metric names
curl -H "X-API-Key: $BFF_API_KEY" \
  "http://localhost:8080/api/v1/metrics/available"

# Query named entities (aggregated by text and label)
curl -H "X-API-Key: $BFF_API_KEY" \
  "http://localhost:8080/api/v1/entities?startDate=2026-01-01T00:00:00Z&endDate=2026-12-31T23:59:59Z"

# Filter entities by source and NER label (PER, ORG, LOC, MISC)
curl -H "X-API-Key: $BFF_API_KEY" \
  "http://localhost:8080/api/v1/entities?startDate=2026-01-01T00:00:00Z&endDate=2026-12-31T23:59:59Z&source=bundesregierung&label=ORG&limit=50"
```

**OpenAPI spec:** `services/bff-api/api/openapi.yaml`. Regenerate server stubs: `make codegen`.

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
| `SentimentExtractor` | `sentiment_score`, `lexicon_version` | German lexicon polarity via SentiWS |
| `NamedEntityExtractor` | `entity_count` | spaCy `de_core_news_lg` NER; raw spans stored in `aer_gold.entities` |

Extractors are independent — one failing extractor does not crash others or route the document to the DLQ. Extractor source code is in `services/analysis-worker/internal/extractors/`.

**Source adapters** select the harmonization strategy based on `source_type` in the Bronze document:

| Source Type | Adapter | Silver Output |
| :--- | :--- | :--- |
| `rss` | `RSSAdapter` | `SilverCore` + `RSSMeta` (feed_url, categories, author, feed_title) |
| *(missing/unknown)* | `LegacyAdapter` | Backward-compatible mapping for pre-Phase 39 documents |

```bash
# Check worker logs
make logs   # Combined logs of all services

# Or directly
tail -f .pids/worker.log
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

**Deduplication:** The crawler maintains a local state file (`.rss-crawler-state.json`) to prevent re-ingestion of previously submitted items across runs.

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

**E2E smoke test** (`scripts/e2e_smoke_test.sh`): Starts a fixture HTTP server → runs the RSS crawler against test fixtures → waits for pipeline processing → queries BFF API endpoints (metrics, entities, available metrics) → validates end-to-end data flow → teardown.

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