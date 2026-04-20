import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';

const routes = [
  '/',
  '/stories',
  '/stories/button',
  '/stories/badge',
  '/stories/tooltip',
  '/stories/skiplink',
  '/stories/dialog',
  '/stories/atmosphere/fallback',
  // The shell renders the WebGLFallback when WebGL2 is unavailable; Playwright's
  // headless Chromium in the CI gate may or may not advertise WebGL2, so we also
  // exercise the explicit `?fallback=1` override which deterministically renders
  // the fallback path.
  '/?fallback=1'
] as const;

const wcagTags = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'];

async function analyze(page: Page) {
  return new AxeBuilder({ page }).withTags(wcagTags).analyze();
}

for (const route of routes) {
  test(`a11y: ${route} (dark)`, async ({ page }) => {
    await page.goto(route);
    await page.evaluate(() => document.fonts.ready);
    const results = await analyze(page);
    expect(results.violations, JSON.stringify(results.violations, null, 2)).toEqual([]);
  });
}

test('a11y: /stories/button (light)', async ({ page }) => {
  await page.goto('/stories/button');
  await page.getByRole('radio', { name: 'Light' }).click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
  const results = await analyze(page);
  expect(results.violations, JSON.stringify(results.violations, null, 2)).toEqual([]);
});

test('a11y: /stories/dialog (dialog open)', async ({ page }) => {
  await page.goto('/stories/dialog');
  await page.getByRole('button', { name: 'Open dialog' }).click();
  await expect(page.getByRole('dialog')).toBeVisible();
  const results = await analyze(page);
  expect(results.violations, JSON.stringify(results.violations, null, 2)).toEqual([]);
});
