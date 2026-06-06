<script lang="ts">
  // Metadata-mining × scatter cell (Phase 131).
  //
  // Each point is one article, positioned by two metrics; the size and colour
  // visual channels are each bound to a further metric dimension. This is the
  // Kriesel principle made concrete: it is not "scatter vs chart", it is real
  // visual channels (x / y / size / colour) bound to real per-article data.
  // Backed by `GET /metrics/scatter`. Observable Plot is lazy-imported so its
  // chunk only ships when this cell is selected (Brief §7 budget).
  import { createQuery } from '@tanstack/svelte-query';
  import { onDestroy } from 'svelte';
  import {
    metricScatterQuery,
    type ScatterResponseDto,
    type ScatterPointDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import { DEFAULT_METRIC_NAME } from '$lib/viewmodes';
  import type { ViewModeCellProps } from '$lib/viewmodes';
  import { type ExportRow, type ExportPayload } from '$lib/viewmodes/cell-export';
  import { composeHowToRead } from '$lib/viewmodes/how-to-read';
  import CellExport from './CellExport.svelte';
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
    reportExtent,
    sharedDomains,
    axisScaleState,
    configOverridden,
    selection
  }: ViewModeCellProps = $props();

  // Resolve the bound channels with sensible defaults so the cell renders even
  // before the user touches the axis pickers. x/y default to two distinct
  // first-class metrics; size/colour stay unbound until chosen.
  const xMetric = $derived(channels?.x ?? 'word_count');
  const yMetric = $derived(channels?.y ?? DEFAULT_METRIC_NAME);
  const sizeMetric = $derived(channels?.size);
  const colorMetric = $derived(channels?.color);

  const scatterQ = createQuery<
    QueryOutcome<ScatterResponseDto>,
    Error,
    QueryOutcome<ScatterResponseDto>
  >(() => {
    const o = metricScatterQuery(ctx, {
      scope,
      scopeId,
      start: windowStart,
      end: windowEnd,
      metadataFilter,
      xMetric,
      yMetric,
      sizeMetric,
      colorMetric,
      maxPoints: 3000
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: dataLayer !== 'silver'
    };
  });

  const data = $derived<ScatterResponseDto | null>(
    scatterQ.data?.kind === 'success' ? scatterQ.data.data : null
  );
  const points = $derived(data?.points ?? []);
  const refusalData = $derived(scatterQ.data?.kind === 'refusal' ? scatterQ.data : null);
  const isNetworkError = $derived(scatterQ.isError || scatterQ.data?.kind === 'network-error');

  const hasSize = $derived(!!sizeMetric);
  const hasColor = $derived(!!colorMetric);

  // Phase 125 (A1) — bivariate correlation: OLS fit + Pearson r over the
  // per-article points. Null when there are too few points or an axis has no
  // variance (a vertical/horizontal cloud has no defined slope or r).
  const regression = $derived.by<{ slope: number; intercept: number; r: number; n: number } | null>(
    () => {
      const rows = points;
      const n = rows.length;
      if (n < 3) return null;
      let sx = 0;
      let sy = 0;
      let sxx = 0;
      let syy = 0;
      let sxy = 0;
      for (const p of rows) {
        sx += p.x;
        sy += p.y;
        sxx += p.x * p.x;
        syy += p.y * p.y;
        sxy += p.x * p.y;
      }
      const dx = n * sxx - sx * sx;
      const dy = n * syy - sy * sy;
      if (dx <= 0 || dy <= 0) return null; // no variance on an axis
      const slope = (n * sxy - sx * sy) / dx;
      const intercept = (sy - slope * sx) / n;
      const r = (n * sxy - sx * sy) / Math.sqrt(dx * dy);
      return { slope, intercept, r, n };
    }
  );
  const rLabel = $derived(regression ? regression.r.toFixed(2) : null);

  // Phase 124 — report the x and y (value) extents so PanelHost can union them
  // into the shared domains for a multi-cell scatter panel.
  $effect(() => {
    if (!reportExtent) return;
    const rows = points;
    if (rows.length === 0) {
      reportExtent('x', null);
      reportExtent('value', null);
      return;
    }
    let xlo = Infinity;
    let xhi = -Infinity;
    let ylo = Infinity;
    let yhi = -Infinity;
    for (const p of rows) {
      if (p.x < xlo) xlo = p.x;
      if (p.x > xhi) xhi = p.x;
      if (p.y < ylo) ylo = p.y;
      if (p.y > yhi) yhi = p.y;
    }
    reportExtent('x', [xlo, xhi]);
    reportExtent('value', [ylo, yhi]);
  });

  let host: HTMLDivElement | undefined = $state();
  let plotEl: HTMLElement | null = null;
  let renderToken = 0;

  $effect(() => {
    const rows = points;
    // Phase 124 — shared x / y domains when the panel is on a shared axis.
    const sharedX = sharedDomains?.x;
    const sharedY = sharedDomains?.value;
    // Phase 125b — cross-panel brushing. Read the set's size synchronously so
    // this render re-runs when the Window selection changes; when non-empty,
    // selected points stay opaque and the rest dim (never hidden).
    const selSet = selection?.ids;
    const selN = selSet?.size ?? 0;
    if (!host || rows.length === 0) return;
    const token = ++renderToken;
    (async () => {
      const Plot = await import('@observablehq/plot');
      if (!host || token !== renderToken) return;
      const next = Plot.plot({
        width: host.clientWidth || 720,
        height: 360,
        marginLeft: 60,
        marginBottom: 44,
        grid: true,
        x: {
          label: `${xMetric} →`,
          labelAnchor: 'center',
          ...(sharedX ? { domain: [...sharedX] } : {})
        },
        y: {
          label: `↑ ${yMetric}`,
          labelAnchor: 'center',
          ...(sharedY ? { domain: [...sharedY] } : {})
        },
        ...(hasSize ? { r: { range: [2, 16] } } : {}),
        ...(hasColor
          ? { color: { scheme: 'viridis', legend: true, label: colorMetric ?? null } }
          : {}),
        marks: [
          Plot.dot(rows, {
            x: 'x',
            y: 'y',
            r: hasSize ? 'size' : 3.2,
            fill: hasColor ? 'color' : 'rgba(82, 131, 184, 0.55)',
            stroke: hasColor ? 'rgba(16,20,26,0.6)' : 'rgba(82, 131, 184, 0.95)',
            strokeWidth: 0.5,
            // Phase 125b — dim non-selected points when a cross-panel selection
            // is active; full opacity otherwise.
            fillOpacity: (d: ScatterPointDto) =>
              selN === 0 ? 0.7 : selSet!.has(d.articleId ?? '') ? 0.95 : 0.08,
            channels: { source: { value: 'source', label: 'source' } },
            tip: true
          }),
          // Phase 125b — emphasis ring on the brushed (selected) points.
          ...(selN > 0
            ? [
                Plot.dot(
                  rows.filter((d) => selSet!.has(d.articleId ?? '')),
                  {
                    x: 'x',
                    y: 'y',
                    r: hasSize ? 'size' : 3.2,
                    fill: 'none',
                    stroke: 'var(--color-fg)',
                    strokeWidth: 1.5
                  }
                )
              ]
            : []),
          // Phase 125 (A1) — OLS trend line + ±CI band, drawn on top so the
          // relationship is legible. Only when a fit is defined (variance on
          // both axes, ≥3 points).
          ...(regression
            ? [
                Plot.linearRegressionY(rows, {
                  x: 'x',
                  y: 'y',
                  stroke: '#e0a050',
                  strokeWidth: 1.5
                })
              ]
            : []),
          // Phase 125b — pointer layer enables `plot.value` (closest datum) so a
          // click can toggle that article in the Window selection.
          ...(selection
            ? [
                Plot.dot(
                  rows,
                  Plot.pointer({
                    x: 'x',
                    y: 'y',
                    r: hasSize ? 'size' : 3.2,
                    fill: 'none',
                    stroke: 'var(--color-accent, #e0a050)',
                    strokeWidth: 2
                  })
                )
              ]
            : [])
        ]
      });
      if (plotEl) plotEl.remove();
      // Phase 125b — click toggles the pointed article in the Window selection.
      if (selection) {
        next.addEventListener('click', () => {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          const d = (next as any).value as ScatterPointDto | undefined;
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

  // Export rows + how-to-read facts.
  const exportRows = $derived<ExportRow[]>(
    points.map((p) => ({
      articleId: p.articleId ?? '',
      source: p.source,
      timestamp: p.timestamp,
      [xMetric]: p.x,
      [yMetric]: p.y,
      ...(hasSize ? { [sizeMetric as string]: p.size ?? '' } : {}),
      ...(hasColor ? { [colorMetric as string]: p.color ?? '' } : {})
    }))
  );
  const howToReadFacts = $derived({
    x: xMetric,
    y: yMetric,
    size: sizeMetric,
    color: colorMetric,
    renderedCount: points.length,
    scales: axisScaleState,
    r: regression?.r,
    configOverridden
  });
  const exportPayload = $derived<ExportPayload>({
    meta: {
      viewMode: 'scatter',
      xMetric,
      yMetric,
      sizeMetric: sizeMetric ?? '(none)',
      colorMetric: colorMetric ?? '(none)',
      scope,
      scopeId,
      windowStart,
      windowEnd,
      truncated: data?.truncated ? 'yes' : 'no'
    },
    summary: {
      points: points.length,
      ...(regression ? { pearson_r: regression.r, slope: regression.slope } : {})
    },
    howToRead: composeHowToRead('metric_scatter', howToReadFacts),
    rows: exportRows
  });
  const exportFilenameParts = $derived([
    'scatter',
    `${xMetric}-vs-${yMetric}`,
    scope === 'source' ? scopeId : 'probe'
  ]);
  let cellEl: HTMLElement | undefined = $state();
  function getNode(): HTMLElement | null {
    return cellEl ?? null;
  }
</script>

<section class="scatter-cell" aria-labelledby="scatter-title" bind:this={cellEl}>
  <header class="cell-header">
    <h3 id="scatter-title" class="cell-title">
      Scatter
      <span class="muted"
        >— <code>{xMetric}</code> × <code>{yMetric}</code> ·
        <strong class="scope-name">{scopeId}</strong>{#if rLabel}
          · <span class="r-badge" title="Pearson correlation coefficient">r = {rLabel}</span
          >{/if}</span
      >
    </h3>
    {#if points.length > 0}
      <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
    {/if}
  </header>

  {#if dataLayer === 'silver'}
    <p class="notice">
      Scatter operates on Gold-layer per-article metrics. Switch to Distribution to explore
      Silver-layer document characteristics.
    </p>
  {:else if scatterQ.isPending}
    <p class="muted" aria-busy="true">Loading scatter…</p>
  {:else if refusalData}
    <RefusalSurface refusal={refusalData} {ctx} />
  {:else if isNetworkError}
    <p class="muted">Could not load the scatter.</p>
  {:else if points.length === 0}
    <p class="muted">
      No articles carry both <code>{xMetric}</code> and <code>{yMetric}</code> in this window.
    </p>
  {:else}
    {#if data?.truncated}
      <p class="truncation-note" role="note">
        Showing the first {points.length} articles (capped) — narrow the window or scope for an exhaustive
        cloud.
      </p>
    {/if}
    <div
      class="plot-host"
      bind:this={host}
      role="img"
      aria-label="Scatter plot of {xMetric} versus {yMetric}"
    ></div>
    <HowToRead presentation="metric_scatter" facts={howToReadFacts} />
  {/if}
</section>

<style>
  .scatter-cell {
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
  .cell-title code {
    font-family: var(--font-mono);
  }

  .plot-host {
    width: 100%;
    min-height: 360px;
  }
  .plot-host :global(text) {
    fill: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: 11px;
  }
  .plot-host :global(svg) {
    background: transparent;
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

  .r-badge {
    font-family: var(--font-mono);
    color: #e0a050;
    font-weight: var(--font-weight-medium);
  }

  .notice {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
    padding: var(--space-4);
    background: var(--color-bg-elevated);
    border: 1px dashed var(--color-border-strong);
    border-radius: var(--radius-md);
    max-width: 36rem;
  }
</style>
