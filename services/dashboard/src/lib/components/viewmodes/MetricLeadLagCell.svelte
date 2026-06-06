<script lang="ts">
  // Phase 125 — generalised metric lead-lag (Rhizome). The lagged cross-
  // correlation of two metrics' hourly mean series over one scope: "does xMetric
  // lead yMetric?". The two metrics bind to channels.x / channels.y (the
  // leadLagAxes lever). Backed by `GET /correlation/lead-lag`. A multi-language
  // scope without an equivalence grant → refusal-as-cell. Reuses the Phase-124
  // lead-lag rendering; Observable Plot is lazy-imported.
  import { createQuery } from '@tanstack/svelte-query';
  import { onDestroy } from 'svelte';
  import {
    correlationLeadLagQuery,
    type CorrelationLeadLagDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import { DEFAULT_METRIC_NAME } from '$lib/viewmodes';
  import type { ViewModeCellProps } from '$lib/viewmodes';
  import { type ExportPayload, type ExportRow } from '$lib/viewmodes/cell-export';
  import { composeHowToRead } from '$lib/viewmodes/how-to-read';
  import { fmtValue, HIDDEN_READOUT, type ReadoutState } from '$lib/viewmodes/cell-readout';
  import CellExport from './CellExport.svelte';
  import CellReadout from './CellReadout.svelte';
  import HowToRead from './HowToRead.svelte';

  let {
    ctx,
    scope,
    scopeId,
    windowStart,
    windowEnd,
    metadataFilter,
    dataLayer = 'gold',
    channels,
    configOverridden
  }: ViewModeCellProps = $props();

  const xMetric = $derived(channels?.x ?? 'word_count');
  const yMetric = $derived(channels?.y ?? DEFAULT_METRIC_NAME);

  const llQ = createQuery<
    QueryOutcome<CorrelationLeadLagDto>,
    Error,
    QueryOutcome<CorrelationLeadLagDto>
  >(() => {
    const o = correlationLeadLagQuery(ctx, {
      scope,
      scopeId,
      start: windowStart,
      end: windowEnd,
      metadataFilter,
      xMetric,
      yMetric
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: dataLayer !== 'silver'
    };
  });

  const result = $derived<CorrelationLeadLagDto | null>(
    llQ.data?.kind === 'success' ? llQ.data.data : null
  );
  const refusalData = $derived(llQ.data?.kind === 'refusal' ? llQ.data : null);
  const isNetworkError = $derived(llQ.isError || llQ.data?.kind === 'network-error');

  type Point = { lagHours: number; correlation: number };
  const definedPoints = $derived<Point[]>(
    (result?.points ?? [])
      .filter((p): p is { lagHours: number; correlation: number } => p.correlation != null)
      .map((p) => ({ lagHours: p.lagHours, correlation: p.correlation }))
  );
  const peakLagHours = $derived<number | null>(result?.peakLagHours ?? null);
  const peakCorrelation = $derived<number | null>(result?.peakCorrelation ?? null);
  const bucketsAtZero = $derived<number>(result?.bucketCountAtZero ?? 0);

  const leadLagSentence = $derived.by<string>(() => {
    if (peakLagHours === null || peakCorrelation === null) {
      return 'No correlated rhythm found in the overlapping window.';
    }
    const r = peakCorrelation.toFixed(2);
    if (peakLagHours === 0) return `In step — best alignment at lag 0 (r = ${r}).`;
    const h = Math.abs(peakLagHours);
    return peakLagHours > 0
      ? `${yMetric} follows ${xMetric} by ${h}h (peak r = ${r}).`
      : `${yMetric} leads ${xMetric} by ${h}h (peak r = ${r}).`;
  });

  let host: HTMLDivElement | undefined = $state();
  let plotEl: HTMLElement | null = null;
  let renderToken = 0;
  const xInvertRef: { current: ((px: number) => number) | null } = { current: null };

  $effect(() => {
    const pts = definedPoints;
    const peakLag = peakLagHours;
    const peakCorr = peakCorrelation;
    if (!host || pts.length === 0) return;
    const token = ++renderToken;
    (async () => {
      const Plot = await import('@observablehq/plot');
      if (!host || token !== renderToken) return;
      const corrs = pts.map((p) => p.correlation);
      const yMin = Math.max(-1, Math.min(0, ...corrs) - 0.05);
      const yMax = Math.min(1, Math.max(0, ...corrs) + 0.05);
      const peak =
        peakLag !== null && peakCorr !== null ? [{ lagHours: peakLag, correlation: peakCorr }] : [];
      const marks = [
        Plot.ruleY([0], { stroke: 'var(--color-border)' }),
        Plot.ruleX([0], { stroke: 'var(--color-border)', strokeDasharray: '3,3' }),
        Plot.line(pts, {
          x: 'lagHours',
          y: 'correlation',
          stroke: 'rgba(154, 143, 184, 0.95)',
          strokeWidth: 1.5
        }),
        Plot.dot(pts, { x: 'lagHours', y: 'correlation', r: 2, fill: 'rgba(154, 143, 184, 0.95)' })
      ];
      if (peak.length > 0) {
        marks.push(
          Plot.ruleX(peak, { x: 'lagHours', stroke: '#c8a85a', strokeDasharray: '2,2' }),
          Plot.dot(peak, {
            x: 'lagHours',
            y: 'correlation',
            r: 5,
            fill: '#c8a85a',
            stroke: 'var(--color-bg-elevated)',
            strokeWidth: 1.5
          })
        );
      }
      const next = Plot.plot({
        width: host.clientWidth,
        height: 240,
        marginLeft: 52,
        marginBottom: 40,
        x: { label: `lag (hours) — ${yMetric} relative to ${xMetric} →`, grid: true, nice: true },
        y: { label: 'correlation', domain: [yMin, yMax], grid: true },
        marks
      });
      if (plotEl) plotEl.remove();
      // eslint-disable-next-line svelte/no-dom-manipulating
      host.appendChild(next as unknown as HTMLElement);
      plotEl = next as unknown as HTMLElement;
      const xScale = (
        next as unknown as { scale: (n: string) => { invert?: (v: number) => number } | undefined }
      ).scale('x');
      xInvertRef.current = xScale?.invert ?? null;
    })();
  });

  let readout = $state<ReadoutState>(HIDDEN_READOUT);
  function onHostMove(ev: MouseEvent): void {
    const invert = xInvertRef.current;
    if (!host || !invert || definedPoints.length === 0) {
      readout = HIDDEN_READOUT;
      return;
    }
    const svg = host.querySelector('svg');
    if (!svg) {
      readout = HIDDEN_READOUT;
      return;
    }
    const lag = invert(ev.clientX - svg.getBoundingClientRect().left);
    let best = definedPoints[0]!;
    for (const p of definedPoints) {
      if (Math.abs(p.lagHours - lag) < Math.abs(best.lagHours - lag)) best = p;
    }
    readout = {
      visible: true,
      x: ev.clientX,
      y: ev.clientY,
      title: `lag ${best.lagHours >= 0 ? '+' : ''}${best.lagHours} h`,
      rows: [{ label: 'correlation', value: fmtValue(best.correlation) }],
      hint:
        best.lagHours > 0
          ? `${yMetric} follows`
          : best.lagHours < 0
            ? `${yMetric} leads`
            : 'in step'
    };
  }

  onDestroy(() => {
    if (plotEl) plotEl.remove();
    plotEl = null;
  });

  const howToReadFacts = $derived({
    x: xMetric,
    y: yMetric,
    renderedCount: bucketsAtZero,
    configOverridden
  });
  const exportRows = $derived<ExportRow[]>(
    (result?.points ?? []).map((p) => ({ lag_hours: p.lagHours, correlation: p.correlation ?? '' }))
  );
  const exportPayload = $derived<ExportPayload>({
    meta: {
      viewMode: 'metric_lead_lag',
      xMetric,
      yMetric,
      scope,
      scopeId,
      windowStart,
      windowEnd
    },
    howToRead: composeHowToRead('metric_lead_lag', howToReadFacts),
    rows: exportRows,
    columns: ['lag_hours', 'correlation']
  });
  const exportFilenameParts = $derived([
    'metric-lead-lag',
    `${xMetric}-vs-${yMetric}`,
    scope === 'source' ? scopeId : 'probe'
  ]);
  let cellEl: HTMLElement | undefined = $state();
  function getNode(): HTMLElement | null {
    return cellEl ?? null;
  }
</script>

<section class="leadlag-cell" aria-labelledby="metric-leadlag-title" bind:this={cellEl}>
  <header class="cell-header">
    <h3 id="metric-leadlag-title" class="cell-title">
      <span>Lead-lag</span>
      <span class="muted">
        — <code>{xMetric}</code> → <code>{yMetric}</code> ·
        <strong class="scope-name">{scopeId}</strong>
      </span>
    </h3>
    {#if result && definedPoints.length > 0}
      <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
    {/if}
  </header>

  {#if dataLayer === 'silver'}
    <p class="muted">Lead-lag operates on Gold-layer per-article metrics.</p>
  {:else if llQ.isPending}
    <p class="muted" aria-busy="true">Computing lead-lag…</p>
  {:else if refusalData}
    <RefusalSurface refusal={refusalData} {ctx} />
  {:else if isNetworkError}
    <p class="muted">Could not load lead-lag.</p>
  {:else if result && definedPoints.length === 0}
    <p class="muted">
      The two metrics have no overlapping hourly buckets in this window, so no lead-lag can be
      computed. Widen the time window or pick metrics the scope carries together.
    </p>
  {:else if result}
    <p class="leadlag-takeaway">{leadLagSentence}</p>
    <div
      class="plot-host"
      bind:this={host}
      role="img"
      aria-label="Lead-lag cross-correlation between {xMetric} and {yMetric}"
      onmousemove={onHostMove}
      onmouseleave={() => (readout = HIDDEN_READOUT)}
    ></div>
    <CellReadout {readout} />
    <HowToRead presentation="metric_lead_lag" facts={howToReadFacts} />
  {/if}
</section>

<style>
  .leadlag-cell {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .cell-header {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
  }
  .cell-title {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-medium);
    color: var(--color-fg);
    margin: 0;
    display: flex;
    gap: var(--space-2);
    align-items: baseline;
    flex-wrap: wrap;
  }
  .cell-title code {
    font-family: var(--font-mono);
  }
  .leadlag-takeaway {
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    margin: 0;
    font-weight: var(--font-weight-medium);
  }
  .plot-host {
    width: 100%;
    min-height: 240px;
  }
  .plot-host :global(text) {
    fill: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: 11px;
  }
  .plot-host :global(svg) {
    background: transparent;
  }
  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
  .muted code {
    font-family: var(--font-mono);
  }
  .scope-name {
    color: var(--color-fg);
    font-weight: var(--font-weight-medium);
    font-family: var(--font-mono);
  }
</style>
