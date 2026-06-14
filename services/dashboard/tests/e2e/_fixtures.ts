// Shared Playwright fixtures for the dashboard E2E suite.
//
// Phase 134 / ADR-040 gated the whole app: the (app) layout calls
// GET /auth/me on mount and bounces unauthenticated visitors to /login.
// These E2E run against the static `pnpm preview` build with mocked BFF
// responses, so they have no real session. This fixture mocks /auth/me as
// an active researcher for every test, restoring the pre-auth behaviour the
// suite assumes. A test that needs the unauthenticated/login flow can
// re-route '**/api/v1/auth/me' to a 401 itself (last route registered wins).
import { test as base } from '@playwright/test';

export { expect, type Route, type Request, type Page } from '@playwright/test';

export const test = base.extend({
  page: async ({ page }, use) => {
    await page.route('**/api/v1/auth/me', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 'e2e-user',
          email: 'e2e@aer.test',
          role: 'researcher',
          status: 'active'
        })
      })
    );
    await use(page);
  }
});
