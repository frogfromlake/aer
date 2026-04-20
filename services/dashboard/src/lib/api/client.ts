// Minimal typed BFF client. Phase 97 commit 2 ships the plumbing only — the
// refusal-aware wrapper + TanStack Query integration lands with Phase 99.
//
// The generated types in `./types.ts` come from services/bff-api/api/openapi.yaml
// via `make codegen-ts`. CI fails on drift (Phase 97 commit 4).

import type { paths } from './types';

export type { paths };

export interface ApiClientConfig {
  /** BFF base URL. Traefik routes /api to the BFF; in dev this is `/` (same origin). */
  baseUrl: string;
  /** Optional fetch override (for tests). */
  fetch?: typeof fetch;
}

export function createApiClient(config: ApiClientConfig) {
  const doFetch = config.fetch ?? fetch;
  const base = config.baseUrl.replace(/\/$/, '');

  return {
    async get(path: string, init?: RequestInit): Promise<Response> {
      return doFetch(`${base}${path}`, {
        ...init,
        method: 'GET',
        headers: {
          Accept: 'application/json',
          ...(init?.headers ?? {})
        }
      });
    }
  };
}
