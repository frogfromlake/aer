import { expect, test, type Route } from '@playwright/test';

// Phase 121b — Iteration 6 dashboard closure E2E coverage.
//
// Two assertions:
//   1. Each of the three sentiment metric URLs (`/reflection/metric/<name>`)
//      renders without surfacing the "Provenance data is not available"
//      error banner — converts the Phase-119 manual smoke check into a
//      regression-locked test.
//   2. A Wikidata-linked entity node in the cooccurrence graph renders an
//      external link whose `href` matches the canonical Wikidata pattern;
//      a node without `wikidataQid` does not render an external-link
//      affordance.
//
// All BFF routes are mocked (mirrors atmosphere.spec.ts / topic-views.spec.ts)
// so the test runs against `pnpm preview` without the live backend stack.

const PROBE_ID = 'probe-0-de-institutional-rss';
const FUNCTION_KEY = 'epistemic_authority';

const SENTIMENT_METRICS = [
  'sentiment_score_sentiws',
  'sentiment_score_bert_multilingual',
  'sentiment_score_bert_de_news'
] as const;

function provenancePayload(metricName: string) {
  return {
    metricName,
    tierClassification: metricName === 'sentiment_score_sentiws' ? 1 : 2,
    algorithmDescription: 'Mocked algorithm description for ' + metricName + '.',
    knownLimitations: ['mock-limitation-a', 'mock-limitation-b'],
    validationStatus: 'unvalidated',
    extractorVersionHash: 'mock-hash',
    culturalContextNotes: ''
  };
}

function contentPayload(metricName: string) {
  return {
    entityId: metricName,
    entityType: 'metric',
    locale: 'en',
    contentVersion: 'v2026-05-test',
    lastReviewedBy: 'e2e-fixture',
    lastReviewedAt: '2026-05-06',
    registers: {
      semantic: { short: 'Mocked semantic short.', long: 'Mocked semantic long.' },
      methodological: {
        short: 'Mocked methodological short.',
        long: 'Mocked methodological long.'
      }
    },
    workingPaperAnchors: ['WP-002 §3', 'ADR-016', 'ADR-023']
  };
}

async function mockSentimentReflection(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/metrics/*/provenance', async (route: Route) => {
    const url = route.request().url();
    const m = url.match(/metrics\/([^/]+)\/provenance/);
    const metricName = m?.[1] ? decodeURIComponent(m[1]) : 'unknown';
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(provenancePayload(metricName))
    });
  });
  await page.route('**/api/v1/content/metric/**', async (route: Route) => {
    const url = route.request().url();
    const m = url.match(/content\/metric\/([^?]+)/);
    const metricName = m?.[1] ? decodeURIComponent(m[1]) : 'unknown';
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(contentPayload(metricName))
    });
  });
}

test.describe('Phase 121b — sentiment metric reflection pages', () => {
  for (const metricName of SENTIMENT_METRICS) {
    test(`/reflection/metric/${metricName} renders without an error banner`, async ({ page }) => {
      await mockSentimentReflection(page);
      await page.goto(`/reflection/metric/${metricName}`);

      // Title is rendered as a <code> inside the metric-title row.
      await expect(page.locator('h1.metric-title code')).toContainText(metricName);

      // The "not available" banner must NOT appear when both provenance
      // and content are present.
      await expect(page.getByText(/Provenance data for/i)).toHaveCount(0);
      await expect(page.getByText(/is not available\./i)).toHaveCount(0);
    });
  }
});

// ---------------------------------------------------------------------------
// Phase 118 / 121b — Wikidata external-link affordance on the cooccurrence
// graph. Asserts on the rendered `href` attribute; we deliberately do NOT
// navigate (the link is `target="_blank"` to a third-party host).
// ---------------------------------------------------------------------------

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

const cooccurrencePayload = {
  scope: 'probe',
  scopeId: PROBE_ID,
  windowStart: '2026-04-20T00:00:00Z',
  windowEnd: '2026-04-27T00:00:00Z',
  topN: 60,
  nodes: [
    {
      text: 'Douglas Adams',
      label: 'PER',
      degree: 1,
      totalCount: 12,
      wikidataQid: 'Q42'
    },
    {
      text: 'Unknown Entity',
      label: 'MISC',
      degree: 1,
      totalCount: 5,
      wikidataQid: null
    }
  ],
  edges: [
    {
      a: 'Douglas Adams',
      b: 'Unknown Entity',
      aLabel: 'PER',
      bLabel: 'MISC',
      weight: 3,
      articleCount: 1
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
    lastReviewedAt: '2026-05-06',
    registers: {
      semantic: { short: 'Mock.', long: 'Mock.' },
      methodological: { short: 'Mock.', long: 'Mock.' }
    }
  };
}

async function mockCooccurrence(page: import('@playwright/test').Page) {
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
  await page.route('**/api/v1/entities/cooccurrence?**', async (route: Route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(cooccurrencePayload)
    });
  });
  // Article preview list endpoint — return empty so the panel does not
  // error out when an entity is selected.
  await page.route('**/api/v1/articles?**', async (route: Route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: [], cursor: null })
    });
  });
}

test.describe('Phase 118 / 121b — Wikidata link on cooccurrence node', () => {
  test('linked entity renders an <a> to the canonical Wikidata QID URL; unlinked entity does not', async ({
    page
  }) => {
    await mockCooccurrence(page);
    await page.goto(`/lanes/${PROBE_ID}/${FUNCTION_KEY}?viewMode=cooccurrence_network`);

    // Wait for the graph to mount: at least one node group must be visible.
    await expect(page.locator('svg.graph g.node').first()).toBeVisible({ timeout: 10_000 });

    // Click the linked node ("Douglas Adams" → Q42).
    const linkedNode = page.locator('svg.graph g.node').filter({ hasText: 'Douglas Adams' });
    await linkedNode.first().click();

    // External-link affordance is present and points at Wikidata's canonical
    // URL pattern (assert on href; do NOT navigate).
    const externalLinks = page.locator('[data-testid="entity-external-links"] a.ext-link');
    await expect(externalLinks).toHaveCount(2);
    await expect(externalLinks.nth(0)).toHaveAttribute('href', 'https://www.wikidata.org/wiki/Q42');
    await expect(externalLinks.nth(1)).toHaveAttribute(
      'href',
      'https://www.wikidata.org/wiki/Special:GoToLinkedPage/enwiki/Q42'
    );
    // a11y: the link announces "external".
    await expect(externalLinks.nth(0)).toHaveAttribute('aria-label', /external/i);

    // Now select the unlinked entity. Close the current panel first.
    await page.locator('button.close-btn').click();
    const unlinkedNode = page.locator('svg.graph g.node').filter({ hasText: 'Unknown Entity' });
    await unlinkedNode.first().click();

    // Panel is open but the external-link affordance is NOT rendered.
    await expect(page.locator('[data-testid="entity-external-links"]')).toHaveCount(0);
  });
});
