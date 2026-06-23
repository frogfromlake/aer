import { expect, test, type Route, type Page } from './_fixtures';

// Phase 141 — AnalysesOverlay characterization net.
//
// The safety harness for the AnalysesOverlay decomposition (the saved-analyses
// global overlay, `?analyses=open`). It pins the overlay's RENDERED regions
// that a markup/scoped-CSS sub-componentisation could silently break: the
// toolbar + filter fieldsets + sortable table header, the per-row owner cells
// and access badges, the live client-side search filter, and the detail drawer
// (owned-with-share-UI vs read-only). Deliberately behavioural, not pixel-based
// — it must stay green across the AnalysisTable / AnalysisDrawer extraction.
//
// The overlay is mounted globally in the (app) layout, so opening it needs no
// Workbench pillar seed — `?analyses=open` over the root Atmosphere page is
// enough. All BFF routes are mocked so the test runs against `pnpm preview`
// with no live backend (mirrors workbench.spec.ts).

const analysesPayload = {
  analyses: [
    {
      id: 'an-owned',
      name: 'Owned editable view',
      description: 'My saved workbench deep link',
      ownerName: 'E2E Owner',
      ownerEmail: 'e2e@aer.test',
      createdAt: '2026-05-01T10:00:00Z',
      updatedAt: '2026-05-10T10:00:00Z',
      permission: 'editable',
      owned: true
    },
    {
      id: 'an-shared',
      name: 'Shared read-only view',
      description: 'Shared with me by a colleague',
      ownerName: 'A. Colleague',
      ownerEmail: 'colleague@aer.test',
      createdAt: '2026-04-01T10:00:00Z',
      updatedAt: '2026-04-05T10:00:00Z',
      permission: 'readable',
      owned: false
    }
  ]
};

const sharesPayload = {
  shares: [{ granteeId: 'g-1', email: 'friend@aer.test', canEdit: false }]
};

const json = (body: unknown) => ({
  status: 200,
  contentType: 'application/json',
  body: JSON.stringify(body)
});

async function mockBff(page: Page) {
  // Catch-all FIRST (see workbench.spec): any unmocked endpoint 401s against
  // the backend-less preview, and any data-layer 401 trips the global
  // unauthenticated redirect that unmounts the (app) layout. A blanket 200
  // makes that impossible; the shaped routes below win (Playwright last-wins).
  await page.route('**/api/v1/**', (route: Route) => route.fulfill(json({})));
  await page.route('**/api/v1/auth/me', (route: Route) =>
    route.fulfill(
      json({ id: 'e2e-user', email: 'e2e@aer.test', role: 'researcher', status: 'active' })
    )
  );
  await page.route('**/api/v1/probes', (route: Route) => route.fulfill(json([])));
  await page.route(/\/api\/v1\/metrics\?/, (route: Route) => route.fulfill(json({ data: [] })));
  // The saved-analyses list (overlay body) + per-analysis shares (drawer).
  await page.route(/\/api\/v1\/analyses$/, (route: Route) => route.fulfill(json(analysesPayload)));
  await page.route(/\/api\/v1\/analyses\/[^/]+\/shares$/, (route: Route) =>
    route.fulfill(json(sharesPayload))
  );
}

test.describe('Phase 141 — AnalysesOverlay characterization', () => {
  test.beforeEach(async ({ page }) => {
    await mockBff(page);
  });

  test('renders the toolbar, filters, sortable table and both rows with badges', async ({
    page
  }) => {
    await page.goto('/?analyses=open');

    const dialog = page.getByRole('dialog', { name: 'Saved analyses' });
    await expect(dialog).toBeVisible();

    // Toolbar: search box only. Phase 148e removed the in-overlay save trigger
    // (saving is initiated from the Workbench action), so neither the "Save
    // current view" button nor the "configure first" hint render here.
    await expect(dialog.getByRole('searchbox', { name: 'Search saved analyses' })).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Save current view' })).toHaveCount(0);
    await expect(dialog.getByText('Configure an analysis in the Workbench to save it')).toHaveCount(
      0
    );

    // Filter fieldsets (each <fieldset> exposes its <legend> as a group).
    await expect(dialog.getByRole('group', { name: 'Show' })).toBeVisible();
    await expect(dialog.getByRole('group', { name: 'Permission' })).toBeVisible();
    await expect(dialog.getByRole('group', { name: 'Created' })).toBeVisible();

    // Sortable column headers.
    await expect(dialog.getByRole('button', { name: /^Name/ })).toBeVisible();
    await expect(dialog.getByRole('button', { name: /^Owner/ })).toBeVisible();
    await expect(dialog.getByRole('button', { name: /^Created/ })).toBeVisible();
    await expect(dialog.getByRole('button', { name: /^Updated/ })).toBeVisible();

    // Both rows render: owned shows "You" + Owner badge; shared shows the owner
    // display name (email rides on the cell title) + the Read-only badge.
    await expect(dialog.getByRole('cell', { name: 'Owned editable view' })).toBeVisible();
    await expect(dialog.getByRole('cell', { name: 'Shared read-only view' })).toBeVisible();
    await expect(dialog.getByRole('cell', { name: 'You' })).toBeVisible();
    await expect(dialog.getByRole('cell', { name: 'A. Colleague' })).toBeVisible();

    const ownedRow = dialog.getByRole('row', { name: 'Open Owned editable view' });
    await expect(ownedRow.getByText('Owner')).toBeVisible();
    const sharedRow = dialog.getByRole('row', { name: 'Open Shared read-only view' });
    await expect(sharedRow.getByText('Read-only', { exact: true })).toBeVisible();
  });

  test('live search filter narrows the list client-side', async ({ page }) => {
    await page.goto('/?analyses=open');

    const dialog = page.getByRole('dialog', { name: 'Saved analyses' });
    await expect(dialog.getByRole('cell', { name: 'Owned editable view' })).toBeVisible();
    await expect(dialog.getByRole('cell', { name: 'Shared read-only view' })).toBeVisible();

    // Matches name + description + ownerEmail; "colleague" is only in the
    // shared row's owner email, so the owned row drops out of the DOM.
    await dialog.getByRole('searchbox', { name: 'Search saved analyses' }).fill('colleague');
    await expect(dialog.getByRole('cell', { name: 'Owned editable view' })).toHaveCount(0);
    await expect(dialog.getByRole('cell', { name: 'Shared read-only view' })).toBeVisible();
  });

  test('owned-row drawer exposes the edit/share/delete controls and existing shares', async ({
    page
  }) => {
    await page.goto('/?analyses=open');

    const dialog = page.getByRole('dialog', { name: 'Saved analyses' });
    await dialog.getByRole('row', { name: 'Open Owned editable view' }).click();

    // The drawer is a sibling <aside> of the dialog — assert at page scope.
    await expect(
      page.getByRole('heading', { name: 'Owned editable view', level: 3 })
    ).toBeVisible();
    await expect(page.getByRole('button', { name: 'Open in Workbench' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Edit' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Shared with' })).toBeVisible();
    await expect(page.getByText('friend@aer.test')).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Recipient email' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Delete' })).toBeVisible();
  });

  test('read-only (not owned) drawer omits edit, share and delete', async ({ page }) => {
    await page.goto('/?analyses=open');

    const dialog = page.getByRole('dialog', { name: 'Saved analyses' });
    await dialog.getByRole('row', { name: 'Open Shared read-only view' }).click();

    await expect(
      page.getByRole('heading', { name: 'Shared read-only view', level: 3 })
    ).toBeVisible();
    await expect(page.getByRole('button', { name: 'Open in Workbench' })).toBeVisible();

    // Not owned + read-only → none of the privileged sections render.
    await expect(page.getByRole('button', { name: 'Edit' })).toHaveCount(0);
    await expect(page.getByRole('heading', { name: 'Shared with' })).toHaveCount(0);
    await expect(page.getByRole('heading', { name: 'Delete' })).toHaveCount(0);
  });
});
