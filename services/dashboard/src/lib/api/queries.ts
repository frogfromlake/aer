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
  params: NonNullable<MetricsParams> = {}
): QueryOptions<MetricsResponseDto> {
  const qs = new URLSearchParams();
  if (params.startDate) qs.set('startDate', params.startDate);
  if (params.endDate) qs.set('endDate', params.endDate);
  if (params.source) qs.set('source', params.source);
  // Phase 122i revision (D1): the BFF `/metrics` endpoint unions
  // `source` (singular) and `sourceIds` (CSV). Multi-source merged Cells
  // pass `sourceIds` so the BFF returns ONE time series over the
  // unioned scope — what composition='merged' was supposed to render
  // from the start. Phase-122h code emitted only `source` (singular),
  // which capped merged-multi-source to "first source only" silently.
  if (params.sourceIds) qs.set('sourceIds', params.sourceIds);
  if (params.metricName) qs.set('metricName', params.metricName);
  if (params.normalization) qs.set('normalization', params.normalization);
  if (params.resolution) qs.set('resolution', params.resolution);
  // Phase 131 — request the per-bucket spread so the time-series cell can
  // draw a ±1σ uncertainty band. Server reads the raw layer when set.
  if (params.includeStddev) qs.set('includeStddev', 'true');

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
    | 'source'
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
    // Phase 122j J3: bumped from 1h → 24h. The BFF YAML catalog only
    // changes on operator deploy; the BFF also sets Cache-Control:
    // max-age=86400 so the same payload lives in the HTTP cache as
    // well — TanStack still returns cached data instantly even after a
    // browser reload, until the operator publishes new content.
    staleTime: 24 * 60 * 60 * 1000
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

// Phase 124 — cross-probe temporal lead-lag. The reference probe is the path
// id; `comparedTo` is the second probe. An ungranted pair returns a 400 whose
// `gate=metric_equivalence` maps to the cross-frame refusal kind, so the cell
// renders it through RefusalSurface like every other equivalence refusal.
export type ProbeLeadLagDto =
  paths['/probes/{probeId}/lead-lag']['get']['responses']['200']['content']['application/json'];

export interface LeadLagParams {
  comparedTo: string;
  start?: string | undefined;
  end?: string | undefined;
  maxLagHours?: number | undefined;
}

export function probeLeadLagQuery(
  ctx: FetchContext,
  probeId: string,
  params: LeadLagParams
): QueryOptions<ProbeLeadLagDto> {
  const sp = new URLSearchParams();
  sp.set('comparedTo', params.comparedTo);
  if (params.start) sp.set('start', params.start);
  if (params.end) sp.set('end', params.end);
  if (params.maxLagHours !== undefined) sp.set('maxLagHours', String(params.maxLagHours));
  return {
    queryKey: [
      'aer',
      'probe-lead-lag',
      probeId,
      params.comparedTo,
      params.start ?? null,
      params.end ?? null,
      params.maxLagHours ?? null
    ] as const,
    queryFn: () =>
      fetchJson<ProbeLeadLagDto>(
        ctx,
        `/probes/${encodeURIComponent(probeId)}/lead-lag?${sp.toString()}`,
        'cross_frame_equivalence_missing'
      ),
    staleTime: FIVE_MINUTES
  };
}

export interface DossierParams {
  // Phase 131a — explicit `| undefined` so callers can pass the
  // window-less mode (`{windowStart: undefined, windowEnd: undefined}`)
  // under `exactOptionalPropertyTypes: true`. Same falsy guard in
  // qs.set still suppresses the query-string entry when absent.
  windowStart?: string | undefined;
  windowEnd?: string | undefined;
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

// Phase 122f — per-source-per-field metadata coverage feeding the Probe
// Dossier panel and the dashboard's field-level Negative-Space rendering
// (Brief §7.7, WP-003 §3.2).
export function probeMetadataCoverageQuery(
  ctx: FetchContext,
  probeId: string
): QueryOptions<MetadataCoverageResponseDto> {
  return {
    queryKey: ['aer', 'probe-metadata-coverage', probeId] as const,
    queryFn: () =>
      fetchJson<MetadataCoverageResponseDto>(
        ctx,
        `/probes/${encodeURIComponent(probeId)}/metadata-coverage`,
        'unspecified'
      ),
    // Coverage is a structural property of the source's emission posture —
    // it changes only as the publisher's website does, on a much slower
    // cadence than the time-series metrics. 5 min keeps it fresh enough.
    staleTime: FIVE_MINUTES
  };
}

export function sourceMetadataCoverageQuery(
  ctx: FetchContext,
  sourceId: string
): QueryOptions<MetadataCoverageResponseDto> {
  return {
    queryKey: ['aer', 'source-metadata-coverage', sourceId] as const,
    queryFn: () =>
      fetchJson<MetadataCoverageResponseDto>(
        ctx,
        `/sources/${encodeURIComponent(sourceId)}/metadata-coverage`,
        'unspecified'
      ),
    staleTime: FIVE_MINUTES
  };
}

// Phase 122g — per-source discovery-coverage (ADR-031). Sibling to
// sourceMetadataCoverageQuery; same fetch pattern, optional windowDays
// for trailing-window control.
export function sourceDiscoveryCoverageQuery(
  ctx: FetchContext,
  sourceId: string,
  windowDays?: number
): QueryOptions<DiscoveryCoverageResponseDto> {
  const qs = windowDays != null ? `?windowDays=${windowDays}` : '';
  return {
    queryKey: ['aer', 'source-discovery-coverage', sourceId, windowDays ?? null] as const,
    queryFn: () =>
      fetchJson<DiscoveryCoverageResponseDto>(
        ctx,
        `/sources/${encodeURIComponent(sourceId)}/discovery-coverage${qs}`,
        'unspecified'
      ),
    staleTime: FIVE_MINUTES
  };
}

export interface ArticleListParams {
  start?: string | undefined;
  end?: string | undefined;
  language?: string;
  entityMatch?: string;
  sentimentBand?: 'negative' | 'neutral' | 'positive';
  limit?: number;
  cursor?: string;
  /** Phase 122d.1 — opt in to per-row chainLength + hasHeadlineChange
   *  fields. Server-side cost is a thin LEFT JOIN against
   *  `aer_gold.article_revisions`. */
  includeRevisions?: boolean;
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
  if (params.includeRevisions) qs.set('includeRevisions', 'true');
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
  start?: string | undefined;
  end?: string | undefined;
}

export interface MetricsAvailableParams {
  startDate?: string | undefined;
  endDate?: string | undefined;
}

export function metricsAvailableQuery(
  ctx: FetchContext,
  params: MetricsAvailableParams
): QueryOptions<AvailableMetricDto[]> {
  const qs = new URLSearchParams();
  if (params.startDate) qs.set('startDate', params.startDate);
  if (params.endDate) qs.set('endDate', params.endDate);
  return {
    queryKey: ['aer', 'metrics-available', params] as const,
    queryFn: () =>
      fetchJson<AvailableMetricDto[]>(ctx, `/metrics/available?${qs.toString()}`, 'unspecified'),
    staleTime: FIVE_MINUTES
  };
}

// Phase 123c (C1) — metrics available across the full (multi-source) panel
// scope. Powers the PanelControls cross-probe guard: only metrics present
// for EVERY scoped source are offered for binding, so a panel spanning
// probes with asymmetric capability (e.g. a German SentiWS-only metric on a
// panel that also holds a French source) can never bind a metric that
// silently yields empty cells. The scope is the union of `scope`/`scopeId`,
// `probeIds`, and `sourceIds`; at least one must be present (the call-site
// gates the query off when the panel has no resolvable scope).
export interface ScopeAvailableMetricsParams {
  scope?: ViewModeScope | undefined;
  scopeId?: string | undefined;
  probeIds?: readonly string[] | undefined;
  sourceIds?: readonly string[] | undefined;
  start?: string | undefined;
  end?: string | undefined;
}

export function scopeAvailableMetricsQuery(
  ctx: FetchContext,
  params: ScopeAvailableMetricsParams
): QueryOptions<ScopeAvailableMetricsDto> {
  const qs = new URLSearchParams();
  if (params.scope) qs.set('scope', params.scope);
  if (params.scopeId) qs.set('scopeId', params.scopeId);
  if (params.probeIds && params.probeIds.length > 0) qs.set('probeIds', params.probeIds.join(','));
  if (params.sourceIds && params.sourceIds.length > 0)
    qs.set('sourceIds', params.sourceIds.join(','));
  if (params.start) qs.set('start', params.start);
  if (params.end) qs.set('end', params.end);
  return {
    queryKey: ['aer', 'scope-available-metrics', params] as const,
    queryFn: () =>
      fetchJson<ScopeAvailableMetricsDto>(
        ctx,
        `/scope/available-metrics?${qs.toString()}`,
        'unspecified'
      ),
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
  if (params.start) qs.set('start', params.start);
  if (params.end) qs.set('end', params.end);
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

// Phase 133 — categorical metadata distribution. `field` is a categorical
// metadata field (section / author / tags / …) carried in the panel's `metric`
// slot when the active presentation is `categorical_distribution`. Top-N values
// by distinct-article count over the scope; absent field → empty distribution.
export function metadataDistributionQuery(
  ctx: FetchContext,
  field: string,
  params: ViewModeQueryParams & { topN?: number }
): QueryOptions<CategoricalDistributionResponseDto> {
  const qs = new URLSearchParams();
  qs.set('scope', params.scope);
  qs.set('scopeId', params.scopeId);
  if (params.start) qs.set('start', params.start);
  if (params.end) qs.set('end', params.end);
  if (params.topN) qs.set('topN', String(params.topN));
  return {
    queryKey: ['aer', 'metadata-distribution', field, params] as const,
    queryFn: () =>
      fetchJson<CategoricalDistributionResponseDto>(
        ctx,
        `/metadata/${encodeURIComponent(field)}/distribution?${qs.toString()}`,
        'unspecified'
      ),
    staleTime: FIVE_MINUTES
  };
}

// Phase 133 — categorical metadata fields available across the panel scope (the
// categorical analog of scopeAvailableMetricsQuery). Reuses the same param
// shape; gates which fields the dimension picker offers.
export function scopeAvailableMetadataQuery(
  ctx: FetchContext,
  params: ScopeAvailableMetricsParams
): QueryOptions<ScopeAvailableMetadataDto> {
  const qs = new URLSearchParams();
  if (params.scope) qs.set('scope', params.scope);
  if (params.scopeId) qs.set('scopeId', params.scopeId);
  if (params.probeIds && params.probeIds.length > 0) qs.set('probeIds', params.probeIds.join(','));
  if (params.sourceIds && params.sourceIds.length > 0)
    qs.set('sourceIds', params.sourceIds.join(','));
  if (params.start) qs.set('start', params.start);
  if (params.end) qs.set('end', params.end);
  return {
    queryKey: ['aer', 'scope-available-metadata', params] as const,
    queryFn: () =>
      fetchJson<ScopeAvailableMetadataDto>(
        ctx,
        `/scope/available-metadata?${qs.toString()}`,
        'unspecified'
      ),
    staleTime: FIVE_MINUTES
  };
}

// Phase 131 — paired-metric scatter. `xMetric` / `yMetric` are required;
// `sizeMetric` / `colorMetric` bind the optional visual channels. The BFF
// pivots `aer_gold.metrics` by article and caps the cloud at `maxPoints`.
export function metricScatterQuery(
  ctx: FetchContext,
  params: ViewModeQueryParams & {
    xMetric: string;
    yMetric: string;
    sizeMetric?: string | undefined;
    colorMetric?: string | undefined;
    maxPoints?: number | undefined;
  }
): QueryOptions<ScatterResponseDto> {
  const qs = new URLSearchParams();
  qs.set('scope', params.scope);
  qs.set('scopeId', params.scopeId);
  if (params.start) qs.set('start', params.start);
  if (params.end) qs.set('end', params.end);
  qs.set('xMetric', params.xMetric);
  qs.set('yMetric', params.yMetric);
  if (params.sizeMetric) qs.set('sizeMetric', params.sizeMetric);
  if (params.colorMetric) qs.set('colorMetric', params.colorMetric);
  if (params.maxPoints) qs.set('maxPoints', String(params.maxPoints));
  return {
    queryKey: ['aer', 'metric-scatter', params] as const,
    queryFn: () =>
      fetchJson<ScatterResponseDto>(ctx, `/metrics/scatter?${qs.toString()}`, 'validation_missing'),
    staleTime: FIVE_MINUTES
  };
}

// Phase 125 — pairwise correlation matrix over an N-metric set. Cross-frame
// without equivalence ⇒ the BFF returns a 400 refusal (gate=metric_equivalence).
export function correlationMatrixQuery(
  ctx: FetchContext,
  params: ViewModeQueryParams & { metrics: string[] }
): QueryOptions<CorrelationMatrixDto> {
  const qs = new URLSearchParams();
  qs.set('scope', params.scope);
  qs.set('scopeId', params.scopeId);
  if (params.start) qs.set('start', params.start);
  if (params.end) qs.set('end', params.end);
  qs.set('metrics', params.metrics.join(','));
  return {
    queryKey: ['aer', 'metric-correlation', params] as const,
    queryFn: () =>
      fetchJson<CorrelationMatrixDto>(
        ctx,
        `/metrics/correlation?${qs.toString()}`,
        'cross_frame_equivalence_missing'
      ),
    staleTime: FIVE_MINUTES
  };
}

// Phase 125 — cross-tab: a categorical field × a numeric metric. Cross-frame
// without equivalence ⇒ the BFF 400-refuses (gate=metric_equivalence).
export function crossTabQuery(
  ctx: FetchContext,
  field: string,
  metric: string,
  params: ViewModeQueryParams & { topN?: number }
): QueryOptions<CrossTabDto> {
  const qs = new URLSearchParams();
  qs.set('scope', params.scope);
  qs.set('scopeId', params.scopeId);
  if (params.start) qs.set('start', params.start);
  if (params.end) qs.set('end', params.end);
  if (params.topN) qs.set('topN', String(params.topN));
  const path = `/metadata/${encodeURIComponent(field)}/by-metric/${encodeURIComponent(metric)}`;
  return {
    queryKey: ['aer', 'metadata-crosstab', field, metric, params] as const,
    queryFn: () =>
      fetchJson<CrossTabDto>(ctx, `${path}?${qs.toString()}`, 'cross_frame_equivalence_missing'),
    staleTime: FIVE_MINUTES
  };
}

// Phase 125 — generalised metric lead-lag (does xMetric lead yMetric over the
// scope?). Cross-frame without equivalence ⇒ the BFF 400-refuses.
export function correlationLeadLagQuery(
  ctx: FetchContext,
  params: ViewModeQueryParams & { xMetric: string; yMetric: string; maxLagHours?: number }
): QueryOptions<CorrelationLeadLagDto> {
  const qs = new URLSearchParams();
  qs.set('scope', params.scope);
  qs.set('scopeId', params.scopeId);
  if (params.start) qs.set('start', params.start);
  if (params.end) qs.set('end', params.end);
  qs.set('xMetric', params.xMetric);
  qs.set('yMetric', params.yMetric);
  if (params.maxLagHours) qs.set('maxLagHours', String(params.maxLagHours));
  return {
    queryKey: ['aer', 'correlation-lead-lag', params] as const,
    queryFn: () =>
      fetchJson<CorrelationLeadLagDto>(
        ctx,
        `/correlation/lead-lag?${qs.toString()}`,
        'cross_frame_equivalence_missing'
      ),
    staleTime: FIVE_MINUTES
  };
}

// Phase 125 — per-article N-metric matrix for parallel coordinates. Cross-frame
// without equivalence ⇒ the BFF 400-refuses.
export function parallelCoordsQuery(
  ctx: FetchContext,
  params: ViewModeQueryParams & { metrics: string[]; maxPoints?: number }
): QueryOptions<ParallelCoordsDto> {
  const qs = new URLSearchParams();
  qs.set('scope', params.scope);
  qs.set('scopeId', params.scopeId);
  if (params.start) qs.set('start', params.start);
  if (params.end) qs.set('end', params.end);
  qs.set('metrics', params.metrics.join(','));
  if (params.maxPoints) qs.set('maxPoints', String(params.maxPoints));
  return {
    queryKey: ['aer', 'metric-parallel', params] as const,
    queryFn: () =>
      fetchJson<ParallelCoordsDto>(
        ctx,
        `/metrics/parallel?${qs.toString()}`,
        'cross_frame_equivalence_missing'
      ),
    staleTime: FIVE_MINUTES
  };
}

// Phase 125 — alluvial flow across an ordered chain of categorical fields.
export function sankeyQuery(
  ctx: FetchContext,
  fields: string[],
  params: ViewModeQueryParams & { topN?: number }
): QueryOptions<SankeyDto> {
  const qs = new URLSearchParams();
  qs.set('scope', params.scope);
  qs.set('scopeId', params.scopeId);
  if (params.start) qs.set('start', params.start);
  if (params.end) qs.set('end', params.end);
  qs.set('fields', fields.join(','));
  if (params.topN) qs.set('topN', String(params.topN));
  return {
    queryKey: ['aer', 'metadata-sankey', fields, params] as const,
    queryFn: () =>
      fetchJson<SankeyDto>(ctx, `/metadata/sankey?${qs.toString()}`, 'validation_missing'),
    staleTime: FIVE_MINUTES
  };
}

export function entityCoOccurrenceQuery(
  ctx: FetchContext,
  params: ViewModeQueryParams & { topN?: number; viewerLanguage?: string; nodeMetric?: string }
): QueryOptions<CoOccurrenceGraphDto> {
  const qs = new URLSearchParams();
  qs.set('scope', params.scope);
  qs.set('scopeId', params.scopeId);
  if (params.start) qs.set('start', params.start);
  if (params.end) qs.set('end', params.end);
  if (params.topN) qs.set('topN', String(params.topN));
  // Phase 123b — cross-lingual relabel: when set, the BFF attaches a
  // viewer-language label per QID-linked node.
  if (params.viewerLanguage) qs.set('viewerLanguage', params.viewerLanguage);
  // Phase 125 — when set, each node carries `metricValue` (mean of this metric
  // over the entity's articles) for the metric size/colour channels.
  if (params.nodeMetric) qs.set('nodeMetric', params.nodeMetric);
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

// Phase 122i / ADR-034 — Multi-Panel Workbench CoOccurrence query.
//
// Posts an explicit list of ScopeGroups so a single Rhizome Cell can merge
// several `(probeIds, sourceIds)` slices into one network. The BFF unions
// the groups and applies two structural gates:
//   - 413 scope_limit_exceeded (>100 sources or >25 probes after union)
//   - 422 cross_language_merge_unsupported (scope spans >1 manifest language)
// Both surface as `RefusalOutcome` with the corresponding `refusalKind`.
//
// The legacy single-scope `entityCoOccurrenceQuery` (GET) stays for
// Phase-122h call-sites and backward compatibility.

export interface CoOccurrenceMultiScopeGroup {
  probeIds: readonly string[];
  sourceIds: readonly string[];
}

export interface CoOccurrenceMultiParams {
  scopes: readonly CoOccurrenceMultiScopeGroup[];
  start?: string | undefined;
  end?: string | undefined;
  topN?: number;
  // Phase 123b — cross-lingual relabel viewer language (omit to keep source form).
  viewerLanguage?: string | undefined;
}

export function entityCoOccurrenceQueryMulti(
  ctx: FetchContext,
  params: CoOccurrenceMultiParams
): QueryOptions<CoOccurrenceGraphDto> {
  return {
    queryKey: ['aer', 'entity-cooccurrence-multi', params] as const,
    queryFn: () =>
      fetchJson<CoOccurrenceGraphDto>(ctx, '/entities/cooccurrence/query', 'validation_missing', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          scopes: params.scopes.map((g) => ({
            probeIds: [...g.probeIds],
            sourceIds: [...g.sourceIds]
          })),
          windowStart: params.start,
          windowEnd: params.end,
          ...(params.topN !== undefined ? { topN: params.topN } : {}),
          ...(params.viewerLanguage ? { viewerLanguage: params.viewerLanguage } : {})
        })
      }),
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
  start?: string | undefined;
  end?: string | undefined;
  bins?: number;
}

export function silverAggregationQuery(
  ctx: FetchContext,
  aggregationType: SilverAggregationType,
  params: SilverAggregationParams
): QueryOptions<SilverAggregationResponseDto> {
  const qs = new URLSearchParams();
  qs.set('sourceId', params.sourceId);
  if (params.start) qs.set('start', params.start);
  if (params.end) qs.set('end', params.end);
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

// -------------------------------------------------------------------------
// Phase 122d.0 — Silent-Edit Observability (ADR-032).
//
// `revision_activity` (Aleph) and `revision_timeline` (Episteme) share
// one BFF endpoint `/revisions`; the difference is the `?resolution=`
// parameter (`snapshot` collapses to one bucket per source, the rest
// bucket on a calendar grain). The per-article chain endpoint
// `/articles/{id}/revisions` carries the Silver-eligibility gate, so
// the call-site surfaces refusals like the rest of the per-article
// surface.
// -------------------------------------------------------------------------

export interface RevisionActivityParams {
  scope: ViewModeScope;
  scopeId: string;
  start?: string | undefined;
  end?: string | undefined;
  resolution: RevisionActivityResolution;
}

export function revisionActivityQuery(
  ctx: FetchContext,
  params: RevisionActivityParams
): QueryOptions<RevisionActivityResponseDto> {
  const qs = new URLSearchParams();
  qs.set('scope', params.scope);
  qs.set('scopeId', params.scopeId);
  if (params.start) qs.set('startDate', params.start);
  if (params.end) qs.set('endDate', params.end);
  qs.set('resolution', params.resolution);
  return {
    queryKey: ['aer', 'revision-activity', params] as const,
    queryFn: () =>
      fetchJson<RevisionActivityResponseDto>(ctx, `/revisions?${qs.toString()}`, 'unspecified'),
    staleTime: FIVE_MINUTES
  };
}

// Phase 122d.3 — discourse-shift trajectory. Same scope/window/resolution
// grammar as `revisionActivityQuery`; reads the `/revisions/discourse-shift`
// aggregation (per-source sentiment-delta + topic-shift over the window).
export function revisionDiscourseShiftQuery(
  ctx: FetchContext,
  params: RevisionActivityParams
): QueryOptions<RevisionDiscourseShiftResponseDto> {
  const qs = new URLSearchParams();
  qs.set('scope', params.scope);
  qs.set('scopeId', params.scopeId);
  if (params.start) qs.set('startDate', params.start);
  if (params.end) qs.set('endDate', params.end);
  qs.set('resolution', params.resolution);
  return {
    queryKey: ['aer', 'revision-discourse-shift', params] as const,
    queryFn: () =>
      fetchJson<RevisionDiscourseShiftResponseDto>(
        ctx,
        `/revisions/discourse-shift?${qs.toString()}`,
        'unspecified'
      ),
    staleTime: FIVE_MINUTES
  };
}

// Phase 122d.3 — Rhizome coordinated-edit clusters. Cross-source
// temporally-clustered silent edits on shared entities. `minSources`
// (default 2) is the coincidence threshold.
export interface RevisionEditClustersParams extends RevisionActivityParams {
  minSources?: number | undefined;
}

export function revisionEditClustersQuery(
  ctx: FetchContext,
  params: RevisionEditClustersParams
): QueryOptions<RevisionEditClustersResponseDto> {
  const qs = new URLSearchParams();
  qs.set('scope', params.scope);
  qs.set('scopeId', params.scopeId);
  if (params.start) qs.set('startDate', params.start);
  if (params.end) qs.set('endDate', params.end);
  qs.set('resolution', params.resolution);
  if (params.minSources) qs.set('minSources', String(params.minSources));
  return {
    queryKey: ['aer', 'revision-edit-clusters', params] as const,
    queryFn: () =>
      fetchJson<RevisionEditClustersResponseDto>(
        ctx,
        `/revisions/edit-clusters?${qs.toString()}`,
        'unspecified'
      ),
    staleTime: FIVE_MINUTES
  };
}

export function articleRevisionsQuery(
  ctx: FetchContext,
  articleId: string
): QueryOptions<ArticleRevisionsResponseDto> {
  return {
    queryKey: ['aer', 'article-revisions', articleId] as const,
    queryFn: () =>
      fetchJson<ArticleRevisionsResponseDto>(
        ctx,
        `/articles/${encodeURIComponent(articleId)}/revisions`,
        'silver_eligibility'
      ),
    staleTime: FIVE_MINUTES
  };
}

// -------------------------------------------------------------------------
// Phase 122d.1 — Diff Substance + Drilldown queries.
// -------------------------------------------------------------------------

export interface RevisionsArticlesParams {
  scope: 'probe' | 'source';
  scopeId: string;
  start?: string | undefined;
  end?: string | undefined;
  hasHeadlineChange?: boolean;
  minChainLength?: number;
  limit?: number;
  cursor?: string;
}

export function revisionsArticlesQuery(
  ctx: FetchContext,
  params: RevisionsArticlesParams
): QueryOptions<RevisionsArticlesPageDto> {
  const qs = new URLSearchParams();
  qs.set('scope', params.scope);
  qs.set('scopeId', params.scopeId);
  if (params.start) qs.set('startDate', params.start);
  if (params.end) qs.set('endDate', params.end);
  if (params.hasHeadlineChange) qs.set('hasHeadlineChange', 'true');
  if (params.minChainLength && params.minChainLength > 1)
    qs.set('minChainLength', String(params.minChainLength));
  if (params.limit) qs.set('limit', String(params.limit));
  if (params.cursor) qs.set('cursor', params.cursor);
  return {
    queryKey: ['aer', 'revisions-articles', params] as const,
    queryFn: () =>
      fetchJson<RevisionsArticlesPageDto>(
        ctx,
        `/revisions/articles?${qs.toString()}`,
        'unspecified'
      ),
    staleTime: FIVE_MINUTES
  };
}

export function articleRevisionDiffQuery(
  ctx: FetchContext,
  articleId: string,
  revisionIndex: number
): QueryOptions<ArticleRevisionDiffDto> {
  return {
    queryKey: ['aer', 'article-revision-diff', articleId, revisionIndex] as const,
    queryFn: () =>
      fetchJson<ArticleRevisionDiffDto>(
        ctx,
        `/articles/${encodeURIComponent(articleId)}/revisions/${revisionIndex}/diff`,
        'silver_eligibility'
      ),
    staleTime: FIVE_MINUTES
  };
}
