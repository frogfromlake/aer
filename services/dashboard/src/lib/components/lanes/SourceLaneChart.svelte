<script lang="ts">
  // Per-source time-series chart within a Function Lane (Phase 106).
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

  interface Props {
    sourceName: string;
    emicDesignation: string | null | undefined;
    ctx: FetchContext;
    windowStart: string;
    windowEnd: string;
    metricName: string;
  }

  let { sourceName, emicDesignation, ctx, windowStart, windowEnd, metricName }: Props = $props();

  const metricsQ = createQuery<
    QueryOutcome<MetricsResponseDto>,
    Error,
    QueryOutcome<MetricsResponseDto>
  >(() => {
    const o = metricsQuery(ctx, {
      startDate: windowStart,
      endDate: windowEnd,
      source: sourceName,
      metricName,
      resolution: 'hourly'
    });
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  function toChartData(result: QueryOutcome<MetricsResponseDto>): { x: number[]; y: number[] } {
    if (result.kind !== 'success') return { x: [], y: [] };
    const pts = result.data.data
      .map((d) => ({ t: Date.parse(d.timestamp) / 1000, v: d.value }))
      .filter((d) => Number.isFinite(d.t) && Number.isFinite(d.v))
      .sort((a, b) => a.t - b.t);
    return { x: pts.map((p) => p.t), y: pts.map((p) => p.v) };
  }

  let chartData = $derived.by(() => {
    if (metricsQ.data?.kind === 'success') return toChartData(metricsQ.data);
    return null;
  });
</script>

<div class="source-lane" aria-labelledby="sl-title-{sourceName}">
  <h3 id="sl-title-{sourceName}" class="source-lane-title">
    <code>{sourceName}</code>
    {#if emicDesignation}
      <span class="emic-note">— {emicDesignation}</span>
    {/if}
  </h3>

  {#if metricsQ.isPending}
    <div class="chart-placeholder" aria-busy="true">
      <p class="muted">Loading {metricName}…</p>
    </div>
  {:else if metricsQ.isError}
    <div class="chart-placeholder">
      <p class="muted">Could not load metrics.</p>
    </div>
  {:else if metricsQ.data?.kind === 'refusal'}
    <RefusalSurface refusal={metricsQ.data} {ctx} />
  {:else if chartData && chartData.x.length > 0}
    <TimeSeriesChart
      x={chartData.x}
      y={chartData.y}
      yLabel={metricName}
      ariaLabel="{metricName} for {sourceName}"
      height={180}
    />
  {:else}
    <div class="chart-placeholder">
      <p class="muted">No {metricName} data in this window.</p>
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
</style>
