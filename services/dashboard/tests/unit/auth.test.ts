import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  acceptInvite,
  adminCreateUser,
  adminListUsers,
  adminReactivate,
  adminResetPassword,
  adminSuspend,
  changePassword,
  deleteMyAccount,
  exportMyData,
  forgotPassword,
  login,
  logout,
  me,
  passkeyDelete,
  passkeyList,
  passkeyRegisterBegin,
  passkeyRegisterFinish,
  resetPassword,
  updateProfile
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

describe('me (three-state session probe — SEC-081)', () => {
  it('returns authenticated + user on 200', async () => {
    resolveWith(200, { id: 'u1', email: 'a@b.de', role: 'admin', status: 'active' });
    const r = await me();
    expect(r.state).toBe('authenticated');
    if (r.state === 'authenticated') expect(r.user.email).toBe('a@b.de');
  });

  it('returns unauthenticated only on a definitive 401', async () => {
    resolveWith(401, { code: 'unauthorized', message: 'no session' });
    expect((await me()).state).toBe('unauthenticated');
  });

  it('returns unknown (not logged-out) on a transient 5xx', async () => {
    resolveWith(503, { message: 'unavailable' });
    expect((await me()).state).toBe('unknown');
  });

  it('returns unknown (not logged-out) on a transport failure', async () => {
    fetchMock.mockRejectedValueOnce(new Error('offline'));
    expect((await me()).state).toBe('unknown');
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

  it('treats 202 (accepted) as success with no data', async () => {
    resolveWith(202);
    const r = await forgotPassword('a@b.de');
    expect(r.ok).toBe(true);
  });

  it('survives a 200 with an empty/unparseable body (json() throws)', async () => {
    fetchMock.mockResolvedValueOnce(
      new Response('not json', { status: 200, headers: { 'Content-Type': 'application/json' } })
    );
    const r = await adminListUsers();
    expect(r.ok).toBe(true);
    if (r.ok) expect(r.data).toBeNull();
  });
});

describe('session + DSGVO + admin + passkey wrappers route to the right method/path', () => {
  // Each wrapper is a distinct function the rest of the app calls; pin its
  // verb + path + body shape so a contract drift surfaces here.
  it('acceptInvite POSTs token/firstName/lastName/password/acceptResponsibleUse', async () => {
    resolveWith(200, { id: 'u', email: 'a@b.de', role: 'researcher', status: 'active' });
    await acceptInvite('tok', 'Ada', 'Lovelace', 'pw', true);
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe('/api/v1/auth/accept-invite');
    expect(init.method).toBe('POST');
    expect(JSON.parse(init.body)).toEqual({
      token: 'tok',
      firstName: 'Ada',
      lastName: 'Lovelace',
      password: 'pw',
      acceptResponsibleUse: true
    });
  });

  it('forgotPassword + resetPassword + changePassword POST to their endpoints', async () => {
    resolveWith(202);
    await forgotPassword('a@b.de');
    expect(fetchMock.mock.calls[0]![0]).toBe('/api/v1/auth/forgot-password');

    resolveWith(204);
    await resetPassword('tok', 'newpw');
    expect(fetchMock.mock.calls[1]![0]).toBe('/api/v1/auth/reset-password');

    resolveWith(204);
    await changePassword('old', 'new');
    const [url, init] = fetchMock.mock.calls[2]!;
    expect(url).toBe('/api/v1/auth/change-password');
    expect(JSON.parse(init.body)).toEqual({ currentPassword: 'old', newPassword: 'new' });
  });

  it('updateProfile PATCHes /auth/me with the names (Phase 148e)', async () => {
    resolveWith(200, {
      id: 'u',
      email: 'a@b.de',
      firstName: 'Ada',
      lastName: 'Lovelace',
      role: 'researcher',
      status: 'active'
    });
    await updateProfile('Ada', 'Lovelace');
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe('/api/v1/auth/me');
    expect(init.method).toBe('PATCH');
    expect(JSON.parse(init.body)).toEqual({ firstName: 'Ada', lastName: 'Lovelace' });
  });

  it('exportMyData GETs /auth/me/export; deleteMyAccount DELETEs /auth/me', async () => {
    resolveWith(200, {
      id: 'u',
      email: 'a@b.de',
      role: 'admin',
      status: 'active',
      createdAt: 'x',
      activeSessionCount: 1
    });
    await exportMyData();
    expect(fetchMock.mock.calls[0]!).toEqual([
      '/api/v1/auth/me/export',
      expect.objectContaining({ method: 'GET' })
    ]);

    resolveWith(204);
    await deleteMyAccount();
    expect(fetchMock.mock.calls[1]![1].method).toBe('DELETE');
  });

  it('admin reactivate + reset-password hit the per-user paths', async () => {
    resolveWith(204);
    await adminReactivate('u7');
    expect(fetchMock.mock.calls[0]![0]).toBe('/api/v1/admin/users/u7/reactivate');

    resolveWith(200, { userId: 'u7', email: 'x@y.de', kind: 'reset', link: 'http://l' });
    await adminResetPassword('u7');
    expect(fetchMock.mock.calls[1]![0]).toBe('/api/v1/admin/users/u7/reset-password');
  });

  it('passkey list + register begin/finish hit the webauthn endpoints', async () => {
    resolveWith(200, { credentials: [] });
    await passkeyList();
    expect(fetchMock.mock.calls[0]![0]).toBe('/api/v1/auth/webauthn/credentials');

    resolveWith(200, { challenge: 'abc' });
    await passkeyRegisterBegin();
    expect(fetchMock.mock.calls[1]![0]).toBe('/api/v1/auth/webauthn/register/begin');

    resolveWith(200, { id: 'cred', createdAt: 'x' });
    await passkeyRegisterFinish({ attestation: 'y' });
    const [url, init] = fetchMock.mock.calls[2]!;
    expect(url).toBe('/api/v1/auth/webauthn/register/finish');
    expect(JSON.parse(init.body)).toEqual({ attestation: 'y' });
  });
});
