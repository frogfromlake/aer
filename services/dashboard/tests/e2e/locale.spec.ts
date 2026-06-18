import { expect, test } from './_fixtures';

// Phase 144 — UI localisation (ADR-041). Validates the runtime behaviour the
// static checks (typecheck / parity-lint) cannot: that a real browser renders
// English by default, switches to German live (no reload) via the SideRail/
// AuthCard selector, deep-links via ?lang=, and reflects the locale in <html
// lang>. Runs on the login page because it is self-contained (no app shell) and
// carries the LocaleSwitch through AuthCard.
//
// The default fixture mocks /auth/me as authenticated, which would bounce the
// login page straight through; re-route it to 401 so the form renders.

test.describe('UI locale switching', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('**/api/v1/auth/me', (route) => route.fulfill({ status: 401, body: '{}' }));
  });

  test('renders English by default and switches to German live', async ({ page }) => {
    await page.goto('/login');

    // English by default (clean URL, no ?lang).
    await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible();
    await expect(page.locator('html')).toHaveAttribute('lang', 'en');

    // Switch to German via the selector (the lang attribute is a stable,
    // locale-independent locator — the accessible name is itself localised).
    await page.locator('button[lang="de"]').first().click();

    await expect(page.getByRole('heading', { name: 'Anmelden' })).toBeVisible();
    await expect(page.locator('html')).toHaveAttribute('lang', 'de');
    // The choice is pinned in the URL for sharing/deep-linking.
    await expect(page).toHaveURL(/[?&]lang=de/);
  });

  test('?lang=de deep-link reproduces the German UI on load', async ({ page }) => {
    await page.goto('/login?lang=de');

    await expect(page.getByRole('heading', { name: 'Anmelden' })).toBeVisible();
    await expect(page.locator('html')).toHaveAttribute('lang', 'de');
    await expect(page.getByText('Der Zugang erfolgt ausschließlich auf Einladung.')).toBeVisible();
  });
});
