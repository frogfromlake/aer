import { describe, expect, it } from 'vitest';

import {
  probeDossierQuery,
  probeEquivalenceQuery,
  probeLeadLagQuery,
  probeMetadataCoverageQuery,
  provenanceQuery,
  sourceDiscoveryCoverageQuery,
  sourceMetadataCoverageQuery,
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

describe('provenanceQuery', () => {
  it('encodes the metric name into the provenance path', async () => {
    const { ctx, calls } = capture({ tier: 'unvalidated' });
    const outcome = await provenanceQuery(ctx, 'sentiment score').queryFn();
    expect(outcome.kind).toBe('success');
    expect(calls[0]!.url).toContain('/metrics/sentiment%20score/provenance');
  });
});

describe('probeEquivalenceQuery', () => {
  it('builds the per-probe equivalence path', async () => {
    const { ctx, calls } = capture({ comparisons: [] });
    await probeEquivalenceQuery(ctx, 'probe-0').queryFn();
    expect(calls[0]!.url).toContain('/probes/probe-0/equivalence');
  });
});

describe('probeLeadLagQuery', () => {
  it('forwards comparedTo + maxLagHours', async () => {
    const { ctx, calls } = capture({ lags: [] });
    await probeLeadLagQuery(ctx, 'probe-0', {
      comparedTo: 'probe-1',
      start: '2026-01-01',
      maxLagHours: 72
    }).queryFn();
    const url = calls[0]!.url;
    expect(url).toContain('/probes/probe-0/lead-lag?');
    expect(url).toContain('comparedTo=probe-1');
    expect(url).toContain('maxLagHours=72');
  });
});

describe('probeDossierQuery', () => {
  it('omits the query string when no window is given', async () => {
    const { ctx, calls } = capture({ probeId: 'probe-0' });
    await probeDossierQuery(ctx, 'probe-0').queryFn();
    expect(calls[0]!.url).toMatch(/\/probes\/probe-0\/dossier$/);
  });

  it('adds the window when supplied', async () => {
    const { ctx, calls } = capture({ probeId: 'probe-0' });
    await probeDossierQuery(ctx, 'probe-0', {
      windowStart: '2026-01-01',
      windowEnd: '2026-02-01'
    }).queryFn();
    expect(calls[0]!.url).toContain('/probes/probe-0/dossier?');
    expect(calls[0]!.url).toContain('windowStart=2026-01-01');
  });
});

describe('probeMetadataCoverageQuery', () => {
  it('builds the per-probe metadata-coverage path', async () => {
    const { ctx, calls } = capture({ sources: [] });
    await probeMetadataCoverageQuery(ctx, 'probe-0').queryFn();
    expect(calls[0]!.url).toContain('/probes/probe-0/metadata-coverage');
  });
});

describe('sourceMetadataCoverageQuery', () => {
  it('builds the per-source metadata-coverage path', async () => {
    const { ctx, calls } = capture({ fields: [] });
    await sourceMetadataCoverageQuery(ctx, 'tagesschau').queryFn();
    expect(calls[0]!.url).toContain('/sources/tagesschau/metadata-coverage');
  });
});

describe('sourceDiscoveryCoverageQuery', () => {
  it('appends windowDays when provided', async () => {
    const { ctx, calls } = capture({ channels: [] });
    await sourceDiscoveryCoverageQuery(ctx, 'tagesschau', 30).queryFn();
    expect(calls[0]!.url).toContain('/sources/tagesschau/discovery-coverage?windowDays=30');
  });

  it('omits windowDays when not provided', async () => {
    const { ctx, calls } = capture({ channels: [] });
    await sourceDiscoveryCoverageQuery(ctx, 'tagesschau').queryFn();
    expect(calls[0]!.url).toMatch(/\/sources\/tagesschau\/discovery-coverage$/);
  });
});
