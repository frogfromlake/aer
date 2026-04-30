<script lang="ts">
  // Lens Bar — Surface II L3 primary controls (Phase 113c rework).
  //
  // Hosts the four levers that define what the lane is showing:
  //
  //   - Function   — EA / PL / CI / SF (the WP-001 discourse function);
  //                  drives a path change to a sibling lane route.
  //   - Layer      — Au Gold / Ag Silver; URL-backed `layer` param.
  //   - Metric     — what the cell is measuring (URL-backed `metric`).
  //   - View       — how the cell renders that measurement (URL-backed
  //                  `viewMode`); together with metric, defines the
  //                  active cell of the View-Mode Matrix.
  //
  // Pre-Phase-113c the metric, view-mode, and Silver-layer levers were
  // split between the ScopeBar and an earlier two-group LensBar. The
  // reframing pulls them all onto the lane surface itself with equal
  // visual weight, and adds the Function group so pillar switching is
  // a primary-interaction lever rather than chrome.
  import { createQuery } from '@tanstack/svelte-query';
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import {
    metricsAvailableQuery,
    contentQuery,
    type AvailableMetricDto,
    type ContentResponseDto,
    type FetchContext,
    type QueryOutcome
  } from '$lib/api/queries';
  import { setUrl, urlState } from '$lib/state/url.svelte';
  import { DEFAULT_LOOKBACK_MS } from '$lib/state/url-internals';
  import {
    DEFAULT_METRIC_NAME,
    cellContentId,
    listPresentations,
    getPresentation
  } from '$lib/viewmodes';
  import type { ViewMode } from '$lib/state/url-internals';

  const LAYER_DESCRIPTIONS: Record<'gold' | 'silver', string> = {
    gold: 'Au Gold — aggregated metrics derived from the full document corpus.',
    silver: 'Ag Silver — document-level data, reviewed per WP-006 §5.2.'
  };

  const FUNCTION_DESCRIPTIONS: Record<string, string> = {
    epistemic_authority: 'Sources that produce and legitimate knowledge claims (WP-001 §3).',
    power_legitimation: 'Sources that frame, justify, or contest political power (WP-001 §3).',
    cohesion_identity:
      'Sources that articulate collective identity and social cohesion (WP-001 §3).',
    subversion_friction: 'Sources that challenge dominant frames or introduce friction (WP-001 §3).'
  };

  interface Props {
    // Discourse functions represented in the current source selection.
    // The current lane's function is always included. Additional keys indicate
    // cross-function sources in a manual multi-source selection.
    activeFunctionKeys?: string[];
  }

  let { activeFunctionKeys = [] }: Props = $props();

  const ctx: FetchContext = { baseUrl: '/api/v1' };
  const url = $derived(urlState());
  let activeMetric = $derived<string>(url.metric ?? DEFAULT_METRIC_NAME);
  let activeView = $derived<ViewMode>(getPresentation(url.viewMode).id);
  let activeLayer = $derived<'gold' | 'silver'>(url.layer === 'silver' ? 'silver' : 'gold');
  const presentations = listPresentations();

  const FUNCTIONS = [
    { key: 'epistemic_authority', abbr: 'EA', label: 'Epistemic Authority' },
    { key: 'power_legitimation', abbr: 'PL', label: 'Power Legitimation' },
    { key: 'cohesion_identity', abbr: 'CI', label: 'Cohesion & Identity' },
    { key: 'subversion_friction', abbr: 'SF', label: 'Subversion & Friction' }
  ] as const;

  let probeId = $derived(page.params.probeId ?? '');
  let activeFunctionKey = $derived(page.params.functionKey ?? '');

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
    // DEFAULT_METRIC_NAME first, then API order (no forced activeMetric
    // insertion — that was causing the selected metric to jump to position 2).
    for (const name of [DEFAULT_METRIC_NAME, ...fromApi]) {
      if (!name || seen[name]) continue;
      seen[name] = true;
      merged.push(name);
    }
    // If activeMetric is not yet in the list (API not loaded), append it.
    if (activeMetric && !seen[activeMetric]) {
      merged.push(activeMetric);
    }
    return merged;
  });

  function pickFunction(key: string) {
    if (key === activeFunctionKey) return;
    // Path-level navigation; query params (metric, viewMode, layer,
    // sourceId) are preserved by the rune store across the goto.
    // eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Surface II route
    void goto(`/lanes/${probeId}/${key}${page.url.search}`);
  }
  function pickMetric(name: string) {
    if (name === activeMetric) return;
    setUrl({ metric: name });
  }
  function pickView(id: ViewMode) {
    if (id === activeView) return;
    setUrl({ viewMode: id });
  }
  function pickLayer(next: 'gold' | 'silver') {
    if (next === activeLayer) return;
    setUrl({ layer: next === 'silver' ? 'silver' : null });
  }

  // Cell content query — same key as FunctionLaneShell's viewModeContentQ so
  // TanStack serves it from cache with no extra network request.
  let activeCellContentId = $derived(cellContentId(activeView, activeMetric));
  const cellContentQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'view_mode', activeCellContentId);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });
  let metricDesc = $derived(
    cellContentQ.data?.kind === 'success' ? cellContentQ.data.data.registers.semantic.short : null
  );

  let activePresentationDesc = $derived(
    presentations.find((p) => p.id === activeView)?.description ?? null
  );

  let composedProbeIds = $derived(url.probeIds);
</script>

<div class="lens-bar" role="region" aria-label="Lane lens controls">
  <!-- Row 1: stable selectors (fixed cardinality, won't grow) -->
  <div class="lens-row">
    <div class="lens-group" role="radiogroup" aria-label="Discourse function">
      <span class="lens-eyebrow" aria-hidden="true">Function</span>
      <div class="lens-options">
        {#each FUNCTIONS as fn (fn.key)}
          {@const isCurrent = activeFunctionKey === fn.key}
          {@const hasSelection = !isCurrent && activeFunctionKeys.includes(fn.key)}
          <button
            type="button"
            role="radio"
            aria-checked={isCurrent}
            class="lens-btn fn fn-{fn.key}"
            class:active={isCurrent}
            class:has-selection={hasSelection}
            title={hasSelection ? `${fn.label} — selected sources include this function` : fn.label}
            onclick={() => pickFunction(fn.key)}
          >
            <span class="fn-abbr">{fn.abbr}</span>
            <span class="fn-name">{fn.label}</span>
          </button>
        {/each}
      </div>
      {#if activeFunctionKey && FUNCTION_DESCRIPTIONS[activeFunctionKey]}
        <p class="lens-desc">{FUNCTION_DESCRIPTIONS[activeFunctionKey]}</p>
      {/if}
    </div>

    <div class="lens-divider" aria-hidden="true"></div>

    <div class="lens-group" role="radiogroup" aria-label="Data layer">
      <span class="lens-eyebrow" aria-hidden="true">Layer</span>
      <div class="lens-options">
        <button
          type="button"
          role="radio"
          aria-checked={activeLayer === 'gold'}
          class="lens-btn layer"
          class:active={activeLayer === 'gold'}
          title="Au Gold — aggregated metrics"
          onclick={() => pickLayer('gold')}
        >
          Au Gold
        </button>
        <button
          type="button"
          role="radio"
          aria-checked={activeLayer === 'silver'}
          class="lens-btn layer silver"
          class:active={activeLayer === 'silver'}
          title="Ag Silver — document-level data (WP-006 §5.2)"
          onclick={() => pickLayer('silver')}
        >
          Ag Silver
        </button>
      </div>
      <p class="lens-desc">{LAYER_DESCRIPTIONS[activeLayer]}</p>
    </div>
  </div>

  <div class="lens-hsep" aria-hidden="true"></div>

  <!-- Row 2: dynamic selectors (Metric and View will grow with new entries) -->
  <div class="lens-row">
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
      {#if metricDesc}
        <p class="lens-desc">{metricDesc}</p>
      {/if}
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
      {#if activePresentationDesc}
        <p class="lens-desc">{activePresentationDesc}</p>
      {/if}
    </div>
  </div>

  {#if composedProbeIds.length > 0}
    <div class="lens-hsep" aria-hidden="true"></div>
    <div class="lens-compose-row" aria-label="Probe composition scope">
      <span class="lens-eyebrow" aria-hidden="true">Composition</span>
      <div class="lens-options">
        <span
          class="compose-indicator"
          title="Multi-probe composition: {composedProbeIds.join(', ')}"
        >
          ⊗ {composedProbeIds.length} probe{composedProbeIds.length !== 1 ? 's' : ''} in scope
        </span>
        {#each composedProbeIds as id (id)}
          <span class="compose-probe-chip">{id}</span>
        {/each}
      </div>
    </div>
  {/if}
</div>

<style>
  .lens-bar {
    display: flex;
    flex-direction: column;
    gap: 0;
    padding: var(--space-2) var(--space-4);
    background: linear-gradient(180deg, rgba(82, 131, 184, 0.08), rgba(82, 131, 184, 0.02));
    border: 1px solid var(--color-accent-muted);
    border-radius: var(--radius-md);
    margin: var(--space-3) var(--space-6) var(--space-4);
  }

  /* Each row is an independent flex line. Row 1: Function + Layer (stable).
     Row 2: Metric + View (grows as new options are added). */
  .lens-row {
    display: flex;
    align-items: stretch;
    gap: var(--space-3);
    padding: var(--space-2) 0;
  }

  .lens-hsep {
    height: 1px;
    background: var(--color-border);
    flex-shrink: 0;
  }

  .lens-group {
    display: flex;
    flex-direction: column;
    gap: 4px;
    flex: 1 1 0%;
    min-width: 160px;
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

  .lens-btn.fn {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
  }

  .lens-btn .fn-abbr {
    font-family: var(--font-mono);
    font-weight: var(--font-weight-semibold);
    letter-spacing: 0.04em;
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

  /* Secondary indicator: selected sources span this function but it's not
     the current lane. Dotted accent border signals "has data here". */
  .lens-btn.fn.has-selection {
    border-color: var(--color-accent-muted);
    border-style: dashed;
    color: var(--color-fg-muted);
  }

  .lens-btn.fn.has-selection:hover,
  .lens-btn.fn.has-selection:focus-visible {
    border-style: solid;
    border-color: var(--color-accent);
    color: var(--color-fg);
  }

  .lens-btn.layer.silver.active {
    color: #7ec4a0;
    background: rgba(126, 196, 160, 0.18);
    border-color: #7ec4a0;
  }

  .lens-desc {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    margin: 2px 0 0;
    line-height: 1.45;
  }

  .lens-divider {
    width: 1px;
    background: var(--color-border);
    align-self: stretch;
    flex-shrink: 0;
  }

  .lens-compose-row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-2) 0;
    flex-wrap: wrap;
  }

  .compose-indicator {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-accent);
    padding: 2px var(--space-2);
    background: rgba(82, 131, 184, 0.1);
    border: 1px solid var(--color-accent-muted);
    border-radius: var(--radius-pill);
    white-space: nowrap;
  }

  .compose-probe-chip {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    padding: 1px var(--space-2);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border-strong);
    border-radius: var(--radius-pill);
    color: var(--color-fg-muted);
  }

  /* Drop the long function name on narrow viewports — the abbr stays. */
  @media (max-width: 1100px) {
    .lens-btn .fn-name {
      display: none;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .lens-btn {
      transition: none;
    }
  }
</style>
