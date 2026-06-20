import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from './_fixtures';
import {
  mockFullBff,
  ALEPH_WORKBENCH_URL,
  COOC_WORKBENCH_URL,
  DOSSIER_URL,
  PROBE_ID
} from './_mocks';

// Phase 128 — Axe a11y sweep over the authenticated app surfaces.
//
// The original a11y.spec.ts covers the static story routes + the Atmosphere
// fallback. This spec extends the audit to every *reachable application state*
// the ROADMAP Phase-128 bullet names: the Workbench (Aleph distribution cell +
// Rhizome D3-force cooccurrence cell, both with the focused-panel control
// strip), the global Dossier overlay, the Reflection surface, and the auth
// surfaces. Zero WCAG 2.2 AA violations is the gate.

const wcagTags = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'];

async function runAxe(page: Page) {
  await page.evaluate(() => document.fonts.ready);
  const results = await new AxeBuilder({ page }).withTags(wcagTags).analyze();
  expect(results.violations, JSON.stringify(results.violations, null, 2)).toEqual([]);
}

test.describe('Phase 128 — Workbench a11y', () => {
  test.beforeEach(async ({ page }) => {
    await mockFullBff(page);
  });

  test('a11y: Workbench Aleph distribution cell', async ({ page }) => {
    await page.goto(ALEPH_WORKBENCH_URL);
    await expect(page.getByRole('region', { name: 'Panel controls' })).toBeVisible();
    await runAxe(page);
  });

  test('a11y: Workbench Rhizome cooccurrence (D3-force) cell', async ({ page }) => {
    await page.goto(COOC_WORKBENCH_URL);
    // The focused panel mounts its control strip; the force layout auto-settles.
    // Axe checks the surrounding DOM/ARIA, not the canvas internals, so we only
    // need the cell mounted, not the simulation at rest.
    await expect(page.getByRole('region', { name: 'Panel controls' })).toBeVisible();
    await runAxe(page);
  });

  test('a11y: Workbench with negative-space disclosure on', async ({ page }) => {
    await page.goto(`${ALEPH_WORKBENCH_URL}&negSpace=1`);
    await expect(page.getByRole('region', { name: 'Panel controls' })).toBeVisible();
    await runAxe(page);
  });
});

test.describe('Phase 128 — Dossier overlay a11y', () => {
  test.beforeEach(async ({ page }) => {
    await mockFullBff(page);
  });

  test('a11y: Dossier overlay (single probe expanded)', async ({ page }) => {
    await page.goto(DOSSIER_URL);
    await expect(page.locator(`article.probe-card#probe-${PROBE_ID}`)).toBeVisible();
    await runAxe(page);
  });

  test('Dossier overlay is a modal: role=dialog/aria-modal + Esc closes it', async ({ page }) => {
    await page.goto(DOSSIER_URL);
    const dialog = page.getByRole('dialog');
    await expect(dialog).toHaveAttribute('aria-modal', 'true');
    await expect(page.locator(`article.probe-card#probe-${PROBE_ID}`)).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
  });
});

test.describe('Phase 128 — Reflection a11y', () => {
  test.beforeEach(async ({ page }) => {
    await mockFullBff(page);
  });

  for (const route of ['/reflection', '/reflection/probes', '/reflection/open-questions']) {
    test(`a11y: ${route}`, async ({ page }) => {
      await page.goto(route);
      await expect(page.getByRole('heading').first()).toBeVisible();
      await runAxe(page);
    });
  }
});

test.describe('Phase 128 — Auth surface a11y', () => {
  test.beforeEach(async ({ page }) => {
    await mockFullBff(page);
    // The auth surfaces must render for an UNauthenticated visitor; override the
    // fixture's active session so /login does not redirect away on mount.
    await page.route('**/api/v1/auth/me', (route) => route.fulfill({ status: 401, body: '{}' }));
  });

  for (const route of [
    '/login',
    '/forgot-password',
    '/reset-password?token=test',
    '/accept-invite?token=test'
  ]) {
    test(`a11y: ${route}`, async ({ page }) => {
      await page.goto(route);
      await expect(page.getByRole('heading').first()).toBeVisible();
      await runAxe(page);
    });
  }

  test('login error is narrated via role=alert', async ({ page }) => {
    // Fail the credential check so the inline notice renders for assistive tech.
    await page.route('**/api/v1/auth/login', (route) => route.fulfill({ status: 401, body: '{}' }));
    await page.goto('/login');
    await page.getByLabel('Email').fill('nobody@aer.test');
    await page.getByLabel('Password').fill('wrong-password');
    await page.getByRole('button', { name: /sign in|log in|anmelden/i }).click();
    // The AuthNotice carries role="alert" + aria-live so the failure is announced.
    await expect(page.getByRole('alert')).toBeVisible();
  });
});
