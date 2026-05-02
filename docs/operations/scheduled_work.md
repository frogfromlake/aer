# Scheduled Work Inventory

This document is the system-of-record for every script and every scheduled workload in the AĒR project. It exists to prevent the operational opacity that emerges when scheduled work accumulates organically without a central inventory.

The project distinguishes **three categories of scripts** plus a fourth category of in-process scheduled work (NATS-cron-style loops inside the analysis worker). Knowing which category a workload belongs to is the single most important architectural decision — it determines where the code lives, how it runs, and who is responsible for keeping it green.

---

## How to read this document

Every entry below has four invariants:

| Field | Meaning |
| :--- | :--- |
| **Purpose** | What the script computes or produces. |
| **Trigger** | What invokes it: `make` target, Git hook, GitHub Actions, NATS-cron loop, manual operator. |
| **Cadence** | How often it runs in production. `On-demand` means human-triggered, no schedule. |
| **Phase introduced** | The ROADMAP phase that introduced the workload, for archaeology. |

Whenever a new script is added, it is registered here at the time of introduction — not after. This is enforced by a checklist item in every ROADMAP phase that touches `scripts/` or adds a corpus extractor (see [Maintenance discipline](#maintenance-discipline) below).

---

## The four categories

### Category A — Build / CI hygiene

Scripts that run on developer machines or in CI to keep the repository in a consistent state. They never run on a schedule and never run in production.

**Examples:** OpenAPI bundling, lint gates, scaffold generators, Git hooks.

**Where they live:** `scripts/`, invoked via `make` targets and CI workflow steps.

**Failure mode if neglected:** generated artefacts drift from their sources, lint passes but contract is broken, CI catches it on next push.

### Category B — One-shot / Ad-hoc operations

Scripts that an operator runs manually for specific events — first-run on a new probe, historical backfill, debugging investigation, schema-migration recompute.

**Examples:** Smoke test, baseline first-run on new probe, one-time historical reconciliation.

**Where they live:** `scripts/`, invoked from a documented procedure in `operations_playbook.md` or `scientific_operations_guide.md`.

**Failure mode if neglected:** the operator forgets to run them at the right moment; downstream data has gaps. Mitigated by phase exit-criteria checklists.

### Category C — Periodic scheduled operations

Standalone scripts that must run on a regular calendar — typically because they are resource-intensive, externally-bound (third-party APIs), or produce build artefacts that downstream services consume.

**Examples:** Quarterly Wikidata alias-index rebuild.

**Where they live:** `scripts/`, scheduled via GitHub Actions workflows in `.github/workflows/`.

**Failure mode if neglected:** the schedule trigger fails silently and downstream artefacts go stale. Mitigated by GitHub Actions failure notifications and the artefact hash verification at consumer startup (e.g., the analysis-worker fails fast if the Wikidata index hash does not match its expected hash).

### In-worker scheduled work (NATS-cron pattern)

Corpus-level computations that run inside the `analysis-worker` container as long-lived `asyncio` loops alongside the per-document NATS consumer. They are NOT standalone scripts — they are extractors registered in `services/analysis-worker/main.py` and configured via environment variables.

**Examples:** Entity co-occurrence sweeps, baseline computation, BERTopic re-fits.

**Where they live:** `services/analysis-worker/internal/extractors/<extractor>.py` plus a `*_extraction_loop` task in `main.py`.

**Failure mode if neglected:** the loop crashes, but Docker healthcheck and structured logs catch it; the missed window is recoverable on next sweep via `ReplacingMergeTree` idempotency.

---

## Decision matrix: where does new scheduled work go?

When introducing a new workload that needs to run on a schedule, choose its category by these rules in order:

| If the work... | ...then it goes in |
| :--- | :--- |
| reads from Silver/Gold, writes to Gold, idempotent via `ingestion_version`, fits the analysis-worker resource envelope | **In-worker NATS-cron extractor** (Phase 102 / Phase 115 pattern) |
| produces a build artefact (Docker image, large binary file), is resource-intensive (>30 min runtime), or depends on external rate-limited APIs | **Category C standalone script + GitHub Actions schedule** |
| transforms a manifest/spec into committed code, runs only on developer/CI machine, is idempotent given identical inputs | **Category A build script in `make codegen`** |
| is a one-shot historical backfill, migration recompute, or debugging tool for an operator | **Category B ad-hoc script with documented procedure** |

The default is the **in-worker NATS-cron** pattern wherever possible. Standalone scripts are an escape hatch for cases where the in-worker pattern does not fit, not the default.

---

## Inventory

### Category A — Build / CI hygiene

| Script | Purpose | Trigger | Cadence | Phase |
| :--- | :--- | :--- | :--- | :--- |
| `scripts/openapi_bundle.py` | Bundle modular OpenAPI specs to single-file artefact for Swagger UI and TS codegen | `make openapi-bundle`, CI | On every spec edit | 96 |
| `scripts/openapi_ref_style_check.sh` | Enforce ADR-021 two-style `$ref` convention | `make openapi-lint`, CI | Every commit | 96 |
| `scripts/clean.sh` | Remove `.pids` and `__pycache__` directories | `make services-clean`, manual | On-demand | early |
| `scripts/clean_infra.sh` | Wipe MinIO / Postgres / ClickHouse persistent volumes (with confirmation) | `make infra-clean[-postgres\|-minio\|-clickhouse]`, manual | On-demand | early |
| `scripts/deps_refresh.sh` | Update pinned external assets (SentiWS lexicon hash, etc.) and rebuild | Manual | On-demand (when an upstream lexicon updates) | 119? |
| `scripts/hooks/pre-commit` | Run `make lint` before commit | Git hook | Every commit | 50 |
| `scripts/hooks/pre-push` | Run `make lint && make audit && make test` before push | Git hook | Every push | 50 |
| `scripts/generate_metric_validity_scaffold.py` *(planned)* | Auto-generate `aer_gold.metric_validity` scaffold from Capability Manifest | `make scaffold-metric-validity`, CI drift gate | On every manifest edit | 118a |

### Category B — One-shot / Ad-hoc operations

| Script | Purpose | Trigger | Cadence | Phase |
| :--- | :--- | :--- | :--- | :--- |
| `scripts/e2e_smoke_test.sh` | Full Bronze→Silver→Gold pipeline smoke test | `make test-e2e`, CI on `main` push | Every main push | early |
| `scripts/compute_baselines.py` | Compute per-source z-score baselines on demand (ad-hoc counterpart to the in-worker `MetricBaselineExtractor`) | Manual, called from operations procedures | On-demand: new probe first-run, schema migration recompute, Workflow 4 walkthroughs | 115 |
| `scripts/reconcile_documents.py` | One-shot backfill of `aer_silver.documents.bronze_object_key` from historical Bronze data | Manual, executed once | One-shot (already executed during Phase 113b) | 113b |

### Category C — Periodic scheduled operations

| Script | Purpose | Trigger | Cadence | Phase |
| :--- | :--- | :--- | :--- | :--- |
| `scripts/build_wikidata_index.py` | Build Wikidata alias SQLite index from a streaming N-Triples parse over `latest-truthy.nt.bz2` (Phase 118b dump-stream pipeline; superseded the Phase 118 SPARQL path), package into `aer-wikidata-index` Docker image | `.github/workflows/wikidata_index_rebuild.yml` (manual dispatch through the Phase 118b validation cycle; quarterly schedule activated in a separate post-merge commit) | Quarterly (1st of Jan/Apr/Jul/Oct, 02:00 UTC) plus on-demand for new-language additions. Wikidata publishes a fresh truthy dump every Wednesday; quarterly cadence balances freshness against build cost. Refresh procedure: `docs/operations/operations_playbook.md#building-and-refreshing-the-wikidata-alias-index`. | 118 / 118b |

### In-worker scheduled extractors (NATS-cron pattern)

| Extractor | Purpose | Configuration | Cadence | Phase |
| :--- | :--- | :--- | :--- | :--- |
| `EntityCoOccurrenceExtractor` | Build entity co-occurrence networks from a rolling Silver window | `CORPUS_EXTRACTION_*` env vars | Weekly (default `604800` seconds) | 102 |
| `MetricBaselineExtractor` | Compute per-source z-score baselines on a 90-day rolling window | `BASELINE_EXTRACTION_*` env vars (`ENABLED`, `INTERVAL_SECONDS=86400`, `WINDOW_SECONDS=7776000`, `INITIAL_DELAY_SECONDS=300`) | Daily | 115 |
| `TopicModelingExtractor` *(planned)* | BERTopic per-language topic discovery on rolling 30-day window | TBD per Phase 120 implementation | Weekly | 120 |

The in-worker extractors live in `services/analysis-worker/internal/extractors/` and are wired into `services/analysis-worker/main.py` as concurrent `asyncio` tasks alongside the per-document NATS consumer. Each extractor is independently togglable via its `*_ENABLED` env variable. All extractors use `ReplacingMergeTree(ingestion_version)` for idempotency — re-runs of the same window produce a new version that supersedes the previous one.

---

## Cron mechanism for Category C — GitHub Actions

The architectural decision (2026-05-02) is to use **GitHub Actions scheduled workflows** for all Category C standalone scripts. Rationale:

- **No new infrastructure.** GitHub Actions is already used for CI; adding a scheduled workflow adds zero operational complexity.
- **Cost.** Free for public repositories, generous limits for private.
- **Notification.** Failed runs are surfaced in the GitHub UI and can email the maintainer; a manual cron container would silently fail.
- **Provenance.** Every run produces a logged record with timestamps, exit codes, and artefact hashes — auditable from the GitHub UI.
- **Solo-developer realism.** No host-level cron, no systemd timer, no extra container in the Compose stack. The schedule survives across machine moves, rebuilds, and re-deployments.

The reference workflow is `.github/workflows/wikidata_index_rebuild.yml`. It defines:

1. **`workflow_dispatch`** trigger — for manual/on-demand rebuilds (e.g., when a new probe adds a language and the alias index needs to extend coverage).
2. **`schedule`** trigger — quarterly cron (`0 2 1 1,4,7,10 *`), commented out until the Phase 118 build script is verified end-to-end.
3. A single job that runs the build script, verifies the output hash, and (when Phase 118's image distribution work is in place) builds and pushes the `aer-wikidata-index` Docker image to GHCR.

When a new Category C workload is introduced, mirror this workflow pattern: create a dedicated `.github/workflows/<workload>_<verb>.yml`, with `workflow_dispatch` for manual runs and `schedule` for the regular cadence. Do not pile multiple unrelated cron jobs into one workflow file.

---

## Maintenance discipline

To prevent this inventory from drifting out of sync with reality, every ROADMAP phase that introduces a new script or a new corpus extractor includes the following exit-criteria bullet:

> **Scheduled-work registration.** Append the new workload to `docs/operations/scheduled_work.md` under the appropriate category, with Purpose, Trigger, Cadence, and Phase fields populated. If the workload is Category C, the corresponding `.github/workflows/<workload>.yml` is committed in the same phase.

Phase 128 (Documentation Sweep) explicitly verifies this discipline holds: the inventory is reconciled against the actual `scripts/` directory, `services/analysis-worker/main.py` extractor loops, and `.github/workflows/` files. Drift is treated as a documentation bug.

---

## Cross-references

- [Operations Playbook → Automated baseline maintenance](operations_playbook.md) — the canonical example of an in-worker NATS-cron extractor.
- [Scientific Operations Guide → Workflow 4: Computing and Updating Baselines](scientific_operations_guide.md) — the operator's view of the ad-hoc baseline script.
- `services/analysis-worker/main.py` — the concrete wiring of `corpus_extraction_loop` and `baseline_extraction_loop`.
- `.github/workflows/ci.yml` — the existing CI workflow; new scheduled workflows live alongside it in `.github/workflows/`.
- `.github/workflows/wikidata_index_rebuild.yml` — the reference Category C workflow.
- [ROADMAP Phase 118](https://github.com/frogfromlake/aer/blob/main/ROADMAP.md) — the planned Wikidata index build (Category C).
- [ROADMAP Phase 118a](https://github.com/frogfromlake/aer/blob/main/ROADMAP.md) — the planned manifest scaffold generator (Category A).
- [ROADMAP Phase 120](https://github.com/frogfromlake/aer/blob/main/ROADMAP.md) — the planned BERTopic in-worker extractor.