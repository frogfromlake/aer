import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  addShare,
  createAnalysis,
  deleteAnalysis,
  getAnalysis,
  listAnalyses,
  listShares,
  removeShare,
  updateAnalysis
} from '../../src/lib/api/analyses';

let fetchMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
  fetchMock = vi.fn();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

function resolveWith(status: number, body?: unknown) {
  fetchMock.mockResolvedValueOnce(
    new Response(body === undefined ? null : JSON.stringify(body), {
      status,
      headers: { 'Content-Type': 'application/json' }
    })
  );
}

describe('listAnalyses / getAnalysis', () => {
  it('GETs the analyses collection', async () => {
    resolveWith(200, { analyses: [] });
    const r = await listAnalyses();
    expect(r.ok).toBe(true);
    expect(fetchMock.mock.calls[0]![0]).toBe('/api/v1/analyses');
  });

  it('GETs a single analysis by id', async () => {
    resolveWith(200, { id: 'an1', name: 'A', state: '{}' });
    await getAnalysis('an1');
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe('/api/v1/analyses/an1');
    expect(init.method).toBe('GET');
  });
});

describe('createAnalysis / updateAnalysis / deleteAnalysis', () => {
  it('POSTs name/description/state', async () => {
    resolveWith(200, { id: 'an2' });
    await createAnalysis('My A', 'desc', '{"x":1}');
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe('/api/v1/analyses');
    expect(init.method).toBe('POST');
    expect(JSON.parse(init.body)).toEqual({ name: 'My A', description: 'desc', state: '{"x":1}' });
  });

  it('PATCHes a partial update', async () => {
    resolveWith(200, { id: 'an2', name: 'renamed' });
    await updateAnalysis('an2', { name: 'renamed' });
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe('/api/v1/analyses/an2');
    expect(init.method).toBe('PATCH');
    expect(JSON.parse(init.body)).toEqual({ name: 'renamed' });
  });

  it('DELETEs and treats 204 as success', async () => {
    resolveWith(204);
    const r = await deleteAnalysis('an2');
    expect(r.ok).toBe(true);
    expect(fetchMock.mock.calls[0]![1].method).toBe('DELETE');
  });
});

describe('shares', () => {
  it('lists shares for an analysis', async () => {
    resolveWith(200, { shares: [] });
    await listShares('an1');
    expect(fetchMock.mock.calls[0]![0]).toBe('/api/v1/analyses/an1/shares');
  });

  it('adds a share with the grantee email + canEdit', async () => {
    resolveWith(200, { granteeId: 'g1', email: 'c@d.de', canEdit: true });
    await addShare('an1', 'c@d.de', true);
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe('/api/v1/analyses/an1/shares');
    expect(JSON.parse(init.body)).toEqual({ email: 'c@d.de', canEdit: true });
  });

  it('removes a share by grantee id', async () => {
    resolveWith(204);
    await removeShare('an1', 'g1');
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe('/api/v1/analyses/an1/shares/g1');
    expect(init.method).toBe('DELETE');
  });

  it('surfaces a typed error on a failed share add', async () => {
    resolveWith(403, { code: 'forbidden_not_shared', message: 'no' });
    const r = await addShare('an1', 'c@d.de', false);
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.code).toBe('forbidden_not_shared');
  });
});
