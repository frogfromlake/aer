<script lang="ts">
  // Phase 125 — pairwise Pearson correlation matrix (Aleph, multivariate).
  // An N×N heatmap over the chosen metric set (`Panel.metricSet`), backed by
  // `GET /metrics/correlation` (per-bucket-mean correlation; stated in the
  // how-to-read). Cross-frame without equivalence → refusal-as-cell. Observable
  // Plot is lazy-imported (Brief §7 budget).
  import { createQuery } from '@tanstack/svelte-query';
  import { onDestroy } from 'svelte';
  import {
    correlationMatrixQuery,
    type CorrelationMatrixDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import type { PresentationCellProps } from '$lib/presentations';
  import { type ExportRow, type ExportPayload } from '$lib/presentations/cell-export';
  import { composeHowToRead } from '$lib/presentations/how-to-read';
  import CellExport from './CellExport.svelte';
  import CellEmptyState from './CellEmptyState.svelte';
  import HowToRead from './HowToRead.svelte';
  import { m } from '$lib/paraglide/messages.js';

  let {
    ctx,
    scope,
    scopeId,
    windowStart,
    windowEnd,
    metadataFilter,
    dataLayer = 'gold',
    metricSet,
    configOverridden
  }: PresentationCellProps = $props();

  // The metric set rides in Panel.metricSet; the matrix needs ≥2. PanelControls
  // seeds a scope-available default on first switch, so an empty set only
  // appears transiently / when the user clears it.
  const metrics = $derived<string[]>([...(metricSet ?? [])]);
  const enoughMetrics = $derived(metrics.length >= 2);

  const corrQ = createQuery<
    QueryOutcome<CorrelationMatrixDto>,
    Error,
    QueryOutcome<CorrelationMatrixDto>
  >(() => {
    const o = correlationMatrixQuery(ctx, {
      scope,
      scopeId,
      start: windowStart,
      end: windowEnd,
      metadataFilter,
      metrics
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: dataLayer !== 'silver' && enoughMetrics
    };
  });

  const data = $derived<CorrelationMatrixDto | null>(
    corrQ.data?.kind === 'success' ? corrQ.data.data : null
  );
  const refusalData = $derived(corrQ.data?.kind === 'refusal' ? corrQ.data : null);
  const isNetworkError = $derived(corrQ.isError || corrQ.data?.kind === 'network-error');

  // Flatten the NxN matrix into Plot.cell rows. `matrix[i][j]` is the
  // correlation of metrics[i] × metrics[j]; null when too few overlapping
  // buckets (rendered blank, never a coerced zero).
  type CorrCell = { row: string; col: string; r: number | null };
  const cells = $derived.by<CorrCell[]>(() => {
    if (!data) return [];
    const out: CorrCell[] = [];
    for (let i = 0; i < data.metrics.length; i++) {
      const rowName = data.metrics[i] ?? '';
      const rowVals = data.matrix[i] ?? [];
      for (let j = 0; j < data.metrics.length; j++) {
        out.push({ row: rowName, col: data.metrics[j] ?? '', r: rowVals[j] ?? null });
      }
    }
    return out;
  });
  const hasAnyValue = $derived(cells.some((c) => c.r !== null));

  let host: HTMLDivElement | undefined = $state();
  let plotEl: HTMLElement | null = null;
  let renderToken = 0;

  $effect(() => {
    const rows = cells;
    const order = data?.metrics ?? [];
    if (!host || rows.length === 0) return;
    const token = ++renderToken;
    (async () => {
      const Plot = await import('@observablehq/plot');
      if (!host || token !== renderToken) return;
      const drawable = rows.filter((c) => c.r !== null);
      // Phase 125 (ISSUE 3) — cells with too few overlapping buckets are honest
      // empties, not absent: render them as a dashed placeholder, never blank
      // space (DISCLOSE-NEVER-COERCE). Hover explains why.
      const blanks = rows.filter((c) => c.r === null);
      const side = Math.max(220, Math.min(480, order.length * 64 + 120));
      const next = Plot.plot({
        width: host.clientWidth || side,
        height: side,
        marginLeft: 120,
        marginBottom: 96,
        marginTop: 12,
        x: { domain: [...order], tickRotate: -40, label: null },
        y: { domain: [...order], label: null },
        color: {
          scheme: 'rdbu',
          domain: [-1, 1],
          legend: true,
          label: m.cells_corr_legend_label()
        },
        marks: [
          Plot.cell(blanks, {
            x: 'col',
            y: 'row',
            fill: 'none',
            stroke: 'currentColor',
            strokeOpacity: 0.3,
            strokeDasharray: '2,2',
            inset: 0.5,
            title: (d: CorrCell) => m.cells_corr_tooltip_blank({ rowName: d.row, colName: d.col })
          }),
          Plot.cell(drawable, {
            x: 'col',
            y: 'row',
            fill: 'r',
            inset: 0.5,
            // Phase 125 (ISSUE 4) — hover tooltip naming the metric pair + r.
            title: (d: CorrCell) =>
              m.cells_corr_tooltip_value({
                rowName: d.row,
                colName: d.col,
                r: (d.r ?? 0).toFixed(2)
              })
          }),
          Plot.text(drawable, {
            x: 'col',
            y: 'row',
            text: (d: CorrCell) => (d.r === null ? '' : d.r.toFixed(2)),
            // Phase 125 (ISSUE 4) — white on saturated cells, strong fg
            // (currentColor; see .plot-host { color }) on pale cells. The old
            // global text-fill override muted these into illegibility.
            fill: (d: CorrCell) => (Math.abs(d.r ?? 0) > 0.5 ? 'white' : 'currentColor'),
            fontSize: 11,
            fontWeight: 600
          })
        ]
      });
      if (plotEl) plotEl.remove();
      // eslint-disable-next-line svelte/no-dom-manipulating
      host.appendChild(next as unknown as HTMLElement);
      plotEl = next as unknown as HTMLElement;
    })();
  });

  onDestroy(() => {
    if (plotEl) plotEl.remove();
    plotEl = null;
  });

  const howToReadFacts = $derived({
    renderedCount: data?.metrics.length,
    configOverridden
  });
  const exportRows = $derived<ExportRow[]>(
    cells.map((c) => ({ metricA: c.row, metricB: c.col, r: c.r ?? '' }))
  );
  const exportPayload = $derived<ExportPayload>({
    meta: {
      viewMode: 'correlation_matrix',
      metrics: metrics.join(', '),
      scope,
      scopeId,
      windowStart,
      windowEnd,
      resolution: data?.resolution ?? ''
    },
    summary: data ? { metricCount: data.metrics.length, bucketCount: data.bucketCount } : undefined,
    howToRead: composeHowToRead('correlation_matrix', howToReadFacts),
    rows: exportRows,
    columns: ['metricA', 'metricB', 'r']
  });
  const exportFilenameParts = $derived([
    'correlation-matrix',
    scope === 'source' ? scopeId : 'probe'
  ]);
  let cellEl: HTMLElement | undefined = $state();
  function getNode(): HTMLElement | null {
    return cellEl ?? null;
  }
</script>

<section class="corr-cell" aria-labelledby="corr-title" bind:this={cellEl}>
  <header class="cell-header">
    <h3 id="corr-title" class="cell-title">
      {m.cells_corr_title()}
      <span class="muted">— <strong class="scope-name">{scopeId}</strong></span>
    </h3>
    {#if data && hasAnyValue}
      <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
    {/if}
  </header>

  {#if dataLayer === 'silver'}
    <p class="muted">{m.cells_corr_silver()}</p>
  {:else if !enoughMetrics}
    <p class="muted">{m.cells_corr_need_metrics()}</p>
  {:else if corrQ.isPending}
    <p class="muted" aria-busy="true">{m.cells_corr_loading()}</p>
  {:else if refusalData}
    <RefusalSurface refusal={refusalData} {ctx} />
  {:else if isNetworkError}
    <p class="muted">{m.cells_corr_error()}</p>
  {:else if data && !hasAnyValue}
    <CellEmptyState />
  {:else if data}
    <div
      class="plot-host"
      bind:this={host}
      role="img"
      aria-label={m.cells_corr_plot_aria({ count: metrics.length })}
    ></div>
    <HowToRead presentation="correlation_matrix" facts={howToReadFacts} />
  {/if}
</section>

<style>
  .corr-cell {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .cell-header {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--space-3);
    flex-wrap: wrap;
  }
  .cell-title {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-medium);
    color: var(--color-fg);
    margin: 0;
    display: flex;
    gap: var(--space-2);
    align-items: baseline;
  }
  .plot-host {
    width: 100%;
    min-height: 240px;
    /* currentColor for axis ticks, the dashed empty-cell stroke, and pale-cell
       value labels — strong fg so r labels stay legible (Phase 125 ISSUE 4). */
    color: var(--color-fg);
  }
  .plot-host :global(text) {
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
  .scope-name {
    color: var(--color-fg);
    font-weight: var(--font-weight-medium);
    font-family: var(--font-mono);
  }
</style>
