<script lang="ts">
  // Phase 125 — parallel coordinates (Aleph, multivariate). One polyline per
  // article across N metric axes (`Panel.metricSet`); each axis is independently
  // min–max normalised so axes on different scales are comparable in shape.
  // Backed by `GET /metrics/parallel`. Cross-frame without equivalence → refusal.
  // Observable Plot is lazy-imported (no new dependency).
  import { createQuery } from '@tanstack/svelte-query';
  import { onDestroy } from 'svelte';
  import { parallelCoordsQuery, type ParallelCoordsDto, type QueryOutcome } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import type { PresentationCellProps } from '$lib/presentations';
  import { type ExportRow, type ExportPayload } from '$lib/presentations/cell-export';
  import { composeHowToRead } from '$lib/presentations/how-to-read';
  import CellExport from './CellExport.svelte';
  import CellEmptyState from './CellEmptyState.svelte';
  import HowToRead from './HowToRead.svelte';

  let {
    ctx,
    scope,
    scopeId,
    windowStart,
    windowEnd,
    metadataFilter,
    dataLayer = 'gold',
    metricSet,
    configOverridden,
    selection
  }: PresentationCellProps = $props();

  const metrics = $derived<string[]>([...(metricSet ?? [])]);
  const enoughMetrics = $derived(metrics.length >= 2);

  const pcQ = createQuery<QueryOutcome<ParallelCoordsDto>, Error, QueryOutcome<ParallelCoordsDto>>(
    () => {
      const o = parallelCoordsQuery(ctx, {
        scope,
        scopeId,
        start: windowStart,
        end: windowEnd,
        metadataFilter,
        metrics,
        maxPoints: 3000
      });
      return {
        queryKey: [...o.queryKey],
        queryFn: o.queryFn,
        staleTime: o.staleTime,
        enabled: dataLayer !== 'silver' && enoughMetrics
      };
    }
  );

  const data = $derived<ParallelCoordsDto | null>(
    pcQ.data?.kind === 'success' ? pcQ.data.data : null
  );
  const refusalData = $derived(pcQ.data?.kind === 'refusal' ? pcQ.data : null);
  const isNetworkError = $derived(pcQ.isError || pcQ.data?.kind === 'network-error');
  const isEmpty = $derived(!!data && data.rows.length === 0);

  // Long-form, per-axis min–max normalised points: one entry per (article, axis).
  type PCPoint = { id: string; articleId: string; axis: string; y: number };
  const longPoints = $derived.by<PCPoint[]>(() => {
    if (!data || data.rows.length === 0) return [];
    const axes = data.metrics;
    // Per-axis extents.
    const mins = axes.map(() => Infinity);
    const maxs = axes.map(() => -Infinity);
    for (const r of data.rows) {
      for (let j = 0; j < axes.length; j++) {
        const v = r.values[j];
        if (v === undefined) continue;
        if (v < mins[j]!) mins[j] = v;
        if (v > maxs[j]!) maxs[j] = v;
      }
    }
    const out: PCPoint[] = [];
    for (const r of data.rows) {
      const id = `${r.source}:${r.articleId}`;
      for (let j = 0; j < axes.length; j++) {
        const v = r.values[j];
        if (v === undefined) continue;
        const span = maxs[j]! - mins[j]!;
        const y = span > 0 ? (v - mins[j]!) / span : 0.5;
        out.push({ id, articleId: r.articleId, axis: axes[j]!, y });
      }
    }
    return out;
  });

  let host: HTMLDivElement | undefined = $state();
  let plotEl: HTMLElement | null = null;
  let renderToken = 0;

  $effect(() => {
    const pts = longPoints;
    const axes = data?.metrics ?? [];
    // Phase 125b — cross-panel brushing: track the selection size synchronously
    // so the render re-runs when it changes.
    const selSet = selection?.ids;
    const selN = selSet?.size ?? 0;
    if (!host || pts.length === 0) return;
    const token = ++renderToken;
    const baseOpacity = Math.max(0.06, Math.min(0.5, 30 / Math.max(1, data?.rows.length ?? 1)));
    (async () => {
      const Plot = await import('@observablehq/plot');
      if (!host || token !== renderToken) return;
      const next = Plot.plot({
        width: host.clientWidth || 720,
        height: 320,
        marginLeft: 40,
        marginRight: 40,
        marginBottom: 84,
        x: { domain: [...axes], label: null, tickRotate: -40 },
        y: { domain: [0, 1], label: 'per-axis min–max', ticks: 4, tickFormat: '.0%' },
        marks: [
          Plot.ruleX(axes, { stroke: 'var(--color-border)' }),
          Plot.line(pts, {
            x: 'axis',
            y: 'y',
            z: 'id',
            // Selected lines (their article ∈ the Window selection) emphasised;
            // the rest dim when a selection is active. Per-group constant since
            // every point of a line shares articleId.
            stroke: (d: PCPoint) =>
              selN > 0 && selSet!.has(d.articleId) ? 'var(--color-fg)' : 'rgba(82, 131, 184, 0.85)',
            strokeWidth: (d: PCPoint) => (selN > 0 && selSet!.has(d.articleId) ? 1.4 : 0.6),
            strokeOpacity: (d: PCPoint) =>
              selN === 0 ? baseOpacity : selSet!.has(d.articleId) ? 0.95 : 0.04
          }),
          // Pointer layer enables `plot.value` (nearest vertex) for click-toggle.
          ...(selection
            ? [Plot.dot(pts, Plot.pointer({ x: 'axis', y: 'y', r: 3, fill: 'var(--color-fg)' }))]
            : [])
        ]
      });
      if (plotEl) plotEl.remove();
      if (selection) {
        next.addEventListener('click', () => {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          const d = (next as any).value as PCPoint | undefined;
          if (d?.articleId) selection.toggle(d.articleId);
        });
      }
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
    renderedCount: data?.rows.length,
    configOverridden
  });
  const exportRows = $derived<ExportRow[]>(
    (data?.rows ?? []).map((r) => {
      const row: ExportRow = { articleId: r.articleId, source: r.source };
      (data?.metrics ?? []).forEach((m, j) => {
        row[m] = r.values[j] ?? '';
      });
      return row;
    })
  );
  const exportPayload = $derived<ExportPayload>({
    meta: {
      viewMode: 'parallel_coordinates',
      metrics: metrics.join(', '),
      scope,
      scopeId,
      windowStart,
      windowEnd,
      truncated: data?.truncated ? 'yes' : 'no'
    },
    summary: data ? { articles: data.rows.length } : undefined,
    howToRead: composeHowToRead('parallel_coordinates', howToReadFacts),
    rows: exportRows,
    columns: ['articleId', 'source', ...metrics]
  });
  const exportFilenameParts = $derived([
    'parallel-coordinates',
    scope === 'source' ? scopeId : 'probe'
  ]);
  let cellEl: HTMLElement | undefined = $state();
  function getNode(): HTMLElement | null {
    return cellEl ?? null;
  }
</script>

<section class="pc-cell" aria-labelledby="pc-title" bind:this={cellEl}>
  <header class="cell-header">
    <h3 id="pc-title" class="cell-title">
      Parallel coordinates
      <span class="muted">— <strong class="scope-name">{scopeId}</strong></span>
    </h3>
    {#if data && data.rows.length > 0}
      <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
    {/if}
  </header>

  {#if dataLayer === 'silver'}
    <p class="muted">Parallel coordinates operate on Gold-layer per-article metrics.</p>
  {:else if !enoughMetrics}
    <p class="muted">Pick at least two metrics in the <strong>Metric set</strong> lever.</p>
  {:else if pcQ.isPending}
    <p class="muted" aria-busy="true">Loading parallel coordinates…</p>
  {:else if refusalData}
    <RefusalSurface refusal={refusalData} {ctx} />
  {:else if isNetworkError}
    <p class="muted">Could not load parallel coordinates.</p>
  {:else if isEmpty}
    <CellEmptyState />
  {:else if data}
    {#if data.truncated}
      <p class="pc-note" role="note">Showing the first {data.rows.length} articles (capped).</p>
    {/if}
    <div
      class="plot-host"
      bind:this={host}
      role="img"
      aria-label="Parallel coordinates across {metrics.length} metrics"
    ></div>
    <HowToRead presentation="parallel_coordinates" facts={howToReadFacts} />
  {/if}
</section>

<style>
  .pc-cell {
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
    min-height: 320px;
  }
  .plot-host :global(text) {
    fill: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: 11px;
  }
  .plot-host :global(svg) {
    background: transparent;
  }
  .pc-note {
    margin: 0;
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
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
