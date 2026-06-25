import { expect, test, type Page } from './_fixtures';
import { mockFullBff } from './_mocks';

// Color Themes feature — runtime behaviour the static checks (typecheck /
// vitest on theme-internals) cannot prove: that a real browser renders the dark
// default, switches theme live via the scope-bar ThemeMenu (no reload) AND via
// the account overlay, persists the choice across a reload (the anti-FOUC
// static/theme-init.js path), removes the attribute for `system`, and that the
// scope-bar quick sign-out returns to /login.
//
// `/reflection` is the lightest authenticated surface carrying the ScopeBar.
// The accessible names are the locale-independent English defaults (the fixture
// does not set a locale).

// The first click can fire before SvelteKit hydration attaches the handler;
// retry the open+select until the attribute flips (same race the a11y/visual
// specs guard with toPass).
async function pickTheme(page: Page, name: string, expectedAttr: string | null) {
  await expect(async () => {
    await page.getByRole('button', { name: 'Appearance' }).click();
    // `exact` so "Light"/"Dark" never also match "High contrast (light/dark)".
    await page.getByRole('menuitemradio', { name, exact: true }).click();
    if (expectedAttr === null) {
      await expect(page.locator('html')).not.toHaveAttribute('data-theme', /.+/, { timeout: 2000 });
    } else {
      await expect(page.locator('html')).toHaveAttribute('data-theme', expectedAttr, {
        timeout: 2000
      });
    }
  }).toPass({ timeout: 15000 });
}

test.describe('Color themes', () => {
  test.beforeEach(async ({ page }) => {
    await mockFullBff(page);
  });

  test('renders the dark default and switches to light live, persisting across reload', async ({
    page
  }) => {
    await page.goto('/reflection');

    // Dark by default (app.html ships data-theme="dark"; no stored choice).
    await expect(page.getByRole('button', { name: 'Appearance' })).toBeVisible();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');

    // Switch to Light via the scope-bar ThemeMenu — live, no reload.
    await pickTheme(page, 'Light', 'light');

    // Persisted: a full reload re-applies it before first paint via
    // static/theme-init.js (proves the localStorage + anti-FOUC path).
    await page.reload();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
  });

  test('the system choice removes the data-theme attribute (OS preference governs)', async ({
    page
  }) => {
    await page.goto('/reflection');
    await expect(page.getByRole('button', { name: 'Appearance' })).toBeVisible();

    await pickTheme(page, 'System', null);
  });

  test('the account overlay switches the same theme', async ({ page }) => {
    await page.goto('/reflection?account=open');

    // The appearance radiogroup lives in the account overlay; switching there
    // drives the same rune, so <html data-theme> flips.
    const lightRadio = page.getByRole('radio', { name: 'Light', exact: true });
    await expect(async () => {
      await lightRadio.click();
      await expect(page.locator('html')).toHaveAttribute('data-theme', 'light', { timeout: 2000 });
    }).toPass({ timeout: 15000 });
    await expect(lightRadio).toHaveAttribute('aria-checked', 'true');
  });

  test('the scope-bar quick sign-out logs out and returns to /login', async ({ page }) => {
    await page.goto('/reflection');
    const signout = page.getByRole('button', { name: 'Sign out' });
    await expect(signout).toBeVisible();

    // Simulate the post-logout unauthenticated state: /auth/logout 204s, and the
    // next /auth/me is a 401 so the /login page stays put instead of bouncing
    // an "authenticated" visitor straight back into the app.
    await page.route('**/api/v1/auth/logout', (route) => route.fulfill({ status: 204, body: '' }));
    await page.route('**/api/v1/auth/me', (route) => route.fulfill({ status: 401, body: '{}' }));

    await signout.click();
    await expect(page).toHaveURL(/\/login/);
  });
});
