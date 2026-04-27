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

function dossierPayload() {
  return {
    probeId: PROBE_ID,
    language: 'de',
    windowStart: '2026-04-20T00:00:00Z',
    windowEnd: '2026-04-27T00:00:00Z',
    functionCoverage: {
      covered: 2,
      total: 4,
      functions: ['epistemic_authority', 'power_legitimation']
    },
    sources: [
      {
        name: 'tagesschau',
        type: 'rss',
        url: 'https://www.tagesschau.de',
        articlesTotal: 100,
        articlesInWindow: 20,
        publicationFrequencyPerDay: 5,
        primaryFunction: 'epistemic_authority',
        secondaryFunction: null,
        emicDesignation: 'Public broadcaster',
        emicContext: 'German public media',
        silverEligible: true
      }
    ]
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
  await page.route(`**/api/v1/probes/${PROBE_ID}/dossier**`, async (route: Route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(dossierPayload())
    });
  });
}

test.describe('Atmosphere — Phase 113c probe descent', () => {
  test('deep-linked probe redirects to /lanes/{id}/dossier and renders the Dossier', async ({
    page
  }) => {
    await mockBff(page);
    await page.goto(`/?probe=${PROBE_ID}`);

    // Surface I no longer hosts an L3 SidePanel; the descent is a path
    // change to Surface II L1 Probe Dossier.
    await expect(page).toHaveURL(new RegExp(`/lanes/${PROBE_ID}/dossier`));

    // The Dossier renders the probe identity and the migrated framing
    // copy that previously lived in the Atmosphere flyout.
    await expect(page.getByRole('heading', { name: PROBE_ID })).toBeVisible();
    await expect(page.getByText(/institutional voice/i)).toBeVisible();
    await expect(page.getByText(/reach is not rendered/i)).toBeVisible();
  });
});

test.describe('Atmosphere — Phase 110 scope contract', () => {
  test('primer link in the scope bar routes to /reflection/primer/globe', async ({ page }) => {
    await mockBff(page);
    await page.goto('/');
    const link = page.getByRole('link', { name: /how to read the globe/i });
    await expect(link).toBeVisible();
    await expect(link).toHaveAttribute('href', '/reflection/primer/globe');
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
