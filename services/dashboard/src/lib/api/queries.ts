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
export type MetricProvenanceDto = components['schemas']['MetricProvenance'];
export type ProbeDossierDto = components['schemas']['ProbeDossier'];
export type ProbeDossierSourceDto = components['schemas']['ProbeDossierSource'];
export type ArticlesPageDto = components['schemas']['ArticlesPage'];
export type ArticleListItemDto = components['schemas']['ArticleListItem'];
export type ArticleDetailDto = components['schemas']['ArticleDetail'];
export type DistributionResponseDto = components['schemas']['DistributionResponse'];
export type CoOccurrenceGraphDto = components['schemas']['CoOccurrenceGraph'];
export type AvailableMetricDto = components['schemas']['AvailableMetric'];
export type SilverAggregationResponseDto = components['schemas']['SilverAggregationResponse'];
export type TopicDistributionResponseDto = components['schemas']['TopicDistributionResponse'];
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

  if (response.status === 400 || response.status === 403) {
    // BFF methodological gate → surfaced as a refusal, not an error.
    // 403 is used by k-anon and silver-eligibility gates (WP-006).
    // Phase 115: a 400 may carry a structured `gate` field; when it does,
    // sharpen the refusal kind so the RefusalSurface picks the right
    // Content Catalog entry and surfaces the structured alternatives.
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
  // raw queries can only fail on validation. The default refusal kind is
  // the within-frame equivalence-missing gate; Phase 115's cross-frame
  // sharpening happens server-side via the structured `gate` field.
  const expected: RefusalKind =
    params.normalization === 'zscore' || params.normalization === 'percentile'
      ? 'normalization_equivalence_missing'
      : 'validation_missing';

  return {
    queryKey: ['aer', 'metrics', params] as const,
    queryFn: () => fetchJson<MetricsResponseDto>(ctx, `/metrics?${qs.toString()}`, expected),
    staleTime: FIVE_MINUTES
  };
}

export function provenanceQuery(
  ctx: FetchContext,
  metricName: string
): QueryOptions<MetricProvenanceDto> {
  return {
    queryKey: ['aer', 'provenance', metricName] as const,
    queryFn: () =>
      fetchJson<MetricProvenanceDto>(
        ctx,
        `/metrics/${encodeURIComponent(metricName)}/provenance`,
        'validation_missing'
      ),
    // Provenance changes only when the operator re-publishes the
    // metric_provenance.yaml or a validation study lands — daily is
    // conservative enough that a stale panel is never the bottleneck.
    staleTime: 60 * 60 * 1000
  };
}

export function contentQuery(
  ctx: FetchContext,
  entityType:
    | 'metric'
    | 'probe'
    | 'discourse_function'
    | 'refusal'
    | 'empty_lane'
    | 'view_mode'
    | 'open_research_question'
    | 'primer',
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

/** Phase 115: per-probe equivalence summary (Probe Dossier "valid comparisons" panel). */
export type ProbeEquivalenceDto =
  paths['/probes/{probeId}/equivalence']['get']['responses']['200']['content']['application/json'];

export function probeEquivalenceQuery(
  ctx: FetchContext,
  probeId: string
): QueryOptions<ProbeEquivalenceDto> {
  return {
    queryKey: ['aer', 'probe-equivalence', probeId] as const,
    queryFn: () =>
      fetchJson<ProbeEquivalenceDto>(
        ctx,
        `/probes/${encodeURIComponent(probeId)}/equivalence`,
        'unspecified'
      ),
    // Equivalence registry only changes on out-of-band review; daily is
    // the right cadence — same as `provenanceQuery`.
    staleTime: 60 * 60 * 1000
  };
}

export interface DossierParams {
  windowStart?: string;
  windowEnd?: string;
}

export function probeDossierQuery(
  ctx: FetchContext,
  probeId: string,
  params: DossierParams = {}
): QueryOptions<ProbeDossierDto> {
  const qs = new URLSearchParams();
  if (params.windowStart) qs.set('windowStart', params.windowStart);
  if (params.windowEnd) qs.set('windowEnd', params.windowEnd);
  const query = qs.toString();
  return {
    queryKey: ['aer', 'probe-dossier', probeId, params] as const,
    queryFn: () =>
      fetchJson<ProbeDossierDto>(
        ctx,
        `/probes/${encodeURIComponent(probeId)}/dossier${query ? `?${query}` : ''}`,
        'unspecified'
      ),
    staleTime: FIVE_MINUTES
  };
}

export interface ArticleListParams {
  start?: string;
  end?: string;
  language?: string;
  entityMatch?: string;
  sentimentBand?: 'negative' | 'neutral' | 'positive';
  limit?: number;
  cursor?: string;
}

export function sourceArticlesQuery(
  ctx: FetchContext,
  sourceId: string,
  params: ArticleListParams = {}
): QueryOptions<ArticlesPageDto> {
  const qs = new URLSearchParams();
  if (params.start) qs.set('start', params.start);
  if (params.end) qs.set('end', params.end);
  if (params.language) qs.set('language', params.language);
  if (params.entityMatch) qs.set('entityMatch', params.entityMatch);
  if (params.sentimentBand) qs.set('sentimentBand', params.sentimentBand);
  if (params.limit) qs.set('limit', String(params.limit));
  if (params.cursor) qs.set('cursor', params.cursor);
  return {
    queryKey: ['aer', 'source-articles', sourceId, params] as const,
    queryFn: () =>
      fetchJson<ArticlesPageDto>(
        ctx,
        `/sources/${encodeURIComponent(sourceId)}/articles?${qs.toString()}`,
        'unspecified'
      ),
    staleTime: FIVE_MINUTES
  };
}

export function articleDetailQuery(
  ctx: FetchContext,
  articleId: string,
  metricName?: string
): QueryOptions<ArticleDetailDto> {
  const qs = new URLSearchParams();
  if (metricName) qs.set('metricName', metricName);
  const query = qs.toString();
  return {
    queryKey: ['aer', 'article-detail', articleId, metricName] as const,
    queryFn: () =>
      fetchJson<ArticleDetailDto>(
        ctx,
        `/articles/${encodeURIComponent(articleId)}${query ? `?${query}` : ''}`,
        'k_anonymity_threshold_not_met'
      ),
    staleTime: FIVE_MINUTES
  };
}

// -------------------------------------------------------------------------
// Phase 107 — View-Mode Matrix queries.
//
// Each MVP cell is backed by exactly one BFF endpoint. The factories below
// thin-wrap those endpoints; the matrix-cell registry under
// `$lib/viewmodes/` decides which factory a given cell uses.
// -------------------------------------------------------------------------

export type ViewModeScope = 'probe' | 'source';

export interface ViewModeQueryParams {
  scope: ViewModeScope;
  scopeId: string;
  start: string;
  end: string;
}

export interface MetricsAvailableParams {
  startDate: string;
  endDate: string;
}

export function metricsAvailableQuery(
  ctx: FetchContext,
  params: MetricsAvailableParams
): QueryOptions<AvailableMetricDto[]> {
  const qs = new URLSearchParams();
  qs.set('startDate', params.startDate);
  qs.set('endDate', params.endDate);
  return {
    queryKey: ['aer', 'metrics-available', params] as const,
    queryFn: () =>
      fetchJson<AvailableMetricDto[]>(ctx, `/metrics/available?${qs.toString()}`, 'unspecified'),
    staleTime: FIVE_MINUTES
  };
}

export function metricDistributionQuery(
  ctx: FetchContext,
  metricName: string,
  params: ViewModeQueryParams & { bins?: number }
): QueryOptions<DistributionResponseDto> {
  const qs = new URLSearchParams();
  qs.set('scope', params.scope);
  qs.set('scopeId', params.scopeId);
  qs.set('start', params.start);
  qs.set('end', params.end);
  if (params.bins) qs.set('bins', String(params.bins));
  return {
    queryKey: ['aer', 'metric-distribution', metricName, params] as const,
    queryFn: () =>
      fetchJson<DistributionResponseDto>(
        ctx,
        `/metrics/${encodeURIComponent(metricName)}/distribution?${qs.toString()}`,
        'validation_missing'
      ),
    staleTime: FIVE_MINUTES
  };
}

export function entityCoOccurrenceQuery(
  ctx: FetchContext,
  params: ViewModeQueryParams & { topN?: number }
): QueryOptions<CoOccurrenceGraphDto> {
  const qs = new URLSearchParams();
  qs.set('scope', params.scope);
  qs.set('scopeId', params.scopeId);
  qs.set('start', params.start);
  qs.set('end', params.end);
  if (params.topN) qs.set('topN', String(params.topN));
  return {
    queryKey: ['aer', 'entity-cooccurrence', params] as const,
    queryFn: () =>
      fetchJson<CoOccurrenceGraphDto>(
        ctx,
        `/entities/cooccurrence?${qs.toString()}`,
        'validation_missing'
      ),
    staleTime: FIVE_MINUTES
  };
}

// -------------------------------------------------------------------------
// Phase 121 — Topic distribution query.
//
// Backs the Episteme-pillar `topic_distribution` and `topic_evolution`
// view-mode cells. The endpoint aggregates BERTopic assignments per
// (language, topic_id) across the resolved scope; `topic_evolution`
// drives time progression by issuing one query per temporal bucket
// (the BFF endpoint itself is windowed, not bucketed — Phase 120 ships
// a single aggregated view; multiple sub-windows = multiple queries).
// -------------------------------------------------------------------------

export interface TopicDistributionParams {
  // Single scope target (probe id or single source name). At least one of
  // `scopeId`, `probeIds`, or `sourceIds` must be present.
  scopeId?: string | undefined;
  scope?: ViewModeScope | undefined;
  probeIds?: readonly string[] | undefined;
  sourceIds?: readonly string[] | undefined;
  // RFC 3339 window. Both optional — BFF defaults to the latest sweep
  // window when omitted.
  start?: string | undefined;
  end?: string | undefined;
  language?: string | undefined;
  minConfidence?: number | undefined;
  includeOutlier?: boolean | undefined;
}

export function topicDistributionQuery(
  ctx: FetchContext,
  params: TopicDistributionParams
): QueryOptions<TopicDistributionResponseDto> {
  const qs = new URLSearchParams();
  if (params.scope) qs.set('scope', params.scope);
  if (params.scopeId) qs.set('scopeId', params.scopeId);
  if (params.probeIds && params.probeIds.length > 0) qs.set('probeIds', params.probeIds.join(','));
  if (params.sourceIds && params.sourceIds.length > 0)
    qs.set('sourceIds', params.sourceIds.join(','));
  if (params.start) qs.set('start', params.start);
  if (params.end) qs.set('end', params.end);
  if (params.language) qs.set('language', params.language);
  if (params.minConfidence !== undefined) qs.set('minConfidence', String(params.minConfidence));
  if (params.includeOutlier) qs.set('includeOutlier', 'true');
  return {
    queryKey: ['aer', 'topic-distribution', params] as const,
    queryFn: () =>
      fetchJson<TopicDistributionResponseDto>(
        ctx,
        `/topics/distribution?${qs.toString()}`,
        'validation_missing'
      ),
    staleTime: FIVE_MINUTES
  };
}

// -------------------------------------------------------------------------
// Phase 111 — Silver-layer aggregation query.
//
// Routes to /api/v1/silver/aggregations/{aggregationType}. Subject to the
// WP-006 §5.2 Silver-eligibility gate; a 403 response is surfaced as a
// `silver_eligibility` refusal so the frontend renders a methodological
// explanation rather than a generic error.
// -------------------------------------------------------------------------

export interface SilverAggregationParams {
  sourceId: string;
  start: string;
  end: string;
  bins?: number;
}

export function silverAggregationQuery(
  ctx: FetchContext,
  aggregationType: SilverAggregationType,
  params: SilverAggregationParams
): QueryOptions<SilverAggregationResponseDto> {
  const qs = new URLSearchParams();
  qs.set('sourceId', params.sourceId);
  qs.set('start', params.start);
  qs.set('end', params.end);
  if (params.bins) qs.set('bins', String(params.bins));
  return {
    queryKey: ['aer', 'silver-aggregation', aggregationType, params] as const,
    queryFn: () =>
      fetchJson<SilverAggregationResponseDto>(
        ctx,
        `/silver/aggregations/${encodeURIComponent(aggregationType)}?${qs.toString()}`,
        'silver_eligibility'
      ),
    staleTime: FIVE_MINUTES
  };
}
