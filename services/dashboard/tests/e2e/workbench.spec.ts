import { expect, test, type Route, type Page } from './_fixtures';

// Phase 141 — Workbench PanelControls characterization net.
//
// The safety harness for the PanelControls decomposition (constraint #3 of the
// giants refactor): it pins the per-lever control strip's RENDERED structure
// and the click→URL reactivity that a markup/CSS sub-componentisation could
// silently break. It is deliberately behavioural, not pixel-based — it must stay
// green across every child-component extraction while the internal markup moves.
//
// Seeds the canonical three-surface grammar directly:
//   /workbench?activePillar=aleph&aleph=<base64url-json>
// where the decoded PillarState is a single Aleph Window with one focused
// distribution Panel over Probe 0:
//   { w:[{ p:[{ s:[{ pi:['probe-0-de-institutional-web'], si:[] }],
//                  c:'m', v:'distribution', m:'sentiment_score_sentiws',
//                  l:'g' }], fi:0 }], aw:0 }
// (computed once via encodePillarState; hardcoded so the spec needs no app-code
// import). A focused Panel mounts PanelControls (PanelHost renders it only when
// `focused`), so the lever strip is present on load.
//
// All BFF routes are mocked so the test runs against `pnpm preview` with no live
// backend (mirrors atmosphere.spec.ts / topic-views.spec.ts).

const PROBE_ID = 'probe-0-de-institutional-web';

// encodePillarState({ windows:[{ panels:[{ scopes:[{ probeIds:[PROBE_ID],
//   sourceIds:[] }], composition:'merged', view:'distribution',
//   metric:'sentiment_score_sentiws', layer:'gold' }], focusedPanelIndex:0 }],
//   activeWindowIndex:0 })
const ALEPH_SEED =
  'eyJ3IjpbeyJwIjpbeyJzIjpbeyJwaSI6WyJwcm9iZS0wLWRlLWluc3RpdHV0aW9uYWwtd2ViIl0sInNpIjpbXX1dLCJjIjoibSIsInYiOiJkaXN0cmlidXRpb24iLCJtIjoic2VudGltZW50X3Njb3JlX3NlbnRpd3MiLCJsIjoiZyJ9XSwiZmkiOjB9XSwiYXciOjB9';

const WORKBENCH_URL = `/workbench?activePillar=aleph&aleph=${ALEPH_SEED}`;

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

function genericContent(entityId: string) {
  return {
    entityId,
    entityType: 'view_mode',
    locale: 'en',
    contentVersion: 'v2026-05-test',
    lastReviewedBy: 'e2e-fixture',
    lastReviewedAt: '2026-05-01',
    registers: {
      semantic: { short: 'Mock semantic.', long: 'Mock semantic long.' },
      methodological: { short: 'Mock methodological.', long: 'Mock methodological long.' }
    }
  };
}

const json = (body: unknown) => ({
  status: 200,
  contentType: 'application/json',
  body: JSON.stringify(body)
});

async function mockBff(page: Page) {
  // Catch-all FIRST: an UNMOCKED endpoint 401s against the backend-less preview,
  // and any data-layer 401 trips the global unauthenticated redirect
  // (/workbench → /login → /), so the Workbench would never mount. A blanket 200
  // makes that impossible; the specific shaped routes below win (last-wins). The
  // auth/me re-assert restores the fixture's active session over the catch-all.
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
  // The distribution cell's data endpoint — kept 200 so the cell renders instead
  // of retry-storming a network error (irrelevant to the control-strip assertions).
  await page.route(/\/api\/v1\/metrics\/[^/]+\/distribution/, (route: Route) =>
    route.fulfill(json(distributionPayload))
  );
  // Metric provenance (CellMethodology) — valid shape so its `knownLimitations`
  // array read does not throw against the catch-all's `{}`.
  await page.route(/\/api\/v1\/metrics\/[^/]+\/provenance/, (route: Route) =>
    route.fulfill(
      json({
        metricName: 'sentiment_score_sentiws',
        tierClassification: 1,
        algorithmDescription: 'SentiWS lexical polarity (mock).',
        knownLimitations: [],
        validationStatus: 'unvalidated',
        extractorVersionHash: 'sha256:e2e-mock'
      })
    )
  );
  // Globe activity series (Phase 135 persistent globe) — empty.
  await page.route('**/api/v1/metrics?**', (route: Route) =>
    route.fulfill(json({ data: [], excludedCount: 0 }))
  );
}

test.describe('Phase 141 — Workbench PanelControls characterization', () => {
  test.beforeEach(async ({ page }) => {
    await mockBff(page);
  });

  test('renders the per-lever control strip for a focused distribution panel', async ({ page }) => {
    await page.goto(WORKBENCH_URL);

    // The panel control strip is present (PanelHost mounts it for the focused panel).
    const strip = page.getByRole('region', { name: 'Panel controls' });
    await expect(strip).toBeVisible();

    // View lever — radiogroup with Distribution active.
    const viewGroup = strip.getByRole('radiogroup', { name: 'View' });
    await expect(viewGroup).toBeVisible();
    await expect(
      viewGroup.getByRole('radio', { name: 'Distribution', exact: true })
    ).toHaveAttribute('aria-checked', 'true');

    // Composition lever — Merged active, Split offered.
    const compGroup = strip.getByRole('radiogroup', { name: 'Composition' });
    await expect(compGroup.getByRole('radio', { name: 'Merged', exact: true })).toHaveAttribute(
      'aria-checked',
      'true'
    );
    await expect(compGroup.getByRole('radio', { name: 'Split', exact: true })).toBeVisible();

    // Metric lever — sentiment is the bound metric.
    const metricGroup = strip.getByRole('radiogroup', { name: 'Metric' });
    await expect(
      metricGroup.getByRole('radio', { name: 'sentiment_score_sentiws' })
    ).toHaveAttribute('aria-checked', 'true');

    // Config lever — distribution declares `bins`, so the Histogram bins group renders.
    await expect(strip.getByRole('group', { name: 'Histogram bins' })).toBeVisible();
  });

  test('clicking Split re-encodes the ?aleph= pillar state and reveals Direction', async ({
    page
  }) => {
    await page.goto(WORKBENCH_URL);

    const strip = page.getByRole('region', { name: 'Panel controls' });
    const compGroup = strip.getByRole('radiogroup', { name: 'Composition' });
    await expect(compGroup.getByRole('radio', { name: 'Merged', exact: true })).toHaveAttribute(
      'aria-checked',
      'true'
    );

    const alephBefore = new URL(page.url()).searchParams.get('aleph');
    expect(alephBefore).toBe(ALEPH_SEED);

    await compGroup.getByRole('radio', { name: 'Split', exact: true }).click();

    // The mutation re-encodes the Aleph pillar state into the URL (composition m→s).
    await expect.poll(() => new URL(page.url()).searchParams.get('aleph')).not.toBe(ALEPH_SEED);

    // Split is now active and the split-direction sub-lever appears.
    await expect(compGroup.getByRole('radio', { name: 'Split', exact: true })).toHaveAttribute(
      'aria-checked',
      'true'
    );
    await expect(strip.getByRole('radiogroup', { name: 'Split direction' })).toBeVisible();
  });
});
