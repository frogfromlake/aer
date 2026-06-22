// Auth API client (Phase 134 / ADR-040).
//
// Thin fetch wrappers over the BFF /auth and /admin endpoints. Browsers
// authenticate by the opaque __Host- session cookie set by the BFF — there is
// NO token in JS. Requests are same-origin (Traefik routes /api), so the cookie
// is sent automatically; `credentials: 'same-origin'` is explicit for clarity.
// CSRF is covered server-side by Sec-Fetch-Site + SameSite=Strict.

import { notifyUnauthenticated } from './queries/shared';

const BASE = '/api/v1';

// SEC-086: pre-auth endpoints legitimately 401 (bad credentials / expired
// token), so their 401 must NOT trip the global unauthenticated handler. The
// session probe `/auth/me` is also excluded — it resolves its own state via
// `me()` (SEC-081) and the (app) gate owns the bounce (with a redirect param).
const PRE_AUTH_PATHS = new Set([
  '/auth/login',
  '/auth/accept-invite',
  '/auth/forgot-password',
  '/auth/reset-password',
  '/auth/me'
]);

export interface AuthUser {
  id: string;
  email: string;
  role: 'admin' | 'researcher';
  status: 'invited' | 'active' | 'suspended';
  // Identity layer (Phase 148e). Set once at invite-accept; optional here until
  // the backend contract ships, so the avatar/header fall back to the email.
  firstName?: string;
  lastName?: string;
}

export interface AdminUser {
  id: string;
  email: string;
  firstName: string;
  lastName: string;
  role: string;
  status: string;
  createdAt: string;
}

export interface ActionLink {
  userId: string;
  email: string;
  kind: string;
  link: string;
  /** Phase 153: true when the link was dispatched via the email relay; false →
   * deliver `link` manually (relay unconfigured or send failed). */
  delivered?: boolean;
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
  // SEC-086: a 401 on a post-auth action means the session expired mid-action;
  // route it through the same global handler the analytics layer uses.
  if (res.status === 401 && !PRE_AUTH_PATHS.has(path)) notifyUnauthenticated();
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

/**
 * Result of a session probe (SEC-081). A non-ok `/auth/me` must NOT collapse to
 * "logged out": only a definitive 401 is `unauthenticated` (clear identity,
 * bounce). A transient failure — transport error (status 0) or a 5xx — is
 * `unknown`: the caller should keep the user in place and retry rather than
 * evicting a valid `__Host-` session on a passing BFF blip.
 */
export type MeResult =
  | { state: 'authenticated'; user: AuthUser }
  | { state: 'unauthenticated' }
  | { state: 'unknown' };

/** Probes the current session. See {@link MeResult} for the three outcomes. */
export async function me(): Promise<MeResult> {
  const r = await send<AuthUser>('GET', '/auth/me');
  if (r.ok) return { state: 'authenticated', user: r.data };
  // Only a real 401 is a logged-out signal; anything else (status 0 transport
  // failure, 5xx) is transient and must not bounce the user.
  if (r.status === 401) return { state: 'unauthenticated' };
  return { state: 'unknown' };
}

export const acceptInvite = (
  token: string,
  firstName: string,
  lastName: string,
  password: string,
  acceptResponsibleUse: boolean
) =>
  send<AuthUser>('POST', '/auth/accept-invite', {
    token,
    firstName,
    lastName,
    password,
    acceptResponsibleUse
  });

export const forgotPassword = (email: string) =>
  send<void>('POST', '/auth/forgot-password', { email });

export const resetPassword = (token: string, password: string) =>
  send<void>('POST', '/auth/reset-password', { token, password });

export const changePassword = (currentPassword: string, newPassword: string) =>
  send<void>('POST', '/auth/change-password', { currentPassword, newPassword });

// Self-service display-name edit (Phase 148e). Returns the updated user so the
// caller can refresh the store; the change is read live everywhere it shows.
export const updateProfile = (firstName: string, lastName: string) =>
  send<AuthUser>('PATCH', '/auth/me', { firstName, lastName });

// --- sessions (SEC-005) ----------------------------------------------------

/** One of the user's own active sessions. Privacy-minimal — no id is exposed;
 *  `current` marks the device making the request. */
export interface SessionSummary {
  createdAt: string;
  lastSeenAt: string;
  userAgent?: string;
  current: boolean;
}

/** Lists the current user's active sessions (where they are logged in). */
export const listSessions = () => send<{ sessions: SessionSummary[] }>('GET', '/auth/sessions');

/** Logs out everywhere: revokes all of the user's sessions (incl. the current
 *  one) and clears the cookie. The caller must then re-authenticate. */
export const revokeAllSessions = () => send<void>('DELETE', '/auth/sessions');

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
