// Saved-analyses API client (Phase 135 / ADR-040). Same session-cookie model
// as auth.ts: same-origin requests, no token in JS.

import type { AuthResult } from './auth';

const BASE = '/api/v1';

export interface AnalysisListItem {
  id: string;
  name: string;
  description: string;
  ownerEmail: string;
  createdAt: string;
  updatedAt: string;
  permission: 'editable' | 'readable';
  owned: boolean;
}

export interface Analysis extends AnalysisListItem {
  state: string;
}

export interface AnalysisShare {
  granteeId: string;
  email: string;
  canEdit: boolean;
}

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
  if (res.status === 204) return { ok: true, data: undefined as T };
  let payload: unknown = null;
  try {
    payload = await res.json();
  } catch {
    /* empty body */
  }
  if (res.ok) return { ok: true, data: payload as T };
  const e = (payload ?? {}) as { code?: string; message?: string };
  return {
    ok: false,
    code: typeof e.code === 'string' ? e.code : 'error',
    message: typeof e.message === 'string' ? e.message : `${res.status} ${res.statusText}`,
    status: res.status
  };
}

export const listAnalyses = () => send<{ analyses: AnalysisListItem[] }>('GET', '/analyses');
export const getAnalysis = (id: string) => send<Analysis>('GET', `/analyses/${id}`);
export const createAnalysis = (name: string, description: string, state: string) =>
  send<AnalysisListItem>('POST', '/analyses', { name, description, state });
export const updateAnalysis = (
  id: string,
  patch: { name?: string; description?: string; state?: string }
) => send<Analysis>('PATCH', `/analyses/${id}`, patch);
export const deleteAnalysis = (id: string) => send<void>('DELETE', `/analyses/${id}`);

export const listShares = (id: string) =>
  send<{ shares: AnalysisShare[] }>('GET', `/analyses/${id}/shares`);
export const addShare = (id: string, email: string, canEdit: boolean) =>
  send<AnalysisShare>('POST', `/analyses/${id}/shares`, { email, canEdit });
export const removeShare = (id: string, granteeId: string) =>
  send<void>('DELETE', `/analyses/${id}/shares/${granteeId}`);
