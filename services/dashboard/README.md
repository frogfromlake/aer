# AĒR Dashboard

SvelteKit static frontend for AĒR. Consumes the BFF API exclusively; holds no credentials in-browser (ADR-008 Zero-Trust, ADR-011/ADR-018 static key handled at the ingress).

Governing documents:

- Design Brief: `docs/design/design_brief.md`
- Visualization Guidelines: `docs/design/visualization_guidelines.md`
- ADR-020 (Frontend Stack): `docs/arc42/09_architecture_decisions.md`

Phase 97 delivers scaffolding only — no user-facing features. A blank "Hello AĒR" page, TypeScript strict mode, ESLint + Prettier, Vitest + Playwright, OpenTelemetry Web SDK, and an 80 kB initial-bundle gate — all at parity with the Go services' quality gates.

## One-time setup

Requires Node 22 (see `.nvmrc`) and pnpm 10 via Corepack. Both versions are pinned in the repo-root `.tool-versions` (SSoT).

```bash
corepack enable
corepack prepare pnpm@10.33.0 --activate
make fe-install        # reads pnpm-lock.yaml with --frozen-lockfile
```

## Developer workflow

**Use `make` targets from the repo root — not `pnpm` directly.** The Makefile is the uniform developer interface across Go, Python, and TypeScript services; the package.json scripts are implementation detail that the Makefile wraps. This keeps CI, pre-commit hooks, and local development on the same commands.

| Command               | What it does                                                            |
| :-------------------- | :---------------------------------------------------------------------- |
| `make fe-install`     | Install dependencies from the frozen lockfile                           |
| `make fe-dev`         | Start the SvelteKit dev server (hot reload, `http://localhost:5173`)    |
| `make fe-preview`     | Build and serve the production bundle locally (`http://localhost:4173`) |
| `make fe-format`      | Auto-format sources with Prettier                                       |
| `make fe-lint`        | ESLint + Prettier check + `svelte-check`                                |
| `make fe-typecheck`   | TypeScript strict typecheck (via `svelte-check`)                        |
| `make fe-test`        | Vitest unit tests                                                       |
| `make fe-test-e2e`    | Playwright end-to-end smoke test                                        |
| `make fe-build`       | Production static build (`build/`)                                      |
| `make fe-bundle-size` | Enforce the 80 kB initial-bundle budget (Design Brief §7)               |
| `make fe-codegen`     | Regenerate TypeScript types from the BFF OpenAPI spec                   |
| `make codegen-ts`     | Repo-root alias of `fe-codegen` (peer of `make codegen` for Go)         |
| `make fe-check`       | Composite: lint + typecheck + test + build + bundle-size gate           |

`make fe-lint` is automatically included in `make lint`, and `make fe-test` in `make test` — the repo-wide gates already cover the frontend.

## Configuration

Build-time environment variables (prefix `PUBLIC_` per SvelteKit's static rules):

| Variable                        | Purpose                                                    | Default       |
| :------------------------------ | :--------------------------------------------------------- | :------------ |
| `PUBLIC_OTLP_ENDPOINT`          | OTLP/HTTP traces endpoint. Empty string disables emission. | `""`          |
| `PUBLIC_DEPLOYMENT_ENVIRONMENT` | Resource attribute `deployment.environment`.               | `development` |

## Architecture

See ADR-020. In brief: SvelteKit with `@sveltejs/adapter-static`, Svelte 5 Runes, TypeScript strict, Vite 8, bundled behind Nginx and routed by Traefik. No Node runtime in production.

- `src/lib/api/types.ts` — **generated** from `services/bff-api/api/openapi.yaml` via `make codegen-ts`. Do not edit.
- `src/lib/api/client.ts` — thin typed fetch wrapper.
- `src/lib/observability/otel.ts` — OpenTelemetry Web SDK init; lazy-loaded by `src/hooks.client.ts` after first paint.
- `scripts/check-bundle-size.mjs` — enforces the 80 kB initial-bundle budget.
