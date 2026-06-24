// Multivariate & relational analytics query factories — scatter, correlation,
// cross-tab, lead-lag, parallel-coords, sankey, co-occurrence, topics & silver
// aggregations (Phase 141 split from queries.ts). See ./shared for QueryOutcome.
import type { PresentationQueryParams, PresentationScope } from './shared';
import { appendMetadataFilter } from './shared';
import type {
  CoOccurrenceGraphDto,
  CorrelationLeadLagDto,
  CorrelationMatrixDto,
  CrossTabDto,
  FetchContext,
  ParallelCoordsDto,
  QueryOptions,
  SankeyDto,
  ScatterResponseDto,
  SilverAggregationResponseDto,
  SilverAggregationType,
  TopicDistributionResponseDto
} from './shared';
import { fetchJson, FIVE_MINUTES } from './shared';

// Phase 131 — paired-metric scatter. `xMetric` / `yMetric` are required;
// `sizeMetric` / `colorMetric` bind the optional visual channels. The BFF
// pivots `aer_gold.metrics` by article and caps the cloud at `maxPoints`.
export function metricScatterQuery(
  ctx: FetchContext,
  params: PresentationQueryParams & {
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
  appendMetadataFilter(qs, params.metadataFilter);
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
  params: PresentationQueryParams & { metrics: string[] }
): QueryOptions<CorrelationMatrixDto> {
  const qs = new URLSearchParams();
  qs.set('scope', params.scope);
  qs.set('scopeId', params.scopeId);
  if (params.start) qs.set('start', params.start);
  if (params.end) qs.set('end', params.end);
  qs.set('metrics', params.metrics.join(','));
  appendMetadataFilter(qs, params.metadataFilter);
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
  params: PresentationQueryParams & { topN?: number }
): QueryOptions<CrossTabDto> {
  const qs = new URLSearchParams();
  qs.set('scope', params.scope);
  qs.set('scopeId', params.scopeId);
  if (params.start) qs.set('start', params.start);
  if (params.end) qs.set('end', params.end);
  if (params.topN) qs.set('topN', String(params.topN));
  appendMetadataFilter(qs, params.metadataFilter);
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
  params: PresentationQueryParams & { xMetric: string; yMetric: string; maxLagHours?: number }
): QueryOptions<CorrelationLeadLagDto> {
  const qs = new URLSearchParams();
  qs.set('scope', params.scope);
  qs.set('scopeId', params.scopeId);
  if (params.start) qs.set('start', params.start);
  if (params.end) qs.set('end', params.end);
  qs.set('xMetric', params.xMetric);
  qs.set('yMetric', params.yMetric);
  if (params.maxLagHours) qs.set('maxLagHours', String(params.maxLagHours));
  appendMetadataFilter(qs, params.metadataFilter);
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
  params: PresentationQueryParams & { metrics: string[]; maxPoints?: number }
): QueryOptions<ParallelCoordsDto> {
  const qs = new URLSearchParams();
  qs.set('scope', params.scope);
  qs.set('scopeId', params.scopeId);
  if (params.start) qs.set('start', params.start);
  if (params.end) qs.set('end', params.end);
  qs.set('metrics', params.metrics.join(','));
  if (params.maxPoints) qs.set('maxPoints', String(params.maxPoints));
  appendMetadataFilter(qs, params.metadataFilter);
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
  params: PresentationQueryParams & { topN?: number }
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
  params: PresentationQueryParams & {
    topN?: number;
    viewerLanguage?: string;
    nodeMetric?: string;
    nodeColorMetric?: string;
    minWeight?: number;
    negativeSpaceOverlay?: 'ghost';
    // Phase 148g — node-first breadth control (top-N entities by weight, edges
    // among them). Omit for legacy edge-first behaviour.
    maxNodes?: number;
  }
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
  // Phase 125 / ISSUE 7 — separate colour-channel metric (each node also carries
  // `metricValueColor`). Sent only when it differs from the size metric.
  if (params.nodeColorMetric && params.nodeColorMetric !== params.nodeMetric)
    qs.set('nodeColorMetric', params.nodeColorMetric);
  // Phase 125b — min co-occurrence weight (edge density floor for the at-scale
  // renderer). Omitted when 0 (no thinning).
  if (params.minWeight && params.minWeight > 0) qs.set('minWeight', String(params.minWeight));
  // Phase 148g — node-first breadth (top-N entities by weight; edges among them).
  if (params.maxNodes && params.maxNodes > 0) qs.set('maxNodes', String(params.maxNodes));
  // Phase 122d.2 — Negative-Space overlay: edges carry `nsSupport` (count of
  // contributing articles with no real publication date) when requested.
  if (params.negativeSpaceOverlay) qs.set('negativeSpaceOverlay', params.negativeSpaceOverlay);
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
  // Phase 148g — node-first breadth (top-N entities by weight; edges among them).
  maxNodes?: number | undefined;
  // Phase 148g — explicit user opt-in to a cross-language merge (>1 manifest
  // language). Default/false → the BFF refuses with 422; true → the union renders
  // as one graph WITHOUT merging node identity across languages.
  allowCrossLanguage?: boolean | undefined;
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
          ...(params.maxNodes && params.maxNodes > 0 ? { maxNodes: params.maxNodes } : {}),
          ...(params.allowCrossLanguage ? { allowCrossLanguage: true } : {}),
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
  scope?: PresentationScope | undefined;
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
