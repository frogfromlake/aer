import { expect, test, type Route } from './_fixtures';

// Phase 121b — Iteration 6 dashboard closure E2E coverage.
//
// Each of the three sentiment metric URLs (`/reflection/metric/<name>`) renders
// without surfacing the "Provenance data is not available" error banner —
// converts the Phase-119 manual smoke check into a regression-locked test.
// (The former cooccurrence Wikidata-link assertion was removed in Phase 149 —
// see the note at the foot of this file.)
//
// All BFF routes are mocked (mirrors atmosphere.spec.ts / topic-views.spec.ts)
// so the test runs against `pnpm preview` without the live backend stack.

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
  // RegExp (not a glob) so it matches regardless of the trailing query string —
  // the provenance query now appends `?locale=…` (Phase 144 i18n), which the old
  // `**/.../provenance` glob did not match, so the request fell through to a 401
  // and the global auth guard bounced the page to /login → /.
  await page.route(/\/api\/v1\/metrics\/[^/]+\/provenance/, async (route: Route) => {
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
// Phase 149 — the former "Wikidata external-link affordance on the cooccurrence
// graph" E2E was removed here. It drove `svg.graph g.node` DOM nodes, but
// Phase 148g retired the SVG renderer: the cooccurrence cell is now a single
// sigma/WebGL canvas (CoOccurrenceNetworkAtScale), whose nodes are canvas pixels
// — not DOM elements a test can click by name, and laid out non-deterministically
// by ForceAtlas2. The affordance it guarded is otherwise fully covered:
//   • the Wikidata/Wikipedia URL shape + the linked-vs-unlinked gating
//     (`wikidataHref` / `wikipediaHref` / `hasExternalLinks`) are unit-tested in
//     tests/unit/cooccurrence-network-internals.test.ts;
//   • the DOM `<a class="ext-link">` now renders in the canvas selection panel
//     (CoOccurrenceNetworkAtScale.svelte) under `{#if selectedEntity.wikidataQid}`.
// ---------------------------------------------------------------------------
