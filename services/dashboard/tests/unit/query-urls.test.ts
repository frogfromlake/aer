import { describe, expect, it } from 'vitest';

import {
  articleDetailQuery,
  articleRevisionDiffQuery,
  articleRevisionsQuery,
  contentQuery,
  metadataDistributionQuery,
  metricDistributionQuery,
  metricsAvailableQuery,
  metricsQuery,
  probeDossierQuery,
  probeLeadLagQuery,
  revisionActivityQuery,
  revisionEditClustersQuery,
  revisionsArticlesQuery,
  scopeAvailableMetadataQuery,
  scopeAvailableMetricsQuery,
  sourceArticlesQuery,
  sourceDiscoveryCoverageQuery,
  type FetchContext
} from '../../src/lib/api/queries';

// Phase 142 — branch coverage for the articles / revisions / probes query
// factories. The existing query suites cover the happy outcome; these drive the
// optional-param branches (window, ids, paging, faceting, normalization) by
// capturing the built URL through a mock fetch and asserting the query string.

function capture(): { ctx: FetchContext; url: () => URL } {
  let last = '';
  const fetchFn = (async (url: RequestInfo | URL) => {
    last = String(url);
    return new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } });
  }) as typeof fetch;
  return { ctx: { baseUrl: '/api/v1', fetch: fetchFn }, url: () => new URL(last, 'http://x') };
}

async function urlOf(opt: { queryFn: () => Promise<unknown> }, c: ReturnType<typeof capture>) {
  await opt.queryFn();
  return c.url();
}

const base = { scope: 'probe' as const, scopeId: 'probe-0' };
const win = { start: '2026-01-01T00:00:00Z', end: '2026-02-01T00:00:00Z' };

describe('articles factories — optional-param branches', () => {
  it('sourceArticlesQuery sets every list/paging/filter param', async () => {
    const c = capture();
    const u = await urlOf(
      sourceArticlesQuery(c.ctx, 'tages schau', {
        ...win,
        language: 'de',
        entityMatch: 'Merkel',
        sentimentBand: 'negative',
        limit: 50,
        cursor: 'abc',
        includeRevisions: true
      }),
      c
    );
    expect(u.pathname).toBe('/api/v1/sources/tages%20schau/articles');
    expect(u.searchParams.get('language')).toBe('de');
    expect(u.searchParams.get('entityMatch')).toBe('Merkel');
    expect(u.searchParams.get('sentimentBand')).toBe('negative');
    expect(u.searchParams.get('limit')).toBe('50');
    expect(u.searchParams.get('cursor')).toBe('abc');
    expect(u.searchParams.get('includeRevisions')).toBe('true');
  });

  it('sourceArticlesQuery with the default empty params sets nothing', async () => {
    const c = capture();
    const u = await urlOf(sourceArticlesQuery(c.ctx, 's'), c);
    expect(u.search).toBe('');
  });

  it('articleDetailQuery appends metricName only when given', async () => {
    const c = capture();
    const withMetric = await urlOf(articleDetailQuery(c.ctx, 'a/1', 'sentiment_score'), c);
    expect(withMetric.pathname).toBe('/api/v1/articles/a%2F1');
    expect(withMetric.searchParams.get('metricName')).toBe('sentiment_score');
    const c2 = capture();
    const without = await urlOf(articleDetailQuery(c2.ctx, 'a1'), c2);
    expect(without.search).toBe('');
  });

  it('metricsAvailableQuery sets the date window when present', async () => {
    const c = capture();
    const u = await urlOf(
      metricsAvailableQuery(c.ctx, { startDate: '2026-01-01', endDate: '2026-02-01' }),
      c
    );
    expect(u.searchParams.get('startDate')).toBe('2026-01-01');
    expect(u.searchParams.get('endDate')).toBe('2026-02-01');
  });

  it('scopeAvailableMetrics + Metadata set scope/probeIds/sourceIds/window when present, skip empties', async () => {
    const c = capture();
    const u = await urlOf(
      scopeAvailableMetricsQuery(c.ctx, {
        scope: 'probe',
        scopeId: 'probe-0',
        probeIds: ['probe-0', 'probe-1'],
        sourceIds: ['tagesschau'],
        ...win
      }),
      c
    );
    expect(u.searchParams.get('probeIds')).toBe('probe-0,probe-1');
    expect(u.searchParams.get('sourceIds')).toBe('tagesschau');
    expect(u.searchParams.get('start')).toBe(win.start);

    const c2 = capture();
    const u2 = await urlOf(
      scopeAvailableMetadataQuery(c2.ctx, { probeIds: [], sourceIds: [] }),
      c2
    );
    // empty id lists are skipped.
    expect(u2.search).toBe('');
  });

  it('metricDistributionQuery sets bins + facet; encodes the metric name', async () => {
    const c = capture();
    const u = await urlOf(
      metricDistributionQuery(c.ctx, 'sentiment/raw', {
        ...base,
        ...win,
        bins: 30,
        metadataFilter: { field: 'section', value: 'Politik' }
      }),
      c
    );
    expect(u.pathname).toBe('/api/v1/metrics/sentiment%2Fraw/distribution');
    expect(u.searchParams.get('bins')).toBe('30');
    expect(u.searchParams.get('metadataFilterField')).toBe('section');
  });

  it('metadataDistributionQuery sets topN', async () => {
    const c = capture();
    const u = await urlOf(metadataDistributionQuery(c.ctx, 'author', { ...base, topN: 12 }), c);
    expect(u.pathname).toBe('/api/v1/metadata/author/distribution');
    expect(u.searchParams.get('topN')).toBe('12');
  });
});

describe('revisions factories — optional-param branches', () => {
  it('revisionActivityQuery sets startDate/endDate when present', async () => {
    const c = capture();
    const u = await urlOf(
      revisionActivityQuery(c.ctx, { ...base, ...win, resolution: 'daily' }),
      c
    );
    expect(u.pathname).toBe('/api/v1/revisions');
    expect(u.searchParams.get('startDate')).toBe(win.start);
    expect(u.searchParams.get('resolution')).toBe('daily');
  });

  it('revisionEditClustersQuery sets minSources when > 0', async () => {
    const c = capture();
    const u = await urlOf(
      revisionEditClustersQuery(c.ctx, { ...base, resolution: 'weekly', minSources: 3 }),
      c
    );
    expect(u.searchParams.get('minSources')).toBe('3');
  });

  it('revisionsArticlesQuery sets every filter; minChainLength only when > 1', async () => {
    const c = capture();
    const u = await urlOf(
      revisionsArticlesQuery(c.ctx, {
        ...base,
        ...win,
        hasHeadlineChange: true,
        minChainLength: 3,
        limit: 20,
        cursor: 'cur'
      }),
      c
    );
    expect(u.searchParams.get('hasHeadlineChange')).toBe('true');
    expect(u.searchParams.get('minChainLength')).toBe('3');
    expect(u.searchParams.get('limit')).toBe('20');
    expect(u.searchParams.get('cursor')).toBe('cur');

    // minChainLength of 1 is the no-op default → omitted.
    const c2 = capture();
    const u2 = await urlOf(
      revisionsArticlesQuery(c2.ctx, { ...base, minChainLength: 1, hasHeadlineChange: false }),
      c2
    );
    expect(u2.searchParams.has('minChainLength')).toBe(false);
    expect(u2.searchParams.has('hasHeadlineChange')).toBe(false);
  });

  it('articleRevisions + diff encode the article id path', async () => {
    const c = capture();
    const u = await urlOf(articleRevisionsQuery(c.ctx, 'a/1'), c);
    expect(u.pathname).toBe('/api/v1/articles/a%2F1/revisions');
    const c2 = capture();
    const u2 = await urlOf(articleRevisionDiffQuery(c2.ctx, 'a/1', 2), c2);
    expect(u2.pathname).toBe('/api/v1/articles/a%2F1/revisions/2/diff');
  });
});

describe('probes factories — optional-param branches', () => {
  it('metricsQuery sets every metric param and picks the zscore refusal kind', async () => {
    const c = capture();
    const u = await urlOf(
      metricsQuery(c.ctx, {
        startDate: '2026-01-01',
        endDate: '2026-02-01',
        source: 'tagesschau',
        sourceIds: 'tagesschau,zeit',
        metricName: 'sentiment_score',
        normalization: 'zscore',
        resolution: 'daily',
        includeStddev: true
      }),
      c
    );
    expect(u.searchParams.get('sourceIds')).toBe('tagesschau,zeit');
    expect(u.searchParams.get('normalization')).toBe('zscore');
    expect(u.searchParams.get('resolution')).toBe('daily');
    expect(u.searchParams.get('includeStddev')).toBe('true');
  });

  it('metricsQuery with the default empty params sets nothing', async () => {
    const c = capture();
    const u = await urlOf(metricsQuery(c.ctx), c);
    expect(u.search).toBe('');
  });

  it('probeLeadLagQuery sets the window + maxLagHours', async () => {
    const c = capture();
    const u = await urlOf(
      probeLeadLagQuery(c.ctx, 'probe-0', {
        comparedTo: 'probe-1',
        ...win,
        maxLagHours: 48
      }),
      c
    );
    expect(u.pathname).toBe('/api/v1/probes/probe-0/lead-lag');
    expect(u.searchParams.get('comparedTo')).toBe('probe-1');
    expect(u.searchParams.get('maxLagHours')).toBe('48');
  });

  it('probeDossierQuery appends the window only when present', async () => {
    const c = capture();
    const u = await urlOf(probeDossierQuery(c.ctx, 'probe-0', { windowStart: win.start }), c);
    expect(u.searchParams.get('windowStart')).toBe(win.start);
    const c2 = capture();
    const u2 = await urlOf(probeDossierQuery(c2.ctx, 'probe-0'), c2);
    expect(u2.search).toBe('');
  });

  it('contentQuery routes the entity type/id with the locale', async () => {
    const c = capture();
    const u = await urlOf(contentQuery(c.ctx, 'view_mode', 'howto/distribution', 'de'), c);
    expect(u.pathname).toBe('/api/v1/content/view_mode/howto%2Fdistribution');
    expect(u.searchParams.get('locale')).toBe('de');
  });

  it('sourceDiscoveryCoverageQuery appends windowDays only when provided', async () => {
    const c = capture();
    const u = await urlOf(sourceDiscoveryCoverageQuery(c.ctx, 'tagesschau', 30), c);
    expect(u.searchParams.get('windowDays')).toBe('30');
    const c2 = capture();
    const u2 = await urlOf(sourceDiscoveryCoverageQuery(c2.ctx, 'tagesschau'), c2);
    expect(u2.search).toBe('');
  });
});
