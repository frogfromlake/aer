import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  adminCreateUser,
  adminSuspend,
  login,
  logout,
  me,
  passkeyDelete
} from '../../src/lib/api/auth';

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

describe('login', () => {
  it('POSTs credentials and returns the user on 200', async () => {
    resolveWith(200, { id: 'u1', email: 'a@b.de', role: 'researcher', status: 'active' });
    const r = await login('a@b.de', 'pw');
    expect(r.ok).toBe(true);
    if (r.ok) expect(r.data.email).toBe('a@b.de');

    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe('/api/v1/auth/login');
    expect(init.method).toBe('POST');
    expect(init.credentials).toBe('same-origin');
    expect(JSON.parse(init.body)).toEqual({ email: 'a@b.de', password: 'pw' });
  });

  it('returns a typed error with code + status on 401', async () => {
    resolveWith(401, { code: 'invalid_credentials', message: 'no' });
    const r = await login('a@b.de', 'wrong');
    expect(r.ok).toBe(false);
    if (!r.ok) {
      expect(r.code).toBe('invalid_credentials');
      expect(r.status).toBe(401);
    }
  });

  it('falls back to a generic code + status text when the error body is empty', async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(null, { status: 500, statusText: 'Server Error' })
    );
    const r = await login('a@b.de', 'pw');
    expect(r.ok).toBe(false);
    if (!r.ok) {
      expect(r.code).toBe('error');
      expect(r.message).toContain('500');
    }
  });

  it('maps a transport failure to a network_error result', async () => {
    fetchMock.mockRejectedValueOnce(new Error('offline'));
    const r = await login('a@b.de', 'pw');
    expect(r.ok).toBe(false);
    if (!r.ok) {
      expect(r.code).toBe('network_error');
      expect(r.status).toBe(0);
    }
  });
});

describe('me', () => {
  it('returns the user on 200', async () => {
    resolveWith(200, { id: 'u1', email: 'a@b.de', role: 'admin', status: 'active' });
    expect(await me()).toMatchObject({ email: 'a@b.de' });
  });

  it('returns null when there is no valid session', async () => {
    resolveWith(401, { code: 'unauthorized', message: 'no session' });
    expect(await me()).toBeNull();
  });
});

describe('no-body and id-path wrappers', () => {
  it('treats 204 as success with no data (logout)', async () => {
    resolveWith(204);
    const r = await logout();
    expect(r.ok).toBe(true);
  });

  it('adminCreateUser POSTs to /admin/users', async () => {
    resolveWith(200, { userId: 'u2', email: 'x@y.de', kind: 'invite', link: 'http://x' });
    await adminCreateUser('x@y.de', 'researcher');
    expect(fetchMock.mock.calls[0]![0]).toBe('/api/v1/admin/users');
  });

  it('adminSuspend POSTs to the per-user suspend path', async () => {
    resolveWith(204);
    await adminSuspend('u9');
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe('/api/v1/admin/users/u9/suspend');
    expect(init.method).toBe('POST');
  });

  it('passkeyDelete DELETEs the credential path', async () => {
    resolveWith(204);
    await passkeyDelete('cred-1');
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe('/api/v1/auth/webauthn/credentials/cred-1');
    expect(init.method).toBe('DELETE');
  });
});
