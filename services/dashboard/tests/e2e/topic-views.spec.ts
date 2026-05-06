import { expect, test, type Route, type Request } from '@playwright/test';

// Phase 121 — Topic view-mode E2E coverage.
//
// Asserts:
//   1. Navigating to a function lane with `?viewMode=topic_distribution`
//      causes the dashboard to call `GET /api/v1/topics/distribution`
//      with the resolved scope.
//   2. The cell renders at least one topic ridge (a non-zero-width bar)
//      from the mocked payload.
//   3. The outlier topic (`topicId == -1`) renders as the
//      `uncategorised` label, not hidden.
//
// All BFF routes are mocked so the test runs against `pnpm preview`
// without requiring the live backend stack (mirrors `atmosphere.spec.ts`).

const PROBE_ID = 'probe-0-de-institutional-rss';
const FUNCTION_KEY = 'epistemic_authority';

const probesPayload = [
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

const dossierPayload = {
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
      primaryFunction: FUNCTION_KEY,
      secondaryFunction: null,
      emicDesignation: 'Public broadcaster',
      emicContext: 'German public media',
      silverEligible: true
    }
  ]
};

const topicsPayload = {
  scope: 'probe',
  scopeId: PROBE_ID,
  windowStart: '2026-04-20T00:00:00Z',
  windowEnd: '2026-04-27T00:00:00Z',
  language: '',
  topics: [
    {
      topicId: 0,
      label: 'energy_climate_policy',
      articleCount: 42,
      avgConfidence: 0.71,
      language: 'de',
      modelHash: 'sha256:demo-fixture-hash'
    },
    {
      topicId: 1,
      label: 'inflation_economy',
      articleCount: 27,
      avgConfidence: 0.68,
      language: 'de',
      modelHash: 'sha256:demo-fixture-hash'
    },
    {
      topicId: -1,
      label: '-1_misc_words',
      articleCount: 9,
      avgConfidence: 0.0,
      language: 'de',
      modelHash: 'sha256:demo-fixture-hash'
    }
  ]
};

function genericContent(entityId: string) {
  return {
    entityId,
    entityType: 'view_mode',
    locale: 'en',
    contentVersion: 'v2026-05-test',
    lastReviewedBy: 'e2e-fixture',
    lastReviewedAt: '2026-05-01',
    registers: {
      semantic: { short: 'Topic distribution.', long: 'Long form.' },
      methodological: {
        short: 'BERTopic Tier 2 (mock).',
        long: 'Long methodological copy (mock).'
      }
    }
  };
}

async function mockBff(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/probes', async (route: Route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(probesPayload)
    });
  });
  await page.route(`**/api/v1/probes/${PROBE_ID}/dossier**`, async (route: Route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(dossierPayload)
    });
  });
  await page.route('**/api/v1/content/**', async (route: Route) => {
    const url = route.request().url();
    const m = url.match(/content\/[^/]+\/([^?]+)/);
    const id = m?.[1] ? decodeURIComponent(m[1]) : 'unknown';
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(genericContent(id))
    });
  });
  await page.route('**/api/v1/metrics?**', async (route: Route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: [], excludedCount: 0 })
    });
  });
  await page.route('**/api/v1/metrics/available**', async (route: Route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify([])
    });
  });
}

test.describe('Phase 121 — topic_distribution view mode', () => {
  test('switching to topic_distribution calls /topics/distribution and renders a ridge', async ({
    page
  }) => {
    await mockBff(page);

    const topicCalls: string[] = [];
    await page.route('**/api/v1/topics/distribution?**', async (route: Route) => {
      topicCalls.push(route.request().url());
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(topicsPayload)
      });
    });

    const topicRequest: Promise<Request> = page.waitForRequest((req) =>
      req.url().includes('/api/v1/topics/distribution')
    );

    await page.goto(`/lanes/${PROBE_ID}/${FUNCTION_KEY}?viewMode=topic_distribution`);

    // The deferred request must hit the BFF — proves the cell is wired.
    const req = await topicRequest;
    expect(req.url()).toContain('/topics/distribution');
    expect(req.url()).toContain('scope=probe');
    expect(req.url()).toContain(`scopeId=${PROBE_ID}`);

    // The cell heading is rendered.
    await expect(page.getByText(/BERTopic distribution/i)).toBeVisible();

    // At least one rendered SVG <rect> from the Plot bar layer — proves
    // a ridge made it into the DOM. We scope to the plot host so we
    // don't pick up unrelated rects in chrome.
    const bars = page.locator('.plot-host svg rect');
    await expect(bars.first()).toBeVisible({ timeout: 5_000 });

    // Outlier label survives normalisation (rendered as "uncategorised").
    await expect(page.getByText(/uncategorised/i).first()).toBeVisible();
  });
});
