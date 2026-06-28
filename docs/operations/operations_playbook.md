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
8. [Capacity & Growth Guardrails (Phase 150)](#capacity-growth-guardrails-phase-150)
9. [Application Services](#application-services)
    - [Ingestion API (Go)](#ingestion-api-go)
    - [BFF API (Go)](#bff-api-go) — incl. [Metric Provenance Config](#metric-provenance-config) *touchpoint*
    - [Analysis Worker (Python)](#analysis-worker-python) — incl. `BiasContext` *touchpoint*
    - [Web Crawl Operations (Phase 122)](#web-crawl-operations-phase-122)
10. [Configuration & Documentation Files](#configuration-documentation-files)
    - [Cultural Calendar Files](#cultural-calendar-files) — *touchpoint*
    - [Probe Dossier](#probe-dossier) — *touchpoint*
11. [Arc42 Documentation Server](#arc42-documentation-server)
12. [Dashboard / Frontend Iteration](#dashboard-frontend-iteration)
13. [Testing](#testing)
14. [Dependency Refresh (Supply-Chain Baseline)](#dependency-refresh-supply-chain-baseline)
15. [Full system reset (one-shot)](#full-system-reset-one-shot)
16. [Volume Management](#volume-management)
17. [Network Architecture](#network-architecture)
18. [Scientific Touchpoints Index](#scientific-touchpoints-index)
19. [Routine Operations (`make` targets)](#routine-operations-make-targets)
20. [One-shot Operations](#one-shot-operations)
21. [Deleted scripts — for the historical record](#deleted-scripts-for-the-historical-record)
22. [Quick Reference Card](#quick-reference-card)

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

All credentials are defined in `.env` (copy from `.env.example` on first setup) for **local development**. The variable names referenced below correspond to the keys in that file. **In production (Phase 155 / ADR-046) the box `.env` holds no secrets** — they are injected at deploy time into a tmpfs as Docker secrets and read via the `<VAR>_FILE` convention (see deploy_runbook Part F). The names are identical; only the delivery differs.

### Required Boot Secrets (Phase 75)

Since Phase 75, the Go services validate a small set of required secrets at start-up and refuse to boot if any of them is empty. This is a defence-in-depth guardrail against accidentally shipping a service with a default or unset credential.

| Variable | Consumed by | Where to set |
|----------|-------------|--------------|
| `BFF_API_KEY` | `bff-api` — **machine credential** (ADR-040): session-OR-key gate; no longer gateway-injected for browsers | `.env` for local/compose, GitHub Actions secret for CI, secret manager for prod |
| `BFF_AUTH_DB_PASSWORD` | `bff-api` (`bff_auth` write role) + `postgres-init-roles` (Phase 134 / ADR-040) | same as above |
| `CLICKHOUSE_PASSWORD` | `bff-api` (ClickHouse client) | same as above |
| `INGESTION_API_KEY` | `ingestion-api` (`X-API-Key` header auth) | same as above |
| `DB_URL` | `ingestion-api` (Postgres client; embeds `POSTGRES_PASSWORD`) | same as above |
| `POSTGRES_PASSWORD` | Postgres init + `DB_URL` | same as above |

**Generate API keys / passwords** with `openssl rand -base64 32`.

**CI / deployment GitHub configuration** is enumerated in `docs/operations/github_repository_configuration.md` (the authoritative ledger of Actions Secrets and Variables). Unit tests that never call `config.Load()` need no secrets; only the nightly e2e smoke (`e2e_smoke_nightly.yml`) and deployment consume them.

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
> on `https://localhost`.
>
> **Auth posture (Phase 134 / ADR-040).** Traefik **no longer injects an API
> key** for browsers. Browsers authenticate with the opaque `__Host-` session
> cookie; the BFF gate accepts **a valid session OR a valid `X-API-Key`**, so a
> browser with neither is rejected (401) — this is the whole-app gate. The
> `X-API-Key` is demoted to a **machine credential**: non-browser callers
> (crawlers, `scripts/build/e2e_smoke_test.sh`) send it directly to the BFF on
> `:8080` on the backend network, never through the public Traefik router.
> **Do not re-add a gateway-injected key middleware** in `compose.yaml` /
> `compose.prod.yaml` — it would defeat the gate. The Traefik label change and
> the BFF middleware (`internal/auth/middleware.go`) must move in lockstep.
>
> **Bootstrap the first admin** with `make create-admin` (prints an
> accept-invite link for `ADMIN_BOOTSTRAP_EMAIL`); self-registration is closed
> (LICENSE §3.2).

### Transactional Email (Phase 153 / ADR-043)

Invite / accept-invite / forgot-password / reset-password links are delivered
through a **provider-agnostic SMTP submission relay** (`net/smtp` + STARTTLS, no
third-party client). The provider is pure `.env` config; the documented default
is **Brevo** (EU/France, DSGVO + DPA, 300 mails/day free). Any relay that speaks
SMTP submission with STARTTLS works.

**Config group (`.env`, mirror placeholders in `.env.example`):**

| Variable | Notes |
|---|---|
| `SMTP_HOST` | Relay host, e.g. `smtp-relay.brevo.com`. **Empty → LogSender fallback** (links logged at WARN, not emailed). |
| `SMTP_PORT` | Submission port, `587` (STARTTLS). Implicit-TLS `465` is not supported. |
| `SMTP_USERNAME` / `SMTP_PASSWORD` | Relay SMTP login + key. |
| `SMTP_FROM_ADDRESS` | Must be a **verified sender** at the provider, or mail is rejected/spam-filed. |
| `SMTP_FROM_NAME` | Display name; defaults to `AĒR`. |

**All-or-nothing.** Empty `SMTP_HOST` boots fine on the LogSender (dev + the
`make create-admin` break-glass path). A *set* `SMTP_HOST` with any missing
credential in the group is a **boot error** — the internet-facing service never
silently half-sends.

**Provider setup (Brevo).** Create a free account → verify a sender address (or
authenticate the sending domain with SPF + DKIM for deliverability) → create an
SMTP key (Brevo dashboard → SMTP & API → SMTP) → put host/port/login/key/from
in `.env`. Free-tier limit is 300 mails/day; an invite + reset workload is far
below that. The DPA is accepted in the Brevo account settings (DSGVO: the
provider processes invitee email addresses — EU residency required).

**Failure & abuse story.**

* A send failure is **logged at ERROR** (`"deliver invite email"` /
  `"deliver reset email"` / `"forgot-password: deliver reset email"`) — a relay
  outage is visible in logs/traces, never a silent drop. Admin-initiated invite
  and reset responses also carry `delivered:false` and the admin UI then shows
  the one-time link for **manual delivery** (the link is always returned as the
  break-glass channel).
* `forgot-password` always returns 202 regardless of send outcome (no user
  enumeration), so its only failure signal is the ERROR log.
* **Rate-limiting** of `forgot-password` (the live mail-bombing vector once a
  real relay is wired) is a Phase 145 security-review finding — track it there.
* Templates are plain text, **bilingual EN+DE**, with **no tracking pixels**
  (anti-surveillance posture). Revisit the free-tier limits + SPF/DKIM on
  `make deps-refresh`.

### Build Cache Headroom

All AĒR image builds (`make ingestion-restart`, `make worker-restart`, `make bff-restart`, `make crawl-probe0`, etc.) use Docker Desktop's default buildx builder. A dedicated `aer` builder with raised cache retention was tried and retired in 2026-05-12 — the separate cache duplicated R2 traffic, the bind-mount-backed buildkit container did not survive Docker Desktop restarts, and the gain over the default GC policy was marginal at AĒR's actual scale.

**Default builder GC policy.** Verify with `docker buildx inspect default`. Today's shape is four GC rules with **20 GiB Reserved Space per rule** (≈ 80 GiB total reserved cache that BuildKit will not prune even under disk pressure) plus a 48 h newest-cache shield and a 60-day long-tail shield. On a 1 TB host (WSL backing disk for Docker Desktop) this is effectively unbounded for AĒR's image set.

**Symptoms that warrant escalation.** If `make worker-restart` starts re-downloading HuggingFace models or `spacy download de_core_news_lg` between consecutive runs (when the worker source has not changed), the build cache has been pruned. Verify with `docker buildx du` — Reclaimable should be close to or above the model-layer sizes (~ 10 GiB HF cache layer + ~ 7 GiB Python deps layer for the worker).

**Escalation path.**

1. **First, check disk pressure.** `df -h /var/lib/docker` (or wherever the docker root lives — `docker info | grep "Docker Root"` confirms). Docker Desktop on WSL2 stores the daemon under `/var/lib/docker` inside the WSL distro's filesystem, mounted from the host's NTFS via a virtual disk. If the WSL filesystem is > 80 % full, BuildKit will GC aggressively regardless of the Reserved Space rules.
2. **If disk is fine, prune dead artefacts before touching GC config.** `docker system df` shows Reclaimable size per resource type. `docker system prune` (without `-a`) removes only stopped containers, unused networks, and dangling images — safe. `docker system prune -a` also removes images not referenced by any container — only run when you are sure no in-progress work depends on them.
3. **If pruning helped temporarily but the issue recurs, raise Docker Desktop's disk allocation.** Settings → Resources → Disk image size. On WSL2 the disk is sparse-allocated by default so raising the cap costs nothing until used. Reasonable target for AĒR: 200 GB.
4. **If you genuinely need a different GC policy** (rare — only justified by demonstrated repeated pruning AFTER steps 1-3): create a docker-container driver builder with explicit `--oci-worker-gc-keepstorage <MB>`. Document the rationale in the operations playbook before adding it back. Avoid making it the default buildx — wire it onto specific Make targets that actually need it (typically `worker-restart`).

The previous `aer` builder is documented here as a known failure mode: do not reintroduce it without solving (i) the separate-cache duplication problem and (ii) the bind-mount survival problem first.

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

**Recovering after operational divergence.** ADR-022 makes recurring reconciliation unnecessary. Phase 120c retired the standalone `reconcile_documents.py` workaround: the canonical recovery path is now the supervised wipe-and-recrawl in [Full system reset (one-shot)](#full-system-reset-one-shot). If Postgres has lost rows that Silver/Gold still hold, run `make reset` followed by `make crawl` — the init containers re-create Postgres schema and the crawler repopulates `documents` / `ingestion_jobs` cleanly. The Bronze/Silver/Gold layers are wiped along the way; build-time artefacts (Wikidata index, BERT models baked into the worker image) survive.

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

**Computing baselines.** `scripts/operations/compute_baselines.py` runs offline against the ClickHouse instance and writes one row per `(metric_name, source, language)` it finds.

```bash
# Required env (read from .env if present):
#   CLICKHOUSE_HOST, CLICKHOUSE_PORT, CLICKHOUSE_USER, CLICKHOUSE_PASSWORD, CLICKHOUSE_DB
# Common flags:
#   --metric <name>     restrict to one metric (default: all)
#   --window <days>     reference window in days (default: 90)
#   --dry-run           compute and print, do not insert

cd services/analysis-worker
python ../../scripts/operations/compute_baselines.py --metric word_count --window 90 --dry-run
python ../../scripts/operations/compute_baselines.py --metric word_count --window 90
```

**Probe 0 example** (compute the `word_count` baseline for tagesschau.de):

```bash
python scripts/operations/compute_baselines.py --metric word_count --window 90
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

**Single-probe note:** A single probe cannot establish *cross-cultural* equivalence — it requires at least two probes from different cultural contexts. Cross-frame `?normalization=zscore` returns HTTP 400 until an admissible grant exists. Since Phase 124 the first grant is in place (temporal Level-1 for Probe 0 × Probe 1 — see [Cross-probe operations](#cross-probe-operations-phase-124) below), so cross-probe z-score on a *temporal* metric now succeeds while intensive metrics (e.g. sentiment) still refuse by design. This is the validation gate (WP-004 §6.3) functioning as intended.

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
in Phase 124, whose rationale is fully captured in WP-004 Appendix B
and needs no additional paraphrase).

#### Cross-probe operations (Phase 124)

Phase 124 ships the **first non-empty equivalence grant** and the cross-probe
comparison surface. Operationally relevant facts:

- **The grant is seeded by migrations, not by hand.** Unlike the generic manual
  workflow above, the first temporal Level-1 grant is reproducible
  infrastructure so it survives `make reset`:
  - ClickHouse `infra/clickhouse/migrations/000028_seed_temporal_equivalence_grant.sql`
    — four `metric_equivalence` rows (`publication_hour`, `publication_weekday` ×
    `de`, `fr`), `equivalence_level='temporal'`, `etic_construct='temporal_rhythm'`.
  - Postgres `infra/postgres/migrations/000023_seed_temporal_equivalence_grant.up.sql`
    — the four full `equivalence_reviews` records (ids 1–4), anchored to
    WP-004 Appendix B.
  - Baselines themselves are NOT seeded — the worker's `MetricBaselineExtractor`
    maintains them for every source/probe automatically (see *Automated baseline
    maintenance* below), so both probes already have `publication_hour` /
    `publication_weekday` baselines.

- **Metric-class-aware gate.** The z-score / percentile gate accepts a
  `temporal` grant only for temporal-axis metrics (`publication_hour`,
  `publication_weekday` — clock/calendar time is a culture-independent axis, so
  z-score reads as a *rhythm* comparison). Intensive/scaled metrics (sentiment)
  still require a `deviation` (Level-2) grant. The Dossier Level-2 *reporting*
  column stays strict — a temporal grant never reports as deviation-comparable.

  ```bash
  # Cross-probe temporal z-score — succeeds (200):
  curl -H "X-API-Key: $BFF_API_KEY" \
    "$BFF/metrics?metricName=publication_hour&normalization=zscore&sourceIds=tagesschau,franceinfo&start=...&end=..."
  # Cross-probe sentiment z-score — refuses (400, gate=metric_equivalence):
  curl -H "X-API-Key: $BFF_API_KEY" \
    "$BFF/metrics?metricName=sentiment_score_bert_multilingual&normalization=zscore&sourceIds=tagesschau,franceinfo&start=...&end=..."
  ```

- **Cross-probe equivalence matrix.** `GET /probes/{probeId}/equivalence?comparedTo=<otherProbeId>`
  evaluates over the union of both probes' sources, reporting what is valid for
  the pair (the temporal metrics show `equivalenceStatus.level=temporal`).

- **Temporal lead-lag.** `GET /probes/{probeId}/lead-lag?comparedTo=<otherProbeId>&maxLagHours=168`
  returns the lagged cross-correlation of hourly publication activity between the
  two probes. It is gated on the same temporal grant; an ungranted pair returns a
  RefusalPayload-shaped 400 (`gate=metric_equivalence`). `bucketCountAtZero`
  discloses the overlapping-sample size — read peaks cautiously when it is small.

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

The standalone `scripts/operations/compute_baselines.py` is **retained** for ad-hoc
operations (first-run on a new probe, manual recompute after a schema
change, Operations-Playbook walkthroughs). Both call paths share the
canonical computation in
`internal.extractors.metric_baseline.compute_baseline_rows`, so the auto
extractor and the script produce byte-identical baselines for the same
input window. Cross-link: [Scientific Operations Guide → Workflow 4:
Computing and Updating Baselines](scientific_operations_guide.md#workflow-4-computing-and-updating-baselines).

### Multi-Resolution Query Routing (Phase 122c)

`GET /api/v1/metrics?resolution=…` no longer reads exclusively from `aer_gold.metrics`. As of Phase 122c (migration `000019_activate_metrics_resolution_views.sql`), the BFF query layer routes per resolution to the right physical table. The dashboard side is unchanged — `services/dashboard/src/lib/components/L2Controls.svelte:16` already exposed the five-resolution selector via Phase 66.

| `?resolution=` | Physical table | TTL anchor |
| :--- | :--- | :--- |
| `5min` | `aer_gold.metrics` (raw) | 365 d on `timestamp` |
| `hourly` | `aer_gold.metrics_hourly` (MV) | 365 d on `bucket` |
| `daily` | `aer_gold.metrics_daily` (MV) | 1825 d on `bucket` |
| `weekly` | `aer_gold.metrics_daily` (rebucket via `toStartOfWeek`) | inherits 1825 d |
| `monthly` | `aer_gold.metrics_monthly` (MV) | indefinite (no TTL) |

The MVs are `AggregatingMergeTree` with two state columns: `value_avg_state AggregateFunction(avg, Float64)` and `sample_count_state AggregateFunction(count)`. Queries through the BFF combine them at read time via `avgMerge(value_avg_state)` and `countMerge(sample_count_state)`. The routing logic is in `Resolution.queryShape` (`services/bff-api/internal/storage/metrics_query.go`).

**Inspecting MV freshness:**

```sql
-- Per-MV row totals and part counts:
SELECT
    table,
    sum(rows)        AS total_rows,
    count()          AS parts,
    sum(bytes)       AS bytes
FROM system.parts
WHERE database = 'aer_gold'
  AND table LIKE 'metrics_%'
  AND active
GROUP BY table
ORDER BY table;

-- Most recent bucket per MV (sanity check that the MV trigger is firing):
SELECT 'hourly'  AS mv, max(bucket) AS latest_bucket FROM aer_gold.metrics_hourly
UNION ALL SELECT 'daily',   max(bucket) FROM aer_gold.metrics_daily
UNION ALL SELECT 'monthly', max(bucket) FROM aer_gold.metrics_monthly
ORDER BY mv;
```

**Forced merge** (rarely needed — MVs catch up automatically as parts merge in the background):

```sql
OPTIMIZE TABLE aer_gold.metrics_hourly FINAL;
OPTIMIZE TABLE aer_gold.metrics_daily FINAL;
OPTIMIZE TABLE aer_gold.metrics_monthly FINAL;
```

**MV semantics caveat.** `aer_gold.metrics` is `ReplacingMergeTree(ingestion_version)`. The MVs trigger on every INSERT (pre-merge), so a re-ingestion of the same row at a higher `ingestion_version` accumulates `countMerge()` inflation in the MVs. `avgMerge()` over identical values is unaffected because adding the same value twice does not shift the mean. Re-ingestion is exceptional in AĒR's worker (used for rare reprocessing, not routine updates); the bounded count inflation is documented at the call site in migration 000019 and in Arc42 §8.8.1. If silent reprocessing patterns ever change, the contract here changes too — revisit the per-MV count semantics first.

The Phase-66 deferred-state record (`infra/clickhouse/migrations/000009_metrics_resolution_views.sql`) is preserved verbatim as the audit-trail of the deferred-design pattern that made activation a single migration. Do not edit 000009 in place — it documents what was deliberately not done at Phase 66.

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

#### The curated dashboard — "AER Pipeline Overview"

One provisioned dashboard (`aer-overview`, uid `aer-overview`) carries the key signals only — no hand-built, unversioned panels. Open Grafana → Dashboards → **AER Pipeline Overview**. It is organised into four rows:

| Row | Panel | Reads | What it tells you |
| :-- | :---- | :---- | :---------------- |
| **HTTP Services** | Request Rate by Service | `http_server_requests_total` | Per-service (ingestion-api, bff-api) requests/sec — load + traffic shape. |
| | Request Latency p95 | `http_server_request_duration_seconds_bucket` | p95 server-side handling time per service — slowness detector. |
| | Error Rate (5xx ratio) | `http_server_requests_total{status=~"5.."}` | Fraction of requests failing per service — the first thing to watch on deploy. |
| **Pipeline** | Events Processed / Quarantined (rate) | `events_processed_total`, `events_quarantined_total` | Worker throughput and quarantine (DLQ) rate. |
| | DLQ Size | `dlq_size` | Objects sitting in `bronze-quarantine` (alert ≥ 50). |
| | Event Processing Duration (p50/p95/p99) | `event_processing_duration_seconds_bucket` | End-to-end per-event NLP cost. |
| | **NATS Consumer Lag** | `nats_consumer_pending`, `nats_consumer_ack_pending` | `pending` = stream messages not yet delivered (backlog); `ack_pending` = delivered-but-in-flight. Sustained rising `pending` = the worker pool is falling behind ingestion. |
| **Infrastructure Health** | Scrape Targets Up | `up` | Per-job UP/DOWN — the definitive "is it reachable" signal for every service incl. MinIO + ClickHouse. |
| | ClickHouse Query Rate | `ClickHouseProfileEvents_Query` | Analytical-DB query throughput. |
| | MinIO Cluster Usage | `minio_cluster_usage_total_bytes` | Object-store bytes used. |
| **Distributed Traces** | Recent Traces / Trace Service Map | Tempo | Latest traces + the linked-span service topology (see correlation below). |

> Metric provenance: per-service HTTP metrics come from a shared middleware (`pkg/middleware/observability.go`, scraped from each Go service's `/metrics`); worker metrics from `internal/metrics.py` (`:8001`); MinIO/ClickHouse from their native Prometheus endpoints (no exporter container). The `route` label is always the chi route *pattern* (e.g. `/api/v1/metrics/{metricName}/distribution`), never the concrete path, to bound cardinality.

#### Finding a trace by its trace-id (logs ↔ traces correlation)

Every backend attaches the active **trace-id** to its logs (`trace_id=…` on each Go access log and every worker `structlog` line) and surfaces it on HTTP responses as the **`X-Trace-Id`** header — including 5xx errors. So the pivot from a symptom to its root-cause trace is always:

1. Get a trace-id — from the `X-Trace-Id` response header of a failing request, from a `trace_id=` log line (`make logs`), or from the `trace_id` column of PostgreSQL's `documents` table.
2. Grafana → **Explore** → select the **Tempo** datasource → query by **TraceID** → paste the id.
3. The trace spans the pipeline across the async hops: Ingestion API → (W3C `traceparent` propagated in the MinIO Bronze object's `UserMetadata`, carried to the worker on the NATS S3-notification) → Analysis Worker → ClickHouse/MinIO Silver. Because AĒR is NATS/MinIO-coordinated, a "distributed trace" here is **linked spans via propagated context**, not an HTTP call chain.

### Prometheus (Metrics & Alerting)

| Property       | Value                                        |
| :------------- | :------------------------------------------- |
| Internal only  | `prometheus:9090` (not exposed to host)       |

Accessed via Grafana. Scrapes every 5 seconds: the OTel Collector (`:8889`), Analysis Worker (`:8001`), Ingestion API (`:8081/metrics`), BFF API (`:8080/metrics`), MinIO (`:9000/minio/v2/metrics/cluster`, public auth on the internal network) and ClickHouse (`:9363`, built-in Prometheus endpoint). Alert rules are defined in `infra/observability/prometheus/alert.rules.yml`. Validate any config change with `make observability-validate` (also a CI gate).

### Tempo (Distributed Tracing)

| Property       | Value                                        |
| :------------- | :------------------------------------------- |
| Internal only  | `tempo:3200` (not exposed to host)            |

Accessed via Grafana. Stores traces with configurable block retention (72h dev / 720h prod). Trace context propagates across the async boundary not via NATS headers but via the **MinIO Bronze object's `UserMetadata`**: the Ingestion API injects the W3C `traceparent` on upload, MinIO echoes it in the S3 event it publishes to NATS JetStream, and the worker extracts it to continue the same trace.

### OpenTelemetry Collector

| Property       | Value                                          |
| :------------- | :--------------------------------------------- |
| gRPC receiver  | `otel-collector:4317` (debug: `localhost:4317`) |
| HTTP receiver  | `otel-collector:4318` (debug: `localhost:4318`) |
| Prom exporter  | `otel-collector:8889`                           |

Configuration: `infra/observability/otel-collector.yaml`. Routes traces to Tempo and metrics to Prometheus.

**Trace sampling** is configurable via `OTEL_TRACE_SAMPLE_RATE` (default: `1.0` = 100%). Production recommendation: `0.1` (10%). The sampler is `ParentBased(TraceIDRatioBased(rate))` in every service, so a sampled root keeps its whole linked-span trace.

### Observability runbook — symptom → trace → root cause

Start every investigation at the **AER Pipeline Overview** dashboard, then drill from a metric to the trace that explains it. Each row below is "what you'd see → first move → likely cause".

| Symptom (on the dashboard) | Pivot | Likely root cause / next step |
| :------------------------- | :---- | :---------------------------- |
| **Error Rate (5xx) spikes for a service** | Reproduce or grab a failing request's `X-Trace-Id`; open it in Tempo. Cross-check the `trace_id=` access log (`make logs`). | Find the failing span: a DB/MinIO error span points at storage; an auth/validation span points at config. The generic `{"error":"internal error"}` body never leaks detail — the trace and server log do. |
| **Request Latency p95 climbs** | Open a slow trace in Tempo; read the span durations. | Which span dominates — ClickHouse query (BFF), MinIO read, or PostgreSQL? Confirm against ClickHouse Query Rate + the store's `up`. |
| **NATS Consumer Lag `pending` rises and stays up** | Compare Events Processed rate vs. ingestion volume; check Event Processing Duration p95. | Worker pool can't keep up: a heavy-NLP backlog (duration p95 high) or too few workers (`WORKER_COUNT`). `ack_pending` pinned at the queue cap = backpressure working as designed. |
| **DLQ Size / Quarantine rate rises** | Inspect a quarantined object in `bronze-quarantine` (see the MinIO section); find its trace via the `documents.trace_id`. | Poison pill (unknown `source_type`, malformed envelope) — the worker routed it to DLQ after `NATS_MAX_DELIVER`. The quarantine span carries the reason. |
| **WorkerDown alert / `up{job="analysis-worker"}=0`** | Check `make logs` for the worker; check the worker container health. | Crash-loop (bad config/missing secret → boot validation `SystemExit`) or OOM. The boot logs name the missing credential. |
| **A store panel is empty but its `up=1`** | The service is reachable but a specific metric name drifted across an image bump. | Confirm the live metric name at the native endpoint (`curl` from inside the backend network) and adjust the dashboard panel; run `make observability-validate`. |
| **A trace stops at the Ingestion span (no worker span)** | Expected if the document was filtered before the worker; otherwise check the MinIO→NATS hop. | Verify the Bronze object carries a `traceparent` in its `UserMetadata` and the worker is consuming (`nats_consumer_pending`). |

If telemetry itself looks dark: confirm `OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318` (in-network host, not `localhost`), that the OTel Collector is up, and that `OTEL_TRACE_SAMPLE_RATE` is not `0`.

---

## Capacity & Growth Guardrails (Phase 150)

AĒR runs on a single box (Hetzner CX43 — 8 vCPU / 16 GB / one disk). The data layer
must not silently fill its disk or fall behind the crawl. These guardrails turn the
Phase-148 capacity model into **live, alerting signals** plus a scale procedure. All
of them ride on metrics that **already exist natively** (Phase 154 added the worker
JetStream-lag gauge + the ClickHouse-native endpoint; `prometheus_client` exposes the
worker's RSS for free) — so no extra exporter container is needed (see "Exporters" at
the end). Rules live in `infra/observability/prometheus/alert.rules.yml` (group
`aer_capacity`), mirrored in `grafana/provisioning/alerting/rules.yaml`; change a
threshold in **both** and run `make observability-validate`.

### Alert → response

These are all `warning` severity (review when convenient — `WorkerDown`,
`DiskSpaceCritical`, `BackupFailed`, and the TLS-critical alerts are the wake-me-up
`critical` set). Each fires only on a **sustained** condition, so a normal 6-hourly
crawl burst does not trip it.

| Alert | Means | First response |
| :---- | :---- | :------------- |
| **WorkerBacklogGrowing** (`nats_consumer_pending > 1000` for 1h) | The worker pool cannot drain ingestion — backlog is not clearing within the hour. Distinct from `CrawlStale` (zero throughput). | Check `WorkerMemoryHigh` + Event Processing Duration p95. Levers, cheapest first: raise `WORKER_COUNT` (if CPU headroom), reduce crawl cadence (the `aer-crawl` timer), or narrow the topic/co-occurrence window. If it persists after tuning → the box is at its throughput limit (see "Scale to the next probe tier"). |
| **WorkerMemoryHigh** (worker RSS > 90% of the 11 GiB cap for 15m) | The Phase-148c memory headroom (~9.3 GiB measured peak) is shrinking; at the cap the container OOMs (→ `WorkerDown`). | Confirm corpus size growth. The two RAM drivers (WP-005 / 148c) are the full-corpus topic E5 embedding and the co-occurrence 1-year window — both scale with corpus. Levers: the Silver `raw_text` storage lever, the topic embedding-cache / window-streaming work (ROADMAP), or raise `deploy.resources.limits.memory` **only if** the host has spare RAM (keep the `WorkerMemoryHigh` `11811160064` constant in lockstep with `compose.yaml`). |
| **WorkerPoisonMessages** (`increase(...[1h]) > 0`) | A document failed processing even after NATS redelivery — usually a source's HTML/feed structure changed and broke an adapter. | Read the worker logs for the failing object; inspect it in `bronze-quarantine` (MinIO section). Fix the adapter or the source's `sources.yaml`; the document replays from archived Bronze once fixed. |
| **MinIOCapacityLow** (usable capacity < 15% for 15m) | Object storage is filling. On this single box it tracks the host disk (so `DiskSpaceLow` likely fires too), but it isolates the object-store dimension. | Run `verify_ilm.sh` (below) — if it reports a violation, ILM is not pruning. Otherwise the corpus is genuinely growing: reduce crawl scope or scale the disk. |
| **ILMCheckStale** (no `verify_ilm.sh` report in 26h) | The retention-enforcement check stopped running — TTLs are no longer being verified live. | Check the `aer-verify-ilm` systemd timer (`systemctl status aer-verify-ilm.timer`); run the script by hand to confirm it works. |
| **ILMViolation** (`aer_ilm_violations > 0`) | Data older than its TTL is lingering — a MinIO lifecycle scan or a ClickHouse TTL merge is not pruning. Unbounded growth **and** a DSGVO right-to-erasure breach. | Run `verify_ilm.sh` to see which layer; check the MinIO bucket lifecycle (`mc ilm rule ls`) and ClickHouse TTL (`SHOW CREATE TABLE`, force a merge with `OPTIMIZE ... FINAL` if a merge is stuck). |

### Verifying ILM enforcement (TTLs actually pruning)

`scripts/operations/verify_ilm.sh` is the "verified live, not just configured" check.
It is **read-only** — it never deletes; it reports whether the configured TTLs are
actually pruning, against the real data:

- **ClickHouse Gold** — counts rows older than their TTL (+grace) in `metrics`,
  `entities`, `entity_links`, `language_detections` (anchor `timestamp`),
  `entity_cooccurrences`, `topic_assignments` (anchor `window_start`). The
  longer-retention resolution MVs are intentionally excluded.
- **MinIO** — `mc find --older-than` on `bronze` (90 d), `silver` (365 d),
  `bronze-quarantine` (30 d).

Lazy expiry (MinIO's scanner, ClickHouse background merges) means freshly-expired
data lingers briefly, so each TTL carries `ILM_GRACE_DAYS` (default 2). The script
writes `aer_ilm_violations` + `aer_ilm_last_check_timestamp_seconds` to the
node-exporter textfile dir (the `ILMCheckStale` / `ILMViolation` alerts read these)
and exits non-zero on a violation.

```bash
# on the box, on demand:
bash scripts/operations/verify_ilm.sh

# continuous (daily, writes the heartbeat the alerts watch):
sudo cp infra/systemd/aer-verify-ilm.{service,timer} /etc/systemd/system/
sudo systemctl daemon-reload && sudo systemctl enable --now aer-verify-ilm.timer
```

> On a freshly-deployed box the check trivially passes (no data is older than any
> TTL yet) — its value is **forward**: it proves enforcement keeps working as the
> corpus ages past the first 90/365-day boundary.

### "Data grew faster than expected" — triage

1. **Is ILM pruning?** Run `verify_ilm.sh`. A violation means growth is a *retention*
   bug (lifecycle/TTL not firing), not real volume — fix that first, it is free.
2. **Which layer is growing?** Bronze (raw HTML, 90 d) is the largest by far. Check
   per-bucket size: `mc du src/bronze src/silver` (MinIO section). ClickHouse:
   `SELECT table, formatReadableSize(sum(bytes)) FROM system.parts WHERE active GROUP BY table` (ClickHouse section).
3. **Is it crawl volume or a single runaway source?** Compare per-source counts
   (`SELECT source, count() FROM aer_gold.metrics GROUP BY source`). A single source
   ballooning = tighten its `sources.yaml` discovery scope; broad growth = a real
   capacity decision (next section).

### "Scale to the next probe tier" — the single-box limit

The Phase-148 model sizes the box for the current probe set. The **trigger to scale**
is any of `WorkerBacklogGrowing`, `WorkerMemoryHigh`, or the disk alerts firing
*persistently after the cheap levers are exhausted* (worker count, crawl cadence,
window narrowing, the `raw_text` storage lever). Occam order:

1. **Tune first (no new cost).** `WORKER_COUNT`, crawl cadence, the analytical-window
   cutoff, the Silver `raw_text` lever (~3× MinIO win, see Stage-B notes). Most
   "we're full" situations are a retention or tuning issue, not a hardware wall.
2. **Vertical scale (the single-box Occam step).** Resize the Hetzner box to more
   RAM/CPU (CX43 → a CCX line or a larger CX). The stack is image-pull-based (Phase
   150 CD), so the move is: snapshot/backup → resize in the Hetzner console → boot →
   `deploy_pull.sh <tag>`. Raise the worker `memory`/`cpus` limits to match (keep the
   `WorkerMemoryHigh` constant in lockstep). No architecture change.
3. **Horizontal split (deferred — only when vertical is exhausted).** Moving a
   datastore (ClickHouse / MinIO) or the worker pool off-box is a real architecture
   change (network posture, backup, the no-direct-HTTP rule still holds via NATS +
   MinIO). Not warranted at single-probe-tier scale; revisit per the Phase-148 model
   when vertical headroom is genuinely gone.

The hundreds-of-probes ambition scales on the per-source-class backbone (validation
labour, not box size, is the real bottleneck) — a single box serves the current tiers;
the trigger above is the honest "this box is full" signal.

### Validating the guardrails (a drill)

To confirm an alert actually fires (not just that the rule parses), force its
condition on the box and watch Grafana → Alerting → the `aer_capacity` group go
`Pending` → `Firing` (and a mail arrive):

- **ILMViolation / ILMCheckStale** — the safest drill (no load needed): write a
  violation to the textfile by hand and let node-exporter pick it up, e.g.
  `printf 'aer_ilm_violations 1\n' | sudo tee /var/lib/aer/textfile/aer_ilm.prom`
  (then restore it by re-running `verify_ilm.sh`). For `ILMCheckStale`, stop the timer
  and wait (or temporarily lower the rule's `26 * 3600`).
- **MinIOCapacityLow / DiskSpaceLow** — temporarily lower the rule threshold (e.g.
  `< 0.95`) and `make observability-validate` + reload Prometheus/Grafana; confirm it
  fires, then revert. Lowering the threshold is safer than filling a production disk.
- **WorkerBacklogGrowing / WorkerMemoryHigh** — lower the threshold (or shorten `for:`)
  the same way, or generate load with a large crawl; confirm `Firing`, then revert.

Document the drill date in this section's history when you run it on the box.

### Exporters (cadvisor / nats-exporter / postgres_exporter) — scope note

The Phase-150 plan listed three dedicated exporters (SEC-052 remainder). **Two are now
superseded by Phase-154 native metrics** and were deliberately NOT added (Occam — no
redundant containers): **cadvisor** (per-container memory/restart) is unneeded because
the worker's RSS comes from `prometheus_client`'s default process collector
(`WorkerMemoryHigh`) and every other container has a hard `mem_limit`; **nats-exporter**
is unneeded because the worker's `nats_consumer_pending` already gives the JetStream
backlog (`WorkerBacklogGrowing`) and `TargetDown` covers NATS-down. The one genuine gap
is **postgres_exporter** (connection-pool saturation early-warning) — deferred pending
operator sign-off because it needs a **new read-only monitoring credential** (a
`pg_monitor` role + DSN secret), which is a security-posture decision, not an
autonomous one. Until then PostgreSQL failure is caught by `TargetDown` on the
bff/ingestion targets. Re-open by adding the role to `postgres-init-roles`, the DSN to
`.env`, the exporter container (digest-pinned), a scrape job, and a
`PostgresConnectionsHigh` alert.

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

### Web Crawl Operations (Phase 122)

The Phase-122 web crawler is a single configurable Python binary under `crawlers/web-crawler/` (Scrapy 2.x + ultimate-sitemap-parser + courlan + feedparser + psycopg2 + structlog). It fetches full-article HTML for every news-website probe and submits documents to the Ingestion API verbatim — Bronze is raw HTML, the analysis worker's `WebAdapter` (Phase 122 / ADR-028) runs trafilatura + extruct + htmldate + readability at the Silver boundary. The crawler binary is identical across probes; per-probe configuration lives in `crawlers/web-crawler/probes/<probe-id>/sources.yaml`. It executes as a one-shot container on the internal `aer-backend` Docker network under Compose profile `crawlers`, reaching `ingestion-api:8081` and `postgres:5432` directly. No host port exposure is required.

```bash
# Probe 0 — German institutional sources.
make crawl-probe0

# Equivalent raw form:
docker compose --profile crawlers run --rm --build web-crawler --probe probe0

# Deprecated alias (retired in Phase 129):
make crawl     # forwards to make crawl-probe0
```

`make crawl-probe0` reads `INGESTION_API_KEY` and the Postgres connection variables from `.env` via Compose interpolation; if `.env` is missing, the target exits with an error before invoking Compose. Profile-gated, so `make up` never starts the crawler automatically. New probes follow the `make crawl-<probe-id>` pattern — see [add-a-source.md](../extending/add-a-source.md) for the per-source mechanics.

**Production scheduling.** The same command runs under any host-side scheduler — cron, systemd timer, Kubernetes `CronJob` wrapping `docker compose run --rm web-crawler --probe <id>`, or a long-running scheduler sidecar on the Compose project. No scheduler is bundled today; pick one per deployment target.

**Per-probe configuration** (`crawlers/web-crawler/probes/<probe-id>/sources.yaml`, baked into the image at build time):

```yaml
sources:
  - name: tagesschau               # Must match a source registered in PostgreSQL
    sitemap_urls:
      - https://www.tagesschau.de/sitemap.xml
    rss_hint_url: https://www.tagesschau.de/index~rss2.xml   # peer-equal discovery channel since Phase 122e (F-A1); sole channel for sources without a public XML sitemap
    politeness:
      delay_seconds: 1.0
      autothrottle: true
      max_concurrent_per_domain: 2
    url_filter:                    # technical filtering only — no section gating
      exclude_extensions: [jpg, png, gif, svg, webp, mp4, mp3, css, js, pdf, ico, woff, woff2]
      exclude_path_prefixes: [/api/, /search/, /suche/, /impressum, /datenschutz, /agb]
      require_html_content_type: true
    content_filter:
      min_word_count: 50
      require_extraction_success: true
    custom_extractors: {}          # Tier-E scaffolded — empty until a specific analysis demands it
```

Changing `sources.yaml` requires rebuilding the image (`make crawl-probe0` passes `--build`, so the next invocation picks it up automatically; CI pipelines should rebuild explicitly via `.github/workflows/web-crawler-build.yml`).

**Probe-scoped temporal cutoff (Phase 122b).** Every `sources.yaml` carries a top-level `probe:` block declaring the uniform discovery horizon for every source on the probe. Cross-source comparability requires a uniform temporal cutoff — without it, a source whose sitemap surfaces 30 years of archive appears louder in cross-source aggregates than a source whose sitemap only goes back 10 years (the **archive-depth bias** failure mode).

```yaml
probe:
  time_window_days: 7              # rolling 7-day watermark for continuous-monitoring cadence (Phase 122e A20 walked this back from the original 1825-day backfill horizon). The corpus accumulates organically across cron runs; the Gold-side MV TTLs (Phase 122c — hourly 365 d / daily 1825 d / monthly indefinite) retain longer history independent of this knob.
  sitemap_strict_lastmod: true     # default-true (Phase 122e A21 / F-A21). When the temporal filter is active, sitemap entries with no <lastmod> are dropped at discovery. Flip to false only for an explicit backfill run.
sources:
  - name: tagesschau
    ...
```

The cutoff is **probe-level by design** — per-source overrides are explicitly rejected. If a future source needs a different horizon, that source belongs in a different probe. At startup the crawler logs a single `crawl_window_configured` structlog line (probe, `time_window_days`, computed `since` ISO timestamp) — that line is the spot-check anchor when verifying a run honoured the configured cutoff. Sitemap and RSS discovery both filter on `lastmod ≥ since`. **Sitemap entries with no parsable `<lastmod>` are dropped at discovery** (Phase 122e A21 / F-A21 — `sitemap_strict_lastmod: true` is the default for continuous-monitoring mode; iter-4 forensics found 638-of-638 of bundesregierung's leaf-sitemap entries are undated, so the original fall-through rule silently bypassed the temporal filter for that source's entire 5-year archive). An explicit backfill run can set `sitemap_strict_lastmod: false` — undated entries then fall through and the worker classifies them as Negative-Space via `timestamp_source = "fetch_at_fallback"` per Brief §7.7. RSS entries without a parsable date fall through unconditionally (RSS items are nearly always recent — the filter is defensive). When `probe.time_window_days` is absent the crawler emits a structured warning and falls back to 365 days — see [add-a-probe.md](../extending/add-a-probe.md) for the new-probe checklist.

**Newest-first iteration.** Within the cutoff window, `_discover_for_source` sorts the merged URL list by `sitemap_lastmod` descending, with `None` lastmods sinking to the end. A partial crawl (Ctrl+C, overnight stop, bandwidth pause) therefore yields the most-recent slice of the cutoff window first. Subsequent runs honour the same ordering — combined with `crawler_state` dedup, a multi-session crawl monotonically advances backward through the cutoff window without revisiting URLs.

**Discovery semantics (four-channel `DiscoveryProtocol` since Phase 122g).** Discovery runs across the per-source `discovery:` block declared in `sources.yaml` (ADR-031). Four channels are supported today for the web crawler — sitemap, rss (plural since Phase 122g), html_sitemap (Phase 122g), archive_index (Phase 122e code; Phase 122g activates per-source). Each channel's entry-date competes fairly in the newest-first sort; the URL union de-duplicates with channel-order precedence (sitemap > rss > html_sitemap > archive_index — the sitemap entry carries the canonical `lastmod` and `sitemap_section` context). Channel-collision attribution is recorded per channel in the telemetry layer so the dashboard reports each channel's actual contribution. The article body always comes from the HTML fetch — RSS bodies are never consumed. Once a URL is in `crawler_state`, the next run sends conditional GET headers (`If-None-Match` / `If-Modified-Since`) and skips on `304 Not Modified`.

**Per-source onboarding (Phase 122g).** Adding a new source runs `aer-audit-source <homepage_url>` to inventory the publisher's discovery surfaces — wraps `trafilatura.feeds.find_feed_urls` + `trafilatura.sitemaps.sitemap_search` + per-source-class HTML-sitemap / archive-walker URL-pattern probes. Output is YAML-shaped for direct paste into the source's `discovery:` block. Operator derives the publisher-specific `article_url_pattern` regex(es) from a sample of real article URLs. See `docs/extending/add-a-source.md` for the canonical workflow.

**Per-channel coverage telemetry (Phase 122g).** Every discovery pass writes one row per `(source, channel)` to Postgres `crawler_discovery_runs` (`urls_discovered`, `urls_after_dedup`, `run_started_at`, `run_completed_at`). When `urls_after_dedup` < `expected_floor_per_run` for **two consecutive runs**, a row lands in `crawler_discovery_alerts` (alert_type=`underflow`); the first below-floor run lands as `underflow_pending`. Recovery (first run back at or above floor) clears both. Inspect telemetry:

```sql
-- Per-channel last-7-days summary for one source.
SELECT channel,
       max(run_started_at)            AS last_run,
       sum(urls_discovered)            AS total_discovered,
       sum(urls_after_dedup)           AS total_after_dedup,
       round(avg(urls_discovered), 1)  AS avg_discovered_per_run
  FROM crawler_discovery_runs
 WHERE source_id = 1
   AND run_started_at > now() - interval '7 days'
 GROUP BY channel
 ORDER BY channel;

-- Active discovery alerts across all sources.
SELECT s.name, a.alert_type, a.consecutive_runs,
       a.expected_floor, a.last_urls_observed, a.last_observed_at
  FROM crawler_discovery_alerts a
  JOIN sources s ON s.id = a.source_id
 WHERE a.alert_type = 'underflow'
 ORDER BY a.last_observed_at DESC;
```

The same telemetry feeds the BFF endpoint `GET /api/v1/sources/{sourceId}/discovery-coverage` and the dashboard's `DiscoveryCoveragePanel` (sibling to Phase 122f's metadata-coverage panel). Coverage degradation is observable within one crawl run; pre-122g coverage gaps were invisible until ad-hoc operator inspection.

**Underflow response runbook.** When a `discovery_underflow` log line fires (or the dashboard panel flips to alert state):

1. Re-run `aer-audit-source <homepage_url>` against the affected source. If the channel the alert names is missing from the audit output, the publisher's surface has changed.
2. For an XML-sitemap underflow: visit `/sitemap.xml` and `/robots.txt` manually; the publisher may have moved their sitemap or changed the `Sitemap:` directive.
3. For an RSS underflow: visit the publisher's RSS catalogue page (`/service/rss`, `/newsletter`, etc.); a feed may have retired or moved.
4. For an HTML-sitemap underflow: visit the configured URL; the publisher may have redesigned the page or changed the HTML structure (the article-URL regex may no longer match).
5. For an archive-index underflow: curl two distant dates and verify the endpoint still returns distinct article lists.
6. Update the source's `discovery:` block in `sources.yaml` to reflect the new surface. Re-deploy. The next run clears the alert.

**Probe-0 Structural Bias #8 — discovery-surface asymmetry — closes for specific instances under Phase 122g.** tagesschau's RSS-only configuration is replaced by RSS + HTML sitemap + archive walker (≈ 250–300 unique articles per 7-day run vs. the prior ≈ 70). bundesregierung's single-RSS-feed configuration is extended to all four publisher-curated feeds. New discovery-surface asymmetries on future sources are caught by the underflow telemetry rather than by post-hoc inspection.

### Silent-Edit Observability — Wayback CDX integration (Phase 122d.0 / ADR-032)

The worker queries the Internet Archive's CDX API as the last step of `harmonize()` for every `source_type='web'` article. The result lives on `WebMeta.wayback_revisions[]` and `wayback_lookup_status`, and a corresponding row-set lands in `aer_gold.article_revisions`. The integration is operator-managed via the `WAYBACK_CDX_*` env vars (`WAYBACK_CDX_ENABLED`, `WAYBACK_CDX_BASE_URL`, `WAYBACK_CDX_TIMEOUT_SECONDS`, `WAYBACK_CDX_RATE_LIMIT_PER_SECOND`, `WAYBACK_CDX_CACHE_TTL_HOURS`, `REPUBLICATION_TRIGGER_MIN_DELTA_DAYS`).

**Fail-silent invariant.** A CDX timeout, network error, malformed response, or rate-limit denial NEVER produces a DLQ event and NEVER aborts harmonisation. Each outcome maps to a typed `wayback_lookup_status` value:

| Status        | What it means                                              | Operator action                                                |
| :------------ | :--------------------------------------------------------- | :------------------------------------------------------------- |
| `ok`          | CDX returned ≥ 1 snapshot.                                 | None — the dashboard renders the chain.                        |
| `no_snapshots`| CDX returned 0 — the URL is not yet archived.              | None — IA coverage is a property of the Internet Archive.       |
| `failed`      | Timeout, network error, non-2xx HTTP, or rate-limit denial.| Spike in `failed` rate indicates an IA outage. See runbook below. |
| `skipped`     | Canonical URL was empty.                                   | Check `WebAdapter.harmonize` — should not normally occur.       |
| `disabled`    | `WAYBACK_CDX_ENABLED=false`.                                | None.                                                          |

**Status distribution health-check.** A healthy production run produces a mix of `ok` and `no_snapshots`; `failed` should be rare. Query the per-source distribution:

```sql
SELECT
    source,
    JSONExtractString(meta, 'wayback_lookup_status') AS status,
    countDistinct(article_id) AS articles
FROM aer_silver.documents
WHERE timestamp > now() - INTERVAL 7 DAY
GROUP BY source, status
ORDER BY source, status;
```

(The Silver projection does not currently carry the CDX status as a typed column; the query above will need the WebMeta JSON path adjusted once Phase 122d.x adds a typed column. Until then, the per-article check is `SELECT count() FROM aer_gold.article_revisions WHERE source = ? GROUP BY revision_trigger`.)

**"IA is down" runbook.** If the `failed` rate spikes:

1. **Verify the outage is on the IA side, not the local stack.** From a worker host: `curl -I -m 10 'https://web.archive.org/cdx/search/cdx?url=tagesschau.de&output=json&limit=1'`. A 5xx, 429, or persistent timeout confirms the IA-side outage.
2. **No emergency action is required.** Articles continue to flow through to Silver and Gold; only the `article_revisions` rows are missing for the duration of the outage. The dashboard's revision cells render the gap honestly via `wayback_lookup_status='failed'`.
3. **Optional: lower the per-worker rate limit.** If the IA is rate-limiting AĒR specifically (uncommon — IA's public CDX has been generous historically), set `WAYBACK_CDX_RATE_LIMIT_PER_SECOND` to `1.0` or lower in `.env` and restart the worker (`make worker-restart`). Re-raise once the IA recovers.
4. **Backfill is automatic.** The next worker run on the same Bronze keys (after a manual `make reset` or a backfill replay) re-issues the CDX lookup — the Postgres cache write is a no-op for stale entries, so a fresh lookup happens after the cache TTL expires (`WAYBACK_CDX_CACHE_TTL_HOURS`, default 24 h). Manually invalidate the cache for a specific URL with `DELETE FROM wayback_cdx_cache WHERE canonical_url = ?`.

**Coverage invariant.** There is no fixed minimum-coverage threshold for the Wayback signal. The Internet Archive's coverage of any given publisher is a property of the IA, not of AĒR, so a probe whose sources have `no_snapshots` is not a degraded state — it is the honest baseline. The dashboard renders that baseline; if an analyst needs deeper coverage, the path is to submit the source's URLs to the IA's save-page-now endpoint (out of scope for AĒR's operational responsibility).

**Republication trigger.** The republication-trigger row family is the Phase-131a UX/conceptual artefact reconciled at the methodological layer: an article whose `sitemap_lastmod` is ≥ 7 days (configurable via `REPUBLICATION_TRIGGER_MIN_DELTA_DAYS`) ahead of its `published_date` emits a synthetic revision row. The dashboard renders these alongside CDX snapshots in the revision chain. A pure-republication-trigger article (no CDX snapshots, only a re-listing event) is the canonical case for publisher-side re-listing without a third-party archive — useful for sources the IA does not yet cover.

**Postgres cache.** `wayback_cdx_cache` is a thin point cache keyed on canonical_url. At one row per canonical URL the table is naturally bounded by the worker's source set. No retention sweep is required; stale rows are overwritten on the next lookup. If a manual purge is needed (e.g. after a large probe-roster change):

```sql
-- Trim the cache to the most recent 90 days of lookups.
DELETE FROM wayback_cdx_cache WHERE fetched_at < now() - INTERVAL '90 days';
```

**Disabling the integration.** Set `WAYBACK_CDX_ENABLED=false` and restart the worker. Existing `aer_gold.article_revisions` rows are retained until the schema TTL (365 days on `snapshot_at`) merges them out; the cell renders an empty state for new articles. Re-enabling resumes lookups on the next harmonisation.

**Robots.txt verification.** Before adding a new source, verify that the polite default `User-Agent` is permitted:

```bash
curl -A "${WEB_CRAWLER_USER_AGENT}" https://<source>/robots.txt | grep -i 'allow\|sitemap'
```

If the `User-Agent` is `Disallow`-ed for the relevant paths, document the case in the probe dossier and either change the source or arrange a polite contact with the operator. Override the `User-Agent` per environment via `WEB_CRAWLER_USER_AGENT` in `.env`; the contact-address requirement is WP-006 §5.1.

**Polite-crawl defaults.** The Scrapy spider runs with `ROBOTSTXT_OBEY = True`, `AUTOTHROTTLE_ENABLED = True`, `DOWNLOAD_DELAY = 1.0`, `CONCURRENT_REQUESTS_PER_DOMAIN = 2`, `RETRY_ENABLED = True`, `RETRY_TIMES = 3`, `DOWNLOAD_TIMEOUT = 30`, `COOKIES_ENABLED = False`. Per-source overrides are permitted in `politeness:` but discouraged; any override is documented in the source's Probe Dossier with methodological justification.

**Dedup-state inspection.** Dedup state lives in the Postgres `crawler_state` table (Phase 122; replaces the legacy JSON file). Inspect with:

```sql
SELECT source_id, count(*) AS rows, max(last_fetched) AS most_recent
FROM crawler_state GROUP BY source_id;

SELECT canonical_url, last_fetched, etag, http_last_modified, sitemap_lastmod
FROM crawler_state WHERE source_id = 1
ORDER BY last_fetched DESC LIMIT 20;
```

A `make reset` (Phase 120b) wipes `postgres_data` and therefore the `crawler_state` rows — the next `make crawl-<probe-id>` re-ingests every URL surfaced by the sitemap. There is no longer a separate `make crawl-reset` target; the dedup state is logically Postgres-tied and travels with `make reset`.

**Resumable iteration (Phase 122e A9).** `make crawl-probe0` is **resumable across sessions**. `Ctrl+C` during a long run, close the laptop, come back the next day — the crawler picks up where it left off because every successfully-fetched URL is recorded in `crawler_state` *as it happens* (not at the end of the run). The next invocation:

1. Re-discovers the full sitemap + RSS feed surface (cheap — minutes for any realistic probe).
2. Skips every URL already in `crawler_state` whose `sitemap_lastmod` hasn't moved (`has_seen()` returns true).
3. Fetches only the unseen URLs, in newest-first order.

Multi-session crawls monotonically advance backward through the cutoff window without revisiting any URL. **Operators iterating on Phase 122e fixes do NOT need `make reset` between iterations** — only when they want a fresh baseline corpus (e.g., a new methodological invariant requires a clean re-run, or a `web_extract.py` fix needs to replay archived Bronze via `scripts/operations/reextract_silver.py` rather than re-fetching). Defaulting to `make reset` between iterations is wasteful: it discards the politeness-budget-spent Bronze and forces every URL through the network again.

**Re-extraction without re-crawl** *(operational realisation of the medallion-architecture decoupling, ADR-028).* Trafilatura version upgrades and bug fixes in the Silver-side extraction pipeline trigger Silver/Gold rebuilds without re-crawling. The mechanism is `scripts/operations/reextract_silver.py` — replays archived Bronze HTML through the worker's `WebAdapter`, rewriting Silver and Gold rows. No politeness budget is spent; no risk that an upstream source has changed or is down between runs:

```bash
python scripts/operations/reextract_silver.py --probe probe0
# Inspect the resulting Silver projection counts:
docker compose exec clickhouse clickhouse-client \
    --query="SELECT count() FROM aer_silver.documents WHERE source_type = 'web'"
```

This is the routine cadence for upgrading the extraction stack — never edit Bronze, always replay it.

**Environment variables** (injected by the `web-crawler` Compose service):

| Variable | Purpose | Default |
| :--- | :--- | :--- |
| `INGESTION_URL` | Ingest endpoint | `http://ingestion-api:8081/api/v1/ingest` |
| `SOURCES_URL` | Sources lookup endpoint | `http://ingestion-api:8081/api/v1/sources` |
| `INGESTION_API_KEY` | `X-API-Key` header value | from `.env` |
| `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` | `crawler_state` connection | `postgres:5432` / from `.env` |
| `WEB_CRAWLER_USER_AGENT` | Polite UA per WP-006 §5.1 (must include contact address) | from `.env` (default in `.env.example`) |

**HTTP timeouts.** Scrapy's `DOWNLOAD_TIMEOUT = 30` caps each fetch; the synchronous ingestion-API client uses a 30 s timeout for both `GET /sources` and `POST /ingest`.

**Graceful shutdown.** Scrapy's `CrawlerProcess` traps `SIGINT` / `SIGTERM` and drains in-flight requests cleanly; any documents already submitted are persisted in `crawler_state` so a re-run after interruption resumes from the last successful URL.

### Removed: legacy RSS Crawler (Pre-Phase-122)

The legacy Go RSS crawler (formerly `crawlers/rss-crawler/`, then archived under `crawlers/_archived/rss-crawler/`) was **removed entirely in Phase 129**. Its source and migration notes live in git history (it was archived in Phase 122 and carried a `MIGRATED.md`). The Phase-122 web crawler + `WebAdapter` cover the same probe scope with full article bodies; the worker still registers an `RssAdapter`, so any archived `rss`-shaped Bronze continues to replay.

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
2. Copy the five files from `docs/probes/probe-0-de-institutional-web/` and replace the content. Keep the headings stable so the structure stays uniform across probes.
3. Add the dossier to `mkdocs.yml` under the `Probes` nav entry.
4. Create a PostgreSQL migration that updates `sources.documentation_url` to point at the new dossier directory:
   ```sql
   UPDATE sources SET documentation_url = 'docs/probes/<probe-id>/' WHERE name = '<source-name>';
   ```

**Probe 0 example** — `docs/probes/probe-0-de-institutional-web/` contains the five files above, populated for tagesschau.de and bundesregierung.de. Migration `000008_update_documentation_url.up.sql` points the `sources.documentation_url` column at this directory.

---

## Documentation Servers (Public / Internal split — ADR-040)

Phase 134 splits the docs into two MkDocs builds (ADR-040). Both serve from the
same `docs/` tree with live reload; they differ only in config + nav.

| Build | Config | URL | Contents |
| :---- | :----- | :-- | :------- |
| **Public** (`docs` service) | `mkdocs.public.yml` | `http://localhost:8000` | Methodology Working Papers, per-probe descriptions, manifesto. What an external researcher needs. The source-card `documentation_url` points here. |
| **Internal** (`docs-internal` service) | `mkdocs.yml` | `http://localhost:8001` | Arc42, ADRs, design, extending guides, operations runbooks, security model. |

```bash
open http://localhost:8000   # public methodology portal
open http://localhost:8001   # internal engineering docs
```

`make docs-build` builds **both** with `--strict` (the same check Phase 129
runs). The public build *excludes* the internal directories entirely (they are
not rendered, not merely hidden), so the engineering internals never ship in
the public site.

> **Deployment access (Phase 149).** The internal docs are **not** publicly
> routed and never should be. In production:
> - `docs-internal` publishes only to the box loopback (`127.0.0.1:8001`,
>   SEC-018); the firewall exposes only 22/80/443. Reach it via an SSH tunnel —
>   the same pattern as the Grafana/MinIO consoles:
>   ```bash
>   ssh -L 8001:localhost:8001 <box>   # then open http://localhost:8001
>   ```
> - The prod overlay hardens the container (SEC-064): livereload is off and only
>   `docs/` + the two `mkdocs.*.yml` configs are mounted **read-only** — no
>   whole-repo mount, so the docs container cannot read `.env`.
> - The public `:8000` portal is firewall-blocked in the gated POC (not
>   Traefik-routed). Routing it publicly is a separate, deliberate decision.
>
> See `docs/operations/monitoring.md` for the full remote-access surface map.

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
make fe-bundle-size      # bundle budgets: 80 kB shell + per-lazy-chunk caps (after fe-build)
make fe-lighthouse       # Lighthouse CI (perf/a11y/best-practices) inside the pinned image
make fe-check            # Composite: lint + typecheck + test + build + bundle-size
```

`fe-test-e2e`, `fe-test-e2e-update`, and `fe-lighthouse` deliberately run inside the Playwright image pinned in `compose.yaml` — browser font rendering is OS-sensitive, so host-local snapshot runs are not byte-comparable to CI, and Lighthouse reuses that image's Chromium (`CHROME_PATH`) for a reproducible score. Always update baselines via `fe-test-e2e-update`, never with a host-local Playwright.

The accessibility + client-performance contract these gates enforce — the WCAG 2.2 AA scope, the Lighthouse thresholds, and the full bundle-budget table — is documented canonically in **Arc42 §10.3 (Accessibility & Performance Envelope)**. `make fe-lighthouse` asserts the budgets in `services/dashboard/lighthouserc.cjs` against the public routes (`/login`, `/stories/button`); the authenticated surfaces are covered by `fe-bundle-size` + the axe e2e sweep + the manual hardware pass below.

> **Note (Lighthouse dependency):** `@lhci/cli` is a dev-only dependency (never shipped in the static bundle). It transitively needs `tslib >= 2`, so `services/dashboard/package.json` pins a `pnpm.overrides` entry `"tslib": "2.8.1"` — without it a transitive `tslib@1.x` wins resolution and the Lighthouse performance/best-practices audits crash with `tslib_1.__spreadArray is not a function`. Revisit this override on any dependency refresh.

### Manual accessibility + hardware pass (not in CI)

Two items in the Brief §10 / Arc42 §10.3 envelope cannot run in headless CI and must be verified by the operator on real hardware. File the results (date, hardware, browser, locale, observations) below this section when run.

**Hardware frame budget** (Brief §10 High-Fidelity target — 60 fps / < 16 ms frame on M1-class hardware):

1. `make fe-preview` (or `make frontend-up`), open the dashboard, and open DevTools → Performance / the FPS meter (Chromium: Rendering → "Frame Rendering Stats").
2. Atmosphere globe (`/`): confirm ≥ 60 fps at rest and during drag/zoom/fly-to.
3. Workbench: open a full-depth split panel (multiple cells) and the Rhizome co-occurrence network; confirm the force layout settles and the steady state holds the frame budget.
4. **Engine-pause check:** open the Dossier overlay (`?dossier=open`) and confirm the globe's `requestAnimationFrame` loop stops (GPU/CPU drops in the Performance panel); background a tab and confirm the same via the Page Visibility pause.

**Screen-reader pass** (NVDA / VoiceOver / Orca):

1. Walk the three surfaces (Atmosphere, Workbench, Reflection), the Dossier overlay, and the auth flows.
2. Confirm the composed "how to read" notes and the refusal prose read coherently.
3. Confirm modal focus order (open → trap → `Esc` → focus restored) and that auth errors are announced (`role="alert"`).
4. Repeat in **both EN and DE** UI locales (toggle via the locale switch).

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

For container Loop A, drop the Vite hop — the browser hits Traefik directly via the `dashboard` router. Non-browser callers (the web crawler, `scripts/build/e2e_smoke_test.sh`) keep sending `X-API-Key` themselves, directly to BFF on `:8080`. See `compose.yaml` (`bff-api-key` middleware label) and ADR-018.

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

**E2E smoke test** (`scripts/build/e2e_smoke_test.sh`): Starts a fixture HTTP server → synthesises `rss`-shaped Bronze fixtures and posts them to the Ingestion API (exercising the still-registered `RssAdapter` path without invoking the retired crawler binary) → waits for pipeline processing → queries BFF API endpoints (metrics, entities, available metrics, provenance) → validates end-to-end data flow including `discourse_function` propagation and multi-resolution queries → teardown.

### Dashboard E2E + visual regression (Playwright)

The dashboard has two test layers: **Vitest** (`make fe-test` — unit, jsdom) and **Playwright** (`make fe-test-e2e` — E2E + visual + axe-a11y in a real Chromium).

- **No backend needed.** Playwright starts its own server (`pnpm build && pnpm preview` on `localhost:4173` — the static production build) and **mocks every BFF call** with `page.route('**/api/v1/...')`. The Phase-134 auth gate is mocked centrally in `tests/e2e/_fixtures.ts` (a `page` fixture stubs `GET /auth/me` as an active researcher); **specs import `test`/`expect` from `./_fixtures`**, not from `@playwright/test`.
- **Pinned browser image.** `make fe-test-e2e` runs inside the `playwright-runner` image pinned in `compose.yaml`, so screenshots are byte-reproducible against CI — you do not install browsers locally.
- **Visual baselines** are committed under `services/dashboard/tests/e2e/__snapshots__/`. A `toHaveScreenshot` diff failure means either a **regression** (fix the code) or an **intended UI change** (regenerate the baseline). On failure the actual / expected / diff PNGs land in `playwright-report/` (uploaded as a CI artefact).

```bash
make fe-test-e2e             # full E2E + visual + a11y gate (pinned image)
make fe-test-e2e-update      # regenerate committed baselines after an INTENDED UI change, then commit the diff
cd services/dashboard && pnpm exec playwright test -g "some title" --headed   # run + watch a single test locally
```

> Quarantined specs carry a `QUARANTINED (Phase 136 → rewrite in 127)` marker — they assert the retired `/lanes/*` grammar and are rewritten against the three-surface Workbench grammar in Phase 127. Don't delete them.

---

## Dependency Refresh (Supply-Chain Baseline)

Phase 84 pinned the stack's external dependencies to specific hashes: every `FROM` line in `services/*/Dockerfile` uses `image:tag@sha256:digest`, `services/analysis-worker/requirements.lock.txt` is generated with `pip-compile --generate-hashes --require-hashes`, and the vendored SentiWS lexicon is guarded by `SENTIWS_SHA256`. This is how we keep the supply chain reproducible, but it means those hashes are frozen in time and must be rotated deliberately. Phase 88 turns that rotation into a single command.

> **Fast path (Phase 137).** The full `make deps-refresh` rebuilds every image with `--no-cache` (multi-GB, ~20–40 min). For the everyday case use **`make deps-refresh-fast`** (= `deps_refresh.sh --skip-build`): it rotates the base-image digests, the pip lock, and the SentiWS hash but skips the local no-cache rebuild + e2e — **CI validates the new pins on push**, so you only pay the local rebuild when you actually need to test the image build itself.

### When to run it

| Trigger | Priority |
|---------|---------|
| Trivy flags HIGH/CRITICAL on a pinned base image (CI red) | Same day |
| `govulncheck` or `pip-audit` flags a vulnerable transitive dep | Same day |
| Monthly maintenance cadence, regardless of signals | Monthly |
| Before cutting a release tag | Always |

> **Open security debt (Phase 134 review, M-2): bump the Go toolchain to ≥ 1.26.4.**
> `govulncheck` on the BFF reports 3 reachable `crypto/x509` advisories (e.g.
> `GO-2026-5037`) present in go 1.26.3 and fixed in 1.26.4 — newly reachable
> because the Phase-134 WebAuthn attestation/assertion path verifies x509
> certificates. Bump `.tool-versions` (`GO_VERSION`/toolchain), the `go`/`toolchain`
> directives in each `go.mod`, the Go builder base images in `services/*/Dockerfile`,
> and CI, then `make deps-refresh`. Not urgent while undeployed; do it before any
> deployment.
| After editing `services/analysis-worker/requirements.txt` | Always (lockfile drift) |
| After bumping a `FROM image:tag` in any Dockerfile | Always (digest drift) |

Security signals from CI should take precedence over the monthly cadence — if Trivy is red on a Monday, do not wait for month-end.

### Happy path

```bash
make deps-refresh
```

The target delegates to `scripts/build/deps_refresh.sh`, which runs four steps, each of which fails loudly and leaves the working tree in an inspectable state:

1. **Base image digest refresh.** Every `FROM image:tag@sha256:…` line across all three service Dockerfiles is deduplicated by image reference. Each unique `image:tag` is `docker pull`ed once, the new digest is resolved via `docker image inspect`, and the old digest is rewritten in place. Shared base images (alpine, python, golang) stay in lockstep across Dockerfiles by construction.
2. **`requirements.lock.txt` regeneration.** `pip-compile --generate-hashes --allow-unsafe` runs inside the *exact* Python image the worker builds from (read back out of the freshly-updated worker Dockerfile), guaranteeing the hash set is byte-compatible with `pip install --require-hashes` at build time. `pip-tools` is pinned via `PIP_TOOLS_VERSION` in `.tool-versions`.
3. **SentiWS hash recomputation.** The SentiWS lexicon is vendored into the repo (`services/analysis-worker/vendor/SentiWS_v2.0.zip`) — both upstream Leipzig hosts went offline, so it is committed rather than fetched at build time. The script `sha256sum`s the vendored file and rewrites `ARG SENTIWS_SHA256=…` in place. Updating the lexicon is therefore a drop-in file replace — swap the vendored zip, rerun `make deps-refresh`, and the hash follows.
4. **No-cache rebuild + e2e smoke test.** `docker compose build --no-cache` forces every image to reproduce from the new baseline, then `make test-e2e` asserts the full ingestion → worker → BFF pipeline still answers correctly. A failure here means *revert, investigate, do not commit*.

On a clean baseline (nothing upstream moved since the last run), the script produces zero file changes and `git diff` is empty. That property is load-bearing — it's how CI can one day run `make deps-refresh --skip-build` on a scheduled job and only open a PR when the diff is non-empty.

After a successful run, review `git diff` (especially to confirm only digests/hashes changed, not tags) and commit the result as a single `chore(supply-chain): refresh dependency baseline` commit.

### Flags

```bash
make deps-refresh ARGS="--dry-run"     # report intent, no writes, no rebuild
make deps-refresh ARGS="--skip-e2e"    # rebuild but skip the 90s+ e2e suite
make deps-refresh ARGS="--skip-build"  # just rewrite files; no rebuild, no e2e
./scripts/build/deps_refresh.sh --help       # full help from the script directly
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

### Updating the vendored SentiWS lexicon

SentiWS is vendored into the repo (`services/analysis-worker/vendor/SentiWS_v2.0.zip`) and COPYed by the worker Dockerfile — there is no download URL anymore (both upstream Leipzig hosts are offline). Attribution + provenance live in `services/analysis-worker/vendor/README.md`.

1. Replace `services/analysis-worker/vendor/SentiWS_v2.0.zip` with the new file (leave the `SENTIWS_SHA256` value alone — it is rewritten next).
2. Run `make deps-refresh`. Step 3 will `sha256sum` the vendored file and rewrite the hash.
3. Update the attribution in `vendor/README.md` if the version/license changed, then commit the new zip together with the hash change.

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

### pip-audit suppressions

`make audit-python` runs `pip-audit` against each Python service's `requirements.txt`. Suppress a finding **only** when there is no upstream fix (or the fix carries disproportionate migration cost) **and** the vulnerable code path is unreachable from our usage — never suppress a reachable, fixable vulnerability.

Suppressions live in a **per-service, co-located** file: `services/<python-service>/pip-audit-ignores.txt` (today only `services/analysis-worker/`, the sole Python service). The Makefile reads the bare vulnerability-ID lines from it into `pip-audit --ignore-vuln` flags; every other line is documentation. This is the pip-audit analog of `.trivyignore` (images) and carries the same discipline: **each entry MUST document (a) the vulnerability ID, (b) the justification, (c) the upstream-fix tracker**, and is revisited on every `make deps-refresh`. If a fix ships, bump the dependency and delete the entry.

A future Python service gets its own `pip-audit-ignores.txt` beside its `requirements.txt`. `find services -name pip-audit-ignores.txt` enumerates every suppression across the tree — the single review surface, without a central file that would decouple a suppression from the dependency set it annotates.

### Failure recovery

The script uses `set -euo pipefail`, so a failure at any step stops immediately with a red log line pointing at the failing step. The partial diff is preserved so you can inspect it:

```bash
git diff                  # see what was rewritten before the failure
git restore .             # throw it away and start over
git stash                 # save it for later inspection
```

`--skip-build` is the fastest way to iterate on a failure in steps 1–3 without paying the rebuild cost on each run.

### Iterating on a service — what's cached, what isn't

The four service Dockerfiles (`worker`, `web-crawler`, `ingestion-api`, `bff-api`) use BuildKit cache mounts so iterative rebuilds don't re-download or re-compile heavy artefacts. **Compose v2 has BuildKit on by default** — no extra flag needed.

Cache layout (each `id=` is a persistent on-host cache, survives `docker system prune`; clear with `docker builder prune --all`):

| Cache id | Holds |
|---|---|
| `aer-analysis-worker-pip` | Python wheels for the worker (incl. the 567 MB spaCy model wheel, scipy/numpy sdists) |
| `aer-analysis-worker-hf-models` | HuggingFace model snapshots (BERT × 2 + BERTopic embedder, ~10 GB) |
| `aer-hf-prefetch-pip` | huggingface_hub + PyYAML for the prefetch stage |
| `aer-web-crawler-pip` | Scrapy / Twisted / lxml wheels |
| `aer-go-modcache` | `$GOMODCACHE` shared across both Go services |
| `aer-go-buildcache` | `$GOCACHE` shared across both Go services |

The worker Dockerfile additionally splits HF model prefetch into its own `hf-prefetch` stage so model weights are **decoupled from `requirements.lock.txt`** — a Python dep bump no longer re-copies the 10 GB cache into the runtime image.

What rebuilds when (warm caches assumed):

| Change | Stages that rebuild | Real cost |
|---|---|---|
| Worker `*.py` source | runtime `COPY services/analysis-worker/` only | seconds |
| Worker `requirements.lock.txt` | builder pip (only the changed wheel re-pulled) | ~minute |
| `language_capabilities.yaml` model revision | `hf-prefetch` (only the changed model downloads) | minutes for one model |
| `prefetch_bert_models.py` | `hf-prefetch` (no downloads — cache holds all weights) + 10 GB local layer copy | ~1 minute |
| Crawler `pyproject.toml` deps | crawler builder pip (cached wheels) | seconds |
| Crawler source | runtime `COPY` only | seconds |
| Go source (`*.go`) | builder `go build` only (warm `$GOCACHE`) | seconds |
| Go deps (`go.mod`/`go.sum`) | `go mod download` + build, both warm | ~seconds |
| Base image digest bump (`make deps-refresh` step 1) | full rebuild, but caches still warm | ~minute |
| `docker compose build --no-cache` | everything, **caches ignored on purpose** | ~all of the above + downloads |

Everyday flow: `make worker-restart`, `make bff-restart`, `make ingestion-restart`, or `make crawl-probe0` rebuilds the corresponding image with caches warm. Use `--no-cache` only for the supply-chain refresh path (`make deps-refresh` step 4 already does this).

### Bumping a model revision

1. Edit the `model_revision:` field for the relevant entry under `services/analysis-worker/configs/language_capabilities.yaml` (Phase 119/120 — the manifest is the single source of truth for HuggingFace pins).
2. `make worker-restart`. Only the new revision downloads; everything else comes from the `aer-analysis-worker-hf-models` cache. The runtime stage's `COPY --from=hf-prefetch /hf-cache /hf-cache` re-copies the 10 GB tree locally, but no network traffic for the unchanged models.
3. Commit the manifest change.

No `make deps-refresh` is needed — model revisions are pinned in the manifest, not in `requirements.lock.txt`.

---

## Wikidata alias index — overview & cross-reference

The Phase 118 entity-linking step depends on a SQLite alias index, built once per quarter from a Wikidata RDF dump and shipped to the analysis-worker via the `aer-wikidata-index` Docker image + the `wikidata-index-init` compose service (worker mounts it read-only at `/data/wikidata/`).

**Architectural framing.** Phase 118 is *Disambiguation*, not Discovery — the index is a metadata sidecar over `aer_gold.entities`, not the canonical entity registry. Coverage is intentionally scoped to institutional editorial discourse; missing entities surface as string-keyed nodes through the BFF's LEFT JOIN, not as analysis failures. See **ADR-027** for the full framing and **WP-002 §4.2** for the methodological background.

**Build mechanism.** Streaming N-Triples parser over `latest-truthy.nt.bz2` via `pyoxigraph` (Phase 118b — superseded the original SPARQL pipeline after empirical evidence of public-endpoint timeouts on bucket-discovery queries). Architecture: Pass B (candidate hydration from local dump) + Pass C (sitelink hydration from SPARQL); each pass has its own resume cache so a multi-hour run survives transient network or process failure.

**Operational procedure → see [`wikidata_index_runbook.md`](./wikidata_index_runbook.md)**, which covers the full quarterly workflow end-to-end:

| Part | Content |
|---|---|
| Prerequisites | venv, dump source, hardware, GHCR auth |
| Part 1 — Production Build | `wikidata_validate.sh prod`, monitoring (RSS/scan%/ETA), resume after interruption |
| Part 2 — Validation | Canonical-entity spot-check, bucket-size sanity, determinism check |
| Part 3 — Deployment | GHCR PAT, `.env` config, build-context staging, image build/push, `.env` verify, stack restart, smoke-test |
| Part 4 — Rollback | Verlustfreier Tag-Flip via GHCR-immutable date-tags |
| Part 5 — Build Registry | Append-only ledger of past builds (date, hash, entity count) |
| Appendix | Common failure modes (Pass-C network outage, OOM, hash mismatch, parse errors) |

The runbook is the canonical local procedure and the GitHub Actions workflow's reference. The CI path (`.github/workflows/wikidata_index_rebuild.yml`) follows the same logical steps but runs on workflow-runner infrastructure.

### Quick-reference

* **Dump source:** `https://dumps.wikimedia.org/wikidatawiki/entities/latest-truthy.nt.bz2` (or the `your.org` mirror). Wikidata publishes a fresh truthy dump every Wednesday.
* **Snapshot-date semantics:** the dump file's mtime, recorded verbatim in the SQLite `build_metadata` table. Determines image tag and audit trail.
* **Determinism contract:** two builds over the same `(dump file, languages, bucket YAML, script version)` produce byte-identical SQLite — see `scripts/build/build_wikidata_index.py` docstring.
* **Hash verification:** the worker hashes the mounted file at startup and refuses to boot on mismatch with `WIKIDATA_INDEX_SHA256`. Leave empty only in dev.
* **Refresh cadence:** quarterly by default (workflow `schedule:` block). Manual `workflow_dispatch` for new-language additions and bucket-YAML changes.

### Bucket DSL — supported `match` keys

The `services/analysis-worker/data/wikidata_type_buckets.yaml` is the system of record for the index scope. Each bucket has a `match` block evaluated by the build script:

| Key | Semantics |
|---|---|
| `qid_any` | Subject QID is in this curated list (e.g., EU institutions). |
| `p31_any` | Entity has any of these as a `wdt:P31` value. |
| `p106_any` | Entity has any of these as a `wdt:P106` value. |
| `min_population` | Entity's `wdt:P1082` is ≥ value. |
| `min_sitelinks` *(top-level)* | Sitelink-count threshold, evaluated *after* Pass C SPARQL hydration. |

All set-valued clauses are AND-combined; empty clauses are no-ops. The Phase-118 `p31_subclass_of` (transitive P279 walk) was **removed** in the 118b post-mortem (ADR-027) — Wikidata's crowdsourced subclass graph is too unreliable on non-trivial subgraphs to drive bucket scope. Replaced by curated `p31_any` lists and (for hand-picked institutions) `qid_any`.

The `where_clause` field is reference-only SPARQL kept for human readers; it is not parsed at build time.

Adding a new domain is a one-PR change: append a YAML entry, run a workflow_dispatch rebuild, verify the new index size + hash, deploy via the tag-flip flow in the runbook. Append-only ordering is part of the determinism guarantee — re-ordering existing entries changes the build hash even if the alias set is unchanged.

---

## Editing the Language Capability Manifest

`services/analysis-worker/configs/language_capabilities.yaml` is the system-of-record for per-language analytical capability across the analysis worker (`NamedEntityExtractor`, `SentimentExtractor`) and the BFF API (`?language=` validator). Phase 118a (ADR-024) introduced it as a single source of truth; before then the per-language settings were scattered across module-level constants in the extractors.

### When to edit

* **Adding a language.** Append a new top-level entry under `languages:` and add the corresponding spaCy model to `services/analysis-worker/requirements.txt` (the BFF needs no extra dependency; it copies the manifest in at image build time). The minimum useful entry has just `iso_code` and `display_name` — the BFF validator will then accept `?language=<code>`. Add `ner` and/or `sentiment_tier1` blocks to wire up extraction.
* **Refining an existing block.** For sentiment, the `sentiment_tier1.features` flags (`negation_dependency`, `compound_split`, `custom_lexicon`) are validated against the extractor's known set — typos fail at startup. The `negation` sub-block lifts what used to be the module-level frozensets in `internal/extractors/sentiment.py`.
* **Declaring a Tier-2 / Tier-2.5 refinement.** Phase 119 introduces `sentiment_tier2_default` and `sentiment_tier2_refinement`. Today these stay unset; the manifest schema accepts them so the scaffold generator picks them up automatically when Phase 119 lands.

### Validation flow

1. Run `make scaffold-metric-validity` after every manifest edit. The script regenerates `infra/clickhouse/seed/metric_validity_scaffold_generated.sql` deterministically from the manifest.
2. Run `make test` (or at least `cd services/analysis-worker && ./.venv/bin/python -m pytest tests/test_phase118a_language_manifest.py tests/test_phase116_multilingual.py tests/test_phase117_sentiment.py`). The Phase 116/117 regression guards will refuse manifest edits that drift the German extractor outputs by more than the documented tolerance.
3. The CI drift gate is `make scaffold-metric-validity-check` (or equivalently `git diff --exit-code` after the regenerate step).
4. The worker's startup validates the manifest via Pydantic. Invalid YAML, an unknown top-level field, or an unsupported `manifest_version` produces a structured `ConfigurationError` and the container refuses to boot — never a silent fallback. The BFF carries the same fail-fast contract.

### `manifest_version` evolution policy

`manifest_version: 1` is the only currently-recognised value. Per ADR-024, any future schema-breaking change ships with:

* a new `manifest_version` integer (no minor versioning);
* a migration note in the ADR addendum;
* simultaneous worker + BFF reader updates (so neither component can be partially-upgraded against an incompatible manifest).

A worker (or BFF) reading a manifest with an unrecognised `manifest_version` refuses to start. This is the wedge that lets us evolve the schema safely without grandfather code paths.

---

## Production recovery & operations (non-wiping)

> On a production box, `make reset` is **refused** (SEC-034) — it wipes all data
> and drops TLS. The recoveries below are the non-destructive alternatives, plus
> the resource budget and credential-rotation procedures for a single-box deploy.

### Re-running a single init container (SEC-062)

Init containers (`minio-init`, `clickhouse-init`, `postgres-init-roles`,
`nats-init`, `wikidata-labels-load`) provision buckets, schema, roles, and
streams idempotently — so re-running one is safe and **does not wipe data**. When
a provisioning step needs to be re-applied (e.g. you rotated a credential, or one
flaked on a cold first boot), re-run just that container — never `make reset`:

```bash
# Re-run one init container against the live stack (idempotent):
docker compose up -d --no-deps --force-recreate minio-init
docker compose up -d --no-deps --force-recreate postgres-init-roles
docker compose up -d --no-deps --force-recreate clickhouse-init
# Watch its exit:
docker compose logs --tail=50 -f minio-init
```

Because each script is idempotent (`mb --ignore-existing`, `CREATE … IF NOT
EXISTS`, `CREATE ROLE … WHERE NOT EXISTS` + `ALTER ROLE`, `ilm import`), a re-run
reconciles the live state to the desired state without touching stored objects,
rows, or messages.

### Recovering a dirty Postgres migration (SEC-059)

The ingestion-api runs the Postgres schema migrations in-process at startup
(golang-migrate). If a migration fails mid-way, golang-migrate marks the version
**dirty** and every subsequent boot refuses to migrate (`Dirty database version
N`), so the ingestion-api crash-loops and the BFF — which needs the schema — is
affected downstream. Recover **without wiping**:

```bash
# 1. Inspect the dirty state (debug-up or on-box psql):
docker compose exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -c "SELECT * FROM schema_migrations;"   # version + dirty flag

# 2. Inspect what the failed migration left behind; finish or undo it BY HAND
#    so the schema matches the migration's intended end-state.

# 3. Clear the dirty flag so the runner re-attempts from the right version:
docker compose exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -c "UPDATE schema_migrations SET dirty = false WHERE version = <N>;"
#    If migration N must be re-run, also set version to N-1.

# 4. Restart ingestion-api so it re-runs the migration cleanly:
docker compose up -d --force-recreate ingestion-api
```

If the schema is too far gone to reconcile by hand, the safe path is **restore
from the last backup** (`docs/operations/backup_restore.md` §5), never a reset.

### Single-box memory budget (SEC-038)

The compose `deploy.resources.limits` sum to roughly **12–13 GB** of RAM across
all services (the worker dominates — BERT/BERTopic + a 1-year co-occurrence
window load into RAM). On the **16 GB** production box (Hetzner CX43) this fits
with headroom; the limits are caps, not reservations, and the worker is the only
heavy steady-state consumer. If you deploy on a smaller box, the OOM-killer will
reap whichever service spikes first (typically postgres/bff/minio while the
worker holds its model memory) — the `HostMemoryLow` alert (see
[monitoring.md](monitoring.md)) fires at <10% available as the early warning.
Do **not** raise the worker's limit above host RAM; scale OUT (more probes →
horizontal workers) rather than UP.

### Rotating a MinIO service-account secret (SEC-044)

`infra/minio/setup.sh` provisions service accounts with `mc admin user add`,
which is a **no-op if the user already exists** — so simply changing
`WORKER_MINIO_SECRET_KEY` in `.env` and re-running the init does **not** rotate
the secret (asymmetric with `postgres-init-roles`, which re-syncs via `ALTER
ROLE`). To actually rotate a MinIO service-account secret:

```bash
# Set the new secret explicitly on the existing user, then restart consumers:
docker compose exec -T minio sh -c '
  mc alias set m http://localhost:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"
  mc admin user svcacct edit --secret-key "<NEW_SECRET>" m <ACCESS_KEY>'   # or:
  # mc admin user remove m <ACCESS_KEY> && re-run minio-init with the new .env value
docker compose up -d --force-recreate analysis-worker bff-api ingestion-api
```

Update `.env` to the new secret in the same change so a later init re-run stays
consistent. (Root credentials `MINIO_ROOT_*` rotate by restarting the `minio`
server with the new value — they are read at server start, not stored.)

---

## Full system reset (one-shot)

A supervised, end-to-end reset of every state-bearing layer in the stack. Use it whenever the Gold layer carries mixed-vintage rows (different rows reflecting different extractor versions) or when a reproducibility gate has been tripped — the `aer_gold` Manifesto §III invariant is "every row reflects the same extractor fingerprint", and a one-shot reset is the canonical way to restore it. The reset wipes runtime data **only**; multi-GB build-time artefacts (BERT models, the Wikidata alias index, Tempo trace history) are preserved.

```bash
make reset                   # Stop → wipe runtime state → up → validate
```

Under the hood `make reset` runs three Make targets in order: `make reset-state` (the wipe), `make up` (recreate via init containers), and `make reset-validate` (per-layer invariant check). Either the meta-target or its constituents can be run individually.

**Wiped — runtime state, recreated by init containers on next `make up`:**

| Volume | Layer | Recreated by |
| :--- | :--- | :--- |
| `aer_postgres_data` | sources, documents, ingestion jobs | `infra/postgres/migrations/` (000001+) |
| `aer_minio_data` | Bronze, Silver, DLQ buckets | `minio-init` (`infra/minio/setup.sh`) |
| `aer_clickhouse_data` | every Gold + Silver projection table | `clickhouse-init` (`infra/clickhouse/migrations/`, 000001 → latest) |

*(The web crawler keeps no dedup volume — conditional-GET state lives in the Postgres `crawler_state` table inside `aer_postgres_data`, so it is wiped with that volume.)*

**Preserved — build-time artefacts, NEVER wiped:**

| Volume | Why preserved |
| :--- | :--- |
| `aer_wikidata_data` | Wikidata alias index built by `scripts/build/build_wikidata_index.py` from `latest-truthy.nt.bz2`. ~30-min rebuild. |
| `aer_tempo_data` | Distributed trace history. Re-creating destroys debugging context for prior incidents. |

`scripts/operations/clean_infra.sh` asserts the preserved set survives the wipe and aborts loudly if any of those volumes is missing afterward — so an accidental `docker volume prune` between two `make reset` invocations cannot silently lose the Wikidata index or trace history.

> **BERT models are NOT in a volume.** The Phase 119 BERT sentiment models and Phase 120 BERTopic embedding model are baked into the worker image at `/hf-cache` and the worker runs with `TRANSFORMERS_OFFLINE=1` — the image is the canonical store. `make reset` does not touch model state because it does not need to: the running container points at the image's `/hf-cache`. Models change only when the worker image is rebuilt (manifest revision rotation → `docker compose build analysis-worker`). Phase-120b removed the previously-documented `aer_hf_cache` volume from the preserved set because it was never actually mounted on the worker — a documentation/wiring drift surfaced during this phase.

### When to run

Run `make reset` when **any** of the following is true:

- A schema migration shipped that altered an existing Gold table's row layout (rename, computation change, semantics shift). Backfill is harder than reset for a pre-deployment POC.
- A new extractor was registered that emits rows tagged with a version string different from the prior extractor — the result is mixed-vintage Gold.
- The Wikidata index, `model_hash`, or any other reproducibility anchor has rotated, and the prior rows no longer correspond to the current extractor fingerprint.
- A targeted infra-layer wipe (`make infra-clean-postgres` etc.) is not enough because a downstream layer references the wiped layer's IDs.

Do **not** run `make reset` to clear a transient "looks weird" state — read the validator output first; the per-layer `✗` is usually more informative than a wipe.

### Operator workflow

```bash
# Pre-flight: confirm no in-flight ingestion. The validator does not
# protect you from racing the wipe against an active crawl.
make logs | grep -E "Processing event|ingest" | tail -5

# Optional: dump out-of-band scientific records (Phase 115; empty by
# design today, but verify before wiping).
docker compose exec postgres psql -U "$POSTGRES_USER" -d aer -c "
  SELECT 'equivalence_reviews' AS t, count(*) FROM equivalence_reviews;"

# Run the supervised reset.
make reset

# Fresh crawl. Every Gold row from this point forward carries the
# canonical extractor fingerprint.
make crawl
```

Validator output (`scripts/operations/reset_validate.sh`) prints `✔` / `·` / `✗` per layer:

| Symbol | Meaning |
| :--- | :--- |
| `✔` | Layer passed the invariant. |
| `·` | Layer skipped — service not running (expected for partial-up scenarios). |
| `✗` | Layer failed. The reset does **not** proceed to a healthy state — investigate the named layer before retrying. |

### Non-interactive runs

```bash
AER_RESET_NONINTERACTIVE=1 make reset
```

`make reset` is interactive by default (it prompts before destroying data). Set `AER_RESET_NONINTERACTIVE=1` for CI-style or scripted runs — the prompt is skipped, the wipe proceeds. Required when running `make reset` from a non-TTY environment (a Cron job, a remote-trigger script, etc.).

### Targeted escape hatches (one-layer wipes)

When the full reset is overkill — a single Gold table is corrupted but the surrounding state is fine — use the per-layer wipes:

```bash
make infra-clean-postgres    # Postgres only — sources, documents, ingestion jobs
make infra-clean-minio       # MinIO only — Bronze, Silver, DLQ buckets
make infra-clean-clickhouse  # ClickHouse only — every Gold + Silver projection
make infra-clean             # All three above, plus the crawler dedup volume
```

These also require interactive confirmation. They do **not** run the post-wipe validator — `make reset` is the canonical path when you want layer-by-layer assurance that the recreated state is clean.

### Common pitfall — image rebuilds that re-download models

`make reset` never re-downloads BERT models because it never touches the worker image. **`docker compose build analysis-worker` does**, and any edit to `services/analysis-worker/configs/language_capabilities.yaml` or `services/analysis-worker/requirements.lock.txt` invalidates the prefetch layer's cache key, forcing a re-download of the multilingual-XLM-R, German-news-BERT, and multilingual-E5-large model weights (~10 GB total). On a fast connection that is ~10 minutes; on a slow one it can be over an hour. The Dockerfile's BuildKit cache mount (`id=aer-analysis-worker-hf-models`) can short-circuit the redownload if the cache is warm, but Docker's cache is not free-tier-permanent — `docker buildx prune` and Docker Desktop disk-pressure eviction can wipe it without warning. If you find yourself re-downloading often, audit which of the prefetch-layer inputs you keep editing and consider whether the change really has to invalidate the prefetch.

---

## Volume Management

```bash
make infra-clean             # Wipe ALL persistent volumes (with confirmation)
make infra-clean-postgres    # Wipe PostgreSQL only
make infra-clean-minio       # Wipe MinIO only (Bronze, Silver, DLQ data)
make infra-clean-clickhouse  # Wipe ClickHouse only (Gold metrics)
```

All wipe commands require interactive confirmation. After wiping, the next `make up` will re-run init containers and migrations automatically. For the supervised end-to-end path that also re-runs init containers and validates the result, see [Full system reset (one-shot)](#full-system-reset-one-shot).

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

## Routine Operations (`make` targets)

The contract introduced by Phase 120c: every operation a developer is expected to perform routinely has a `Makefile` target. Anything not on this list is either a build-time concern (codegen, lint, test) or a one-shot recorded in [One-shot Operations](#one-shot-operations) below. Running a raw `python scripts/...` invocation in steady state is a deliberate, documented exception, not the default workflow.

| Target | Purpose |
| :--- | :--- |
| `make up` / `make down` / `make restart` | Bring the full stack (infra + services + dashboard) up, down, or rebounce it. |
| `make backend-up` / `make backend-down` / `make backend-rebuild` | Backend without the dashboard container — pair with `make fe-dev`. |
| `make infra-up` / `make infra-down` | Start or stop only the infrastructure layer (DBs, NATS, MinIO, observability). |
| `make services-up` / `make services-down` / `make services-restart` | Manage all application services together. |
| `make services-clean` | Stop services and wipe their workspace state (`scripts/operations/clean.sh`). |
| `make ingestion-up\|down\|restart`, `make worker-up\|down\|restart`, `make bff-up\|down\|restart` | Per-service control. |
| `make debug-up` / `make debug-down` | Expose internal infra ports to localhost for tooling (psql, mc, curl). |
| `make logs` | Tail the live container logs across the stack. |
| `make crawl` | Run the web crawler (`crawl-probe0` + `crawl-probe1`) as one-shot containers on `aer-backend`. |
| `make crawl-probe<id>` | Crawl a single probe. Conditional-GET dedup state lives in the Postgres `crawler_state` table; `make reset` is the canonical wipe (there is no separate `crawl-reset` target). |
| `make reset` / `make reset-state` / `make reset-validate` | Phase-120b supervised reset: wipe runtime state volumes, re-up via init containers, validate. The canonical recovery path. |
| `make infra-clean[-postgres\|-minio\|-clickhouse]` | One-layer wipe (interactive confirmation). Prefer `make reset` for full resets. |
| `make build-services` | Compile Go binaries into `./bin/`. |
| `make codegen` | Regenerate Go types/stubs from the OpenAPI contracts. |
| `make codegen-ts` / `make fe-codegen` | Regenerate the dashboard's TypeScript API types from the BFF spec. |
| `make openapi-bundle` / `make openapi-lint` | Bundle modular OpenAPI specs / enforce ADR-021 `$ref` style. |
| `make scaffold-metric-validity` / `make scaffold-metric-validity-check` | Regenerate the per-language `metric_validity` seed scaffold from the Capability Manifest. |
| `make tidy` | `go mod tidy` across all Go modules. |
| `make lint` / `make lint-go-pkg` | Run all linters (golangci-lint, ruff). |
| `make audit` / `make audit-go` / `make audit-python` | Run vulnerability scans. |
| `make test` / `make test-go` / `make test-go-pkg` / `make test-go-crawlers` / `make test-python` / `make test-e2e` | Run the test suites. |
| `make fe-install` / `make fe-dev` / `make fe-build` / `make fe-test` / `make fe-test-e2e` / `make fe-check` | Frontend developer loop and gates. |
| `make fe-image-build` / `make fe-image-size` | Build and budget-check the dashboard container image. |
| `make deps-refresh` | Maintainer-only: rotate the entire pinned supply-chain baseline (base image digests, `requirements.lock.txt`, `SENTIWS_SHA256`). |
| `make swagger-up` / `make swagger-down` | Bundle OpenAPI specs and start Swagger UI on `:8089`. |
| `make setup` | Install the developer tooling pinned in `.tool-versions`. |
| `make help` | Print a self-documenting menu of the above. |

A fresh contributor should be able to onboard with this section plus `make help` alone.

---

## One-shot Operations

Scripts under `scripts/operations/` are operator-invokable one-shots. They have an explicit "when to run / when not to run" contract — running one in steady state is incorrect, not just unnecessary.

### `scripts/operations/compute_baselines.py`

**When to run:** once after a fresh `make reset && make crawl` if you do not want to wait 24 h for the in-worker `MetricBaselineExtractor` daily loop (Phase 115) to populate `aer_gold.metric_baselines`. Also: as the canonical example in [Workflow 4 of the Scientific Operations Guide](scientific_operations_guide.md#workflow-4-computing-and-updating-baselines), and as the explicit first-baseline-run step on a new probe (Phase 124 worked example).

**When NOT to run:** in steady state. The `MetricBaselineExtractor` corpus loop inside the analysis worker is the canonical source. Running the standalone script while the loop is also active is harmless (`ReplacingMergeTree(compute_date)` collapses duplicate keys), but it is a sign that the loop is not actually doing what it is supposed to be doing — investigate that first.

Both call paths share the canonical computation in `internal.extractors.metric_baseline.compute_baseline_rows`, so byte-for-byte identical baselines are produced for the same input window. See [Metric Baselines & Equivalence](#metric-baselines-equivalence-wp-004) above for the full procedure.

### `scripts/operations/clean.sh`, `scripts/operations/clean_infra.sh`, `scripts/operations/reset_validate.sh`

These are not invoked directly by operators in steady state — they are wrapped by `make services-clean`, `make infra-clean[-postgres|-minio|-clickhouse]`, `make reset`, and `make reset-validate` respectively. They are listed here for greppability: a `scripts/operations/` path under a Makefile target's command body is the sanctioned shape; a raw `./scripts/operations/<x>.sh` invocation in a runbook is a smell.

---

## Deleted scripts — for the historical record

Phase 120c retired six scripts that targeted data scenarios the supervised wipe-and-recrawl path (`make reset`, Phase 120b) now supersedes. Listed here so a future contributor can find them in `git log` without re-deriving why they are gone.

| Script | Removed | Rationale |
| :--- | :--- | :--- |
| `scripts/backfill_article_id.py` | Phase 120c | Patched pre-Phase-43 documents lacking an `article_id` — those rows no longer exist after the Phase 120b reset. |
| `scripts/backfill_silver_projection.py` | Phase 120c | Backfilled `aer_silver.documents` from historical Silver objects — superseded by the canonical Bronze→Silver pipeline running over a freshly recrawled corpus. |
| `scripts/backfill_entity_links.py` | Phase 120c | Quarterly post-rebuild Wikidata link backfill against historical `aer_gold.entities` — pre-Phase-118 entity rows no longer exist; the current pipeline links at write time. |
| `scripts/backfill_bert_sentiment.py` | Phase 120c | Generated Phase-119 BERT sentiment metrics on Silver envelopes that pre-dated the Phase-119 worker image — every Silver envelope now post-dates that image. |
| `scripts/reconcile_documents.py` | Phase 120c | One-shot `aer_silver.documents.bronze_object_key` repair from historical Bronze data (Phase 113b). The wipe-and-recrawl pattern is the canonical recovery route under ADR-022. |
| `scripts/replay_bronze.py` | Phase 120c | Mid-iteration NATS replay over Bronze to rebuild Silver/Gold without re-crawling. `make reset` followed by `make crawl` is now the canonical replacement and is faster end-to-end on the working dataset. |

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
## Interpreting Negative-Space density (Phase 122d.2 / ADR-039)

Negative Space discloses what AĒR does not/cannot see. Operationally, two per-source signals are worth watching (both reflexive disclosures, never source defects):

- **Temporal-Provenance-Absence share** (`temporalProvenanceAbsentCount / articlesInWindow`, on the Dossier source card). A high or rising share means many of that source's articles have no real publication date (`timestamp_source='fetch_at_fallback'`) — usually a discovery/extraction gap (the publisher's date metadata is not being parsed). Investigate the source's `discovery:` block + the worker's `WebAdapter` date extraction, not the data. Per WP-005 §3.1 a publication gap is an observation gap, not a discourse gap.
- **Silent-Edit density** — articles with post-publication headline/content changes (the ∅ SE badge). A spike is a candidate for editorial review, not an error.

A source moving from `structurallyAbsent=false → true` on a Tier-B field is itself a measurement (WP-006 §4.3 interpretive versioning) — note it.

The Negative-Space toggle (`?negSpace=1`, SideRail / Shift-N) is self-disclosing per cell; it never coerces an absent value to zero. The globe deliberately makes **no** geographic coverage claim (a source's discourse reach is unmeasurable).
