<script lang="ts">
  // EDA × distribution cell (Phase 107).
  // Per-scope histogram + quantile summary backed by
  // `GET /api/v1/metrics/{name}/distribution` (Gold) or
  // `GET /api/v1/silver/aggregations/{type}` (Silver, Phase 111).
  // Renders a histogram with Observable Plot, lazy-imported so Plot lands
  // in its own chunk only when the user picks this view (Brief §7 bundle
  // budget).
  import { createQuery } from '@tanstack/svelte-query';
  import { onDestroy } from 'svelte';
  import {
    metricDistributionQuery,
    silverAggregationQuery,
    type DistributionResponseDto,
    type SilverAggregationResponseDto,
    type SilverAggregationType,
    type QueryOutcome
  } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import type { ViewModeCellProps } from '$lib/viewmodes';

  let {
    ctx,
    scope,
    scopeId,
    windowStart,
    windowEnd,
    metricName,
    dataLayer = 'gold'
  }: ViewModeCellProps = $props();

  // Map Gold metric names to the closest Silver aggregation type.
  // Falls back to `word_count` — the universally available Silver field.
  const GOLD_TO_SILVER: Record<string, SilverAggregationType> = {
    word_count: 'word_count',
    entity_count: 'raw_entity_count'
  };
  let silverAggType = $derived<SilverAggregationType>(
    (GOLD_TO_SILVER[metricName] as SilverAggregationType | undefined) ?? 'word_count'
  );

  const distQ = createQuery<
    QueryOutcome<DistributionResponseDto>,
    Error,
    QueryOutcome<DistributionResponseDto>
  >(() => {
    const o = metricDistributionQuery(ctx, metricName, {
      scope,
      scopeId,
      start: windowStart,
      end: windowEnd
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: dataLayer !== 'silver'
    };
  });

  const silverDistQ = createQuery<
    QueryOutcome<SilverAggregationResponseDto>,
    Error,
    QueryOutcome<SilverAggregationResponseDto>
  >(() => {
    const useSilver = dataLayer === 'silver' && scope === 'source';
    const o = silverAggregationQuery(ctx, silverAggType, {
      sourceId: scopeId,
      start: windowStart,
      end: windowEnd
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: useSilver
    };
  });

  // Normalised distribution data regardless of layer.
  let activeDist = $derived.by(() => {
    if (dataLayer === 'silver') {
      if (silverDistQ.data?.kind !== 'success') return null;
      return silverDistQ.data.data.distribution ?? null;
    }
    if (distQ.data?.kind !== 'success') return null;
    return { bins: distQ.data.data.bins, summary: distQ.data.data.summary };
  });

  let isPending = $derived(dataLayer === 'silver' ? silverDistQ.isPending : distQ.isPending);
  let refusalData = $derived(
    dataLayer === 'silver'
      ? silverDistQ.data?.kind === 'refusal'
        ? silverDistQ.data
        : null
      : distQ.data?.kind === 'refusal'
        ? distQ.data
        : null
  );
  let isNetworkError = $derived(
    dataLayer === 'silver'
      ? silverDistQ.isError || silverDistQ.data?.kind === 'network-error'
      : distQ.isError || distQ.data?.kind === 'network-error'
  );

  let host: HTMLDivElement | undefined = $state();
  let plotEl: HTMLElement | null = null;
  let renderToken = 0;

  $effect(() => {
    const data = activeDist;
    if (!host || !data) return;
    const token = ++renderToken;
    (async () => {
      const Plot = await import('@observablehq/plot');
      if (!host || token !== renderToken) return;
      const rows = data.bins.map((b) => ({
        center: (b.lower + b.upper) / 2,
        width: b.upper - b.lower,
        count: b.count
      }));
      const next = Plot.plot({
        width: host.clientWidth,
        height: 220,
        marginLeft: 56,
        marginBottom: 36,
        x: { label: metricName, grid: false },
        y: { label: 'count', grid: true },
        marks: [
          Plot.rectY(rows, {
            x1: (d: { center: number; width: number }) => d.center - d.width / 2,
            x2: (d: { center: number; width: number }) => d.center + d.width / 2,
            y: 'count',
            fill: 'rgba(82, 131, 184, 0.55)',
            stroke: 'rgba(82, 131, 184, 0.95)'
          }),
          Plot.ruleX([data.summary.median], { stroke: '#e0a050', strokeWidth: 1.5 }),
          Plot.ruleX([data.summary.p25, data.summary.p75], {
            stroke: '#e0a050',
            strokeDasharray: '3,2',
            strokeOpacity: 0.7
          }),
          Plot.ruleY([0])
        ]
      });
      // Observable Plot returns a detached SVG/figure node that we own; the
      // host div is a stable empty container, so a direct mount is safe.
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

  function fmt(n: number): string {
    if (!Number.isFinite(n)) return '—';
    return Math.abs(n) >= 100 ? n.toFixed(0) : n.toFixed(3);
  }
</script>

<section class="dist-cell" aria-labelledby="dist-title-{metricName}">
  <header class="cell-header">
    <h3 id="dist-title-{metricName}" class="cell-title">
      <code>{dataLayer === 'silver' ? silverAggType : metricName}</code>
      <span class="muted">— distribution ({scope})</span>
      {#if dataLayer === 'silver'}
        <span class="layer-badge silver" aria-label="Silver layer data">Ag</span>
      {/if}
    </h3>
  </header>

  {#if dataLayer === 'silver' && scope !== 'source'}
    <p class="muted">Narrow to a single source to view Silver-layer distribution.</p>
  {:else if isPending}
    <p class="muted" aria-busy="true">Loading distribution…</p>
  {:else if refusalData}
    <RefusalSurface refusal={refusalData} {ctx} />
  {:else if isNetworkError}
    <p class="muted">Could not load distribution.</p>
  {:else if activeDist && activeDist.summary.count === 0}
    <p class="muted">No samples for {metricName} in this window.</p>
  {:else if activeDist}
    {@const s = activeDist.summary}
    <div class="plot-host" bind:this={host} role="img" aria-label="Histogram of {metricName}"></div>
    <dl class="summary" aria-label="Quantile summary">
      <div>
        <dt>n</dt>
        <dd>{s.count}</dd>
      </div>
      <div>
        <dt>min</dt>
        <dd>{fmt(s.min)}</dd>
      </div>
      <div>
        <dt>p05</dt>
        <dd>{fmt(s.p05)}</dd>
      </div>
      <div>
        <dt>p25</dt>
        <dd>{fmt(s.p25)}</dd>
      </div>
      <div>
        <dt>median</dt>
        <dd>{fmt(s.median)}</dd>
      </div>
      <div>
        <dt>mean</dt>
        <dd>{fmt(s.mean)}</dd>
      </div>
      <div>
        <dt>p75</dt>
        <dd>{fmt(s.p75)}</dd>
      </div>
      <div>
        <dt>p95</dt>
        <dd>{fmt(s.p95)}</dd>
      </div>
      <div>
        <dt>max</dt>
        <dd>{fmt(s.max)}</dd>
      </div>
    </dl>
  {/if}
</section>

<style>
  .dist-cell {
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

  .layer-badge {
    font-size: 10px;
    padding: 1px var(--space-2);
    border-radius: var(--radius-pill);
    font-family: var(--font-mono);
    font-weight: var(--font-weight-semibold);
  }

  .layer-badge.silver {
    background: rgba(126, 196, 160, 0.15);
    border: 1px solid #7ec4a0;
    color: #7ec4a0;
  }

  .plot-host {
    width: 100%;
    min-height: 220px;
  }

  /* Plot's default text fills are dark; coerce to muted on the AĒR theme. */
  .plot-host :global(text) {
    fill: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: 11px;
  }

  .plot-host :global(svg) {
    background: transparent;
  }

  .summary {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(72px, 1fr));
    gap: var(--space-2) var(--space-4);
    margin: 0;
    padding: var(--space-3) 0 0;
    border-top: 1px solid var(--color-border);
  }

  .summary div {
    display: flex;
    flex-direction: column;
  }

  .summary dt {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
    font-family: var(--font-mono);
  }

  .summary dd {
    margin: 0;
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
</style>
