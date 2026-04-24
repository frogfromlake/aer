<script lang="ts">
  // Thin Svelte 5 wrapper over uPlot (Design Brief §5.9 — scientific
  // charts are framework-agnostic; the wrapper exists only to tie uPlot's
  // imperative lifecycle to Svelte's reactive one). uPlot itself is
  // dynamic-imported so it lands in a lazy chunk and never hits the
  // initial bundle — L3 is the first layer that needs it.
  import { onDestroy, onMount } from 'svelte';
  import 'uplot/dist/uPlot.min.css';

  interface Props {
    /** Seconds-epoch x-axis values. */
    x: number[];
    /** Primary series values aligned with `x`. */
    y: number[];
    /** Optional lower uncertainty band, aligned with `x`. */
    yLow?: number[] | null;
    /** Optional upper uncertainty band, aligned with `x`. */
    yHigh?: number[] | null;
    /** Axis/series label for the y dimension. */
    yLabel: string;
    /** Visual weight multiplier (Epistemic Weight — 0.4..1.0). */
    weight?: number;
    /** Accessible chart label. */
    ariaLabel: string;
    /** Plot height in px. Defaults to 200 to keep the 99b footprint. */
    height?: number;
  }

  let {
    x,
    y,
    yLow = null,
    yHigh = null,
    yLabel,
    weight = 1,
    ariaLabel,
    height = 200
  }: Props = $props();

  // uPlot ships as CJS `export = uPlot` with an ambient namespace; in
  // ESM/bundler resolution it arrives as `{ default: Ctor }`. We keep
  // the type loose and let the runtime cast happen at import time.
  interface UPlotChart {
    setSize(size: { width: number; height: number }): void;
    setData(data: unknown): void;
    destroy(): void;
  }
  type UPlotCtor = new (opts: unknown, data: unknown, target: HTMLElement) => UPlotChart;

  let host: HTMLDivElement | undefined = $state();
  let chart: UPlotChart | null = null;
  let ro: ResizeObserver | null = null;

  function currentData(): [number[], number[], number[], number[]] {
    const low = yLow && yLow.length === x.length ? yLow : y;
    const high = yHigh && yHigh.length === x.length ? yHigh : y;
    return [x, y, low, high];
  }

  onMount(() => {
    if (!host) return;
    (async () => {
      const mod = (await import('uplot')) as unknown as { default: UPlotCtor };
      const UPlot = mod.default;
      if (!host) return;
      const w = Math.max(0.3, Math.min(1, weight));
      const stroke = `rgba(82, 131, 184, ${w})`;
      const bandFill = `rgba(82, 131, 184, ${Math.max(0.08, w * 0.18)})`;
      const opts = {
        width: host.clientWidth,
        height,
        scales: { x: { time: true } },
        axes: [
          { stroke: '#888', grid: { stroke: 'rgba(255,255,255,0.06)' } },
          { stroke: '#888', grid: { stroke: 'rgba(255,255,255,0.06)' }, label: yLabel }
        ],
        series: [
          {},
          { label: yLabel, stroke, width: 1.5 },
          { label: 'lower', stroke: 'transparent' },
          { label: 'upper', stroke: 'transparent' }
        ],
        bands: [{ series: [3, 2], fill: bandFill }],
        legend: { show: false },
        cursor: { drag: { x: true, y: false } }
      };
      chart = new UPlot(opts, currentData(), host);
      ro = new ResizeObserver(() => {
        if (chart && host) chart.setSize({ width: host.clientWidth, height });
      });
      ro.observe(host);
    })();
  });

  $effect(() => {
    if (chart) chart.setData(currentData());
  });

  onDestroy(() => {
    ro?.disconnect();
    ro = null;
    chart?.destroy();
    chart = null;
  });
</script>

<div
  bind:this={host}
  class="chart"
  role="img"
  aria-label={ariaLabel}
  style:opacity={Math.max(0.4, Math.min(1, weight))}
></div>

<style>
  .chart {
    width: 100%;
  }
  .chart :global(.u-legend) {
    display: none;
  }
</style>
