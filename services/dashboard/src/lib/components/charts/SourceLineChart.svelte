<script lang="ts">
  // Per-source time-series chart within the Workbench (Phase 106).
  // Extracted as a subcomponent so createQuery is called at component init,
  // not inside a reactive derived block.
  import { createQuery } from '@tanstack/svelte-query';
  import {
    metricsQuery,
    type MetricsResponseDto,
    type FetchContext,
    type QueryOutcome
  } from '$lib/api/queries';
  import TimeSeriesChart from '$lib/components/TimeSeriesChart.svelte';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import { urlState } from '$lib/state/url.svelte';
  import type { Resolution, Normalization } from '$lib/state/url-internals';
  import { m } from '$lib/paraglide/messages.js';

  interface Props {
    // Phase 122i revision (D1). Either a single source (legacy + split
    // composition) OR a multi-source merge (composition='merged' with
    // multiple sources in the panel's scope). When `sourceNames.length
    // > 1`, the BFF unions them server-side and returns one time series
    // over the joint corpus — exactly what Merged composition promises.
    sourceName?: string;
    sourceNames?: readonly string[];
    emicDesignation: string | null | undefined;
    ctx: FetchContext;
    /** RFC 3339 bounds, or `undefined` for the whole dataset (no time filter).
     *  The x-axis derives its domain from the returned data, so an absent
     *  window renders the full observed range rather than an empty axis. */
    windowStart?: string | undefined;
    windowEnd?: string | undefined;
    metricName: string;
    /** Phase 131 — render the ±1σ uncertainty band (default true). When set,
     *  the metrics query requests `includeStddev` and the chart draws a band
     *  at value ± stddev per bucket. */
    showBand?: boolean;
    /** Phase 131 (BUG4) — per-panel temporal resolution. Falls back to the
     *  global URL resolution for legacy callers that don't pass it. */
    resolution?: Resolution | undefined;
    /** Phase 131 (BUG4) — Compare/normalization mode threaded into the query. */
    normalization?: Normalization | undefined;
    /** Phase 124 — shared y-axis domain for multi-cell time-series panels.
     *  Forwarded to TimeSeriesChart; null/absent = free (auto) scale. */
    yDomain?: readonly [number, number] | null;
  }

  let {
    sourceName,
    sourceNames,
    emicDesignation,
    ctx,
    windowStart,
    windowEnd,
    metricName,
    showBand = true,
    resolution,
    normalization,
    yDomain = null
  }: Props = $props();

  // Resolve effective scope: prefer multi-source list when present.
  const resolvedSources = $derived<readonly string[]>(
    sourceNames && sourceNames.length > 0 ? sourceNames : sourceName ? [sourceName] : []
  );
  const isMergedMulti = $derived(resolvedSources.length > 1);
  const displayName = $derived(
    isMergedMulti
      ? m.cells_chart_merged_display({
          count: resolvedSources.length,
          list: resolvedSources.join(', ')
        })
      : (resolvedSources[0] ?? '')
  );

  // Phase 122h Findings round 2 §F: honour the user-selected resolution
  // from the URL state (the Episteme Resolution selector writes here).
  // Hardcoded `hourly` previously meant the selector had no effect.
  const url = $derived(urlState());
  // Phase 131 (BUG4): prefer the per-panel resolution; fall back to the global
  // URL resolution for legacy callers that don't thread it.
  const activeResolution = $derived(resolution ?? url.resolution ?? 'hourly');

  const metricsQ = createQuery<
    QueryOutcome<MetricsResponseDto>,
    Error,
    QueryOutcome<MetricsResponseDto>
  >(() => {
    const o = metricsQuery(ctx, {
      ...(windowStart ? { startDate: windowStart } : {}),
      ...(windowEnd ? { endDate: windowEnd } : {}),
      // For single-source (split or legacy), pass `source`. For multi-
      // source merge, pass `sourceIds` so the BFF unions server-side.
      ...(isMergedMulti
        ? { sourceIds: resolvedSources.join(',') }
        : resolvedSources[0]
          ? { source: resolvedSources[0] }
          : {}),
      metricName,
      resolution: activeResolution,
      includeStddev: showBand,
      ...(normalization && normalization !== 'raw' ? { normalization } : {})
    });
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  interface ChartSeries {
    x: number[];
    y: number[];
    low: number[] | null;
    high: number[] | null;
  }

  function toChartData(result: QueryOutcome<MetricsResponseDto>): ChartSeries {
    if (result.kind !== 'success') return { x: [], y: [], low: null, high: null };
    const pts = result.data.data
      .map((d) => ({ t: Date.parse(d.timestamp) / 1000, v: d.value, s: d.stddev ?? null }))
      .filter((d) => Number.isFinite(d.t) && Number.isFinite(d.v))
      .sort((a, b) => a.t - b.t);
    // The band is drawn only when requested AND the response actually carried
    // a spread (>0 for at least one bucket) — a flat band adds no information.
    const hasSpread = showBand && pts.some((p) => p.s !== null && p.s > 0);
    return {
      x: pts.map((p) => p.t),
      y: pts.map((p) => p.v),
      low: hasSpread ? pts.map((p) => p.v - (p.s ?? 0)) : null,
      high: hasSpread ? pts.map((p) => p.v + (p.s ?? 0)) : null
    };
  }

  let chartData = $derived.by(() => {
    if (metricsQ.data?.kind === 'success') return toChartData(metricsQ.data);
    return null;
  });

  // SEC-077: the BFF caps the series at a row limit and discloses it via
  // `truncated`. Surface a note so a wide window/scope is never read as a
  // complete series (the chart would otherwise just end at the earliest buckets).
  let truncated = $derived(
    metricsQ.data?.kind === 'success' && metricsQ.data.data.truncated === true
  );
</script>

<div class="source-lane" aria-labelledby="sl-title-{displayName}">
  <h3 id="sl-title-{displayName}" class="source-lane-title">
    {#if isMergedMulti}
      <code
        >{resolvedSources.length === 1
          ? m.cells_chart_merged_heading_one({ count: resolvedSources.length })
          : m.cells_chart_merged_heading_other({ count: resolvedSources.length })}</code
      >
      <span class="emic-note">— {resolvedSources.join(' ∪ ')}</span>
    {:else}
      <code>{resolvedSources[0] ?? ''}</code>
      {#if emicDesignation}
        <span class="emic-note">— {emicDesignation}</span>
      {/if}
    {/if}
  </h3>

  {#if metricsQ.isPending}
    <div class="chart-placeholder" aria-busy="true">
      <p class="muted">{m.cells_chart_loading({ metric: metricName })}</p>
    </div>
  {:else if metricsQ.isError}
    <div class="chart-placeholder">
      <p class="muted">{m.cells_chart_load_error()}</p>
    </div>
  {:else if metricsQ.data?.kind === 'refusal'}
    <RefusalSurface refusal={metricsQ.data} {ctx} />
  {:else if chartData && chartData.x.length > 0}
    {#if truncated}
      <p class="truncation-note" role="note">{m.cells_ts_truncated()}</p>
    {/if}
    <TimeSeriesChart
      x={chartData.x}
      y={chartData.y}
      yLow={chartData.low}
      yHigh={chartData.high}
      yLabel={metricName}
      ariaLabel={m.cells_chart_series_aria({ metric: metricName, name: displayName })}
      height={180}
      {yDomain}
    />
  {:else}
    <div class="chart-placeholder">
      <p class="muted">{m.cells_chart_no_data({ metric: metricName })}</p>
    </div>
  {/if}
</div>

<style>
  .source-lane {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .source-lane-title {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-medium);
    color: var(--color-fg-muted);
    margin: 0;
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    flex-wrap: wrap;
  }

  .source-lane-title code {
    font-family: var(--font-mono);
    color: var(--color-fg);
  }

  .emic-note {
    font-size: var(--font-size-xs);
    font-style: italic;
    color: var(--color-fg-subtle);
  }

  .chart-placeholder {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 180px;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .truncation-note {
    margin: 0;
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    padding: var(--space-2) var(--space-3);
    background: color-mix(in srgb, var(--color-status-expired) 8%, var(--color-surface));
    border-left: 3px solid var(--color-status-expired);
    border-radius: var(--radius-sm);
  }
</style>
