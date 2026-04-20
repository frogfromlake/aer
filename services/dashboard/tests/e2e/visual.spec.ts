import { expect, test, type Page } from '@playwright/test';

const storyRoutes = [
  { path: '/stories', slug: 'index' },
  { path: '/stories/button', slug: 'button' },
  { path: '/stories/badge', slug: 'badge' },
  { path: '/stories/tooltip', slug: 'tooltip' },
  { path: '/stories/skiplink', slug: 'skiplink' }
] as const;

type Theme = 'dark' | 'light';

async function setTheme(page: Page, theme: Theme) {
  const button = page.getByRole('radio', { name: theme === 'dark' ? 'Dark' : 'Light' });
  await button.click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
}

async function settle(page: Page) {
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(100);
}

for (const theme of ['dark', 'light'] as const) {
  test.describe(`theme=${theme}`, () => {
    for (const { path, slug } of storyRoutes) {
      test(`visual: ${slug}`, async ({ page }) => {
        await page.goto(path);
        await setTheme(page, theme);
        await settle(page);
        await expect(page).toHaveScreenshot(`${slug}-${theme}.png`, { fullPage: true });
      });
    }

    // The atmosphere/fallback story uses a layout reset (no theme toggle); the
    // fallback panel is theme-independent dark, so we snapshot it once outside
    // the theme loop. This still exercises the route's static markup + tokens.

    test(`visual: dialog-open`, async ({ page }) => {
      await page.goto('/stories/dialog');
      await setTheme(page, theme);
      await settle(page);
      await page.getByRole('button', { name: 'Open dialog' }).click();
      await expect(page.getByRole('dialog')).toBeVisible();
      await settle(page);
      await expect(page).toHaveScreenshot(`dialog-open-${theme}.png`, { fullPage: true });
    });
  });
}

test('visual: atmosphere-fallback', async ({ page }) => {
  await page.goto('/stories/atmosphere/fallback');
  await settle(page);
  await expect(page).toHaveScreenshot('atmosphere-fallback.png', { fullPage: true });
});
