<script lang="ts">
  import { sanitizePlotA11y } from '$lib/presentations/plot-a11y';
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
  import CellTitleBar from './CellTitleBar.svelte';
  import { m } from '$lib/paraglide/messages.js';
  import { metricLabel, fieldLabel, metricSubjectAndModel } from '$lib/state/labels.svelte';
  import { useProbeLabels } from '$lib/presentations/use-probe-labels.svelte';
  import type { CellTitleSpec } from '$lib/presentations/cell-title';

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

  // Phase 148e — unified cell title. Pair subject (field × metric); the metric
  // side is stripped to its subject noun (model lives in the axis / how-to-read,
  // not the title). Scope resolved (no raw probe id leak).
  const probeLabels = useProbeLabels(() => ctx);
  const titleSpec = $derived<CellTitleSpec>({
    presentation: m.domain_presentation_cross_tab_label(),
    subject: {
      kind: 'pair',
      left: fieldLabel(field),
      op: '×',
      right: metric ? metricSubjectAndModel(metric).subject : '—'
    },
    scope: { kind: 'single', label: probeLabels.labelFor(scopeId) },
    idSeed: `ct-title-${field}`
  });

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
        x: {
          label: m.cells_ct_axis_mean({ metric: metric ? metricLabel(metric) : '' }),
          grid: true
        },
        y: { domain: r.map((d) => d.value), label: null },
        color: {
          scheme: 'rdbu',
          legend: true,
          label: m.cells_ct_axis_mean({ metric: metric ? metricLabel(metric) : '' })
        },
        marks: [
          Plot.barX(r, {
            x: 'mean',
            y: 'value',
            fill: 'mean',
            channels: {
              articles: { value: 'articles', label: m.cells_ct_channel_articles() },
              std: { value: 'std', label: '±σ' }
            },
            tip: true
          }),
          Plot.ruleX([0])
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
  <CellTitleBar spec={titleSpec}>
    {#snippet actions()}
      {#if data && rows.length > 0}
        <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
      {/if}
    {/snippet}
  </CellTitleBar>

  {#if !field}
    <p class="muted">{m.cells_ct_need_field()}</p>
  {:else if !metric}
    <p class="muted">{m.cells_ct_need_metric()}</p>
  {:else if ctQ.isPending}
    <p class="muted" aria-busy="true">{m.cells_ct_loading()}</p>
  {:else if refusalData}
    <RefusalSurface refusal={refusalData} {ctx} />
  {:else if isNetworkError}
    <p class="muted">{m.cells_ct_error()}</p>
  {:else if isEmpty}
    <CellEmptyState label={fieldLabel(field)} />
  {:else if data}
    <div
      class="plot-host"
      bind:this={host}
      role="img"
      aria-label={m.cells_ct_plot_aria({
        metric: metric ? metricLabel(metric) : '',
        field: fieldLabel(field)
      })}
    ></div>
    <p class="ct-note">
      <strong>{data.distinctValues}</strong>
      {data.distinctValues === 1
        ? m.cells_ct_note_distinct_one()
        : m.cells_ct_note_distinct_other()}{#if data.distinctValues > data.categories.length}
        · {m.cells_ct_note_showing_top({ count: data.categories.length })}{/if}
    </p>
  {/if}
</section>

<style>
  .ct-cell {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
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
</style>
