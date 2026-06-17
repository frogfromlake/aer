import { describe, expect, it } from 'vitest';

import {
  articleRevisionDiffQuery,
  articleRevisionsQuery,
  revisionActivityQuery,
  revisionDiscourseShiftQuery,
  revisionEditClustersQuery,
  revisionsArticlesQuery,
  type FetchContext
} from '../../src/lib/api/queries';

type FetchFn = typeof globalThis.fetch;

function capture(body: unknown = {}) {
  const calls: { url: string; init: RequestInit | undefined }[] = [];
  const fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ url: String(input), init });
    return new Response(JSON.stringify(body), {
      status: 200,
      headers: { 'Content-Type': 'application/json' }
    });
  }) as unknown as FetchFn;
  return { calls, ctx: { baseUrl: '/api/v1', fetch } as FetchContext };
}

const ACTIVITY = {
  scope: 'source' as const,
  scopeId: 's1',
  start: '2026-01-01',
  end: '2026-02-01',
  resolution: 'daily' as const
};

describe('revisionActivityQuery', () => {
  it('maps the window to startDate/endDate + resolution on /revisions', async () => {
    const { ctx, calls } = capture({ buckets: [] });
    const outcome = await revisionActivityQuery(ctx, ACTIVITY).queryFn();
    expect(outcome.kind).toBe('success');
    const url = calls[0]!.url;
    expect(url).toContain('/revisions?');
    expect(url).toContain('startDate=2026-01-01');
    expect(url).toContain('resolution=daily');
  });
});

describe('revisionDiscourseShiftQuery', () => {
  it('reads the discourse-shift aggregation', async () => {
    const { ctx, calls } = capture({ sources: [] });
    await revisionDiscourseShiftQuery(ctx, ACTIVITY).queryFn();
    expect(calls[0]!.url).toContain('/revisions/discourse-shift?');
  });
});

describe('revisionEditClustersQuery', () => {
  it('forwards the minSources coincidence threshold', async () => {
    const { ctx, calls } = capture({ clusters: [] });
    await revisionEditClustersQuery(ctx, { ...ACTIVITY, minSources: 3 }).queryFn();
    expect(calls[0]!.url).toContain('/revisions/edit-clusters?');
    expect(calls[0]!.url).toContain('minSources=3');
  });
});

describe('articleRevisionsQuery', () => {
  it('encodes the article id into the per-article revisions path', async () => {
    const { ctx, calls } = capture({ revisions: [] });
    await articleRevisionsQuery(ctx, 'a/b id').queryFn();
    expect(calls[0]!.url).toContain('/articles/a%2Fb%20id/revisions');
  });
});

describe('revisionsArticlesQuery', () => {
  it('forwards the drilldown filters + cursor', async () => {
    const { ctx, calls } = capture({ articles: [], nextCursor: null });
    await revisionsArticlesQuery(ctx, {
      scope: 'probe',
      scopeId: 'p1',
      hasHeadlineChange: true,
      minChainLength: 3,
      limit: 25,
      cursor: 'abc'
    }).queryFn();
    const url = calls[0]!.url;
    expect(url).toContain('/revisions/articles?');
    expect(url).toContain('hasHeadlineChange=true');
    expect(url).toContain('minChainLength=3');
    expect(url).toContain('cursor=abc');
  });

  it('omits minChainLength when not greater than 1', async () => {
    const { ctx, calls } = capture({ articles: [] });
    await revisionsArticlesQuery(ctx, {
      scope: 'source',
      scopeId: 's1',
      minChainLength: 1
    }).queryFn();
    expect(calls[0]!.url).not.toContain('minChainLength');
  });
});

describe('articleRevisionDiffQuery', () => {
  it('builds the per-revision diff path', async () => {
    const { ctx, calls } = capture({ paragraphs: [] });
    await articleRevisionDiffQuery(ctx, 'a1', 2).queryFn();
    expect(calls[0]!.url).toContain('/articles/a1/revisions/2/diff');
  });
});
