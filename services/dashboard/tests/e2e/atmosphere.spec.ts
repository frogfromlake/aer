import { expect, test, type Route } from '@playwright/test';

// End-to-end coverage for Phase 99b (ROADMAP "Surface I — First Contact").
// The full pipeline — BFF → typed client → Svelte state → engine API →
// WebGL rendering — is exercised against mocked BFF responses so these
// tests run against `pnpm preview` without requiring the backend stack.
// Tempo trace verification is out of scope here; it belongs to the
// integrated stack smoke test (`make test-e2e`).

const PROBE_ID = 'probe-0-de-institutional-rss';

const probePayload = [
  {
    probeId: PROBE_ID,
    language: 'de',
    sources: ['tagesschau', 'bundesregierung'],
    emissionPoints: [
      { latitude: 53.5511, longitude: 9.9937, label: 'Hamburg' },
      { latitude: 52.52, longitude: 13.405, label: 'Berlin' }
    ]
  }
];

function metricsPayload() {
  // Two sources × one hour of publications. With a 24-hour window, the
  // route's activity derivation yields `(6 + 4) / 24 ≈ 0.4` docs/hour —
  // enough to drive a visible pulse and a non-zero fallback readout.
  const now = new Date().toISOString();
  return {
    data: [
      { timestamp: now, value: 6, source: 'tagesschau', metricName: 'publication_hour' },
      { timestamp: now, value: 4, source: 'bundesregierung', metricName: 'publication_hour' }
    ],
    excludedCount: 0
  };
}

function contentPayload() {
  return {
    entityId: PROBE_ID,
    entityType: 'probe',
    locale: 'en',
    contentVersion: 'v2026-04-test',
    lastReviewedBy: 'e2e-fixture',
    lastReviewedAt: '2026-04-01',
    registers: {
      semantic: {
        short: 'German public-institutional news probe.',
        long: 'A bundle of German public-broadcaster and government sources, observed jointly as a single institutional voice.'
      },
      methodological: {
        short: 'Emission-point aggregation over sources tagesschau, bundesregierung.',
        long: 'Publication-hour counts are summed across the bound sources in the selected window. Reach is not measured and is not inferred from emission geometry.'
      }
    }
  };
}

async function mockBff(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/probes', async (route: Route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(probePayload)
    });
  });
  await page.route('**/api/v1/metrics?**', async (route: Route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(metricsPayload())
    });
  });
  await page.route(`**/api/v1/content/probe/${PROBE_ID}**`, async (route: Route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(contentPayload())
    });
  });
}

test.describe('Atmosphere — WebGL2 path', () => {
  test('deep-linked probe opens the side panel with Content Catalog emic content', async ({
    page
  }) => {
    await mockBff(page);
    await page.goto(`/?probe=${PROBE_ID}`);

    // The 3D canvas is the engine's rendering target.
    await expect(page.getByRole('figure', { name: /AĒR atmosphere/ })).toBeVisible();

    // Side panel opens from the URL-carried ?probe= on first probe payload.
    // The emic label is the first emission point: Hamburg.
    const panel = page.getByRole('dialog', { name: /Hamburg/ });
    await expect(panel).toBeVisible();
    await expect(panel.getByText(PROBE_ID)).toBeVisible();

    // Progressive Semantics renders the semantic register first.
    await expect(panel.getByText(/institutional voice/i)).toBeVisible();

    // Reach is explicitly not claimed.
    await expect(panel.getByText(/reach is not rendered/i)).toBeVisible();
  });
});

test.describe('Atmosphere — WebGL2 fallback', () => {
  test('fallback surface lists active probes with language, emission points, and publication rate', async ({
    page
  }) => {
    await mockBff(page);
    await page.goto('/?fallback=1');

    // Fallback title is the Greek name, matching the 99a static build.
    await expect(page.getByRole('heading', { level: 1, name: 'ἀήρ' })).toBeVisible();

    const probes = page.getByRole('region', { name: /Active probes/i });
    await expect(probes).toBeVisible();

    await expect(probes.getByText(PROBE_ID)).toBeVisible();
    await expect(probes.getByLabel('Language')).toHaveText('de');
    await expect(probes.getByText('Hamburg')).toBeVisible();
    await expect(probes.getByText('Berlin')).toBeVisible();
    await expect(probes.getByText(/docs\/hour/)).toBeVisible();

    // Reach is explicitly not reported.
    await expect(probes.getByText(/Reach is not reported/i)).toBeVisible();
  });
});
