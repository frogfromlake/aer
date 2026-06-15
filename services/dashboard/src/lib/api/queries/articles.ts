import type {
  ArticleDetailDto,
  ArticlesPageDto,
  AvailableMetricDto,
  CategoricalDistributionResponseDto,
  DistributionResponseDto,
  FetchContext,
  PresentationQueryParams,
  PresentationScope,
  QueryOptions,
  ScopeAvailableMetadataDto,
  ScopeAvailableMetricsDto,
  MetricsAvailableParams
} from './shared';
import { appendMetadataFilter, fetchJson, FIVE_MINUTES } from './shared';

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
// `$lib/presentations/` decides which factory a given cell uses.
// -------------------------------------------------------------------------

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
  scope?: PresentationScope | undefined;
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
  params: PresentationQueryParams & { bins?: number }
): QueryOptions<DistributionResponseDto> {
  const qs = new URLSearchParams();
  qs.set('scope', params.scope);
  qs.set('scopeId', params.scopeId);
  if (params.start) qs.set('start', params.start);
  if (params.end) qs.set('end', params.end);
  if (params.bins) qs.set('bins', String(params.bins));
  appendMetadataFilter(qs, params.metadataFilter);
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
  params: PresentationQueryParams & { topN?: number }
): QueryOptions<CategoricalDistributionResponseDto> {
  const qs = new URLSearchParams();
  qs.set('scope', params.scope);
  qs.set('scopeId', params.scopeId);
  if (params.start) qs.set('start', params.start);
  if (params.end) qs.set('end', params.end);
  if (params.topN) qs.set('topN', String(params.topN));
  appendMetadataFilter(qs, params.metadataFilter);
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
