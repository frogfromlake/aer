import { expect, test, type Page, type Route } from './_fixtures';

// Phase 144c — the Open Research Questions hub renders its 50-question prose
// catalog from a per-locale static source (open-questions.ts + open-questions.de.ts).
// The static parity test guarantees the German entries exist; this spec proves the
// runtime behaviour: a `?lang=de` deep-link actually renders the German prose (the
// research-data tier, not just the Paraglide shell), and English is the default.
//
// The page is an (app) surface, so the persistent globe + auth fire on mount. The
// default fixture mocks those; a catch-all (registered first, then the specifics)
// guards against any stray /api/v1 call 401ing and tripping the auth redirect.

const json = (body: unknown) => ({
  status: 200,
  contentType: 'application/json',
  body: JSON.stringify(body)
});

async function mockBff(page: Page) {
  await page.route('**/api/v1/**', (route: Route) => route.fulfill(json({})));
  await page.route('**/api/v1/auth/me', (route: Route) =>
    route.fulfill(
      json({ id: 'e2e-user', email: 'e2e@aer.test', role: 'researcher', status: 'active' })
    )
  );
  await page.route('**/api/v1/probes', (route: Route) => route.fulfill(json([])));
  await page.route(/\/api\/v1\/metrics\?/, (route: Route) => route.fulfill(json({ data: [] })));
}

// A distinctive German string from the wp-001-q1 entry (transcribed from the
// approved DE WP) — present only when the catalog renders in German.
const DE_QUESTION_LABEL = 'Regionale Sondenzuordnung';
const EN_QUESTION_LABEL = 'Regional probe mapping';

test.describe('Phase 144c — Open Research Questions DE localization', () => {
  test.beforeEach(async ({ page }) => {
    await mockBff(page);
  });

  test('?lang=de renders the German question catalog', async ({ page }) => {
    await page.goto('/reflection/open-questions?lang=de');

    await expect(page.locator('html')).toHaveAttribute('lang', 'de');
    // The catalog prose (research-data tier) is German, not just the shell.
    await expect(page.getByText(DE_QUESTION_LABEL, { exact: false }).first()).toBeVisible();
    await expect(
      page
        .getByText('Für Kulturanthropologen und Area-Studies-Wissenschaftler', { exact: false })
        .first()
    ).toBeVisible();
    // The English source string is gone.
    await expect(page.getByText(EN_QUESTION_LABEL, { exact: false })).toHaveCount(0);
  });

  test('renders the English catalog by default', async ({ page }) => {
    await page.goto('/reflection/open-questions');

    await expect(page.locator('html')).toHaveAttribute('lang', 'en');
    await expect(page.getByText(EN_QUESTION_LABEL, { exact: false }).first()).toBeVisible();
    await expect(page.getByText(DE_QUESTION_LABEL, { exact: false })).toHaveCount(0);
  });
});
