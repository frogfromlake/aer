// Revision-history & editorial-diff query factories (Phase 141 split from
// queries.ts). See ./shared for the QueryOutcome contract.
import type { PresentationScope } from './shared';
import type {
  ArticleRevisionDiffDto,
  ArticleRevisionsResponseDto,
  FetchContext,
  QueryOptions,
  RevisionActivityResolution,
  RevisionActivityResponseDto,
  RevisionDiscourseShiftResponseDto,
  RevisionEditClustersResponseDto,
  RevisionsArticlesPageDto
} from './shared';
import { fetchJson, FIVE_MINUTES } from './shared';

export interface RevisionActivityParams {
  scope: PresentationScope;
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
