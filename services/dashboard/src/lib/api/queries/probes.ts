// Probe-level query factories — probes, per-probe metrics/provenance/content,
// equivalence, lead-lag, dossier & coverage (Phase 141 split from queries.ts).
// See ./shared for the QueryOutcome contract.
import type { paths } from '../types';
import type {
  ContentResponseDto,
  DiscoveryCoverageResponseDto,
  FetchContext,
  MetadataCoverageResponseDto,
  MetadataFieldsResponseDto,
  MetricProvenanceDto,
  MetricsParams,
  MetricsResponseDto,
  ProbeDossierDto,
  ProbeDto,
  QueryOptions,
  RefusalKind
} from './shared';
import { fetchJson, FIVE_MINUTES } from './shared';

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

// Task C — corpus-wide per-field extraction status feeding the Reflection
// "metadata fields" surface. Unscoped (whole corpus); slow cadence like the
// per-source coverage above.
export function metadataFieldsQuery(ctx: FetchContext): QueryOptions<MetadataFieldsResponseDto> {
  return {
    queryKey: ['aer', 'metadata-fields'] as const,
    queryFn: () => fetchJson<MetadataFieldsResponseDto>(ctx, '/metadata-fields', 'unspecified'),
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
