# Operations Playbook

> **Purpose:** Practical reference for accessing, inspecting, and debugging every component in the AĒR infrastructure stack.
> For *architectural rationale*, see the [arc42 documentation](../arc42/01_introduction_and_goals.md). For *scientific workflows* (when and why to populate tables), see the [Scientific Operations Guide](scientific_operations_guide.md). This document covers *how to operate*.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Stack Lifecycle](#stack-lifecycle)
3. [PostgreSQL (Metadata Index)](#postgresql-metadata-index)
    - [Source Classifications (WP-001)](#source-classifications-wp-001) — *touchpoint*
    - [Retention & Reconciliation (Phase 113b / ADR-022)](#retention-reconciliation-phase-113b-adr-022)
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
11. [Dashboard / Frontend Iteration](#dashboard-frontend-iteration)
12. [Testing](#testing)
13. [Dependency Refresh (Supply-Chain Baseline)](#dependency-refresh-supply-chain-baseline)
14. [Volume Management](#volume-management)
15. [Network Architecture](#network-architecture)
16. [Scientific Touchpoints Index](#scientific-touchpoints-index)
17. [Quick Reference Card](#quick-reference-card)

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

### Required Boot Secrets (Phase 75)

Since Phase 75, the Go services validate a small set of required secrets at start-up and refuse to boot if any of them is empty. This is a defence-in-depth guardrail against accidentally shipping a service with a default or unset credential.

| Variable | Consumed by | Where to set |
|----------|-------------|--------------|
| `BFF_API_KEY` | `bff-api` (`X-API-Key` header auth) | `.env` for local/compose, GitHub Actions secret for CI, secret manager for prod |
| `CLICKHOUSE_PASSWORD` | `bff-api` (ClickHouse client) | same as above |
| `INGESTION_API_KEY` | `ingestion-api` (`X-API-Key` header auth) | same as above |
| `DB_URL` | `ingestion-api` (Postgres client; embeds `POSTGRES_PASSWORD`) | same as above |
| `POSTGRES_PASSWORD` | Postgres init + `DB_URL` | same as above |

**Generate API keys** with `openssl rand -base64 32`.

**CI (GitHub Actions):** add the four secrets above in `Settings → Secrets and variables → Actions` and surface them as env vars in any job that boots the services (integration and e2e jobs). Unit tests that never call `config.Load()` are unaffected.

---

## Stack Lifecycle

```bash
make up                  # Start full stack (infra + services + dashboard + debug ports)
make down                # Stop everything
make restart             # Full restart

make infra-up            # Infrastructure only (Traefik, databases, NATS, observability)
make services-up         # Application services only (requires infra running)
make backend-up          # Stack minus the dashboard container (frontend dev loop)

make debug-up            # Expose backend ports to localhost
make debug-down          # Close debug ports (services keep running)

make logs                # Tail container logs (Ctrl+C safe)

make test-e2e            # Run Docker Compose end-to-end smoke test
```

> **Everything runs in containers.** `make up` brings up the full stack via
> Docker Compose (infra + ingestion-api + analysis-worker + bff-api + dashboard +
> debug port forwarder), gated by service healthchecks. For frontend
> iteration, `make backend-up` skips the dashboard container and `make fe-dev`
> serves the SvelteKit dev server on `:5173`, proxying `/api` through Traefik
> on `https://localhost`. The browser never sees `BFF_API_KEY`: Traefik
> attaches it as `X-API-Key` to every `/api/*` request server-side (see the
> `bff-api-key` middleware in `compose.yaml`). Non-browser callers (crawlers,
> `scripts/e2e_smoke_test.sh`) keep sending `X-API-Key` directly to the BFF
> on `:8080`.

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

### Retention & Reconciliation (Phase 113b / ADR-022)

AĒR runs two retention horizons by design. PostgreSQL holds *operational* metadata pruned at the operational horizon; MinIO Silver and ClickHouse Gold hold *analytical* truth pruned at the analytical horizon.

| Layer / Table                         | Retention | Pruner                                                                                       |
|---------------------------------------|-----------|----------------------------------------------------------------------------------------------|
| MinIO `bronze`                        | 90 days   | MinIO ILM (`infra/minio/setup.sh`)                                                           |
| MinIO `bronze-quarantine` (DLQ)       | 30 days   | MinIO ILM                                                                                    |
| Postgres `documents`, `ingestion_jobs` | 90 days   | `ingestion-api` background goroutine (`startRetentionCleanup`) — see Migration 004           |
| MinIO `silver`                        | 365 days  | MinIO ILM                                                                                    |
| ClickHouse `aer_silver.documents`     | 365 days  | ClickHouse TTL on `timestamp`                                                                |
| ClickHouse `aer_gold.metrics`         | 365 days  | ClickHouse TTL on `timestamp`                                                                |
| ClickHouse `aer_gold.entities`        | 365 days  | ClickHouse TTL on `timestamp`                                                                |
| ClickHouse `aer_gold.language_detections` | 365 days | ClickHouse TTL on `timestamp`                                                              |
| ClickHouse `aer_gold.entity_cooccurrences` | 365 days | ClickHouse TTL on `window_start`                                                          |

ADR-022 makes the BFF read article resolution and per-source dossier counts from the analytical layer (`aer_silver.documents`), so the 90/365 split is structurally fine: the dossier and L5 Evidence stay coherent even when the Postgres row for an article has been retention-deleted. Postgres `documents` becomes an *operational soft cache* — load-bearing during ingestion (idempotency keys, lifecycle status), not load-bearing for read-path resolution.

**When to run reconciliation.** ADR-022 makes recurring reconciliation unnecessary. Run `scripts/reconcile_documents.py` only in these one-shot scenarios:

* Closing the historical drift that pre-dates ADR-022 — Postgres rows already pruned before this ADR landed.
* Recovering after an operational outage that lost Postgres rows while Silver/Gold remained intact (e.g. an accidental volume wipe of Postgres alone).

**Procedure:**

```bash
# 1. Inspect the divergence first.
psql -h localhost -p 5432 -U $POSTGRES_USER -d $POSTGRES_DB -c "
  SELECT s.name,
         COALESCE(c.cnt, 0) AS pg_documents,
         (SELECT count(DISTINCT article_id)
            FROM aer_silver.documents
           WHERE source = s.name) AS ch_silver_docs
    FROM sources s
    LEFT JOIN (
      SELECT j.source_id, count(*) AS cnt
        FROM documents d JOIN ingestion_jobs j ON j.id = d.job_id
        WHERE d.status = 'processed'
       GROUP BY j.source_id
    ) c ON c.source_id = s.id
   ORDER BY s.name;
"
# (the ch_silver_docs subquery only works via Postgres if you have a CH FDW;
#  otherwise run the count separately against ClickHouse and compare by eye)

# 2. Dry-run the reconciliation to see what would change.
python scripts/reconcile_documents.py --dry-run

# 3. Execute. Idempotent — safe to interrupt and re-run.
python scripts/reconcile_documents.py

# 4. Confirm divergence has closed.
#    Repeat step 1 — pg_documents should now meet or exceed ch_silver_docs
#    for every source whose Silver envelopes are still present in MinIO.
```

The script never deletes; it only inserts missing rows under `ON CONFLICT (bronze_object_key) DO NOTHING`. Synthetic ingestion-jobs created during reconciliation are tagged `status = 'reconciled'` and pinned to 00:00 UTC of the document's day, so they never collide with real ingestion jobs.

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

#### Granting metric equivalence (WP-004 §5.2) — Phase 115

Metric equivalence is granted **out of band** by an interdisciplinary review.
There is no in-band UI for it; the workflow is intentionally manual so a
methodological review record exists for every grant. The procedure
mirrors the WP-006 §5.2 Silver-eligibility review pattern (Phase 103).

1. **Postgres `equivalence_reviews` insert** — record the full review
   prose, reviewer, date, and working-paper anchor. The `notes_summary`
   field is the concise (≤ 280-char) rationale that will mirror to the
   ClickHouse row in step 2.

   ```sql
   INSERT INTO equivalence_reviews
       (etic_construct, metric_name, language, source_type,
        equivalence_level, reviewer, review_date,
        rationale, working_paper_anchor, notes_summary, confidence)
   VALUES
       ('evaluative_polarity', 'sentiment_score_sentiws',
        'de', 'rss', 'deviation',
        'Dr. <name>, Univ. <institution>',
        DATE '2026-05-01',
        '<full prose rationale, may be multi-paragraph>',
        'WP-004#section-5.2',
        '<≤280-char concise summary referenced by the dashboard methodology tray>',
        0.85);
   ```

2. **ClickHouse `metric_equivalence` insert** — write the structured
   row that the BFF read path consults. The `notes` value is the same
   `notes_summary` recorded in step 1, copied here so the BFF serves
   the rationale without a cross-database join. `validated_by` should
   point back to the Postgres row (e.g. its `id` or a stable
   reviewer attribution).

   ```sql
   INSERT INTO aer_gold.metric_equivalence
       (etic_construct, metric_name, language, source_type,
        equivalence_level, validated_by, validation_date, confidence, notes)
   VALUES
       ('evaluative_polarity', 'sentiment_score_sentiws',
        'de', 'rss', 'deviation',
        'equivalence_reviews#42',
        now(), 0.85,
        '<≤280-char concise summary, identical to notes_summary in step 1>');
   ```

3. **WP-004 Appendix B** — append the full rationale to the
   methodological appendix; the `working_paper_anchor` URL fragment
   should resolve there.

**Notes column convention.** The ClickHouse `notes` column carries a
≤ 280-char concise summary intended for read-path display in the
dashboard methodology tray. It is not authoritative; the Postgres
`equivalence_reviews` row holds the full review prose. An empty
string is valid (e.g. the temporal-Level grant for Probe 0 × Probe 1
in Phase 123, whose rationale is fully captured in WP-004 Appendix B
and needs no additional paraphrase).

#### Automated baseline maintenance (Phase 115)

Periodic baseline computation runs **inside the analysis worker**
(`MetricBaselineExtractor`) on the same NATS-cron pattern as the
co-occurrence sweep:

| Variable                                      | Default | Effect                                              |
| --------------------------------------------- | ------- | --------------------------------------------------- |
| `BASELINE_EXTRACTION_ENABLED`                 | `true`  | Toggle the loop; `false` reverts to manual-only.    |
| `BASELINE_EXTRACTION_INTERVAL_SECONDS`        | `86400` | One sweep per day.                                  |
| `BASELINE_EXTRACTION_WINDOW_SECONDS`          | `7776000` | Rolling 90-day window.                            |
| `BASELINE_EXTRACTION_INITIAL_DELAY_SECONDS`   | `300`   | Grace period after worker startup before the first sweep. |

The standalone `scripts/compute_baselines.py` is **retained** for ad-hoc
operations (first-run on a new probe, manual recompute after a schema
change, Operations-Playbook walkthroughs). Both call paths share the
canonical computation in
`internal.extractors.metric_baseline.compute_baseline_rows`, so the auto
extractor and the script produce byte-identical baselines for the same
input window. Cross-link: [Scientific Operations Guide → Workflow 4:
Computing and Updating Baselines](scientific_operations_guide.md#workflow-4-computing-and-updating-baselines).

---

## MinIO (Data Lake — Object Storage)

S3-compatible storage for the Medallion Architecture layers (Bronze, Silver, DLQ).

**Connection (debug profile required):**

| Property      | Value                                |
| :------------ | :----------------------------------- |
| API           | `http://localhost:9000`              |
| Console (UI)  | `http://localhost:9001`              |
| Console (TLS) | `https://$MINIO_CONSOLE_HOST` (via Traefik) |
| Root user     | `$MINIO_ROOT_USER` / `$MINIO_ROOT_PASSWORD` (admin only) |
| Ingestion API | `$INGESTION_MINIO_ACCESS_KEY` / `$INGESTION_MINIO_SECRET_KEY` (`aer_ingestion_policy`, Phase 79) |
| Analysis Worker | `$WORKER_MINIO_ACCESS_KEY` / `$WORKER_MINIO_SECRET_KEY` (`aer_worker_policy`, Phase 79) |
| Internal host | `minio:9000` (from containers)      |

**Service accounts (Phase 79).** Each application service authenticates with its own MinIO user — the root credentials are reserved for `minio-init` and `setup.sh`. `aer_ingestion` has write access to the Bronze bucket; `aer_worker` has read access to Bronze and write access to Silver and the DLQ. Policies are declared and attached in `infra/minio/setup.sh`.

**Bronze bucket ENV overrides (Phase 77).** Both services read the bucket name from environment variables — `INGESTION_BRONZE_BUCKET` for the Ingestion API and `WORKER_BRONZE_BUCKET` for the Analysis Worker. Default is `bronze`; both must match. Defined in `.env.example` and wired in `compose.yaml`.

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
| `SentimentExtractor` | `sentiment_score_sentiws` | German lexicon polarity via SentiWS, with Phase 117 dependency-based negation scope, `compound-split` head/tail decomposition, and `data/custom_lexicon.yaml` merge hook |
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

#### Custom lexicon extension (Phase 117)

The `SentimentExtractor` merges `services/analysis-worker/data/custom_lexicon.yaml` with the versioned SentiWS file at startup. This is the designated out-of-band mechanism for adding neologisms (e.g. `toxisch`, `Querdenker`, `Wutbürger`) without patching the SentiWS data files.

File format:

```yaml
# data/custom_lexicon.yaml — entries are merged with SentiWS at startup.
# Surface forms are case-insensitive; polarity is a float in [-1.0, 1.0].
toxisch: -0.6
Querdenker: -0.4
Wutbürger: -0.5
```

Workflow to add an entry:

1. Edit `services/analysis-worker/data/custom_lexicon.yaml` and commit.
2. Rebuild the worker image: `make build-services` (or rebuild via Compose).
3. Restart the worker: `make worker-restart`.
4. Verify: the worker logs `SentiWS lexicon loaded` with `custom_entries=N` and a fresh `lexicon_hash` — that hash also feeds the extractor's `version_hash` (Phase 46), so any rows produced after the restart can be distinguished from prior rows by provenance.
5. Optional: re-process historical Bronze documents to apply the new entries to past data — see the reprocessing section above.

The custom file is checked into git so the lexicon evolution is auditable. Do not hand-edit the SentiWS files in `data/sentiws/` for neologisms — those are the versioned baseline.

### RSS Crawler

The RSS crawler is a standalone Go binary under `crawlers/rss-crawler/` with its own `go.mod` (no `pkg/` dependency). It fetches German institutional RSS feeds and submits documents to the Ingestion API. **Module ownership** is independent of the service modules — but **at runtime** it executes as a one-shot container on the internal `aer-backend` Docker network under Compose profile `crawlers`, reaching `ingestion-api:8081` directly. No host port exposure is required; dev and prod invocations are identical.

```bash
# Standard path: builds the image if needed, runs once, removes the container.
make crawl

# Equivalent raw form:
docker compose --profile crawlers run --rm --build rss-crawler

# Wipe dedup state (re-ingests every feed item on the next run):
make crawl-reset
```

`make crawl` reads `INGESTION_API_KEY` from `.env` via Compose interpolation — if `.env` is missing, the target exits with an error before invoking Compose. Profile-gated, so `make up` never starts the crawler automatically.

**Production scheduling.** The same command runs under any host-side scheduler — cron, systemd timer, Kubernetes `CronJob` wrapping `docker compose run --rm rss-crawler`, or a long-running scheduler sidecar (e.g. Ofelia) on the Compose project. No scheduler is bundled today; pick one per deployment target.

**Feed configuration** (`crawlers/rss-crawler/feeds.yaml`, baked into the image at build time):

```yaml
feeds:
  - name: bundesregierung   # Must match a source registered in PostgreSQL
    url: https://www.bundesregierung.de/breg-de/feed
  - name: tagesschau
    url: https://www.tagesschau.de/index~rss2.xml
```

Changing `feeds.yaml` requires rebuilding the image (`make crawl` passes `--build` so the next invocation picks it up automatically; CI pipelines should rebuild explicitly).

Adding a new RSS feed requires:
1. A new entry in `feeds.yaml`.
2. A PostgreSQL seed migration in `infra/postgres/migrations/` registering the source name.
3. A Probe Dossier and Probe Classification workflow (see [Scientific Operations Guide → Workflow 1](scientific_operations_guide.md#workflow-1-classifying-a-new-probe)).

**Deduplication.** The crawler maintains a JSON state file at `/state/rss-crawler-state.json` inside the container, persisted in the named volume `rss_crawler_state`. State is written immediately after each feed's batch is successfully submitted — not once at the end — so a crash or interruption mid-run does not cause re-ingestion of already-processed feeds. `make infra-clean`, `make infra-clean-postgres`, and `make infra-clean-minio` wipe this volume automatically, because they invalidate bronze or the `documents` / `sources` tables that dedup state is logically tied to. `make infra-clean-clickhouse` does *not* touch it (gold is downstream, bronze stays authoritative). If you wipe backend volumes by any other means — raw `docker compose down -v`, `docker volume rm`, manual prune — run `make crawl-reset` afterwards so the crawler doesn't skip every item as "already seen" against state that no longer matches what's in bronze.

**Environment variables** (injected by the `rss-crawler` Compose service):

| Variable | Purpose | Default |
| :--- | :--- | :--- |
| `INGESTION_URL` | Ingest endpoint | `http://ingestion-api:8081/api/v1/ingest` |
| `SOURCES_URL` | Sources lookup endpoint | `http://ingestion-api:8081/api/v1/sources` |
| `INGESTION_API_KEY` | `X-API-Key` header value | from `.env` |
| `STATE_FILE` | Path inside `rss_crawler_state` volume | `/state/rss-crawler-state.json` (set via CMD) |

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

## API Contract Reference (Swagger UI)

Introduced in Phase 96. The BFF and Ingestion APIs each own a modular OpenAPI 3.0 spec under `services/<service>/api/`. A dev-only `swagger-ui` container serves a browsable HTML reference backed by bundled single-file artifacts.

| Property       | Value                                |
| :------------- | :----------------------------------- |
| URL            | `http://localhost:8089`              |
| Compose profile | `dev` (absent from default `make up`) |
| Bind address   | `127.0.0.1` (loopback only; never through Traefik) |

### Start, stop, refresh

```bash
# Start: bundle specs and launch the container in one step.
make swagger-up          # bundles both specs, then: docker compose --profile dev up -d swagger-ui

# Browse. Use the spec dropdown (top-right) to switch between
# "BFF API" and "Ingestion API".
open http://localhost:8089

# Stop when done.
make swagger-down
```

### Editing workflow

1. Edit the modular source under `services/<service>/api/` (`openapi.yaml`, `paths/*.yaml`, `schemas/*.yaml`, `parameters/*.yaml`, `responses/*.yaml`).
2. `make openapi-lint` — enforces the two-style `$ref` convention (see §8.19 / ADR-021).
3. `make codegen` — regenerates Go types from the modular source; CI fails on drift.
4. `make openapi-bundle` — rebuilds `services/<service>/api/openapi.bundle.yaml`.
5. Refresh the Swagger UI browser tab — bundles are mounted read-only, no container restart needed.

### Troubleshooting

| Symptom | Cause | Fix |
| :--- | :--- | :--- |
| Swagger UI shows "Failed to load API definition" | Bundle is stale or missing | `make openapi-bundle` |
| `make openapi-lint` fails with `in-document $ref found in external file` | A `$ref: '#/...'` was added inside `schemas/`, `parameters/`, or `responses/` | Rewrite to external-file form (`../schemas/X.yaml`); see §8.19 |
| `make codegen` produces a diff on `generated.go` after a spec change | Expected if the spec's external surface changed | Inspect the diff; if it collapses a named type to `interface{}` or an inline struct, a path-level `#/components/...` ref was switched to an external-file ref — revert |
| `make openapi-bundle` crashes with `FileNotFoundError` | An external-file ref points at a missing file | Check the ref target; file paths are relative to the referring file |

See Arc42 §8.19 for the full convention and ADR-021 for the underlying decision.

---

## Dashboard / Frontend Iteration

The dashboard (`services/dashboard/`, SvelteKit + static adapter) has two iteration loops and one container build path. All targets live under the `fe-*` / `frontend-*` Makefile prefixes; `pnpm` is the package manager, pinned via `.tool-versions`.

### The two loops

**Loop A — full container** (default, used by `make up` and CI):

```bash
make up                  # brings up dashboard alongside backend, routed via Traefik
open https://localhost/  # browser hits the static build served by the dashboard container
```

The dashboard service is gated behind the `dashboard` Compose profile and is included in `make up`. Use this loop to verify the production build behaves the same as what CI ships.

**Loop B — Vite dev server with containerized backend** (fast frontend iteration):

```bash
make backend-up          # full stack minus the dashboard container
make fe-dev              # SvelteKit dev server on http://localhost:5173
```

`fe-dev` refuses to start unless `bff-api` is running. Vite proxies `/api/*` to Traefik on `https://localhost`, where the `bff-api-key` middleware injects `X-API-Key` server-side. **The browser bundle ships zero secrets** — `BFF_API_KEY` lives only in `.env` and the Traefik label.

### Codegen chain (OpenAPI → typed client)

The frontend's HTTP client is generated from the same BFF spec the Go server uses. Run after every `services/bff-api/api/**` change:

```bash
make openapi-bundle      # bundles modular spec → services/bff-api/api/openapi.bundle.yaml
make fe-codegen          # openapi-typescript → services/dashboard/src/lib/api/types.ts
                         # (alias: make codegen-ts)
```

`fe-codegen` depends on `openapi-bundle`, so a single `make fe-codegen` is usually enough. Commit the regenerated `types.ts` alongside the spec change. CI fails on drift the same way `make codegen` does for Go.

### Quality gates

```bash
make fe-format           # Prettier (writes)
make fe-lint             # ESLint + Prettier check + svelte-check
make fe-typecheck        # svelte-check strict TypeScript
make fe-test             # Vitest unit tests
make fe-test-e2e         # Playwright (visual + a11y) inside pinned image from compose.yaml
make fe-test-e2e-update  # Refresh visual baselines (commit the diff)
make fe-build            # Static build → services/dashboard/build/
make fe-bundle-size      # 80 kB gzipped initial-bundle budget (after fe-build)
make fe-check            # Composite: lint + typecheck + test + build + bundle-size
```

`fe-test-e2e` and `fe-test-e2e-update` deliberately run inside the Playwright image pinned in `compose.yaml` — browser font rendering is OS-sensitive, so host-local snapshot runs are not byte-comparable to CI. Always update baselines via `fe-test-e2e-update`, never with a host-local Playwright.

### Container build

```bash
make fe-image-build      # docker compose build dashboard + size gate
make fe-image-size       # check aer-dashboard:local against the 50 MB budget
make frontend-up         # bring up just the dashboard container (dashboard profile)
make frontend-down
make frontend-restart
```

The dashboard image budget (50 MB) is enforced post-build because Docker itself cannot gate on size. `frontend-up` is the quickest way to swap from Loop B back to Loop A without restarting the rest of the stack.

### Auth chain (where the API key flows)

```
Browser ── /api/* ──► Vite (5173) ──► Traefik (https://localhost)
                                          │  bff-api-key middleware
                                          │  injects X-API-Key: $BFF_API_KEY
                                          ▼
                                       bff-api
```

For container Loop A, drop the Vite hop — the browser hits Traefik directly via the `dashboard` router. Non-browser callers (RSS crawler, `scripts/e2e_smoke_test.sh`) keep sending `X-API-Key` themselves, directly to BFF on `:8080`. See `compose.yaml` (`bff-api-key` middleware label) and ADR-018.

### Authoring flow (typical change)

1. Edit `services/dashboard/src/...`.
2. If the change touches the API: edit `services/bff-api/api/...` first, then `make codegen` (Go) and `make fe-codegen` (TS).
3. `make fe-lint && make fe-typecheck` while iterating.
4. `make fe-test` for unit changes; `make fe-test-e2e` if visuals or interaction surfaces moved.
5. Before pushing: `make fe-check` (full gate), then `make fe-image-build` if you touched `Dockerfile`, dependencies, or anything that could affect image size.

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

## Dependency Refresh (Supply-Chain Baseline)

Phase 84 pinned the stack's external dependencies to specific hashes: every `FROM` line in `services/*/Dockerfile` uses `image:tag@sha256:digest`, `services/analysis-worker/requirements.lock.txt` is generated with `pip-compile --generate-hashes --require-hashes`, and the SentiWS lexicon download is guarded by `SENTIWS_SHA256`. This is how we keep the supply chain reproducible, but it means those hashes are frozen in time and must be rotated deliberately. Phase 88 turns that rotation into a single command.

### When to run it

| Trigger | Priority |
|---------|---------|
| Trivy flags HIGH/CRITICAL on a pinned base image (CI red) | Same day |
| `govulncheck` or `pip-audit` flags a vulnerable transitive dep | Same day |
| Monthly maintenance cadence, regardless of signals | Monthly |
| Before cutting a release tag | Always |
| After editing `services/analysis-worker/requirements.txt` | Always (lockfile drift) |
| After bumping a `FROM image:tag` in any Dockerfile | Always (digest drift) |

Security signals from CI should take precedence over the monthly cadence — if Trivy is red on a Monday, do not wait for month-end.

### Happy path

```bash
make deps-refresh
```

The target delegates to `scripts/deps_refresh.sh`, which runs four steps, each of which fails loudly and leaves the working tree in an inspectable state:

1. **Base image digest refresh.** Every `FROM image:tag@sha256:…` line across all three service Dockerfiles is deduplicated by image reference. Each unique `image:tag` is `docker pull`ed once, the new digest is resolved via `docker image inspect`, and the old digest is rewritten in place. Shared base images (alpine, python, golang) stay in lockstep across Dockerfiles by construction.
2. **`requirements.lock.txt` regeneration.** `pip-compile --generate-hashes --allow-unsafe` runs inside the *exact* Python image the worker builds from (read back out of the freshly-updated worker Dockerfile), guaranteeing the hash set is byte-compatible with `pip install --require-hashes` at build time. `pip-tools` is pinned via `PIP_TOOLS_VERSION` in `.tool-versions`.
3. **SentiWS hash recomputation.** The script greps the SentiWS URL out of `services/analysis-worker/Dockerfile`, downloads it with curl, runs `sha256sum`, and rewrites `ARG SENTIWS_SHA256=…` in place. Changing the lexicon URL is therefore a one-line Dockerfile edit — rerun `make deps-refresh` and the hash follows.
4. **No-cache rebuild + e2e smoke test.** `docker compose build --no-cache` forces every image to reproduce from the new baseline, then `make test-e2e` asserts the full ingestion → worker → BFF pipeline still answers correctly. A failure here means *revert, investigate, do not commit*.

On a clean baseline (nothing upstream moved since the last run), the script produces zero file changes and `git diff` is empty. That property is load-bearing — it's how CI can one day run `make deps-refresh --skip-build` on a scheduled job and only open a PR when the diff is non-empty.

After a successful run, review `git diff` (especially to confirm only digests/hashes changed, not tags) and commit the result as a single `chore(supply-chain): refresh dependency baseline` commit.

### Flags

```bash
make deps-refresh ARGS="--dry-run"     # report intent, no writes, no rebuild
make deps-refresh ARGS="--skip-e2e"    # rebuild but skip the 90s+ e2e suite
make deps-refresh ARGS="--skip-build"  # just rewrite files; no rebuild, no e2e
./scripts/deps_refresh.sh --help       # full help from the script directly
```

Use `--dry-run` the first time you run the target on an unfamiliar machine, or when you want to see the upstream digest drift without committing to a rebuild cycle. Use `--skip-e2e` only when CI is the smoke test (e.g. refreshing inside a branch that will open a PR immediately).

### Adding a new Python dependency

1. Add the dependency to `services/analysis-worker/requirements.txt` with a pinned version.
2. Run `make deps-refresh`. Step 2 will regenerate `requirements.lock.txt` with hashes for the new dep and its transitives. Steps 1, 3, and 4 still run — the lockfile is the only expected non-trivial diff.
3. Commit both `requirements.txt` and `requirements.lock.txt`.

Do not edit `requirements.lock.txt` by hand; the build will reject it at `pip install --require-hashes` time.

### Bumping a base image tag

1. Edit the `FROM image:NEW_TAG@sha256:…` line in the relevant Dockerfile. Leave the digest alone — it will be overwritten on the next step.
2. Run `make deps-refresh`. Step 1 will pull the new tag, resolve its digest, and rewrite it. If the tag is shared across Dockerfiles, update *all* copies to the same new tag before running so the dedupe pass covers them in one pull.
3. Step 2 will also rerun pip-compile against the new Python image (if the Python tag moved) — review the lockfile diff carefully.
4. Commit.

### Changing the SentiWS download URL

1. Edit the URL in the `wget` line of `services/analysis-worker/Dockerfile`. Leave the `SENTIWS_SHA256` value alone.
2. Run `make deps-refresh`. Step 3 will download the new URL, sha256sum it, and rewrite the hash.
3. Commit both the URL change and the hash change together.

### Trivy triage after a refresh

If the post-refresh Trivy run still shows HIGH/CRITICAL, the vulnerable package was not fixed upstream yet. Options, in order of preference:

| Severity | Upstream status | Action |
|----------|----------------|-------|
| HIGH/CRITICAL | Fix available in a newer base image | Bump the `FROM image:tag`, rerun `make deps-refresh` |
| HIGH/CRITICAL | Fix available via explicit `apt-get install -y --only-upgrade <pkg>` | Add it to the existing `RUN apt-get` block in the Dockerfile, rerun refresh |
| HIGH/CRITICAL | No upstream fix, package unreachable from our code | Add a time-boxed entry to `.trivyignore` with a justification comment and a review date; open a tracking issue |
| HIGH/CRITICAL | No upstream fix, package IS reachable | Block release; escalate |
| MEDIUM/LOW | Any | CI does not fail on these; refresh monthly |

`.trivyignore` is the supply-chain equivalent of a `// TODO`. Any entry there must have a date and an owner.

### Failure recovery

The script uses `set -euo pipefail`, so a failure at any step stops immediately with a red log line pointing at the failing step. The partial diff is preserved so you can inspect it:

```bash
git diff                  # see what was rewritten before the failure
git restore .             # throw it away and start over
git stash                 # save it for later inspection
```

`--skip-build` is the fastest way to iterate on a failure in steps 1–3 without paying the rebuild cost on each run.

---

## Building and refreshing the Wikidata alias index

The Phase 118 entity-linking step depends on a SQLite alias index built once per quarter from the Wikidata SPARQL endpoint. The index is shipped to the analysis-worker via the `aer-wikidata-index` Docker image and the `wikidata-index-init` compose service; the worker mounts it read-only at `/data/wikidata/`.

### Prerequisites

* Network: a stable internet connection to `query.wikidata.org`. The build performs paginated SPARQL over several hours; transient 429 / 5xx responses are retried with exponential backoff.
* Disk: 1 GiB free on the runner is plenty — the staging DB peaks below 200 MB and the canonicalised output is 50–150 MB.
* Runtime: 2–6 h for the initial three-language scope (`de,en,fr`). Re-runs are full rebuilds; the SPARQL endpoint does not support incremental.
* Authority: write access to `ghcr.io/frogfromlake/aer-wikidata-index` (the workflow's `GITHUB_TOKEN` already has `packages:write`).

### Snapshot-date convention

The build is parameterised by a snapshot date that filters Wikidata entities by `schema:dateModified <= <snapshot>T00:00:00Z`. Two builds with identical (snapshot date, languages, bucket YAML, script version) produce byte-identical SQLite files. The default is "yesterday at 00:00 UTC"; production rebuilds pin a specific date so the resulting hash is reproducible.

### Triggering a rebuild

There are two paths:

```text
# Path A — manual (workflow_dispatch). Used for: end-to-end testing during
# Phase-118 development, urgent refreshes outside the schedule, and any
# rebuild that adds new languages.
gh workflow run wikidata_index_rebuild.yml \
  -f snapshot_date=2026-04-30 \
  -f languages=de,en,fr

# Path B — scheduled (cron). Activated in a separate post-merge commit
# after Phase 118 ships and a manual run has been verified end-to-end.
# Cron: 1st of Jan/Apr/Jul/Oct at 02:00 UTC.
```

The workflow runs the build script, uploads the resulting `wikidata_aliases.db` as a 90-day-retention artifact, and pushes the `aer-wikidata-index` image tagged with both the snapshot date and `latest`.

### Hash verification

Each build emits its sha256 to:

1. The GitHub Actions step summary (visible in the run UI).
2. The image label `org.aer.wikidata.sha256`.
3. The `wikidata_aliases.db.sha256` sidecar baked into the image (standard `sha256sum -c` format).
4. The artifact filename suffix.

To enforce that the analysis-worker uses an exact build, set `WIKIDATA_INDEX_SHA256` in `.env` to the expected hex digest. On startup, the worker computes the sha256 of the mounted file and refuses to boot on mismatch — this is the silent-drift guard. Leave the variable empty to accept whatever the volume contains (recommended only for development).

### Deploying a new index

```bash
# Pin the new tag in .env (Image-Pinning-Policy):
WIKIDATA_INDEX_TAG=2026-04-30
WIKIDATA_INDEX_SHA256=<sha256 from step summary>

# Re-pull and re-init:
docker compose pull wikidata-index-init
docker compose up -d wikidata-index-init analysis-worker
```

The `wikidata-index-init` container copies the new DB into the `wikidata_data` volume and verifies the sidecar via `sha256sum -c`. The analysis-worker depends on it via `condition: service_completed_successfully`, so the worker only starts once the volume is populated. Rolling back is one tag flip plus `docker compose up -d wikidata-index-init analysis-worker` — the previous image tag and the previous quarterly artifact are both still recoverable.

### Refresh cadence

* **Quarterly** by default (`schedule:` in the workflow).
* **On Probe expansion** — when a new probe introduces a language not in the current `LANGUAGES` list, run `workflow_dispatch` with the extended set before the probe's first ingestion.
* **On bucket-YAML change** — when `services/analysis-worker/data/wikidata_type_buckets.yaml` is edited (new entity type bucket, refined min-sitelinks threshold), trigger a manual rebuild.

### Extending the type-bucket YAML

The `wikidata_type_buckets.yaml` is the system of record for the index scope. Adding a new domain is a one-PR change:

1. Append a bucket entry with `name`, `description`, `where_clause` (SPARQL fragment), and optional `min_sitelinks` / `min_population`.
2. Trigger a manual rebuild via `workflow_dispatch`.
3. Verify the resulting index size + hash match expectations (the step summary logs both).
4. Deploy via the tag-flip flow above.

Append-only ordering is part of the determinism guarantee — re-ordering existing entries changes the build hash even if the alias set is unchanged. Sort additions to the end.

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
| MinIO Console        | `http://localhost:9001`           | `$MINIO_ROOT_USER` / `$MINIO_ROOT_PASSWORD` (admin); per-service: `$INGESTION_MINIO_*`, `$WORKER_MINIO_*` |
| MinIO API            | `http://localhost:9000`           | same                                           |
| NATS                 | `localhost:4222`                  | none                                           |
| NATS Monitor         | `http://localhost:8222`           | none                                           |
| Grafana              | `http://localhost:3000`           | `$GF_SECURITY_ADMIN_USER` / `$GF_SECURITY_ADMIN_PASSWORD` |
| Ingestion API        | `http://localhost:8081`           | `$INGESTION_API_KEY`                           |
| BFF API              | `http://localhost:8080`           | `$BFF_API_KEY`                                 |
| OTel Collector       | `localhost:4317` (gRPC) / `4318`  | none                                           |
| Arc42 Docs           | `http://localhost:8000`           | none                                           |
| Swagger UI (dev)     | `http://localhost:8089`           | none — `make swagger-up` to start             |