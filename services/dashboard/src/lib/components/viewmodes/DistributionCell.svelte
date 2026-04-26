<script lang="ts">
  // EDA × distribution cell (Phase 107).
  // Per-scope histogram + quantile summary backed by
  // `GET /api/v1/metrics/{name}/distribution`. Renders a histogram with
  // Observable Plot, lazy-imported so Plot lands in its own chunk only
  // when the user picks this view (Brief §7 bundle budget).
  import { createQuery } from '@tanstack/svelte-query';
  import { onDestroy } from 'svelte';
  import {
    metricDistributionQuery,
    type DistributionResponseDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import type { ViewModeCellProps } from '$lib/viewmodes';

  let { ctx, scope, scopeId, windowStart, windowEnd, metricName }: ViewModeCellProps = $props();

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
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  let host: HTMLDivElement | undefined = $state();
  let plotEl: HTMLElement | null = null;
  let renderToken = 0;

  $effect(() => {
    const data = distQ.data?.kind === 'success' ? distQ.data.data : null;
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
      <code>{metricName}</code>
      <span class="muted">— distribution ({scope})</span>
    </h3>
  </header>

  {#if distQ.isPending}
    <p class="muted" aria-busy="true">Loading distribution…</p>
  {:else if distQ.data?.kind === 'refusal'}
    <RefusalSurface refusal={distQ.data} {ctx} />
  {:else if distQ.isError || distQ.data?.kind === 'network-error'}
    <p class="muted">Could not load distribution.</p>
  {:else if distQ.data?.kind === 'success' && distQ.data.data.summary.count === 0}
    <p class="muted">No samples for {metricName} in this window.</p>
  {:else if distQ.data?.kind === 'success'}
    {@const s = distQ.data.data.summary}
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
