# AĒR Dashboard

SvelteKit static frontend for AĒR. Consumes the BFF API exclusively; holds no credentials in-browser (ADR-008 Zero-Trust, ADR-011/ADR-018 static key handled at the ingress).

Governing documents:

- Design Brief: `docs/design/design_brief.md`
- Visualization Guidelines: `docs/design/visualization_guidelines.md`
- ADR-020 (Frontend Stack): `docs/arc42/09_architecture_decisions.md`

Phase 97 delivers scaffolding only — no user-facing features. A blank "Hello AĒR" page, TypeScript strict mode, ESLint + Prettier, Vitest + Playwright, OpenTelemetry Web SDK, and an 80 kB initial-bundle gate — all at parity with the Go services' quality gates.

Phase 98 adds the design-system foundation on top of that scaffolding: design tokens (`src/lib/design/tokens.css`), self-hosted Inter + IBM Plex Mono, Viridis/Cividis scales, Epistemic Weight visual treatments, five base Svelte components (`src/lib/components/base/`), a route-based story harness at `/stories`, and a Playwright visual + `@axe-core/playwright` WCAG 2.2 AA gate (`make fe-test-e2e`). See `docs/design/design_system.md` for details.

Phase 99a adds the 3D Atmosphere engine foundation. The engine lives in its own workspace package `packages/engine-3d/` (`@aer/engine-3d`) — vanilla three.js with custom GLSL (no r3f/Threlte per ADR-020 §5.9) — and exposes a narrow framework-agnostic imperative API (`AtmosphereEngine`). It renders a rotating Earth with vector landmasses (Natural Earth 1:50m, baked at build time via `pnpm run bake-landmass` into `static/data/landmass.json`), a real-time NOAA-driven day/night terminator, a Rayleigh/Mie atmospheric halo, OrbitControls with a 10s-idle auto-rotation, and an optional borders layer (default off). The shell statically imports only `@aer/engine-3d/capability` (the WebGL2 / `prefers-reduced-motion` probes) and dynamic-imports the engine module from inside `src/lib/components/atmosphere/AtmosphereCanvas.svelte` — so browsers without WebGL2 fall back to `WebGLFallback.svelte` and never download three.js. Four atmosphere stories (`/stories/atmosphere/{paused,terminator,flyto,fallback}`) drive the imperative API in isolation. The bundle gate now enforces both the shell budget and a separate ≤250 kB-gzipped budget for the lazy engine chunk (`scripts/check-bundle-size.mjs`). The engine accepts probe/activity/propagation inputs but does not yet render them — that wiring lands in Phase 99b. Brief §3.1 + Arc42 §8.17 + ADR-020 record the boundary.

## One-time setup

Requires Node 22 (see `.nvmrc`) and pnpm 10 via Corepack. Both versions are pinned in the repo-root `.tool-versions` (SSoT).

```bash
corepack enable
corepack prepare pnpm@10.33.0 --activate
make fe-install        # reads pnpm-lock.yaml with --frozen-lockfile
```

## Developer workflow

**Use `make` targets from the repo root — not `pnpm` directly.** The Makefile is the uniform developer interface across Go, Python, and TypeScript services; the package.json scripts are implementation detail that the Makefile wraps. This keeps CI, pre-commit hooks, and local development on the same commands.

| Command                   | What it does                                                                           |
| :------------------------ | :------------------------------------------------------------------------------------- |
| `make fe-install`         | Install dependencies from the frozen lockfile                                          |
| `make fe-dev`             | Start the SvelteKit dev server (hot reload, `http://localhost:5173`)                   |
| `make fe-preview`         | Build and serve the production bundle locally (`http://localhost:4173`)                |
| `make fe-format`          | Auto-format sources with Prettier                                                      |
| `make fe-lint`            | ESLint + Prettier check + `svelte-check`                                               |
| `make fe-typecheck`       | TypeScript strict typecheck (via `svelte-check`)                                       |
| `make fe-test`            | Vitest unit tests                                                                      |
| `make fe-test-e2e`        | Playwright visual + axe a11y gate (runs inside the pinned Playwright image; Phase 98c) |
| `make fe-test-e2e-update` | Regenerate the committed Playwright snapshots after intentional visual changes         |
| `make fe-build`           | Production static build (`build/`)                                                     |
| `make fe-bundle-size`     | Enforce the 80 kB initial-bundle budget (Design Brief §7)                              |
| `make fe-codegen`         | Regenerate TypeScript types from the BFF OpenAPI spec                                  |
| `make codegen-ts`         | Repo-root alias of `fe-codegen` (peer of `make codegen` for Go)                        |
| `make fe-check`           | Composite: lint + typecheck + test + build + bundle-size gate                          |

`make fe-lint` is automatically included in `make lint`, and `make fe-test` in `make test` — the repo-wide gates already cover the frontend.

## Configuration

Build-time environment variables (prefix `PUBLIC_` per SvelteKit's static rules). Because the static adapter inlines these at build time, **changing a value requires rebuilding the image** — `docker compose build dashboard`. Both values are promoted to Docker build args in `services/dashboard/Dockerfile` and wired via the `args:` block of the `dashboard` service in `compose.yaml` (Phase 98d). Defaults live in the repo-root `.env.example`; see Arc42 §7.4.4 for the deployment-view rationale.

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
