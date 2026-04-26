<script lang="ts">
  // Lens Bar — Surface II L3 (Phase 113c).
  //
  // Pairs the two core levers of a function-lane cell: the *metric*
  // (what the cell is measuring) and the *view mode* (how the cell
  // renders that measurement). Together they define the active cell of
  // the View-Mode Matrix (Brief §4.2.5 / Arc42 §8.13). Both selectors
  // are URL-backed so deep-links to a specific (metric, view) cell
  // restore exactly.
  //
  // Pre-Phase-113c these levers lived as compact controls in the
  // ScopeBar, where they read as secondary chrome. They are not
  // chrome — they are the primary interaction on L3. The bar lifts
  // them onto the lane surface itself, both visually emphasized.
  import { createQuery } from '@tanstack/svelte-query';
  import {
    metricsAvailableQuery,
    type AvailableMetricDto,
    type FetchContext,
    type QueryOutcome
  } from '$lib/api/queries';
  import { setUrl, urlState } from '$lib/state/url.svelte';
  import { DEFAULT_LOOKBACK_MS } from '$lib/state/url-internals';
  import { DEFAULT_METRIC_NAME, listPresentations, getPresentation } from '$lib/viewmodes';
  import type { ViewMode } from '$lib/state/url-internals';

  const ctx: FetchContext = { baseUrl: '/api/v1' };
  const url = $derived(urlState());
  let activeMetric = $derived<string>(url.metric ?? DEFAULT_METRIC_NAME);
  let activeView = $derived<ViewMode>(getPresentation(url.viewMode).id);
  const presentations = listPresentations();

  let dateWindow = $derived.by(() => {
    const now = Date.now();
    const fromMs = url.from ? Date.parse(url.from) : now - DEFAULT_LOOKBACK_MS;
    const toMs = url.to ? Date.parse(url.to) : now;
    const startD = new Date(Number.isFinite(fromMs) ? fromMs : now - DEFAULT_LOOKBACK_MS);
    const endD = new Date(Number.isFinite(toMs) ? toMs : now);
    return {
      startDate: startD.toISOString().slice(0, 10),
      endDate: endD.toISOString().slice(0, 10)
    };
  });

  const availQ = createQuery<
    QueryOutcome<AvailableMetricDto[]>,
    Error,
    QueryOutcome<AvailableMetricDto[]>
  >(() => {
    const o = metricsAvailableQuery(ctx, dateWindow);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  let metrics = $derived.by<string[]>(() => {
    const fromApi =
      availQ.data?.kind === 'success' ? availQ.data.data.map((m) => m.metricName) : [];
    const seen: Record<string, true> = {};
    const merged: string[] = [];
    for (const name of [DEFAULT_METRIC_NAME, activeMetric, ...fromApi]) {
      if (!name || seen[name]) continue;
      seen[name] = true;
      merged.push(name);
    }
    return merged;
  });

  function pickMetric(name: string) {
    if (name === activeMetric) return;
    setUrl({ metric: name });
  }
  function pickView(id: ViewMode) {
    if (id === activeView) return;
    setUrl({ viewMode: id });
  }
</script>

<div class="lens-bar" role="region" aria-label="Lane lens controls">
  <div class="lens-group" role="radiogroup" aria-label="Metric">
    <span class="lens-eyebrow" aria-hidden="true">Metric</span>
    <div class="lens-options">
      {#each metrics as m (m)}
        <button
          type="button"
          role="radio"
          aria-checked={activeMetric === m}
          class="lens-btn metric"
          class:active={activeMetric === m}
          title="Switch to {m}"
          onclick={() => pickMetric(m)}
        >
          <code>{m}</code>
        </button>
      {/each}
    </div>
  </div>

  <div class="lens-divider" aria-hidden="true"></div>

  <div class="lens-group" role="radiogroup" aria-label="View mode">
    <span class="lens-eyebrow" aria-hidden="true">View</span>
    <div class="lens-options">
      {#each presentations as p (p.id)}
        <button
          type="button"
          role="radio"
          aria-checked={activeView === p.id}
          class="lens-btn view"
          class:active={activeView === p.id}
          title={p.description}
          onclick={() => pickView(p.id)}
        >
          {p.label}
        </button>
      {/each}
    </div>
  </div>
</div>

<style>
  .lens-bar {
    display: flex;
    align-items: stretch;
    gap: var(--space-3);
    flex-wrap: wrap;
    padding: var(--space-3) var(--space-4);
    background: linear-gradient(180deg, rgba(82, 131, 184, 0.08), rgba(82, 131, 184, 0.02));
    border: 1px solid var(--color-accent-muted);
    border-radius: var(--radius-md);
    margin: var(--space-3) var(--space-6) var(--space-4);
  }

  .lens-group {
    display: flex;
    flex-direction: column;
    gap: 4px;
    flex: 1 1 auto;
    min-width: 0;
  }

  .lens-eyebrow {
    font-size: 10px;
    font-family: var(--font-mono);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-accent);
    font-weight: var(--font-weight-semibold);
  }

  .lens-options {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-1);
  }

  .lens-btn {
    appearance: none;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    color: var(--color-fg-muted);
    padding: 4px var(--space-3);
    border-radius: var(--radius-sm);
    font-size: var(--font-size-xs);
    font-family: var(--font-ui);
    cursor: pointer;
    transition:
      background-color var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard),
      border-color var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .lens-btn.metric code {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: inherit;
  }

  .lens-btn:hover,
  .lens-btn:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .lens-btn.active {
    color: var(--color-fg);
    background: rgba(82, 131, 184, 0.25);
    border-color: var(--color-accent);
  }

  .lens-divider {
    width: 1px;
    background: var(--color-border);
    align-self: stretch;
    flex-shrink: 0;
  }

  @media (prefers-reduced-motion: reduce) {
    .lens-btn {
      transition: none;
    }
  }
</style>
