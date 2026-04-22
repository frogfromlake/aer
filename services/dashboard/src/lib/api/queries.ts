// TanStack Query integration for the AĒR BFF.
//
// The typed fetch wrappers in this module are the only place that converts
// between the wire format (OpenAPI-generated types) and the rest of the
// frontend. Every query returns a `QueryOutcome<T>` — a discriminated
// union of `success | refusal | network-error` — so call-sites can
// pattern-match on the outcome without ever pulling apart an HTTP status
// again. This is the entry point Design Brief §5.4 refers to when it
// says the BFF's methodological gates (HTTP 400) should surface as
// refusals rather than errors; the RefusalSurface primitive renders the
// `refusal` branch directly.
//
// Query keys follow a stable schema: `['aer', <entity>, ...params]`. The
// leading `'aer'` namespaces the app's keys so a future embed of this
// dashboard inside a larger SvelteKit app cannot collide.

import type { paths, components } from './types';

export type ProbeDto = components['schemas']['Probe'];
export type EmissionPointDto = components['schemas']['EmissionPoint'];
export type ContentResponseDto = components['schemas']['ContentResponse'];
export type MetricsResponseDto =
  paths['/metrics']['get']['responses'][200]['content']['application/json'];
export type MetricsParams = paths['/metrics']['get']['parameters']['query'];

// Canonical refusal kinds currently authored in the Content Catalog
// (see ROADMAP Phase 94 seed content). A query hook names the kind it
// expects when its gate trips, so the RefusalSurface can look up the
// right dual-register text without having to parse the BFF message.
export type RefusalKind =
  | 'normalization_equivalence_missing'
  | 'validation_missing'
  | 'k_anonymity_threshold_not_met'
  | 'unspecified';

export interface RefusalOutcome {
  readonly kind: 'refusal';
  /** The refusal type as classified by the caller (maps to a Content Catalog entry). */
  readonly refusalKind: RefusalKind;
  /** Raw BFF message, shown if the Content Catalog lookup itself fails. */
  readonly message: string;
  readonly httpStatus: number;
}

export interface NetworkErrorOutcome {
  readonly kind: 'network-error';
  readonly message: string;
  /** Present only when a response was received — absent for true transport failures. */
  readonly httpStatus?: number;
}

export interface SuccessOutcome<T> {
  readonly kind: 'success';
  readonly data: T;
}

export type QueryOutcome<T> = SuccessOutcome<T> | RefusalOutcome | NetworkErrorOutcome;

// -------------------------------------------------------------------------
// Low-level fetch wrapper. Keeps the I/O surface tiny and fully synchronous
// at the call-sites that matter (the query-option factories below). A
// thrown `NetworkErrorOutcome` is the only failure mode TanStack Query
// retries on; refusals are returned as data so `useQuery` renders them
// through the normal success branch with the discriminated union intact.
// -------------------------------------------------------------------------

export interface FetchContext {
  /** BFF base URL. In every deployment this is `/api/v1` — Traefik routes
   *  same-origin requests to the BFF and injects the X-API-Key header
   *  server-side, so the browser bundle never carries a secret. */
  readonly baseUrl: string;
  /** Fetch override (tests). */
  readonly fetch?: typeof fetch;
}

async function fetchJson<T>(
  ctx: FetchContext,
  path: string,
  expectedRefusal: RefusalKind,
  init?: RequestInit
): Promise<QueryOutcome<T>> {
  const doFetch = ctx.fetch ?? fetch;
  const base = ctx.baseUrl.replace(/\/$/, '');
  let response: Response;
  try {
    response = await doFetch(`${base}${path}`, {
      ...init,
      method: 'GET',
      headers: {
        Accept: 'application/json',
        ...(init?.headers ?? {})
      }
    });
  } catch (err) {
    // Transport-level failure (DNS, offline, CORS preflight): no status.
    const message = err instanceof Error ? err.message : 'network request failed';
    throw { kind: 'network-error', message } satisfies NetworkErrorOutcome;
  }

  if (response.status === 400) {
    // BFF methodological gate → surfaced as a refusal, not an error.
    const message = await safeMessage(response);
    return {
      kind: 'refusal',
      refusalKind: expectedRefusal,
      message,
      httpStatus: 400
    };
  }

  if (!response.ok) {
    // 5xx or unexpected status: a true error. Throw so TanStack retries.
    const message = await safeMessage(response);
    throw {
      kind: 'network-error',
      message,
      httpStatus: response.status
    } satisfies NetworkErrorOutcome;
  }

  const data = (await response.json()) as T;
  return { kind: 'success', data };
}

async function safeMessage(response: Response): Promise<string> {
  try {
    const body = (await response.json()) as { message?: unknown };
    if (typeof body?.message === 'string') return body.message;
  } catch {
    // fallthrough
  }
  return `${response.status} ${response.statusText}`;
}

// -------------------------------------------------------------------------
// Query-option factories. These return plain options objects — call-sites
// pass them to `createQuery(...)` from `@tanstack/svelte-query`. Keeping
// the factories pure (no `QueryClient` creation at module scope) means
// queries are trivially testable without a provider.
// -------------------------------------------------------------------------

export interface QueryOptions<T> {
  readonly queryKey: readonly unknown[];
  readonly queryFn: () => Promise<QueryOutcome<T>>;
  readonly staleTime: number;
}

const FIVE_MINUTES = 5 * 60 * 1000;

export function probesQuery(ctx: FetchContext): QueryOptions<ProbeDto[]> {
  return {
    queryKey: ['aer', 'probes'] as const,
    queryFn: () => fetchJson<ProbeDto[]>(ctx, '/probes', 'unspecified'),
    // Probes change only when the operator edits probe config; hourly
    // refresh is more than enough and keeps the shell's request volume
    // dominated by the much shorter metric windows.
    staleTime: 60 * 60 * 1000
  };
}

export function metricsQuery(
  ctx: FetchContext,
  params: MetricsParams
): QueryOptions<MetricsResponseDto> {
  const qs = new URLSearchParams();
  qs.set('startDate', params.startDate);
  qs.set('endDate', params.endDate);
  if (params.source) qs.set('source', params.source);
  if (params.metricName) qs.set('metricName', params.metricName);
  if (params.normalization) qs.set('normalization', params.normalization);
  if (params.resolution) qs.set('resolution', params.resolution);

  // Normalization is the one parameter that trips an equivalence gate;
  // raw queries can only fail on validation. We encode this so the
  // RefusalSurface picks the correct Content Catalog entry by default.
  const expected: RefusalKind =
    params.normalization === 'zscore' ? 'normalization_equivalence_missing' : 'validation_missing';

  return {
    queryKey: ['aer', 'metrics', params] as const,
    queryFn: () => fetchJson<MetricsResponseDto>(ctx, `/metrics?${qs.toString()}`, expected),
    staleTime: FIVE_MINUTES
  };
}

export function contentQuery(
  ctx: FetchContext,
  entityType: 'metric' | 'probe' | 'discourse_function' | 'refusal',
  entityId: string,
  locale: 'en' | 'de' = 'en'
): QueryOptions<ContentResponseDto> {
  return {
    queryKey: ['aer', 'content', entityType, entityId, locale] as const,
    queryFn: () =>
      fetchJson<ContentResponseDto>(
        ctx,
        `/content/${entityType}/${encodeURIComponent(entityId)}?locale=${locale}`,
        'unspecified'
      ),
    // Content is versioned server-side; caching aggressively is safe.
    staleTime: 60 * 60 * 1000
  };
}
