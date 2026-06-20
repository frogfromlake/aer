// Shared BFF mock for the E2E suite — consolidates the per-spec inline mocks
// (workbench / dossier / cooccurrence) into one reusable helper so the a11y
// surface sweep (a11y-app.spec.ts) can render every reachable Workbench /
// Dossier / Reflection state without re-deriving the route table each time.
//
// The catch-all 200 MUST be registered first: an unmocked endpoint 401s against
// the backend-less `pnpm preview`, and any data-layer 401 trips the global
// unauthenticated redirect (/workbench → /login), so the surface would never
// mount. Shaped routes registered afterwards win (Playwright is last-wins).
import type { Page, Route } from '@playwright/test';

export const PROBE_ID = 'probe-0-de-institutional-web';

// Canonical base64url-json pillar seeds (computed once via the app encoder,
// hardcoded so the spec needs no app-code import — mirrors workbench.spec.ts /
// iteration6-closure.spec.ts).
//
// Aleph: one focused distribution Panel over Probe 0 (sentiment_score_sentiws).
export const ALEPH_SEED =
  'eyJ3IjpbeyJwIjpbeyJzIjpbeyJwaSI6WyJwcm9iZS0wLWRlLWluc3RpdHV0aW9uYWwtd2ViIl0sInNpIjpbXX1dLCJjIjoibSIsInYiOiJkaXN0cmlidXRpb24iLCJtIjoic2VudGltZW50X3Njb3JlX3NlbnRpd3MiLCJsIjoiZyJ9XSwiZmkiOjB9XSwiYXciOjB9';
// Rhizome: one focused cooccurrence_network Panel over Probe 0 (entity_cooccurrence).
export const COOC_SEED =
  'eyJ3IjpbeyJwIjpbeyJzIjpbeyJwaSI6WyJwcm9iZS0wLWRlLWluc3RpdHV0aW9uYWwtd2ViIl0sInNpIjpbXX1dLCJjIjoibSIsInYiOiJjb29jY3VycmVuY2VfbmV0d29yayIsIm0iOiJlbnRpdHlfY29vY2N1cnJlbmNlIiwibCI6ImcifV0sImZpIjowfV0sImF3IjowfQ';

export const ALEPH_WORKBENCH_URL = `/workbench?activePillar=aleph&aleph=${ALEPH_SEED}`;
export const COOC_WORKBENCH_URL = `/workbench?activePillar=rhizome&rhizome=${COOC_SEED}`;
export const DOSSIER_URL = `/?dossier=open&selectedProbes=${PROBE_ID}`;

const probesPayload = [
  {
    probeId: PROBE_ID,
    displayName: 'Probe 0 — German institutional web',
    shortName: 'Probe 0',
    language: 'de',
    country: 'DE',
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
  capabilities: {
    sentimentBackbone: 'sentiws',
    sentimentEnrichments: ['bert_multilingual'],
    silentEditObservability: true,
    discourseFunctionClassifier: 'deferred (per-probe primary function only)'
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
    },
    {
      name: 'bundesregierung',
      type: 'web',
      url: 'https://www.bundesregierung.de',
      articlesTotal: 40,
      articlesInWindow: 8,
      publicationFrequencyPerDay: 1,
      primaryFunction: 'power_legitimation',
      secondaryFunction: null,
      emicDesignation: 'Federal government',
      emicContext: 'German executive press office',
      silverEligible: true
    }
  ]
};

const distributionPayload = {
  metricName: 'sentiment_score_sentiws',
  scope: 'probe',
  scopeId: PROBE_ID,
  windowStart: '2026-04-20T00:00:00Z',
  windowEnd: '2026-04-27T00:00:00Z',
  clampedUpper: 1,
  overflowCount: 0,
  bins: [
    { lower: -1, upper: 0, count: 12 },
    { lower: 0, upper: 1, count: 30 }
  ],
  summary: {
    count: 42,
    min: -1,
    max: 1,
    mean: 0.1,
    median: 0.05,
    p05: -0.8,
    p25: -0.2,
    p75: 0.4,
    p95: 0.9
  }
};

const cooccurrencePayload = {
  scope: 'probe',
  scopeId: PROBE_ID,
  windowStart: '2026-04-20T00:00:00Z',
  windowEnd: '2026-04-27T00:00:00Z',
  topN: 60,
  nodes: [
    { text: 'Douglas Adams', label: 'PER', degree: 1, totalCount: 12, wikidataQid: 'Q42' },
    { text: 'Unknown Entity', label: 'MISC', degree: 1, totalCount: 5, wikidataQid: null }
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

function provenancePayload(metricName: string) {
  return {
    metricName,
    tierClassification: metricName === 'sentiment_score_sentiws' ? 1 : 2,
    algorithmDescription: 'Mocked algorithm description for ' + metricName + '.',
    knownLimitations: [],
    validationStatus: 'unvalidated',
    extractorVersionHash: 'sha256:e2e-mock',
    culturalContextNotes: ''
  };
}

function genericContent(entityId: string, entityType: string) {
  return {
    entityId,
    entityType,
    locale: 'en',
    contentVersion: 'v2026-05-test',
    lastReviewedBy: 'e2e-fixture',
    lastReviewedAt: '2026-05-01',
    registers: {
      semantic: { short: 'Mock semantic.', long: 'Mock semantic long.' },
      methodological: { short: 'Mock methodological.', long: 'Mock methodological long.' }
    },
    workingPaperAnchors: ['WP-002 §3']
  };
}

const json = (body: unknown) => ({
  status: 200,
  contentType: 'application/json',
  body: JSON.stringify(body)
});

const activeMe = {
  id: 'e2e-user',
  email: 'e2e@aer.test',
  role: 'researcher',
  status: 'active'
};

/**
 * Registers the full shaped BFF route table for the authenticated app surfaces
 * (Workbench Aleph/Rhizome cells, the Dossier overlay, Reflection). Call once
 * per test before navigating. `last-wins` lets a caller override any route
 * afterwards (e.g. an auth surface re-routing /auth/me to a 401).
 */
export async function mockFullBff(page: Page): Promise<void> {
  await page.route('**/api/v1/**', (route: Route) => route.fulfill(json({})));
  await page.route('**/api/v1/auth/me', (route: Route) => route.fulfill(json(activeMe)));
  await page.route('**/api/v1/probes', (route: Route) => route.fulfill(json(probesPayload)));
  await page.route(`**/api/v1/probes/${PROBE_ID}/dossier**`, (route: Route) =>
    route.fulfill(json(dossierPayload))
  );
  await page.route('**/api/v1/content/**', (route: Route) => {
    const url = route.request().url();
    const typeMatch = url.match(/content\/([^/]+)\/([^?]+)/);
    const entityType = typeMatch?.[1] ?? 'unknown';
    const id = typeMatch?.[2] ? decodeURIComponent(typeMatch[2]) : 'unknown';
    return route.fulfill(json(genericContent(id, entityType)));
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
  await page.route(/\/api\/v1\/metrics\/[^/]+\/distribution/, (route: Route) =>
    route.fulfill(json(distributionPayload))
  );
  await page.route(/\/api\/v1\/metrics\/[^/]+\/provenance/, (route: Route) => {
    const m = route
      .request()
      .url()
      .match(/metrics\/([^/]+)\/provenance/);
    return route.fulfill(json(provenancePayload(m?.[1] ? decodeURIComponent(m[1]) : 'unknown')));
  });
  await page.route('**/api/v1/entities/cooccurrence?**', (route: Route) =>
    route.fulfill(json(cooccurrencePayload))
  );
  await page.route('**/api/v1/articles?**', (route: Route) =>
    route.fulfill(json({ data: [], cursor: null }))
  );
  // Persistent-globe activity series (Phase 135) — empty so the globe never storms.
  await page.route('**/api/v1/metrics?**', (route: Route) =>
    route.fulfill(json({ data: [], excludedCount: 0 }))
  );
}
