<script lang="ts">
  import { sanitizePlotA11y } from '$lib/presentations/plot-a11y';
  // Phase 133 — categorical metadata distribution cell (Aleph).
  // One bar per category value of a categorical metadata FIELD (section /
  // author / tags / …), ranked by distinct-article count, backed by
  // `GET /api/v1/metadata/{field}/distribution`. Field-driven: the chosen field
  // rides in `metricName` (Panel.metric) because the presentation declares
  // `usesMetadataField`. Observable Plot is lazy-imported (Brief §7).
  import { createQuery } from '@tanstack/svelte-query';
  import { onDestroy } from 'svelte';
  import {
    metadataDistributionQuery,
    type CategoricalDistributionResponseDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import type { PresentationCellProps } from '$lib/presentations';
  import { type ExportRow, type ExportPayload } from '$lib/presentations/cell-export';
  import { composeHowToRead } from '$lib/presentations/how-to-read';
  import {
    fmtValue,
    markIndexFromEvent,
    HIDDEN_READOUT,
    type ReadoutState
  } from '$lib/presentations/cell-readout';
  import CellExport from './CellExport.svelte';
  import CellReadout from './CellReadout.svelte';
  import CellEmptyState from './CellEmptyState.svelte';
  import HowToRead from './HowToRead.svelte';
  import { m } from '$lib/paraglide/messages.js';
  import { fieldLabel } from '$lib/state/labels.svelte';

  let {
    ctx,
    scope,
    scopeId,
    windowStart,
    windowEnd,
    metricName,
    metadataFilter,
    topN,
    configOverridden
  }: PresentationCellProps = $props();

  // The categorical FIELD lives in the metric slot. `topN` (Top N lever) caps
  // the number of bars; default 20, BFF-clamped to [1, 200].
  const field = $derived(metricName);
  const activeTopN = $derived(topN ?? 20);

  const distQ = createQuery<
    QueryOutcome<CategoricalDistributionResponseDto>,
    Error,
    QueryOutcome<CategoricalDistributionResponseDto>
  >(() => {
    const o = metadataDistributionQuery(ctx, field, {
      scope,
      scopeId,
      start: windowStart,
      end: windowEnd,
      metadataFilter,
      topN: activeTopN
    });
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  const data = $derived(distQ.data?.kind === 'success' ? distQ.data.data : null);
  const refusalData = $derived(distQ.data?.kind === 'refusal' ? distQ.data : null);
  const isNetworkError = $derived(distQ.isError || distQ.data?.kind === 'network-error');
  const isEmpty = $derived(!!data && data.categories.length === 0);

  // Plot rows = the top-N category bars. The long tail is disclosed TEXTUALLY
  // ("showing top N of M") rather than as an aggregate bar: for a list field the
  // tail weight is a value-occurrence sum, not a distinct-article count, so a
  // single "other" bar on the distinct-article y-axis would overstate it (and a
  // real value literally named "(other)" would collide on the ordinal x-domain).
  const plotRows = $derived.by(() => {
    if (!data) return [] as Array<{ value: string; articles: number }>;
    return data.categories.map((c) => ({ value: c.value, articles: c.articles }));
  });

  let host: HTMLDivElement | undefined = $state();
  let plotEl: HTMLElement | null = null;
  let renderToken = 0;

  $effect(() => {
    const rows = plotRows;
    if (!host || rows.length === 0) return;
    const token = ++renderToken;
    (async () => {
      const Plot = await import('@observablehq/plot');
      if (!host || token !== renderToken) return;
      const next = Plot.plot({
        width: host.clientWidth,
        height: 260,
        marginLeft: 56,
        marginBottom: 84,
        x: {
          label: fieldLabel(field),
          // Preserve the BFF rank order (articles desc).
          domain: rows.map((r) => r.value),
          tickRotate: -40
        },
        y: { label: m.cells_cat_axis_articles(), grid: true, tickFormat: 'd' },
        marks: [
          Plot.barY(rows, {
            x: 'value',
            y: 'articles',
            fill: 'rgba(82, 131, 184, 0.6)',
            stroke: 'rgba(82, 131, 184, 0.95)'
          }),
          Plot.ruleY([0])
        ]
      });
      if (plotEl) plotEl.remove();
      // eslint-disable-next-line svelte/no-dom-manipulating
      host.appendChild(sanitizePlotA11y(next as unknown as HTMLElement));
      plotEl = next as unknown as HTMLElement;
    })();
  });

  onDestroy(() => {
    if (plotEl) plotEl.remove();
    plotEl = null;
  });

  // Exact-value hover readout. Plot.barY renders one <rect> per bar in input
  // order, so the DOM-order rect index maps onto `plotRows`.
  let readout = $state<ReadoutState>(HIDDEN_READOUT);
  function onPlotMove(ev: MouseEvent): void {
    const rows = plotRows;
    const idx = markIndexFromEvent(ev.target, 'rect');
    if (idx === null || !rows[idx]) {
      readout = HIDDEN_READOUT;
      return;
    }
    const r = rows[idx];
    readout = {
      visible: true,
      x: ev.clientX,
      y: ev.clientY,
      title: fieldLabel(field),
      rows: [
        { label: m.cells_cat_readout_value(), value: r.value },
        { label: m.cells_cat_readout_articles(), value: fmtValue(r.articles) }
      ]
    };
  }

  const exportRows = $derived<ExportRow[]>(
    (data?.categories ?? []).map((c) => ({ value: c.value, articles: c.articles }))
  );
  const exportPayload = $derived<ExportPayload>({
    meta: {
      viewMode: 'categorical_distribution',
      field,
      scope,
      scopeId,
      windowStart,
      windowEnd,
      topN: activeTopN
    },
    summary: data
      ? {
          totalArticles: data.totalArticles,
          distinctValues: data.distinctValues,
          shown: data.categories.length,
          otherArticles: data.otherArticles
        }
      : undefined,
    howToRead: composeHowToRead('categorical_distribution', {
      topN: activeTopN,
      renderedCount: data?.categories.length,
      distinctValues: data?.distinctValues,
      configOverridden
    }),
    rows: exportRows,
    columns: ['value', 'articles']
  });
  const exportFilenameParts = $derived([
    'category-distribution',
    field,
    scope === 'source' ? scopeId : 'probe'
  ]);
  let cellEl: HTMLElement | undefined = $state();
  function getNode(): HTMLElement | null {
    return cellEl ?? null;
  }
</script>

<section class="cat-cell" aria-labelledby="cat-title-{field}" bind:this={cellEl}>
  <header class="cell-header">
    <h3 id="cat-title-{field}" class="cell-title">
      <code>{fieldLabel(field)}</code>
      <span class="muted"
        >— {m.cells_cat_subtitle()} · <strong class="scope-name">{scopeId}</strong></span
      >
    </h3>
    {#if data && data.categories.length > 0}
      <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
    {/if}
  </header>

  {#if distQ.isPending}
    <p class="muted" aria-busy="true">{m.cells_cat_loading()}</p>
  {:else if refusalData}
    <RefusalSurface refusal={refusalData} {ctx} />
  {:else if isNetworkError}
    <p class="muted">{m.cells_cat_error()}</p>
  {:else if isEmpty}
    <CellEmptyState label={fieldLabel(field)} />
  {:else if data}
    <div
      class="plot-host"
      bind:this={host}
      role="img"
      aria-label={m.cells_cat_plot_aria({ field: fieldLabel(field) })}
      onmousemove={onPlotMove}
      onmouseleave={() => (readout = HIDDEN_READOUT)}
    ></div>
    <CellReadout {readout} />
    <p class="cat-note">
      <strong>{fmtValue(data.totalArticles)}</strong>
      {data.totalArticles === 1
        ? m.cells_cat_note_articles_one()
        : m.cells_cat_note_articles_other()} ·
      <strong>{data.distinctValues}</strong>
      {data.distinctValues === 1
        ? m.cells_cat_note_distinct_one()
        : m.cells_cat_note_distinct_other()}{#if data.distinctValues > data.categories.length}
        · {m.cells_cat_note_showing_top({ count: data.categories.length })}{/if}
    </p>
    <HowToRead
      presentation="categorical_distribution"
      facts={{
        topN: activeTopN,
        renderedCount: data.categories.length,
        distinctValues: data.distinctValues,
        configOverridden
      }}
    />
  {/if}
</section>

<style>
  .cat-cell {
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
  }
  .cell-title code {
    font-family: var(--font-mono);
  }
  .plot-host {
    width: 100%;
    min-height: 260px;
  }
  .plot-host :global(text) {
    fill: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: 11px;
  }
  .plot-host :global(svg) {
    background: transparent;
  }
  .cat-note {
    margin: var(--space-2) 0 0;
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
