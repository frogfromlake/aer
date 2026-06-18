import { expect, test, type Route, type Page } from './_fixtures';

// Phase 141 — Working-Paper reader (wp/[id]/+page.svelte) characterization net.
//
// Safety harness for the WP-page decomposition (WpBreadcrumb / WpTableOfContents
// / WpPaperBody). Pins the rendered regions a markup/scoped-CSS sub-
// componentisation could silently break: the ScopeBar breadcrumb, the TOC nav,
// the paper title + section headings, and the "Back to Workbench" referrer link.
//
// The page is a client-side load (ssr=false, served via the SPA fallback) that
// fetches the paper markdown from /content/papers/{locale}/{id}.md (Phase 144 —
// the path is locale-aware per ADR-042) and renders it; getPaperMeta() is a local
// table (wp-001 exists), so mocking the markdown fetch + auth is enough to render
// the whole page against `pnpm preview` (no backend).

const PAPER_MD = `# WP-001 — Test Paper Title

Intro paragraph before the first section.

## 1. Scope and Method

First section body paragraph.

## 2. Findings

Second section body paragraph.
`;

const json = (body: unknown) => ({
  status: 200,
  contentType: 'application/json',
  body: JSON.stringify(body)
});

async function mockBff(page: Page) {
  // Catch-all FIRST (see workbench.spec): an unmocked /api/v1 401s and trips the
  // global unauthenticated redirect that unmounts the (app) layout.
  await page.route('**/api/v1/**', (route: Route) => route.fulfill(json({})));
  await page.route('**/api/v1/auth/me', (route: Route) =>
    route.fulfill(
      json({ id: 'e2e-user', email: 'e2e@aer.test', role: 'researcher', status: 'active' })
    )
  );
  await page.route('**/api/v1/probes', (route: Route) => route.fulfill(json([])));
  await page.route(/\/api\/v1\/metrics\?/, (route: Route) => route.fulfill(json({ data: [] })));
  // The paper markdown (static path, NOT under /api/v1) — the page's client load
  // fetches this and renders it into sections. Match any locale segment
  // (/content/papers/{locale}/{id}.md, Phase 144) — a single-segment `*` glob
  // would miss the locale dir and leave the body unrendered.
  await page.route('**/content/papers/**', (route: Route) =>
    route.fulfill({ status: 200, contentType: 'text/markdown', body: PAPER_MD })
  );
}

test.describe('Phase 141 — Working-Paper reader characterization', () => {
  test.beforeEach(async ({ page }) => {
    await mockBff(page);
  });

  test('renders the breadcrumb, TOC, and paper sections', async ({ page }) => {
    await page.goto('/reflection/wp/wp-001');

    // Breadcrumb (ScopeBar) — root link + the WP id.
    await expect(page.getByRole('link', { name: 'Back to Reflection surface' })).toBeVisible();
    await expect(page.locator('.breadcrumb-id')).toHaveText('WP-001');

    // Paper title (h1).
    await expect(page.getByRole('heading', { level: 1, name: /Test Paper Title/ })).toBeVisible();

    // TOC nav lists the numbered sections.
    const toc = page.getByRole('navigation', { name: 'Table of contents' });
    await expect(toc).toBeVisible();
    await expect(toc.getByRole('link', { name: /Scope and Method/ })).toBeVisible();
    await expect(toc.getByRole('link', { name: /Findings/ })).toBeVisible();

    // Section headings render in the paper body.
    await expect(page.getByRole('heading', { level: 2, name: /Scope and Method/ })).toBeVisible();
    await expect(page.getByRole('heading', { level: 2, name: /Findings/ })).toBeVisible();
  });

  test('shows the Back-to-Workbench link when arrived from a Workbench referrer', async ({
    page
  }) => {
    await page.goto(
      '/reflection/wp/wp-001?from=workbench&probe=probe-0-de-institutional-web&fn=epistemic_authority&pillar=aleph'
    );

    const back = page.getByRole('link', { name: 'Back to Workbench' });
    await expect(back).toBeVisible();
    const href = await back.getAttribute('href');
    expect(href).toContain('/workbench?');
    expect(href).toContain('probeId=probe-0-de-institutional-web');
    expect(href).toContain('functionKey=epistemic_authority');
    expect(href).toContain('viewingMode=aleph');
  });
});
