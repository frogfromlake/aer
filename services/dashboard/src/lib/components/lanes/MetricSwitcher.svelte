<script lang="ts">
  // Metric Switcher (Phase 113c).
  // Sits in the Surface II top scope bar next to the View-Mode switcher.
  // The active metric is URL-backed via `?metric=…`; switching is a single
  // replaceState write so the lane shell re-renders the cell in place.
  // The available list is sourced from `/metrics/available` over the same
  // window the lane page reads from the URL — falls back to the registered
  // default when the endpoint has not produced a list yet.
  import { createQuery } from '@tanstack/svelte-query';
  import {
    metricsAvailableQuery,
    type AvailableMetricDto,
    type FetchContext,
    type QueryOutcome
  } from '$lib/api/queries';
  import { setUrl, urlState } from '$lib/state/url.svelte';
  import { DEFAULT_LOOKBACK_MS } from '$lib/state/url-internals';
  import { DEFAULT_METRIC_NAME } from '$lib/viewmodes';

  const ctx: FetchContext = { baseUrl: '/api/v1' };
  const url = $derived(urlState());
  let activeMetric = $derived<string>(url.metric ?? DEFAULT_METRIC_NAME);

  // Use the same lookback window as the lane page so the available list
  // reflects what is actually plotted. Date-only granularity matches the
  // /metrics/available contract.
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

  // Always include the default + active metric so the picker is functional
  // before /metrics/available has resolved (or if it returns nothing).
  let metrics = $derived.by<{ metricName: string }[]>(() => {
    const fromApi =
      availQ.data?.kind === 'success' ? availQ.data.data.map((m) => m.metricName) : [];
    const seen: Record<string, true> = {};
    const merged: { metricName: string }[] = [];
    for (const name of [DEFAULT_METRIC_NAME, activeMetric, ...fromApi]) {
      if (!name || seen[name]) continue;
      seen[name] = true;
      merged.push({ metricName: name });
    }
    return merged;
  });

  function pick(name: string) {
    if (name === activeMetric) return;
    setUrl({ metric: name });
  }
</script>

<label class="switcher" aria-label="Metric">
  <span class="label" aria-hidden="true">Metric</span>
  <select value={activeMetric} onchange={(e) => pick((e.currentTarget as HTMLSelectElement).value)}>
    {#each metrics as m (m.metricName)}
      <option value={m.metricName}>{m.metricName}</option>
    {/each}
  </select>
</label>

<style>
  .switcher {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    flex-shrink: 0;
  }

  .label {
    font-size: 10px;
    font-family: var(--font-mono);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
  }

  /* Match the dark-mode select styling used by L2Controls so the dropdown
     does not regress to the OS default light chrome on Surface II. */
  select {
    appearance: none;
    -webkit-appearance: none;
    background-color: rgba(0, 0, 0, 0.55);
    background-image:
      linear-gradient(45deg, transparent 50%, var(--color-fg-muted) 50%),
      linear-gradient(135deg, var(--color-fg-muted) 50%, transparent 50%);
    background-position:
      calc(100% - 14px) 50%,
      calc(100% - 9px) 50%;
    background-size:
      5px 5px,
      5px 5px;
    background-repeat: no-repeat;
    color: var(--color-fg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 2px 22px 2px var(--space-2);
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    cursor: pointer;
  }

  select:hover,
  select:focus-visible {
    border-color: var(--color-accent);
    outline: none;
  }

  select option {
    background: var(--color-surface);
    color: var(--color-fg);
  }
</style>
