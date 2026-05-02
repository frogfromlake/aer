# Developer Quickstart

> A short reference for the everyday local development experience.
> For the full operational reference, see the [Operations Playbook](operations_playbook.md).

---

## First-time setup

```bash
cp .env.example .env                       # fill in REPLACE-ME values
cp scripts/hooks/pre-commit .git/hooks/    # mandatory: lint on commit
cp scripts/hooks/pre-push   .git/hooks/    # mandatory: lint + test on push
chmod +x .git/hooks/pre-*

make fe-install                            # pnpm install for the dashboard
```

Required tooling versions live in `.tool-versions` (Go, Python, pnpm,
golangci-lint, oapi-codegen, …). Use `asdf install` or match by hand.

---

## The two daily loops

### A. Full container stack (default)

```bash
make up        # infra + services + dashboard, gated by healthchecks
open https://localhost/
make down
```

Use this to verify the production build behaves like CI.

### B. Frontend hot-reload + containerized backend

```bash
make backend-up    # full stack minus the dashboard container
make fe-dev        # SvelteKit on http://localhost:5173 (proxies /api → Traefik)
```

Vite owns `/`, Traefik injects `X-API-Key` server-side via the `bff-api-key`
middleware — the browser bundle ships zero secrets.

Switch back to Loop A any time with `make frontend-up`.

### Feeding real data into either loop

Both loops come up with empty bronze/silver/gold — the atmosphere has
nothing to display until you ingest something. Kick off a one-shot crawl:

```bash
make crawl          # runs rss-crawler as a one-shot container on aer-backend
make crawl-reset    # wipes dedup state when backend volumes were cleared
```

`make crawl` builds and runs the crawler under Compose profile `crawlers` — no
`debug-up` required, identical command in dev and prod. The dedup state lives
in the `rss_crawler_state` named volume. `make infra-clean`, `-postgres`, and
`-minio` wipe it automatically (bronze / documents are gone, so dedup must go
too). If you wipe volumes any other way (raw `docker compose down -v`,
`docker volume rm`), run `make crawl-reset` manually — otherwise the next
crawl skips every item as "already seen" and silently ingests nothing.

---

## Implementing a change — order of operations

The pre-push hook (`make lint && make test`) catches most things, but a few
artifacts must be regenerated **by hand** before you push or CI will fail.
Always do them in this order, *before* committing.

### Common feature shapes (jump table)

| You're building…                          | Sections to visit |
| :---------------------------------------- | :---------------- |
| New BFF endpoint surfaced in the dashboard | §1 → §2 → §4 → §8 |
| New BFF endpoint, no dashboard change      | §1 → §2 → §8      |
| New metric/entity extractor (analysis-worker) | §3 → §7 (if new ClickHouse column) → §8 |
| New data source (crawler + adapter)        | §2 → §3 → §7 → §8 |
| Dashboard-only change (story, copy, layout)| §4 → §8           |
| Dependency bump or base-image rotation     | §6 → §8           |
| Schema-only change (migration, no code)    | §7 → §8           |


### 1. If you changed an OpenAPI spec (`services/*/api/**`)

```bash
make openapi-lint          # enforce two-style $ref convention
make codegen               # Go server stubs (BFF) — commit generated.go
make fe-codegen            # TS client types → src/lib/api/types.ts
make openapi-bundle        # refresh single-file bundles (Swagger UI)
```

CI fails on drift for `make codegen` and `make fe-codegen` — the generated
files MUST be committed.

### 2. If you changed Go code

```bash
make lint                  # golangci-lint across all Go modules
make test-go               # Testcontainers integration tests (needs Docker)
```

If you added a new env var or required secret: update `.env.example` AND the
boot-secret table in `docs/operations_playbook.md`.

### 3. If you changed Python code (analysis-worker)

```bash
make lint                  # ruff
make test-python           # pytest
```

If you added a Python dep: update `requirements.txt`, then `make deps-refresh`
to regenerate the lockfile + base-image digest baseline.

### 4. If you changed dashboard source (`services/dashboard/src/**`)

```bash
make fe-lint               # ESLint + Prettier + svelte-check
make fe-typecheck          # strict TypeScript
make fe-test               # Vitest unit
```

```bash
make fe-lint-fix           # optional: Auto-fix ESLint + Prettier issues (no svelte-check auto-fix)
```

**If the change is visible on screen** (markup, styles, story content, copy):

```bash
make fe-test-e2e           # Playwright visual + a11y in pinned image
```

If the diff is *intentional* (you meant to change what's rendered):

```bash
make fe-test-e2e-update    # regenerate visual baselines
git add services/dashboard/tests/e2e/__snapshots__/
```

> **This is the trap that bit Phase 99b** — the page changed, the baseline
> didn't, CI failed. Always run `fe-test-e2e-update` after intentional UI
> changes, and commit the new PNGs in the same commit as the source change.

If the diff is *unintentional*, fix the regression — don't update the baseline.

### 5. If you changed the dashboard `Dockerfile` or deps

```bash
make fe-image-build        # builds + checks 50 MB budget
```

### 6. If you added or changed any `Dockerfile` or pinned base image

```bash
make deps-refresh          # rotates digests, lockfiles, SENTIWS_SHA256
```

See `docs/operations_playbook.md#dependency-refresh-supply-chain-baseline` for
the runbook.

### 7. If you added a database migration

- PostgreSQL: drop a new file in `infra/postgres/migrations/` (`0000NN_name.up.sql` + `.down.sql`). Runs automatically on `ingestion-api` startup.
- ClickHouse: drop a new file in `infra/clickhouse/migrations/`. Runs via the `clickhouse-init` container.

Validate end-to-end:

```bash
make down && make up       # fresh boot, migrations apply
make test-e2e              # full pipeline smoke test
```

### 8. Final gate before push

```bash
make fe-check              # composite frontend gate (lint + typecheck + test + build + bundle-size)
make test                  # backend full suite (Go integration + Python unit)
make test-e2e              # only required when you touched the pipeline contract
```

The pre-push hook covers `make lint && make test`. Everything else is on you.

---

## Most-used commands

```bash
# Stack
make up | down | restart
make backend-up                 # everything except dashboard container
make infra-up | services-up
make debug-up | debug-down      # expose backend ports on localhost
make logs                       # tail combined service logs

# Tests
make test                       # Go integration + Python unit
make test-e2e                   # full pipeline smoke test in containers

# Frontend
make fe-dev                     # Vite dev server (5173)
make fe-check                   # lint + typecheck + test + build + bundle-size
make fe-test-e2e                # Playwright (visual + a11y) inside pinned image
make fe-test-e2e-update         # refresh visual baselines
make fe-codegen                 # OpenAPI → src/lib/api/types.ts

# Codegen
make openapi-bundle             # bundle modular OpenAPI specs
make codegen                    # Go server stubs from BFF spec

# Misc
make crawl                      # one-shot RSS crawl (containerized, runs on aer-backend)
make crawl-reset                # wipe crawler dedup state so next crawl re-ingests everything
make swagger-up | swagger-down  # API reference at http://localhost:8089
```

---

## Where things live

- **`compose.yaml`** — SSoT for image tags, healthchecks, networks. Profiles:
  `dashboard` (frontend container), `debug` (port forwarder), `dev` (swagger-ui),
  `e2e` (Playwright runner).
- **`.tool-versions`** — SSoT for developer tooling versions.
- **`.env`** — all credentials and endpoints (copy from `.env.example`).
- **`Makefile`** — primary developer interface; every command above is defined here.
- **`docs/operations_playbook.md`** — full reference: how to inspect every
  database, browse MinIO, query ClickHouse, refresh dependencies, etc.

---

## Common gotchas

- **`fe-dev` refuses to start** — backend isn't running. Run `make backend-up`.
- **Playwright snapshots differ on host** — never run host-local Playwright for
  baselines; always use `make fe-test-e2e` / `make fe-test-e2e-update`, which
  run inside the pinned image so byte-comparison matches CI.
- **`make codegen` shows a diff in CI** — you edited a generated file or forgot
  to run codegen after editing the OpenAPI spec.
- **Edited a Go service but the old behavior persists** — `make backend-restart`
  only re-launches existing images; it does not rebuild. After changing service
  code, run the per-service restart target (e.g. `make bff-restart`,
  `make ingestion-restart`, `make worker-restart`) — these pass `--build
  --force-recreate` so the new binary lands in the container.
- **Service won't start with "required env empty"** — see the boot-secret table
  in `operations_playbook.md` (Phase 75 guard).
- **Need to talk to Postgres/ClickHouse/MinIO from the host** — run `make debug-up`
  first, then connect to `localhost:<port>`.
