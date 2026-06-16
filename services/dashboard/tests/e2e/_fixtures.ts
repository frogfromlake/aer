// Shared Playwright fixtures for the dashboard E2E suite.
//
// Phase 134 / ADR-040 gated the whole app: the (app) layout calls
// GET /auth/me on mount and bounces unauthenticated visitors to /login.
// These E2E run against the static `pnpm preview` build with mocked BFF
// responses, so they have no real session. This fixture mocks /auth/me as
// an active researcher for every test, restoring the pre-auth behaviour the
// suite assumes. A test that needs the unauthenticated/login flow can
// re-route '**/api/v1/auth/me' to a 401 itself (last route registered wins).
//
// Phase 135 / ADR-040 then mounted the Atmosphere globe PERSISTENTLY in the
// (app) layout, so EVERY authenticated surface (Workbench, Reflection, …) — not
// just `/` — fires the globe's probe + activity queries on mount. Against the
// backend-less preview those would hit the real BFF (401) and refetch-storm,
// stranding the page. We therefore also stub `/probes` (empty list) and the
// globe's `/metrics?…` activity query (empty series) as defaults here, so any
// (app) page renders its own content instead of a perpetual loading/redirect.
// A spec that needs real probe/metric data re-routes these itself (last wins).
// The `/metrics?…` matcher is a RegExp anchored on the `?` so it never shadows
// the per-metric `/metrics/{name}/provenance` path (no query string there).
import { test as base } from '@playwright/test';

export { expect, type Route, type Request, type Page } from '@playwright/test';

const json = (body: unknown) => ({
  status: 200,
  contentType: 'application/json',
  body: JSON.stringify(body)
});

export const test = base.extend({
  page: async ({ page }, use) => {
    await page.route('**/api/v1/auth/me', (route) =>
      route.fulfill(
        json({ id: 'e2e-user', email: 'e2e@aer.test', role: 'researcher', status: 'active' })
      )
    );
    // Persistent-globe defaults (Phase 135) — empty so the globe renders without
    // markers and never storms. Spec-specific routes override these.
    await page.route('**/api/v1/probes', (route) => route.fulfill(json([])));
    await page.route(/\/api\/v1\/metrics\?/, (route) => route.fulfill(json({ data: [] })));
    await use(page);
  }
});
