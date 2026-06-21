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

import type { paths, components } from '../types';

// 401 handler hook (Phase 134 / ADR-040). Decoupled via registration so the
// data layer does not import the auth/navigation modules (keeps it unit-
// testable). The app registers `handleUnauthenticated` in the root layout.
let onUnauthenticated: (() => void) | null = null;
export function setUnauthenticatedHandler(fn: () => void): void {
  onUnauthenticated = fn;
}

// SEC-086: the auth/analyses clients (auth.ts, analyses.ts) route their own
// 401s through this same registered handler so a session expiry mid-action
// clears the cached identity and bounces, instead of surfacing as a per-
// component error while the gated UI keeps rendering. Exposed as an invoker
// (not the raw ref) to preserve the data layer's decoupling from the
// auth/navigation modules.
export function notifyUnauthenticated(): void {
  onUnauthenticated?.();
}

export type ProbeDto = components['schemas']['Probe'];
export type EmissionPointDto = components['schemas']['EmissionPoint'];
export type ContentResponseDto = components['schemas']['ContentResponse'];
export type MetricsResponseDto =
  paths['/metrics']['get']['responses'][200]['content']['application/json'];
export type MetricsParams = paths['/metrics']['get']['parameters']['query'];
export type MetricProvenanceDto = components['schemas']['MetricProvenance'];
export type ProbeDossierDto = components['schemas']['ProbeDossier'];
export type ProbeDossierSourceDto = components['schemas']['ProbeDossierSource'];
export type ArticlesPageDto = components['schemas']['ArticlesPage'];
export type ArticleListItemDto = components['schemas']['ArticleListItem'];
export type ArticleDetailDto = components['schemas']['ArticleDetail'];
export type DistributionResponseDto = components['schemas']['DistributionResponse'];
export type CoOccurrenceGraphDto = components['schemas']['CoOccurrenceGraph'];
// Phase 131 — paired-metric scatter (visual-channel binding). Sourced from
// the path response (the bundler inlines these schemas into the operation
// rather than registering them under components.schemas).
export type ScatterResponseDto =
  paths['/metrics/scatter']['get']['responses'][200]['content']['application/json'];
export type ScatterPointDto = ScatterResponseDto['points'][number];
// Phase 125 — pairwise Pearson correlation matrix over an N-metric set.
export type CorrelationMatrixDto =
  paths['/metrics/correlation']['get']['responses'][200]['content']['application/json'];
// Phase 125 — cross-tab of a categorical field × a numeric metric.
export type CrossTabDto =
  paths['/metadata/{field}/by-metric/{metric}']['get']['responses'][200]['content']['application/json'];
// Phase 125 — generalised metric lead-lag (two metrics' hourly series).
export type CorrelationLeadLagDto =
  paths['/correlation/lead-lag']['get']['responses'][200]['content']['application/json'];
// Phase 125 — per-article N-metric matrix for parallel coordinates.
export type ParallelCoordsDto =
  paths['/metrics/parallel']['get']['responses'][200]['content']['application/json'];
// Phase 125 — alluvial flow across an ordered chain of categorical fields.
export type SankeyDto =
  paths['/metadata/sankey']['get']['responses'][200]['content']['application/json'];
export type AvailableMetricDto = components['schemas']['AvailableMetric'];
// Phase 123c (C1) — per-source metric availability across a multi-source
// scope. `available` = metrics present in Gold for EVERY scoped source (the
// intersection safe to bind on a cross-probe panel); `partial` = metrics on
// only some sources, surfaced as an explanatory hint, never offered for
// binding. Sourced from the path response (bundler-inlined, like Scatter).
export type ScopeAvailableMetricsDto =
  paths['/scope/available-metrics']['get']['responses'][200]['content']['application/json'];
export type PartialMetricDto = ScopeAvailableMetricsDto['partial'][number];
// Phase 133 — categorical metadata distribution + per-scope availability.
export type CategoricalDistributionResponseDto =
  components['schemas']['CategoricalDistributionResponse'];
export type CategoryCountDto = CategoricalDistributionResponseDto['categories'][number];
export type ScopeAvailableMetadataDto = components['schemas']['ScopeAvailableMetadata'];
export type PartialMetadataFieldDto = ScopeAvailableMetadataDto['partial'][number];
export type SilverAggregationResponseDto = components['schemas']['SilverAggregationResponse'];
export type TopicDistributionResponseDto = components['schemas']['TopicDistributionResponse'];
export type MetadataCoverageResponseDto = components['schemas']['MetadataCoverageResponse'];
export type MetadataCoverageSourceDto = components['schemas']['MetadataCoverageSource'];
export type MetadataCoverageFieldDto = components['schemas']['MetadataCoverageField'];
// Task C — corpus-wide per-field extraction status (Reflection metadata-fields surface).
export type MetadataFieldsResponseDto = components['schemas']['MetadataFieldsResponse'];
export type MetadataFieldStatDto = components['schemas']['MetadataFieldStat'];
// Phase 122g — per-channel discovery telemetry (ADR-031).
export type DiscoveryCoverageResponseDto = components['schemas']['DiscoveryCoverageResponse'];
export type DiscoveryCoveragePerChannelDto = components['schemas']['DiscoveryCoveragePerChannel'];
// Phase 122d.0 — Silent-Edit Observability (ADR-032).
export type RevisionActivityResponseDto = components['schemas']['RevisionActivityResponse'];
export type RevisionActivityEntryDto = components['schemas']['RevisionActivityEntry'];
// Phase 122d.3 — Silent-Edit Discourse Shift trajectory.
export type RevisionDiscourseShiftResponseDto =
  components['schemas']['RevisionDiscourseShiftResponse'];
export type RevisionDiscourseShiftEntryDto = components['schemas']['RevisionDiscourseShiftEntry'];
// Phase 122d.3 — coordinated cross-source edit clusters (Rhizome).
export type RevisionEditClustersResponseDto = components['schemas']['RevisionEditClustersResponse'];
export type RevisionEditClusterEntryDto = components['schemas']['RevisionEditClusterEntry'];
export type ArticleRevisionsResponseDto = components['schemas']['ArticleRevisionsResponse'];
export type ArticleRevisionEntryDto = ArticleRevisionsResponseDto['revisions'][number];
export type RevisionActivityResolution = 'snapshot' | 'daily' | 'weekly' | 'monthly';
// Phase 122d.1 — Diff Substance + Drilldown surfaces.
export type ArticleRevisionDiffDto = components['schemas']['ArticleRevisionDiff'];
export type ArticleRevisionDiffOpDto = ArticleRevisionDiffDto['diffParagraphs'][number];
export type RevisionsArticlesPageDto = components['schemas']['RevisionsArticlesPage'];
export type RevisionsArticlesItemDto = RevisionsArticlesPageDto['items'][number];
export type TopicDistributionEntryDto = TopicDistributionResponseDto['topics'][number];
export type SilverAggregationType =
  | 'cleaned_text_length'
  | 'word_count'
  | 'raw_entity_count'
  | 'cleaned_text_length_by_hour'
  | 'word_count_by_source'
  | 'cleaned_text_length_vs_word_count';

// Canonical refusal kinds currently authored in the Content Catalog
// (see ROADMAP Phase 94 seed content). A query hook names the kind it
// expects when its gate trips, so the RefusalSurface can look up the
// right dual-register text without having to parse the BFF message.
export type RefusalKind =
  | 'normalization_equivalence_missing'
  | 'cross_frame_equivalence_missing'
  | 'validation_missing'
  | 'k_anonymity_threshold_not_met'
  | 'silver_eligibility'
  | 'invalid_language'
  // Phase 122i / ADR-034 — Multi-Panel Workbench refusal kinds.
  | 'cross_language_merge_unsupported'
  | 'scope_limit_exceeded'
  // Phase 130 / ADR-035 — client-side merged-cross-probe guard (Brief §1.3).
  | 'merged_cross_probe_unsupported'
  | 'unspecified';

export interface RefusalOutcome {
  readonly kind: 'refusal';
  /** The refusal type as classified by the caller (maps to a Content Catalog entry). */
  readonly refusalKind: RefusalKind;
  /** Raw BFF message, shown if the Content Catalog lookup itself fails. */
  readonly message: string;
  readonly httpStatus: number;
  /** Phase 115: concrete user-actionable alternatives carried by the refusal payload. */
  readonly alternatives?: readonly string[];
  /** Phase 115: anchor into the methodological surface (e.g. WP-004#section-5.2). */
  readonly workingPaperAnchor?: string;
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

export async function fetchJson<T>(
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
      method: 'GET',
      ...init,
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

  // 401 (Phase 134 / ADR-040): the session is missing or expired. NOT a
  // methodological refusal — clear the cached identity and bounce to /login.
  // (400 stays a refusal by design; auth-403 is handled by the auth-specific
  // clients, not here.)
  if (response.status === 401) {
    onUnauthenticated?.();
    throw {
      kind: 'network-error',
      message: 'unauthenticated',
      httpStatus: 401
    } satisfies NetworkErrorOutcome;
  }

  // Methodological gates → surfaced as refusals, not errors.
  //   - 400/403: existing validation + WP-006 silver/k-anon gates.
  //   - 413/422 (Phase 122i / ADR-034): scope-limit + cross-language gates
  //     on the Multi-Panel Workbench POST endpoint.
  if (
    response.status === 400 ||
    response.status === 403 ||
    response.status === 413 ||
    response.status === 422
  ) {
    const refusal = await safeRefusal(response, expectedRefusal);
    return refusal;
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

/**
 * safeRefusal reads the response body once and returns a RefusalOutcome that
 * (a) carries the BFF message, (b) sharpens `refusalKind` if the body's
 * structured `gate` field identifies the Phase-115 cross-frame gate, and (c)
 * pulls the structured `alternatives` and `workingPaperAnchor` through so
 * the RefusalSurface can render them under the methodological register
 * without a second lookup.
 */
async function safeRefusal(
  response: Response,
  expectedRefusal: RefusalKind
): Promise<RefusalOutcome> {
  let message = `${response.status} ${response.statusText}`;
  let gate: string | undefined;
  let alternatives: readonly string[] | undefined;
  let workingPaperAnchor: string | undefined;
  try {
    const body = (await response.json()) as {
      message?: unknown;
      gate?: unknown;
      alternatives?: unknown;
      workingPaperAnchor?: unknown;
    };
    if (typeof body?.message === 'string') message = body.message;
    if (typeof body?.gate === 'string') gate = body.gate;
    if (typeof body?.workingPaperAnchor === 'string') workingPaperAnchor = body.workingPaperAnchor;
    if (Array.isArray(body?.alternatives)) {
      alternatives = body.alternatives.filter((a): a is string => typeof a === 'string');
    }
  } catch {
    // fallthrough — opaque refusal body
  }
  let refusalKind = expectedRefusal;
  if (gate === 'metric_equivalence') refusalKind = 'cross_frame_equivalence_missing';
  // Phase 118a / 121b: invalid_language is an engineering-procedural gate
  // (unknown ?language=). Route to its own Content Catalog entry so the
  // operator-facing methodology tray is used instead of a generic refusal.
  if (gate === 'invalid_language') refusalKind = 'invalid_language';
  // Phase 122i / ADR-034 — Multi-Panel Workbench gates.
  if (gate === 'cross_language_merge_unsupported') refusalKind = 'cross_language_merge_unsupported';
  if (gate === 'scope_limit_exceeded') refusalKind = 'scope_limit_exceeded';
  const out: RefusalOutcome = {
    kind: 'refusal',
    refusalKind,
    message,
    httpStatus: response.status
  };
  if (alternatives !== undefined)
    (out as { alternatives: readonly string[] }).alternatives = alternatives;
  if (workingPaperAnchor !== undefined)
    (out as { workingPaperAnchor: string }).workingPaperAnchor = workingPaperAnchor;
  return out;
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

export const FIVE_MINUTES = 5 * 60 * 1000;

// --- shared per-article query param helpers (used across query domains) ---
export type PresentationScope = 'probe' | 'source';

export interface PresentationQueryParams {
  scope: PresentationScope;
  scopeId: string;
  start?: string | undefined;
  end?: string | undefined;
  // Phase 125a faceting / small-multiples: when set, the per-article computation
  // is restricted to articles whose categorical field carries the given value —
  // one facet value per sub-cell. Threaded into every per-article query factory
  // below; part of `params`, so it varies the TanStack queryKey automatically.
  metadataFilter?: { field: string; value: string } | undefined;
}

// appendMetadataFilter writes the Phase-125a faceting params onto a query string
// when both halves are present. Both must be set together (the BFF ignores a
// half-supplied pair); a blank field/value is treated as "no facet".
export function appendMetadataFilter(
  qs: URLSearchParams,
  mf?: { field: string; value: string } | undefined
): void {
  if (mf && mf.field && mf.value) {
    qs.set('metadataFilterField', mf.field);
    qs.set('metadataFilterValue', mf.value);
  }
}

export interface MetricsAvailableParams {
  startDate?: string | undefined;
  endDate?: string | undefined;
  // Task B — locale for the per-metric displayLabel ('en' | 'de').
  locale?: string | undefined;
}
