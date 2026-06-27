import { expect, test, type Route } from './_fixtures';

// End-to-end coverage for Phase 99b (ROADMAP "Surface I — First Contact").
// The full pipeline — BFF → typed client → Svelte state → engine API →
// WebGL rendering — is exercised against mocked BFF responses so these
// tests run against `pnpm preview` without requiring the backend stack.
// Tempo trace verification is out of scope here; it belongs to the
// integrated stack smoke test (`make test-e2e`).

const PROBE_ID = 'probe-0-de-institutional-web';

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

test.describe('Atmosphere — probe descent into the Dossier overlay (Phase 123a)', () => {
  // Rewritten in Phase 127 from the retired `/lanes/{id}/dossier` route. The
  // probe→Dossier descent now opens the Dossier as a GLOBAL OVERLAY over
  // Surface I (`?dossier=open` + `?selectedProbes=`), not a route change. The
  // selected probe's card renders auto-expanded over the persistent globe.
  // (ProbeCard internals are pinned by dossier.spec.ts; this test asserts the
  // descent contract — the deep link opens the overlay for that probe.)
  test('a deep-linked probe opens the Dossier overlay for that probe', async ({ page }) => {
    await mockBff(page);
    await page.goto(`/?dossier=open&selectedProbes=${PROBE_ID}`);

    // The descent is an overlay, not a navigation: the dossier grammar persists.
    await expect(page).toHaveURL(/dossier=open/);

    // The deep-linked probe's card renders (auto-expanded from selectedProbes).
    const card = page.locator(`article.probe-card#probe-${PROBE_ID}`);
    await expect(card).toBeVisible();
    await expect(card.locator('.probe-id')).toHaveText(PROBE_ID);
  });
});

// The "How to read the globe" primer was deliberately removed from the Atmosphere
// chrome (see AtmosphereChrome/AtmosphereSurface) and now lives on the Reflection
// landing. This guards its CURRENT home — reachability of the globe primer route —
// by href, because the link label is i18n (assert on the stable route, not text).
test.describe('Reflection — globe primer', () => {
  test('the globe primer link is reachable from the Reflection landing', async ({ page }) => {
    await mockBff(page);
    await page.goto('/reflection');
    const link = page.locator('a[href="/reflection/primer/globe"]');
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
