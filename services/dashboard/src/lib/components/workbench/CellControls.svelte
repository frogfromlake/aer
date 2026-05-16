<script lang="ts">
  // CellControls — Phase 122h Findings round 2 §F1.
  //
  // Per-cell control strip used by every Pillar Shell. Exposes the four
  // levers that define the active cell of the View-Mode Matrix:
  //
  //   - Metric         (`url.metric`)         — dynamic list from `/metrics/available`
  //   - Darstellung    (`url.viewMode`)       — pillar-filtered presentations
  //   - Layer          (`url.layer`)          — gold | silver
  //   - Vergleich      (`url.normalization`)  — raw | zscore | percentile
  //
  // Replaces the lever surface that lived in the retired `LensBar`. The
  // controls write through `setUrl` so deep-links restore byte-identically.
  import { createQuery } from '@tanstack/svelte-query';
  import {
    metricsAvailableQuery,
    type AvailableMetricDto,
    type FetchContext,
    type QueryOutcome
  } from '$lib/api/queries';
  import { setUrl, urlState } from '$lib/state/url.svelte';
  import { DEFAULT_METRIC_NAME, presentationsForPillar, resolvePresentation } from '$lib/viewmodes';
  import {
    DEFAULT_LOOKBACK_MS,
    type Normalization,
    type Resolution,
    type ViewMode,
    type ViewingMode
  } from '$lib/state/url-internals';
  import { updatePanel, type PanelPath } from '$lib/workbench/panel-mutators';

  interface Props {
    pillar: ViewingMode;
    /** When the Pillar dictates a fixed view (Rhizome entry-questions),
     *  the parent can lock the view selection — the View row still
     *  renders, but as an informational badge instead of a selector. */
    lockedView?: ViewMode;
    /** Phase 122i — when set, the controls bind to the addressed Panel
     *  in the new Pillar→Window→Panel state instead of the legacy flat
     *  URL params. The pillar prop must match `panelPath.pillar`. */
    panelPath?: PanelPath;
  }

  let { pillar, lockedView, panelPath }: Props = $props();

  const ctx: FetchContext = { baseUrl: '/api/v1' };
  const url = $derived(urlState());

  // Phase 122i — resolve the addressed Panel when bound. Returns `null`
  // when the path is stale (e.g. the user just removed the panel) so the
  // controls fall back to legacy flat-URL behaviour for one frame
  // instead of crashing.
  const boundPanel = $derived.by(() => {
    if (!panelPath) return null;
    return (
      url.pillars?.[panelPath.pillar]?.windows[panelPath.windowIndex]?.panels[
        panelPath.panelIndex
      ] ?? null
    );
  });
  const isPanelBound = $derived(boundPanel !== null);
  const isPanelLocked = $derived(boundPanel?.locked === true);

  const dateWindow = $derived.by(() => {
    const now = Date.now();
    const fromMs = url.from ? Date.parse(url.from) : now - DEFAULT_LOOKBACK_MS;
    const toMs = url.to ? Date.parse(url.to) : now;
    return {
      startDate: new Date(Number.isFinite(fromMs) ? fromMs : now - DEFAULT_LOOKBACK_MS)
        .toISOString()
        .slice(0, 10),
      endDate: new Date(Number.isFinite(toMs) ? toMs : now).toISOString().slice(0, 10)
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

  const activeMetric = $derived(
    boundPanel ? boundPanel.metric : (url.metric ?? DEFAULT_METRIC_NAME)
  );
  const activeLayer = $derived<'gold' | 'silver'>(
    boundPanel ? boundPanel.layer : url.layer === 'silver' ? 'silver' : 'gold'
  );
  const activeNormalization = $derived<Normalization>(
    boundPanel ? (boundPanel.normalization ?? 'raw') : (url.normalization ?? 'raw')
  );
  const activeResolution = $derived<Resolution>(
    boundPanel ? (boundPanel.resolution ?? 'daily') : (url.resolution ?? 'daily')
  );

  const RESOLUTIONS: ReadonlyArray<{ id: Resolution; label: string }> = [
    { id: 'hourly', label: 'Hourly' },
    { id: 'daily', label: 'Daily' },
    { id: 'weekly', label: 'Weekly' },
    { id: 'monthly', label: 'Monthly' }
  ];

  const presentations = $derived(presentationsForPillar(pillar as ViewingMode));
  const activePresentation = $derived(
    lockedView
      ? (presentations.find((p) => p.id === lockedView) ??
          resolvePresentation(boundPanel?.view ?? url.viewMode, pillar))
      : resolvePresentation(boundPanel?.view ?? url.viewMode, pillar)
  );

  // Per-view capability flags (Phase 122h Findings round 3). Cells that
  // don't consume the metric / resolution prop get the corresponding
  // control hidden so the UI never misleads the user about what changes
  // when they click.
  const viewUsesMetric = $derived(activePresentation.usesMetric ?? true);
  const viewUsesResolution = $derived(activePresentation.usesResolution ?? false);

  // Metric list: DEFAULT first, then API order. Defensive — `availQ` may
  // be pending or refusing; in that case we still surface the default so
  // the picker is never empty.
  const metrics = $derived.by<string[]>(() => {
    const seen: Record<string, true> = {};
    const merged: string[] = [];
    const fromApi =
      availQ.data?.kind === 'success' ? availQ.data.data.map((m) => m.metricName) : [];
    for (const name of [DEFAULT_METRIC_NAME, ...fromApi]) {
      if (!name || seen[name]) continue;
      seen[name] = true;
      merged.push(name);
    }
    if (activeMetric && !seen[activeMetric]) merged.push(activeMetric);
    return merged;
  });

  function pickMetric(name: string) {
    if (name === activeMetric) return;
    if (panelPath) {
      updatePanel(panelPath, (p) => ({ ...p, metric: name }));
      return;
    }
    setUrl({ metric: name });
  }
  function pickView(id: ViewMode) {
    if (id === activePresentation.id) return;
    if (panelPath) {
      updatePanel(panelPath, (p) => ({ ...p, view: id }));
      return;
    }
    setUrl({ viewMode: id });
  }
  function pickLayer(next: 'gold' | 'silver') {
    if (next === activeLayer) return;
    if (panelPath) {
      updatePanel(panelPath, (p) => ({ ...p, layer: next }));
      return;
    }
    setUrl({ layer: next === 'silver' ? 'silver' : null });
  }
  function pickNorm(next: Normalization) {
    if (next === activeNormalization) return;
    if (panelPath) {
      updatePanel(panelPath, (p) => {
        const out = { ...p };
        if (next === 'raw') delete out.normalization;
        else out.normalization = next;
        return out;
      });
      return;
    }
    setUrl({ normalization: next === 'raw' ? null : next });
  }
  function pickResolution(next: Resolution) {
    if (next === activeResolution) return;
    if (panelPath) {
      updatePanel(panelPath, (p) => ({ ...p, resolution: next }));
      return;
    }
    setUrl({ resolution: next });
  }
  function pickComposition(next: 'merged' | 'split') {
    if (!panelPath || !boundPanel) return;
    if (boundPanel.composition === next) return;
    updatePanel(panelPath, (p) => ({ ...p, composition: next }));
  }
</script>

<section class="cell-controls" aria-label="Cell controls" class:locked={isPanelLocked}>
  {#if isPanelLocked && boundPanel}
    <div class="locked-banner" role="status">
      🔒 Locked to <strong>{boundPanel.lockedFunction ?? 'discourse function'}</strong> — return to the
      Probe Dossier to recombine scope.
    </div>
  {/if}

  {#if isPanelBound && boundPanel && !isPanelLocked}
    <!-- Phase 122i — Composition row appears only when CellControls is
         bound to a Panel. The legacy global-state path has no
         composition concept. -->
    <div class="ctrl-row" role="radiogroup" aria-label="Composition">
      <span class="ctrl-eyebrow">Composition</span>
      <div class="ctrl-options">
        <button
          type="button"
          role="radio"
          aria-checked={boundPanel.composition === 'merged'}
          class="ctrl-btn"
          class:active={boundPanel.composition === 'merged'}
          onclick={() => pickComposition('merged')}
          title="One Cell aggregates all ScopeGroups in this panel"
        >
          Merged
        </button>
        <button
          type="button"
          role="radio"
          aria-checked={boundPanel.composition === 'split'}
          class="ctrl-btn"
          class:active={boundPanel.composition === 'split'}
          onclick={() => pickComposition('split')}
          title="One Cell per source or per ScopeGroup (small-multiples)"
        >
          Split
        </button>
      </div>
    </div>
  {/if}

  <!-- View / Darstellung row — always visible. When `lockedView` is set
       (Rhizome entry-questions), renders as an informational badge so the
       user sees WHICH view is active without being able to switch. -->
  <div class="ctrl-row" role="radiogroup" aria-label="View">
    <span class="ctrl-eyebrow">View</span>
    {#if lockedView}
      <span class="ctrl-locked" title={activePresentation.description}>
        {activePresentation.label}
        <span class="ctrl-locked-hint">(set by the entry-question)</span>
      </span>
    {:else}
      <div class="ctrl-options">
        {#each presentations as p (p.id)}
          <button
            type="button"
            role="radio"
            aria-checked={activePresentation.id === p.id}
            class="ctrl-btn"
            class:active={activePresentation.id === p.id}
            title={p.description}
            onclick={() => pickView(p.id)}
          >
            {p.label}
          </button>
        {/each}
      </div>
    {/if}
  </div>

  <!-- Metric row — only when the active view consumes a metric. BERTopic
       and co-occurrence cells ignore metricName, so the row is omitted
       entirely for those views (no misleading no-op selector). -->
  {#if viewUsesMetric}
    <div class="ctrl-row" role="radiogroup" aria-label="Metric">
      <span class="ctrl-eyebrow">Metric</span>
      <div class="ctrl-options">
        {#each metrics as m (m)}
          <button
            type="button"
            role="radio"
            aria-checked={activeMetric === m}
            class="ctrl-btn metric-btn"
            class:active={activeMetric === m}
            onclick={() => pickMetric(m)}
          >
            <code>{m}</code>
          </button>
        {/each}
      </div>
    </div>
  {/if}

  <!-- Resolution row — only when the active view bins values along a
       time axis. Distribution / topic_* / cooccurrence cells aggregate
       differently and ignore resolution; the row stays hidden there. -->
  {#if viewUsesResolution}
    <div class="ctrl-row" role="radiogroup" aria-label="Time resolution">
      <span class="ctrl-eyebrow">Resolution</span>
      <div class="ctrl-options">
        {#each RESOLUTIONS as r (r.id)}
          <button
            type="button"
            role="radio"
            aria-checked={activeResolution === r.id}
            class="ctrl-btn"
            class:active={activeResolution === r.id}
            onclick={() => pickResolution(r.id)}
          >
            {r.label}
          </button>
        {/each}
      </div>
    </div>
  {/if}

  <!-- Layer + Vergleich on one row to save vertical space -->
  <div class="ctrl-row ctrl-row-split">
    <div class="ctrl-group" role="radiogroup" aria-label="Data layer">
      <span class="ctrl-eyebrow">Layer</span>
      <div class="ctrl-options">
        <button
          type="button"
          role="radio"
          aria-checked={activeLayer === 'gold'}
          class="ctrl-btn layer-btn"
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
          class="ctrl-btn layer-btn silver"
          class:active={activeLayer === 'silver'}
          title="Ag Silver — document-level data (WP-006 §5.2)"
          onclick={() => pickLayer('silver')}
        >
          Ag Silver
        </button>
      </div>
    </div>

    <div class="ctrl-group" role="radiogroup" aria-label="Normalization">
      <span class="ctrl-eyebrow">Compare</span>
      <div class="ctrl-options">
        <button
          type="button"
          role="radio"
          aria-checked={activeNormalization === 'raw'}
          class="ctrl-btn"
          class:active={activeNormalization === 'raw'}
          title="Raw values"
          onclick={() => pickNorm('raw')}
        >
          raw
        </button>
        <button
          type="button"
          role="radio"
          aria-checked={activeNormalization === 'zscore'}
          class="ctrl-btn"
          class:active={activeNormalization === 'zscore'}
          title="Z-score deviation (Phase 115 cross-frame gate)"
          onclick={() => pickNorm('zscore')}
        >
          deviation
        </button>
        <button
          type="button"
          role="radio"
          aria-checked={activeNormalization === 'percentile'}
          class="ctrl-btn"
          class:active={activeNormalization === 'percentile'}
          title="Percentile rank within scope"
          onclick={() => pickNorm('percentile')}
        >
          percentile
        </button>
      </div>
    </div>
  </div>
</section>

<style>
  .cell-controls {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    background: linear-gradient(180deg, rgba(82, 131, 184, 0.08), rgba(82, 131, 184, 0.02));
    border: 1px solid var(--color-accent-muted);
    border-radius: var(--radius-md);
  }

  .cell-controls.locked {
    background: linear-gradient(180deg, rgba(150, 150, 150, 0.1), rgba(150, 150, 150, 0.04));
    border-color: var(--color-border);
  }

  .locked-banner {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    padding: var(--space-1) var(--space-2);
    background: var(--color-surface);
    border-radius: var(--radius-sm);
  }

  .ctrl-row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
  }

  .ctrl-row-split {
    gap: var(--space-4);
  }

  .ctrl-group {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
  }

  .ctrl-eyebrow {
    font-size: 10px;
    font-family: var(--font-mono);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-accent);
    font-weight: var(--font-weight-semibold);
    min-width: 3.5rem;
    flex-shrink: 0;
  }

  .ctrl-options {
    display: inline-flex;
    flex-wrap: wrap;
    gap: var(--space-1);
  }

  .ctrl-btn {
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

  .ctrl-btn.metric-btn code {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: inherit;
  }

  .ctrl-btn:hover,
  .ctrl-btn:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .ctrl-btn.active {
    color: var(--color-fg);
    background: rgba(82, 131, 184, 0.25);
    border-color: var(--color-accent);
  }

  .ctrl-btn.layer-btn.silver.active {
    color: #7ec4a0;
    background: rgba(126, 196, 160, 0.18);
    border-color: #7ec4a0;
  }

  .ctrl-locked {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    padding: 3px var(--space-3);
    background: rgba(82, 131, 184, 0.18);
    border: 1px solid var(--color-accent);
    border-radius: var(--radius-sm);
    font-family: var(--font-ui);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
  }

  .ctrl-locked-hint {
    font-size: 10.5px;
    color: var(--color-fg-subtle);
    font-style: italic;
  }

  @media (prefers-reduced-motion: reduce) {
    .ctrl-btn {
      transition: none;
    }
  }
</style>
