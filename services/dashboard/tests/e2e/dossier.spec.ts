import { expect, test, type Route, type Page } from './_fixtures';

// Phase 141 — ProbeCard characterization net (Dossier overlay).
//
// Safety harness for the ProbeCard decomposition (the ProbeDfCards child). It
// pins ProbeCard's rendered regions a markup/scoped-CSS sub-componentisation
// could silently break: the header (title / machine id / language badge), the
// capability matrix, and the Discourse-Function cards (covered vs uncovered +
// the per-card expand toggle revealing the source grid).
//
// The Dossier is a global overlay mounted in the (app) layout, opened by
// `?dossier=open`; a probe carried in `?selectedProbes=` auto-expands its
// ProbeCard. So a single seed over the root page renders an expanded card with
// no Workbench pillar state. All BFF routes mocked (mirrors workbench.spec.ts).

const PROBE_ID = 'probe-0-de-institutional-web';

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

function genericContent(entityId: string) {
  return {
    entityId,
    entityType: 'probe',
    locale: 'en',
    contentVersion: 'v2026-05-test',
    lastReviewedBy: 'e2e-fixture',
    lastReviewedAt: '2026-05-01',
    registers: {
      semantic: { short: 'Mock semantic.', long: 'Mock emic frame paragraph.' },
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
  await page.route(/\/api\/v1\/metrics\?/, (route: Route) => route.fulfill(json({ data: [] })));
}

const DOSSIER_URL = `/?dossier=open&selectedProbes=${PROBE_ID}`;

test.describe('Phase 141 — ProbeCard characterization', () => {
  test.beforeEach(async ({ page }) => {
    await mockBff(page);
  });

  test('renders the expanded card header, capability matrix, and DF cards', async ({ page }) => {
    await page.goto(DOSSIER_URL);

    const card = page.locator(`article.probe-card#probe-${PROBE_ID}`);
    await expect(card).toBeVisible();

    // Header — display name (title), machine id, language badge.
    await expect(
      card.getByRole('heading', { name: 'Probe 0 — German institutional web' })
    ).toBeVisible();
    await expect(card.locator('.probe-id')).toHaveText(PROBE_ID);
    await expect(card.locator('.lang-badge')).toHaveText('de');

    // Capability matrix (Phase 123a).
    await expect(card.getByRole('heading', { name: 'Capabilities' })).toBeVisible();
    await expect(card.getByText('Sentiment backbone')).toBeVisible();

    // Discourse-Function cards — the four canonical functions render; two are
    // covered (one source each) and the rest read "uncovered".
    await expect(card.getByRole('heading', { name: 'Discourse Functions' })).toBeVisible();
    await expect(card.locator('.df-list > li > .df-card')).toHaveCount(4);
    await expect(card.locator('.df-card.covered')).toHaveCount(2);
    await expect(
      card.locator('.df-card:not(.covered)').first().getByText('uncovered')
    ).toBeVisible();
  });

  test('expanding a covered DF card reveals its source grid', async ({ page }) => {
    await page.goto(DOSSIER_URL);

    const card = page.locator(`article.probe-card#probe-${PROBE_ID}`);
    const coveredCard = card.locator('.df-card.covered').first();

    // Closed by default — no body.
    await expect(coveredCard.locator('.df-body')).toHaveCount(0);

    await coveredCard.getByRole('button').click();

    // The body opens with the source grid (one source-card for the covered fn).
    await expect(coveredCard.locator('.df-body')).toBeVisible();
    await expect(coveredCard.locator('.source-grid > li')).toHaveCount(1);
  });
});
