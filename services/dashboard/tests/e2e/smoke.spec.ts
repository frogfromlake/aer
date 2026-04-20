import { expect, test } from '@playwright/test';

test('landing page renders Hello AĒR', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('Hello AĒR');
});
