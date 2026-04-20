import { expect, test } from '@playwright/test';

test('landing page mounts the atmosphere canvas (WebGL2 path)', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveTitle(/Atmosphere/);
  await expect(page.getByRole('figure', { name: /AĒR atmosphere/ })).toBeVisible();
});

test('landing page renders the WebGL2 fallback when forced', async ({ page }) => {
  await page.goto('/?fallback=1');
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('ἀήρ');
});

test('stories index lists every component', async ({ page }) => {
  await page.goto('/stories');
  for (const title of ['Button', 'Dialog', 'Tooltip', 'Badge', 'SkipLink']) {
    await expect(page.getByRole('link', { name: new RegExp(`^${title}`) }).first()).toBeVisible();
  }
});
