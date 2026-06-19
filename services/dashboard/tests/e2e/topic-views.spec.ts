import { expect, test, type Route, type Request, type Page } from './_fixtures';

// Phase 121 → rewritten in Phase 127.
//
// topic_distribution is an Aleph presentation rendered as a Workbench cell under
// the canonical three-surface grammar (`/workbench?activePillar=aleph&aleph=<base64url-json>`),
// not the retired `/lanes/{id}/{fn}?viewMode=` route. Asserts the cell is wired:
//   1. mounting the topic_distribution cell calls `GET /api/v1/topics/distribution`
//      with the panel's resolved scope (`probeIds=`);
//   2. the cell renders at least one topic ridge (a Plot <rect>).
//
// All BFF routes are mocked so the test runs against `pnpm preview` with no live
// backend (mirrors workbench.spec.ts — catch-all 200 first, shaped routes win).

const PROBE_ID = 'probe-0-de-institutional-web';

// encodePillarState({ windows:[{ panels:[{ scopes:[{ probeIds:[PROBE_ID],
//   sourceIds:[] }], composition:'merged', view:'topic_distribution',
//   metric:'sentiment_score_sentiws', layer:'gold' }], focusedPanelIndex:0 }],
//   activeWindowIndex:0 }) — computed once via the app encoder, hardcoded.
const TOPIC_SEED =
  'eyJ3IjpbeyJwIjpbeyJzIjpbeyJwaSI6WyJwcm9iZS0wLWRlLWluc3RpdHV0aW9uYWwtd2ViIl0sInNpIjpbXX1dLCJjIjoibSIsInYiOiJ0b3BpY19kaXN0cmlidXRpb24iLCJtIjoic2VudGltZW50X3Njb3JlX3NlbnRpd3MiLCJsIjoiZyJ9XSwiZmkiOjB9XSwiYXciOjB9';

const WORKBENCH_URL = `/workbench?activePillar=aleph&aleph=${TOPIC_SEED}`;

const probesPayload = [
  {
    probeId: PROBE_ID,
    displayName: 'Probe 0 — German institutional web',
    shortName: 'Probe 0',
    language: 'de',
    sources: ['tagesschau', 'bundesregierung'],
    emissionPoints: [{ latitude: 52.52, longitude: 13.405, label: 'Berlin' }]
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
      type: 'web',
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

const availableMetricsPayload = [
  { metricName: 'sentiment_score_sentiws', validationStatus: 'unvalidated' },
  { metricName: 'word_count', validationStatus: 'unvalidated' }
];

const scopeAvailableMetricsPayload = {
  scopedSources: ['tagesschau', 'bundesregierung'],
  available: ['sentiment_score_sentiws', 'word_count'],
  partial: []
};

const scopeAvailableMetadataPayload = {
  scopedSources: ['tagesschau', 'bundesregierung'],
  available: [],
  partial: []
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
      modelHash: 'sha256:demo'
    },
    {
      topicId: 1,
      label: 'inflation_economy',
      articleCount: 27,
      avgConfidence: 0.68,
      language: 'de',
      modelHash: 'sha256:demo'
    },
    {
      topicId: -1,
      label: '-1_misc_words',
      articleCount: 9,
      avgConfidence: 0.0,
      language: 'de',
      modelHash: 'sha256:demo'
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
      methodological: { short: 'BERTopic Tier 2 (mock).', long: 'Long methodological copy (mock).' }
    }
  };
}

const json = (body: unknown) => ({
  status: 200,
  contentType: 'application/json',
  body: JSON.stringify(body)
});

async function mockBff(page: Page) {
  // Catch-all FIRST so an unmocked endpoint never 401s into the auth redirect.
  await page.route('**/api/v1/**', (route: Route) => route.fulfill(json({})));
  await page.route('**/api/v1/auth/me', (route: Route) =>
    route.fulfill(
      json({ id: 'e2e-user', email: 'e2e@aer.test', role: 'researcher', status: 'active' })
    )
  );
  await page.route('**/api/v1/probes', (route: Route) => route.fulfill(json(probesPayload)));
  await page.route(`**/api/v1/probes/${PROBE_ID}/dossier**`, (route: Route) =>
    route.fulfill(json(dossierPayload))
  );
  await page.route('**/api/v1/content/**', (route: Route) => {
    const m = route
      .request()
      .url()
      .match(/content\/[^/]+\/([^?]+)/);
    const id = m?.[1] ? decodeURIComponent(m[1]) : 'unknown';
    return route.fulfill(json(genericContent(id)));
  });
  await page.route('**/api/v1/metrics/available**', (route: Route) =>
    route.fulfill(json(availableMetricsPayload))
  );
  await page.route('**/api/v1/scope/available-metrics**', (route: Route) =>
    route.fulfill(json(scopeAvailableMetricsPayload))
  );
  await page.route('**/api/v1/scope/available-metadata**', (route: Route) =>
    route.fulfill(json(scopeAvailableMetadataPayload))
  );
  // Globe activity series (Phase 135 persistent globe) — empty.
  await page.route(/\/api\/v1\/metrics\?/, (route: Route) =>
    route.fulfill(json({ data: [], excludedCount: 0 }))
  );
}

test.describe('Phase 127 — topic_distribution Workbench cell', () => {
  test('mounting the cell calls /topics/distribution and renders a ridge', async ({ page }) => {
    await mockBff(page);

    await page.route('**/api/v1/topics/distribution?**', (route: Route) =>
      route.fulfill(json(topicsPayload))
    );
    const topicRequest: Promise<Request> = page.waitForRequest((req) =>
      req.url().includes('/api/v1/topics/distribution')
    );

    await page.goto(WORKBENCH_URL);

    // The deferred request must hit the BFF with the panel's resolved scope —
    // proves the Aleph topic cell is wired under the three-surface grammar.
    const req = await topicRequest;
    expect(req.url()).toContain('/topics/distribution');
    expect(req.url()).toContain('scope=probe');
    expect(req.url()).toContain(`scopeId=${PROBE_ID}`);

    // At least one rendered Plot <rect> inside the cell's plot host — proves a
    // ridge made it into the DOM.
    const bars = page.locator('.plot-host svg rect');
    await expect(bars.first()).toBeVisible({ timeout: 10_000 });
  });
});
