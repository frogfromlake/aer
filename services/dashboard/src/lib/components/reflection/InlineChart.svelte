<script lang="ts">
  // Distill-style inline interactive chart for Working Paper prose.
  //
  // Embeds a live Observable Plot cell that fetches real Probe 0 data from
  // the BFF. The reader can adjust a parameter (e.g. time window) and see
  // the effect on real data, making the paper's argument testable rather
  // than illustrated abstractly (Design Brief §4.3.3).
  //
  // The `cellId` prop selects the specific demo cell to render:
  //   'sentiment-window-demo' — WP-002 §3: sentiment distribution for
  //     variable time windows on Probe 0, showing how window size affects
  //     the apparent distribution shape.
  import { createQuery } from '@tanstack/svelte-query';
  import * as Plot from '@observablehq/plot';
  import { onMount } from 'svelte';
  import type { MetricsResponseDto, FetchContext, QueryOutcome } from '$lib/api/queries';
  import { metricsQuery } from '$lib/api/queries';

  interface Props {
    cellId: string;
    ctx?: FetchContext;
  }

  let { cellId, ctx = { baseUrl: '/api/v1' } }: Props = $props();

  // --- Sentiment window demo parameters ---
  const PROBE_0 = 'probe-0';
  const WINDOW_OPTIONS = [
    { label: '7 days', days: 7 },
    { label: '30 days', days: 30 },
    { label: '90 days', days: 90 }
  ] as const;
  type WindowOption = (typeof WINDOW_OPTIONS)[number];

  let selectedWindow = $state<WindowOption>(WINDOW_OPTIONS[1]!);

  const endDate = $derived(new Date().toISOString().split('T')[0]!);
  const startDate = $derived(() => {
    // eslint-disable-next-line svelte/prefer-svelte-reactivity
    const d = new Date();
    d.setDate(d.getDate() - selectedWindow.days);
    return d.toISOString().split('T')[0]!;
  });

  const metricsQ = createQuery<
    QueryOutcome<MetricsResponseDto>,
    Error,
    QueryOutcome<MetricsResponseDto>
  >(() => {
    const start = startDate();
    const end = endDate;
    const o = metricsQuery(ctx, {
      startDate: start,
      endDate: end,
      metricName: 'sentiment_score',
      source: PROBE_0
    });
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  let chartEl = $state<HTMLDivElement | undefined>(undefined);

  // Re-render the plot whenever query data or the container element changes
  $effect(() => {
    if (!chartEl) return;
    const outcome = metricsQ.data;
    if (outcome?.kind !== 'success') return;

    const points = outcome.data.data
      .filter((d) => d.metricName === 'sentiment_score')
      .map((d) => ({ time: new Date(d.timestamp), value: d.value }));

    if (points.length === 0) {
      // eslint-disable-next-line svelte/no-dom-manipulating
      chartEl.innerHTML = '<p class="no-data">No sentiment data for the selected window.</p>';
      return;
    }

    const plot = Plot.plot({
      width: chartEl.clientWidth || 540,
      height: 180,
      marginLeft: 48,
      marginBottom: 32,
      style: {
        background: 'transparent',
        color: 'var(--color-fg)',
        fontFamily: 'var(--font-ui)',
        fontSize: '11px'
      },
      x: { type: 'time', label: null, tickFormat: '%b %d' },
      y: {
        label: 'Sentiment score',
        domain: [-1, 1],
        grid: true,
        ticks: 5
      },
      marks: [
        Plot.ruleY([0], { stroke: 'var(--color-border)', strokeDasharray: '3,3' }),
        Plot.lineY(points, {
          x: 'time',
          y: 'value',
          stroke: 'var(--color-accent)',
          strokeWidth: 1.5,
          curve: 'monotone-x'
        }),
        Plot.dot(points, {
          x: 'time',
          y: 'value',
          r: 2,
          fill: 'var(--color-accent)',
          opacity: 0.6
        })
      ]
    });

    // eslint-disable-next-line svelte/no-dom-manipulating
    chartEl.innerHTML = '';
    // eslint-disable-next-line svelte/no-dom-manipulating
    chartEl.appendChild(plot);
  });

  onMount(() => {
    // Trigger resize observation so the plot fills its container
    const obs = new ResizeObserver(() => {
      // The $effect above will re-run because chartEl.clientWidth changed
      // but $effect is not reactive to DOM size. We force a re-trigger by
      // briefly touching the element reference.
      if (chartEl) chartEl.style.opacity = '1';
    });
    if (chartEl) obs.observe(chartEl);
    return () => obs.disconnect();
  });
</script>

<div class="cell" aria-label="Interactive chart: {cellId}">
  {#if cellId === 'sentiment-window-demo'}
    <header class="cell-header">
      <span class="cell-label">Live demonstration — Probe 0 sentiment</span>
      <div class="controls" role="group" aria-label="Time window">
        {#each WINDOW_OPTIONS as opt (opt.days)}
          <button
            type="button"
            class="window-btn"
            class:active={selectedWindow === opt}
            aria-pressed={selectedWindow === opt}
            onclick={() => (selectedWindow = opt)}
          >
            {opt.label}
          </button>
        {/each}
      </div>
    </header>

    <div class="chart-area" bind:this={chartEl} aria-busy={metricsQ.isPending}>
      {#if metricsQ.isPending}
        <p class="state-msg" aria-live="polite">Loading data…</p>
      {:else if metricsQ.isError || metricsQ.data?.kind === 'network-error'}
        <p class="state-msg error">
          Could not reach the BFF. Connect to the AĒR backend to see live data.
        </p>
      {:else if metricsQ.data?.kind === 'refusal'}
        <p class="state-msg">
          Data access refused: {metricsQ.data.message}
        </p>
      {/if}
    </div>

    <footer class="cell-footer">
      <span class="cell-caption">
        SentiWS v2.0 lexicon · Tier 1, unvalidated · 5-minute aggregation buckets · Probe 0 sources
        only
      </span>
      <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
      <a href="/reflection/wp/wp-002?section=3" class="cell-link">Read §3 in full →</a>
    </footer>
  {:else}
    <div class="placeholder">
      <p>Interactive cell <code>{cellId}</code> — coming in a future iteration.</p>
    </div>
  {/if}
</div>

<style>
  .cell {
    margin: var(--space-6) 0;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    background: var(--color-bg-elevated);
    overflow: hidden;
  }

  .cell-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--color-border);
  }

  .cell-label {
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-muted);
    font-weight: var(--font-weight-medium);
  }

  .controls {
    display: flex;
    gap: var(--space-1);
  }

  .window-btn {
    padding: 2px var(--space-2);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-pill);
    background: transparent;
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    cursor: pointer;
    transition: all var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .window-btn:hover,
  .window-btn:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .window-btn.active {
    color: var(--color-fg);
    background: var(--color-surface);
    border-color: var(--color-accent-muted);
  }

  .chart-area {
    padding: var(--space-3) var(--space-4);
    min-height: 8rem;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .chart-area :global(svg) {
    max-width: 100%;
  }

  .state-msg {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
    text-align: center;
  }

  .state-msg.error {
    color: var(--color-status-expired);
  }

  :global(.no-data) {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    text-align: center;
  }

  .cell-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-4);
    border-top: 1px solid var(--color-border);
    background: var(--color-surface);
  }

  .cell-caption {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    font-family: var(--font-mono);
  }

  .cell-link {
    font-size: var(--font-size-xs);
    color: var(--color-accent);
    text-decoration: none;
    border-bottom: 1px solid currentColor;
    padding-bottom: 1px;
  }

  .cell-link:hover,
  .cell-link:focus-visible {
    color: var(--color-fg);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .placeholder {
    padding: var(--space-5);
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    border: 1px dashed var(--color-border);
    border-radius: var(--radius-md);
    margin: var(--space-4);
  }

  @media (prefers-reduced-motion: reduce) {
    .window-btn {
      transition: none;
    }
  }
</style>
