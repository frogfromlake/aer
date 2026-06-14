<script lang="ts">
  // Phase 125 — cross-tab (Aleph): a categorical metadata FIELD × a numeric
  // METRIC → the metric's mean per category value. The field rides in
  // `metricName` (Panel.metric, usesMetadataField); the numeric metric binds to
  // `channels.x`. Backed by `GET /metadata/{field}/by-metric/{metric}`.
  // Cross-frame without equivalence → refusal-as-cell. Plot is lazy-imported.
  import { createQuery } from '@tanstack/svelte-query';
  import { onDestroy } from 'svelte';
  import { crossTabQuery, type CrossTabDto, type QueryOutcome } from '$lib/api/queries';
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
    metricName,
    channels,
    topN,
    configOverridden
  }: PresentationCellProps = $props();

  const field = $derived(metricName); // the categorical field (Group by)
  const metric = $derived(channels?.x); // the numeric metric (crossMetric lever)
  const activeTopN = $derived(topN ?? 20);
  const ready = $derived(!!field && !!metric);

  const ctQ = createQuery<QueryOutcome<CrossTabDto>, Error, QueryOutcome<CrossTabDto>>(() => {
    const o = crossTabQuery(ctx, field, metric ?? '', {
      scope,
      scopeId,
      start: windowStart,
      end: windowEnd,
      metadataFilter,
      topN: activeTopN
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: dataLayer !== 'silver' && ready
    };
  });

  const data = $derived<CrossTabDto | null>(ctQ.data?.kind === 'success' ? ctQ.data.data : null);
  const refusalData = $derived(ctQ.data?.kind === 'refusal' ? ctQ.data : null);
  const isNetworkError = $derived(ctQ.isError || ctQ.data?.kind === 'network-error');
  const rows = $derived(data?.categories ?? []);
  const isEmpty = $derived(!!data && rows.length === 0);

  let host: HTMLDivElement | undefined = $state();
  let plotEl: HTMLElement | null = null;
  let renderToken = 0;

  $effect(() => {
    const r = rows;
    if (!host || r.length === 0) return;
    const token = ++renderToken;
    (async () => {
      const Plot = await import('@observablehq/plot');
      if (!host || token !== renderToken) return;
      const next = Plot.plot({
        width: host.clientWidth || 640,
        height: Math.max(160, r.length * 26 + 60),
        marginLeft: 140,
        marginRight: 16,
        x: { label: `mean ${metric}`, grid: true },
        y: { domain: r.map((d) => d.value), label: null },
        color: { scheme: 'rdbu', legend: true, label: `mean ${metric}` },
        marks: [
          Plot.barX(r, {
            x: 'mean',
            y: 'value',
            fill: 'mean',
            channels: {
              articles: { value: 'articles', label: 'articles' },
              std: { value: 'std', label: '±σ' }
            },
            tip: true
          }),
          Plot.ruleX([0])
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
    renderedCount: data?.categories.length,
    distinctValues: data?.distinctValues,
    configOverridden
  });
  const exportRows = $derived<ExportRow[]>(
    rows.map((c) => ({ value: c.value, articles: c.articles, mean: c.mean, std: c.std }))
  );
  const exportPayload = $derived<ExportPayload>({
    meta: {
      viewMode: 'cross_tab',
      field,
      metric: metric ?? '',
      scope,
      scopeId,
      windowStart,
      windowEnd,
      topN: activeTopN
    },
    summary: data
      ? { distinctValues: data.distinctValues, shown: data.categories.length }
      : undefined,
    howToRead: composeHowToRead('cross_tab', howToReadFacts),
    rows: exportRows,
    columns: ['value', 'articles', 'mean', 'std']
  });
  const exportFilenameParts = $derived([
    'cross-tab',
    field,
    metric ?? 'metric',
    scope === 'source' ? scopeId : 'probe'
  ]);
  let cellEl: HTMLElement | undefined = $state();
  function getNode(): HTMLElement | null {
    return cellEl ?? null;
  }
</script>

<section class="ct-cell" aria-labelledby="ct-title-{field}" bind:this={cellEl}>
  <header class="cell-header">
    <h3 id="ct-title-{field}" class="cell-title">
      <code>{field}</code> × <code>{metric ?? '—'}</code>
      <span class="muted">— cross-tab · <strong class="scope-name">{scopeId}</strong></span>
    </h3>
    {#if data && rows.length > 0}
      <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
    {/if}
  </header>

  {#if dataLayer === 'silver'}
    <p class="muted">Cross-tab operates on Gold-layer per-article metrics.</p>
  {:else if !field}
    <p class="muted">Pick a metadata field in the <strong>Group by</strong> lever.</p>
  {:else if !metric}
    <p class="muted">Pick a numeric metric in the <strong>Metric</strong> lever.</p>
  {:else if ctQ.isPending}
    <p class="muted" aria-busy="true">Loading cross-tab…</p>
  {:else if refusalData}
    <RefusalSurface refusal={refusalData} {ctx} />
  {:else if isNetworkError}
    <p class="muted">Could not load the cross-tab.</p>
  {:else if isEmpty}
    <CellEmptyState label={field} />
  {:else if data}
    <div
      class="plot-host"
      bind:this={host}
      role="img"
      aria-label="Mean {metric} per {field} category"
    ></div>
    <p class="ct-note">
      <strong>{data.distinctValues}</strong> distinct {data.distinctValues === 1
        ? 'value'
        : 'values'}{#if data.distinctValues > data.categories.length}
        · showing top {data.categories.length}{/if}
    </p>
    <HowToRead presentation="cross_tab" facts={howToReadFacts} />
  {/if}
</section>

<style>
  .ct-cell {
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
    min-height: 160px;
  }
  .plot-host :global(text) {
    fill: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: 11px;
  }
  .plot-host :global(svg) {
    background: transparent;
  }
  .ct-note {
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
