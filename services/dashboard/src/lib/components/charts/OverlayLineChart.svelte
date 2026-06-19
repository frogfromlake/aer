<script lang="ts">
  // OverlayLineChart — Phase 122k §14c finding 2.
  //
  // Renders N sources as N separate viridis-coloured lines on a SHARED
  // chart canvas. Different from the SourceLineChart's merged path
  // (which unions sources server-side and returns one aggregate series)
  // and from split composition (which renders N independent charts).
  // Overlay sits between them: visually one chart, but the per-source
  // structure is preserved as distinct lines.
  //
  // Each source produces an independent BFF query; the chart merges the
  // resulting series via a unified x-axis (union of all timestamps) and
  // hands uPlot N series with viridis colour assignments.
  import { createQueries } from '@tanstack/svelte-query';
  import { onDestroy, onMount } from 'svelte';
  import 'uplot/dist/uPlot.min.css';
  import {
    metricsQuery,
    type FetchContext,
    type MetricsResponseDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import { urlState } from '$lib/state/url.svelte';
  import type { Resolution, Normalization } from '$lib/state/url-internals';
  import {
    fmtValue,
    fmtTimestamp,
    HIDDEN_READOUT,
    type ReadoutRow,
    type ReadoutState
  } from '$lib/presentations/cell-readout';
  import CellReadout from '$lib/components/presentations/CellReadout.svelte';
  import { m } from '$lib/paraglide/messages.js';

  interface Props {
    sourceNames: readonly string[];
    ctx: FetchContext;
    /** RFC 3339 bounds, or `undefined` for the whole dataset (no time filter);
     *  the x-axis derives its domain from the returned data. */
    windowStart?: string | undefined;
    windowEnd?: string | undefined;
    metricName: string;
    height?: number;
    /** Phase 131 (BUG4) — per-panel resolution / Compare; fall back to URL. */
    resolution?: Resolution | undefined;
    normalization?: Normalization | undefined;
  }

  let {
    sourceNames,
    ctx,
    windowStart,
    windowEnd,
    metricName,
    height = 220,
    resolution,
    normalization
  }: Props = $props();

  const url = $derived(urlState());
  const activeResolution = $derived(resolution ?? url.resolution ?? 'hourly');

  // Five-stop viridis palette. For N > 5 sources the colours cycle —
  // good enough for K1 production (single probe with ≤ 3 sources).
  const VIRIDIS: ReadonlyArray<string> = ['#440154', '#3b528b', '#21918c', '#5ec962', '#fde725'];

  function colourFor(i: number): string {
    return VIRIDIS[i % VIRIDIS.length]!;
  }

  type SeriesResult = {
    isPending: boolean;
    isError: boolean;
    data: QueryOutcome<MetricsResponseDto> | undefined;
  };

  const queries = createQueries(() => ({
    queries: sourceNames.map((name) => {
      const o = metricsQuery(ctx, {
        ...(windowStart ? { startDate: windowStart } : {}),
        ...(windowEnd ? { endDate: windowEnd } : {}),
        source: name,
        metricName,
        resolution: activeResolution,
        ...(normalization && normalization !== 'raw' ? { normalization } : {})
      });
      return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
    })
  })) as unknown as readonly SeriesResult[];

  const seriesData = $derived.by(() => {
    const out: { name: string; x: number[]; y: number[]; colour: string }[] = [];
    for (let i = 0; i < sourceNames.length; i++) {
      const r = queries[i];
      if (!r || r.data?.kind !== 'success') continue;
      const pts = r.data.data.data
        .map((d) => ({ t: Date.parse(d.timestamp) / 1000, v: d.value }))
        .filter((p) => Number.isFinite(p.t) && Number.isFinite(p.v))
        .sort((a, b) => a.t - b.t);
      out.push({
        name: sourceNames[i]!,
        x: pts.map((p) => p.t),
        y: pts.map((p) => p.v),
        colour: colourFor(i)
      });
    }
    return out;
  });

  // Build a unified x-axis (union of all timestamps, sorted) and align
  // each series' y-values via interpolation-free lookup. Missing y at a
  // timestamp becomes `null` (uPlot draws a gap).
  const aligned = $derived.by(() => {
    // eslint-disable-next-line svelte/prefer-svelte-reactivity -- ephemeral local set, not shared state
    const xSet = new Set<number>();
    for (const s of seriesData) for (const t of s.x) xSet.add(t);
    const x = [...xSet].sort((a, b) => a - b);
    const ys: (number | null)[][] = seriesData.map((s) => {
      // eslint-disable-next-line svelte/prefer-svelte-reactivity -- ephemeral local map, not shared state
      const idx = new Map<number, number>();
      for (let i = 0; i < s.x.length; i++) idx.set(s.x[i]!, s.y[i]!);
      return x.map((t) => idx.get(t) ?? null);
    });
    return { x, ys };
  });

  const anyPending = $derived(queries.some((q) => q.isPending));
  const allEmpty = $derived(seriesData.length === 0 || seriesData.every((s) => s.x.length === 0));

  interface UPlotChart {
    setSize(size: { width: number; height: number }): void;
    setData(data: unknown): void;
    destroy(): void;
  }
  type UPlotCtor = new (opts: unknown, data: unknown, target: HTMLElement) => UPlotChart;

  let host: HTMLDivElement | undefined = $state();
  let chart: UPlotChart | null = null;
  let ro: ResizeObserver | null = null;
  let mounted = false;
  let lastSeriesShape = '';

  function currentData(): (number | (number | null)[])[] {
    // uPlot expects a tuple of arrays: [x, y0, y1, ...]. The cast keeps
    // the call-site readable while honouring uPlot's mixed-shape input.
    const out: unknown[] = [aligned.x];
    for (const y of aligned.ys) out.push(y);
    return out as unknown as (number | (number | null)[])[];
  }

  // Phase 132 — exact-value hover readout via uPlot's native cursor. The
  // chart is recreated on series-shape change; this mutable ref keeps the
  // cursor hook reading the live series names/colours regardless.
  let readout = $state<ReadoutState>(HIDDEN_READOUT);
  const meta = { series: [] as { name: string; colour: string }[] };
  $effect(() => {
    meta.series = seriesData.map((s) => ({ name: s.name, colour: s.colour }));
  });

  interface UPlotCursorView {
    cursor: { idx: number | null; left: number; top: number };
    data: (number | null)[][];
    over: HTMLElement;
  }
  function onCursor(u: UPlotCursorView): void {
    const idx = u.cursor.idx;
    if (idx == null || !u.data?.[0]) {
      readout = HIDDEN_READOUT;
      return;
    }
    const rows: ReadoutRow[] = [];
    for (let i = 0; i < meta.series.length; i++) {
      const v = u.data[i + 1]?.[idx];
      if (v == null) continue;
      rows.push({
        label: meta.series[i]!.name,
        value: fmtValue(v),
        swatch: meta.series[i]!.colour
      });
    }
    if (rows.length === 0) {
      readout = HIDDEN_READOUT;
      return;
    }
    const rect = u.over.getBoundingClientRect();
    readout = {
      visible: true,
      x: rect.left + u.cursor.left,
      y: rect.top + u.cursor.top,
      title: fmtTimestamp(u.data[0][idx] as number),
      rows
    };
  }

  function chartOpts(width: number) {
    return {
      width,
      height,
      scales: { x: { time: true } },
      axes: [
        { stroke: '#888', grid: { stroke: 'rgba(255,255,255,0.06)' } },
        { stroke: '#888', grid: { stroke: 'rgba(255,255,255,0.06)' }, label: metricName }
      ],
      series: [
        {},
        ...seriesData.map((s) => ({
          label: s.name,
          stroke: s.colour,
          width: 1.5,
          // Phase 122k §14c — sources with sparser timestamp coverage
          // (e.g. bundesregierung vs. tagesschau) get `null` at every
          // timestamp where the OTHER source has data but they don't.
          // `spanGaps: true` tells uPlot to draw a continuous line over
          // those nulls instead of fragmenting at every gap. Each
          // series effectively traces its own real datapoints connected
          // smoothly across the shared x-axis.
          spanGaps: true
        }))
      ],
      legend: { show: false },
      cursor: { drag: { x: true, y: false } },
      hooks: { setCursor: [(u: unknown) => onCursor(u as UPlotCursorView)] }
    };
  }

  onMount(() => {
    mounted = true;
  });

  // Phase 122k §14c — chart lifecycle via $effect (NOT onMount).
  //
  // The chart `<div bind:this={host}>` is conditionally mounted: it only
  // appears when queries finish and data is non-empty. `onMount` fires
  // at component-mount time when the div doesn't exist yet → `host` is
  // undefined → chart never gets created. Using $effect lets us react
  // to BOTH host availability AND series shape changes:
  //
  //   - First eligible render (host present, data ready): create chart.
  //   - Series shape changes (e.g., one source's query resolves later):
  //     destroy and recreate so uPlot picks up the new series count.
  //   - Data-only updates within the same series shape: setData().
  $effect(() => {
    if (!host || !mounted) return;
    if (seriesData.length === 0) return;
    const shape = seriesData.map((s) => s.name).join('|');
    if (chart && shape === lastSeriesShape) {
      chart.setData(currentData());
      return;
    }
    // Capture local refs for the async closure so the cleanup function
    // doesn't double-destroy if the effect re-fires before async finishes.
    const localHost = host;
    const targetShape = shape;
    if (chart) {
      chart.destroy();
      chart = null;
    }
    lastSeriesShape = targetShape;
    (async () => {
      const mod = (await import('uplot')) as unknown as { default: UPlotCtor };
      const UPlot = mod.default;
      if (!mounted || lastSeriesShape !== targetShape) return;
      chart = new UPlot(chartOpts(localHost.clientWidth || 400), currentData(), localHost);
      ro?.disconnect();
      ro = new ResizeObserver(() => {
        if (chart && localHost) chart.setSize({ width: localHost.clientWidth, height });
      });
      ro.observe(localHost);
    })();
  });

  onDestroy(() => {
    mounted = false;
    ro?.disconnect();
    ro = null;
    chart?.destroy();
    chart = null;
  });
</script>

<div class="overlay-lane">
  <header class="overlay-header">
    <h3 class="overlay-title">
      <code
        >{sourceNames.length === 1
          ? m.cells_overlay_heading_one({ count: sourceNames.length })
          : m.cells_overlay_heading_other({ count: sourceNames.length })}</code
      >
    </h3>
    <ul class="legend" role="list">
      {#each seriesData as s (s.name)}
        <li>
          <span class="swatch" style:background={s.colour} aria-hidden="true"></span>
          <code class="legend-name">{s.name}</code>
        </li>
      {/each}
    </ul>
  </header>

  {#if anyPending}
    <div class="chart-placeholder" aria-busy="true">
      <p class="muted">{m.cells_chart_loading({ metric: metricName })}</p>
    </div>
  {:else if allEmpty}
    <div class="chart-placeholder">
      <p class="muted">{m.cells_chart_no_data({ metric: metricName })}</p>
    </div>
  {:else}
    <div
      bind:this={host}
      class="chart"
      role="img"
      aria-label={m.cells_overlay_aria({ metric: metricName, count: sourceNames.length })}
      onmouseleave={() => (readout = HIDDEN_READOUT)}
    ></div>
  {/if}
  <CellReadout {readout} />
</div>

<style>
  .overlay-lane {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .overlay-header {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--space-3);
    flex-wrap: wrap;
  }

  .overlay-title {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-medium);
    color: var(--color-fg-muted);
    margin: 0;
  }
  .overlay-title code {
    font-family: var(--font-mono);
    color: var(--color-fg);
  }

  .legend {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }
  .legend li {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .swatch {
    display: inline-block;
    width: 10px;
    height: 10px;
    border-radius: 2px;
  }
  .legend-name {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
  }

  .chart {
    width: 100%;
  }
  .chart :global(.u-legend) {
    display: none;
  }

  .chart-placeholder {
    min-height: 8rem;
    display: grid;
    place-items: center;
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
</style>
