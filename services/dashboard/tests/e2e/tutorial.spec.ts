import { expect, test, type Page } from './_fixtures';
import type { Locator } from '@playwright/test';
import { mockFullBff } from './_mocks';

// Guided tour (Slice 1) — the runtime behaviour the pure-step unit test cannot
// prove: the scope-bar Guide button launches the spotlight tour, the card walks
// forward through stops, Skip/Esc end it, and it re-launches. Runs on the
// Atmosphere surface (`/`), where every Slice-1 target (SideRail + ScopeBar)
// is mounted.
//
// Controls are queried WITHIN the dialog: Playwright's default name match is a
// substring, so a bare "Back" also hits the scope-bar history "Go back" — the
// dialog scope removes every such collision (chrome + welcome ambient).

async function launchTour(page: Page): Promise<Locator> {
  const dialog = page.getByRole('dialog', { name: 'Guided tour' });
  // First click can fire before SvelteKit hydration attaches the handler; retry
  // the launch until the dialog appears (same race the a11y/visual specs guard).
  await expect(async () => {
    await page.getByRole('button', { name: 'Guided tour' }).click();
    await expect(dialog).toBeVisible({ timeout: 2000 });
  }).toPass({ timeout: 15000 });
  return dialog;
}

test.describe('Guided tour', () => {
  test.beforeEach(async ({ page }) => {
    await mockFullBff(page);
    await page.goto('/');
  });

  test('launches from the scope bar and walks forward through stops', async ({ page }) => {
    const dialog = await launchTour(page);

    // Stop 1 — centred welcome card.
    await expect(dialog.getByRole('heading', { name: 'Welcome to AĒR' })).toBeVisible();
    await expect(dialog.getByText('Step 1 of', { exact: false })).toBeVisible();

    // Next → stop 2 (the SideRail surface anchors).
    await dialog.getByRole('button', { name: 'Next' }).click();
    await expect(dialog.getByRole('heading', { name: 'Three surfaces' })).toBeVisible();

    // Back returns to the welcome stop.
    await dialog.getByRole('button', { name: 'Back' }).click();
    await expect(dialog.getByRole('heading', { name: 'Welcome to AĒR' })).toBeVisible();
  });

  test('Skip ends the tour; the Guide button re-launches it', async ({ page }) => {
    const dialog = await launchTour(page);
    await dialog.getByRole('button', { name: 'Skip' }).click();
    await expect(dialog).toBeHidden();

    // Re-launch starts again from the first stop.
    await page.getByRole('button', { name: 'Guided tour' }).click();
    await expect(dialog.getByRole('heading', { name: 'Welcome to AĒR' })).toBeVisible();
  });

  test('Escape ends the tour', async ({ page }) => {
    const dialog = await launchTour(page);
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
  });

  test('deep-link ?guide=open auto-starts the tour', async ({ page }) => {
    await page.goto('/?guide=open');
    const dialog = page.getByRole('dialog', { name: 'Guided tour' });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole('heading', { name: 'Welcome to AĒR' })).toBeVisible();
  });

  test('opens the Scope Editor, then seeds a demo panel, and restores the URL on skip', async ({
    page
  }) => {
    const dialog = await launchTour(page);

    // Walk welcome → … → globe → scopeeditor (8 × Next). The scope step lands on
    // the bare Workbench, where the create-mode ScopeEditor auto-opens.
    for (let i = 0; i < 8; i++) {
      await dialog.getByRole('button', { name: 'Next' }).click();
    }
    await expect(
      dialog.getByRole('heading', { name: 'The working surface starts here' })
    ).toBeVisible();
    await expect(page).toHaveURL(/\/workbench/);
    await expect(page).not.toHaveURL(/activePillar/); // bare surface, no seed yet

    // Two more scope stops stay on the bare surface (choose probes/sources, then
    // the Create-panel button), still with the ScopeEditor open and no seed.
    await dialog.getByRole('button', { name: 'Next' }).click();
    await expect(dialog.getByRole('heading', { name: 'Choose probes & sources' })).toBeVisible();
    await dialog.getByRole('button', { name: 'Next' }).click();
    await expect(
      dialog.getByRole('heading', { name: 'Create the panel — off you go' })
    ).toBeVisible();
    await expect(page).not.toHaveURL(/activePillar/);

    // Next → pillars: the controller swaps in the demo seed (split panel).
    await dialog.getByRole('button', { name: 'Next' }).click();
    await expect(dialog.getByRole('heading', { name: 'Three Pillars' })).toBeVisible();
    await expect(page).toHaveURL(/activePillar=aleph/);

    // Skip undoes the seeded demo: the pre-tour URL ('/') is restored.
    await dialog.getByRole('button', { name: 'Skip' }).click();
    await expect(dialog).toBeHidden();
    await expect(page).not.toHaveURL(/activePillar/);
  });

  test('walks the whole tour through every surface and finishes on the globe', async ({ page }) => {
    const dialog = await launchTour(page);

    // Walk to completion: click the primary button (Next, or Done on the final
    // stop) until the tour closes. This traverses all three surfaces — Atmosphere
    // → the seeded Workbench demo → Reflection → back — proving no stop hard-
    // stalls and the cross-route navigation fires end to end. A short settle
    // between clicks lets each step's route/target resolve; the loop is bounded.
    let reachedReflection = false;
    for (let i = 0; i < 40; i++) {
      if (!(await dialog.isVisible())) break;
      if (page.url().includes('/reflection')) reachedReflection = true;
      const done = dialog.getByRole('button', { name: 'Done' });
      if (await done.isVisible().catch(() => false)) {
        await done.click();
        break;
      }
      await dialog.getByRole('button', { name: 'Next' }).click();
      await page.waitForTimeout(150);
    }

    // The Reflection surface was visited on the way (its four stops live there),
    // the tour closed, and the pre-tour URL ('/') is restored (demo undone).
    expect(reachedReflection).toBe(true);
    await expect(dialog).toBeHidden();
    await expect(page).not.toHaveURL(/activePillar/);
  });
});
