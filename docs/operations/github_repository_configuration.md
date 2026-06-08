# GitHub Repository Configuration — Secrets & Variables

*Living ledger of what must be configured under **Settings → Secrets and variables → Actions** for CI and (deferred) deployment to work. Finalised in Phase 129 (Documentation Sweep); kept current as phases add credentials.*

The CI that runs on every push/PR (`ci.yml`: python/go/security pipelines) uses **Testcontainers with ephemeral, internal credentials** and needs **no repository secrets**. Secrets are consumed only by the **nightly end-to-end smoke** (`e2e_smoke_nightly.yml`), which boots the full Compose stack, and (later) by deployment.

`GITHUB_TOKEN` is provided automatically by Actions (used by the GHCR image-build workflows) — you do not create it.

---

## A. GitHub Actions **Secrets** — consumed by CI today (nightly e2e smoke)

`e2e_smoke_nightly.yml` copies `.env.example` → `.env` and overrides these from secrets. Non-secret values (`POSTGRES_USER`, `POSTGRES_DB`, `CLICKHOUSE_USER`, `BFF_DB_USER`, `BFF_AUTH_DB_USER`, `GF_SECURITY_ADMIN_USER`) come from the `.env.example` defaults and are **not** secrets.

| Secret | Purpose |
|--------|---------|
| `BFF_API_KEY` | BFF machine API key (ADR-040: now the machine credential). |
| `INGESTION_API_KEY` | Ingestion API key. |
| `CLICKHOUSE_PASSWORD` | ClickHouse password. |
| `POSTGRES_PASSWORD` | Postgres superuser password (also synthesised into `DB_URL`). |
| `BFF_DB_PASSWORD` | `bff_readonly` role password (analytics read pool). |
| **`BFF_AUTH_DB_PASSWORD`** | **NEW — Phase 134 / ADR-040.** `bff_auth` role password (auth write pool). `postgres-init-roles` provisions the role with it and the BFF connects with it. |
| `MINIO_ROOT_USER` / `MINIO_ROOT_PASSWORD` | MinIO root (setup only). |
| `INGESTION_MINIO_ACCESS_KEY` / `INGESTION_MINIO_SECRET_KEY` | Ingestion MinIO service account. |
| `WORKER_MINIO_ACCESS_KEY` / `WORKER_MINIO_SECRET_KEY` | Worker MinIO service account. |
| `BFF_MINIO_ACCESS_KEY` / `BFF_MINIO_SECRET_KEY` | BFF MinIO service account. |
| `GF_SECURITY_ADMIN_PASSWORD` | Grafana admin password. |

**Phase 134 delta: add exactly one new secret — `BFF_AUTH_DB_PASSWORD`** (generate with `openssl rand -base64 32`). Nothing else changes for CI.

---

## B. Secrets — deployment only (deferred)

When AĒR is deployed (LICENSE §4c gates this behind Phase 134), the production `.env` needs the full runtime secret set above **plus**:

| Secret | Purpose |
|--------|---------|
| `ACME_EMAIL` | Let's Encrypt registration (compose.prod). |

The non-secret auth tuning (`BFF_SECURE_COOKIES`, `BFF_SESSION_*`, `BFF_ARGON2_*`, `BFF_*_TTL_SECONDS`) ships with production-safe defaults and need not be set unless overriding.

---

## C. **Variables** (non-secret) — deployment only (deferred)

No repository **Variables** are used today. At deployment these low-sensitivity values are configured (as Variables or deployment env), not as secrets:

| Variable | Purpose |
|----------|---------|
| `BFF_PUBLIC_BASE_URL` | Origin for invite / reset links (e.g. `https://aer.example`). Phase 134. |
| `ADMIN_BOOTSTRAP_EMAIL` | First admin's email for `make create-admin`. Phase 134. |
| `GRAFANA_HOST` / `MINIO_CONSOLE_HOST` | Traefik host routing (compose.prod). |

---

## Summary of the Phase 134 (Auth-1) GitHub delta

- **One new Actions Secret to create now: `BFF_AUTH_DB_PASSWORD`** (so the nightly e2e smoke keeps booting the stack).
- **Two deployment-time Variables** introduced (not needed until deploy): `BFF_PUBLIC_BASE_URL`, `ADMIN_BOOTSTRAP_EMAIL`.
- Everything else auth-related is non-secret config with safe defaults.
