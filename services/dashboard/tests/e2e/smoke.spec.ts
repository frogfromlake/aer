import { expect, test } from '@playwright/test';

test('landing page renders AĒR title', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('ἀήρ');
});

test('stories index lists every component', async ({ page }) => {
  await page.goto('/stories');
  for (const title of ['Button', 'Dialog', 'Tooltip', 'Badge', 'SkipLink']) {
    await expect(page.getByRole('link', { name: new RegExp(`^${title}`) }).first()).toBeVisible();
  }
});
