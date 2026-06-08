/* eslint-disable svelte/no-navigation-without-resolve -- internal auth navigation (/login) */
// Current-user auth state (Phase 134 / ADR-040).
//
// A module-scoped Svelte 5 rune holding the authenticated user (or null). The
// session itself lives in the opaque __Host- cookie; this is just the cached
// identity for the UI, hydrated from GET /auth/me.

import { browser } from '$app/environment';
import { goto } from '$app/navigation';
import * as authApi from '$lib/api/auth';

let currentUser = $state<authApi.AuthUser | null>(null);
let checked = $state(false);

/** The authenticated user, or null. Reactive. */
export function user(): authApi.AuthUser | null {
  return currentUser;
}

/** Whether the initial /auth/me check has completed (avoids UI flicker). */
export function authChecked(): boolean {
  return checked;
}

export function isAdmin(): boolean {
  return currentUser?.role === 'admin';
}

export function setUser(u: authApi.AuthUser | null): void {
  currentUser = u;
  checked = true;
}

/** Hydrate the current user from the server. Safe to call repeatedly. */
export async function refreshMe(): Promise<authApi.AuthUser | null> {
  const u = await authApi.me();
  currentUser = u;
  checked = true;
  return u;
}

/** Log out server-side, clear local state, bounce to /login. */
export async function doLogout(): Promise<void> {
  await authApi.logout();
  currentUser = null;
  checked = true;
  if (browser) await goto('/login');
}

const AUTH_ROUTES = ['/login', '/accept-invite', '/forgot-password', '/reset-password'];

function onAuthRoute(pathname: string): boolean {
  return AUTH_ROUTES.some(
    (r) => pathname === r || pathname.startsWith(`${r}?`) || pathname.startsWith(`${r}/`)
  );
}

/**
 * Called when any API request returns 401 (session missing/expired). Clears the
 * cached identity and redirects to /login — unless we are already on a pre-auth
 * route (avoids a redirect loop).
 */
export function handleUnauthenticated(): void {
  currentUser = null;
  checked = true;
  if (browser && !onAuthRoute(window.location.pathname)) {
    void goto('/login');
  }
}
