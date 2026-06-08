// Auth API client (Phase 134 / ADR-040).
//
// Thin fetch wrappers over the BFF /auth and /admin endpoints. Browsers
// authenticate by the opaque __Host- session cookie set by the BFF — there is
// NO token in JS. Requests are same-origin (Traefik routes /api), so the cookie
// is sent automatically; `credentials: 'same-origin'` is explicit for clarity.
// CSRF is covered server-side by Sec-Fetch-Site + SameSite=Strict.

const BASE = '/api/v1';

export interface AuthUser {
  id: string;
  email: string;
  role: 'admin' | 'researcher';
  status: 'invited' | 'active' | 'suspended';
}

export interface AdminUser {
  id: string;
  email: string;
  role: string;
  status: string;
  createdAt: string;
}

export interface ActionLink {
  userId: string;
  email: string;
  kind: string;
  link: string;
}

export interface UserDataExport {
  id: string;
  email: string;
  role: string;
  status: string;
  createdAt: string;
  responsibleUseAcceptedAt?: string | null;
  activeSessionCount: number;
  lastSeenAt?: string | null;
}

export interface PasskeyMeta {
  id: string;
  name?: string | null;
  createdAt: string;
  lastUsedAt?: string | null;
}

/** Discriminated result: success with data, or a typed auth error. */
export type AuthResult<T> =
  | { ok: true; data: T }
  | { ok: false; code: string; message: string; status: number };

async function send<T>(method: string, path: string, body?: unknown): Promise<AuthResult<T>> {
  const init: RequestInit = {
    method,
    credentials: 'same-origin',
    headers: {
      Accept: 'application/json',
      ...(body !== undefined ? { 'Content-Type': 'application/json' } : {})
    }
  };
  if (body !== undefined) init.body = JSON.stringify(body);

  let res: Response;
  try {
    res = await fetch(`${BASE}${path}`, init);
  } catch (err) {
    return {
      ok: false,
      code: 'network_error',
      message: err instanceof Error ? err.message : 'network error',
      status: 0
    };
  }
  if (res.status === 204 || res.status === 202) {
    return { ok: true, data: undefined as T };
  }
  let payload: unknown = null;
  try {
    payload = await res.json();
  } catch {
    /* empty body */
  }
  if (res.ok) {
    return { ok: true, data: payload as T };
  }
  const err = (payload ?? {}) as { code?: string; message?: string };
  return {
    ok: false,
    code: typeof err.code === 'string' ? err.code : 'error',
    message: typeof err.message === 'string' ? err.message : `${res.status} ${res.statusText}`,
    status: res.status
  };
}

// --- session ---------------------------------------------------------------

export const login = (email: string, password: string) =>
  send<AuthUser>('POST', '/auth/login', { email, password });

export const logout = () => send<void>('POST', '/auth/logout');

/** Returns the current user, or null when there is no valid session. */
export async function me(): Promise<AuthUser | null> {
  const r = await send<AuthUser>('GET', '/auth/me');
  return r.ok ? r.data : null;
}

export const acceptInvite = (token: string, password: string, acceptResponsibleUse: boolean) =>
  send<AuthUser>('POST', '/auth/accept-invite', { token, password, acceptResponsibleUse });

export const forgotPassword = (email: string) =>
  send<void>('POST', '/auth/forgot-password', { email });

export const resetPassword = (token: string, password: string) =>
  send<void>('POST', '/auth/reset-password', { token, password });

export const changePassword = (currentPassword: string, newPassword: string) =>
  send<void>('POST', '/auth/change-password', { currentPassword, newPassword });

// --- DSGVO -----------------------------------------------------------------

export const exportMyData = () => send<UserDataExport>('GET', '/auth/me/export');
export const deleteMyAccount = () => send<void>('DELETE', '/auth/me');

// --- admin -----------------------------------------------------------------

export const adminListUsers = () => send<{ users: AdminUser[] }>('GET', '/admin/users');
export const adminCreateUser = (email: string, role: string) =>
  send<ActionLink>('POST', '/admin/users', { email, role });
export const adminSuspend = (id: string) => send<void>('POST', `/admin/users/${id}/suspend`);
export const adminReactivate = (id: string) => send<void>('POST', `/admin/users/${id}/reactivate`);
export const adminResetPassword = (id: string) =>
  send<ActionLink>('POST', `/admin/users/${id}/reset-password`);

// --- WebAuthn (passkeys) ---------------------------------------------------

export const passkeyList = () =>
  send<{ credentials: PasskeyMeta[] }>('GET', '/auth/webauthn/credentials');
export const passkeyDelete = (id: string) =>
  send<void>('DELETE', `/auth/webauthn/credentials/${id}`);
export const passkeyRegisterBegin = () =>
  send<Record<string, unknown>>('POST', '/auth/webauthn/register/begin');
export const passkeyRegisterFinish = (body: unknown) =>
  send<PasskeyMeta>('POST', '/auth/webauthn/register/finish', body);
