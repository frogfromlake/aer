<script lang="ts">
  // PanelControls — Phase 122h Findings round 2 §F1.
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

  // Phase 122k F5 — per-Panel window. The dateWindow derives from the
  // bound panel's own windowStart/windowEnd when set; otherwise falls
  // back to the global url.from/url.to. The Window date inputs below
  // mutate panel.windowStart/windowEnd via updatePanel.
  const dateWindow = $derived.by(() => {
    const now = Date.now();
    const panelStart = boundPanel?.windowStart;
    const panelEnd = boundPanel?.windowEnd;
    const fromSrc = panelStart ?? url.from;
    const toSrc = panelEnd ?? url.to;
    const fromMs = fromSrc ? Date.parse(fromSrc) : now - DEFAULT_LOOKBACK_MS;
    const toMs = toSrc ? Date.parse(toSrc) : now;
    return {
      startDate: new Date(Number.isFinite(fromMs) ? fromMs : now - DEFAULT_LOOKBACK_MS)
        .toISOString()
        .slice(0, 10),
      endDate: new Date(Number.isFinite(toMs) ? toMs : now).toISOString().slice(0, 10),
      isPanelOverride: panelStart !== undefined || panelEnd !== undefined
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

  const activeMetric = $derived(boundPanel ? boundPanel.metric : DEFAULT_METRIC_NAME);
  const activeLayer = $derived<'gold' | 'silver'>(boundPanel ? boundPanel.layer : 'gold');
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
          resolvePresentation(boundPanel?.view ?? null, pillar))
      : resolvePresentation(boundPanel?.view ?? null, pillar)
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
    }
    // Phase 122k: metric/view/layer are panel-state only. No-op when no
    // panel context (the empty-state path will be wired in K3 to open the
    // ScopeEditor instead of rendering PanelControls).
  }
  function pickView(id: ViewMode) {
    if (id === activePresentation.id) return;
    if (panelPath) {
      updatePanel(panelPath, (p) => ({ ...p, view: id }));
    }
  }
  function pickLayer(next: 'gold' | 'silver') {
    if (next === activeLayer) return;
    if (panelPath) {
      updatePanel(panelPath, (p) => ({ ...p, layer: next }));
    }
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
  function pickComposition(next: 'merged' | 'split' | 'overlay') {
    if (!panelPath || !boundPanel) return;
    if (boundPanel.composition === next) return;
    updatePanel(panelPath, (p) => ({ ...p, composition: next }));
  }
  function pickSplitDirection(next: 'horizontal' | 'vertical') {
    if (!panelPath || !boundPanel) return;
    if ((boundPanel.splitDirection ?? 'horizontal') === next) return;
    updatePanel(panelPath, (p) => ({ ...p, splitDirection: next }));
  }
  function toggleCollapsed() {
    if (!panelPath || !boundPanel) return;
    const next = !(boundPanel.cellControlsCollapsed === true);
    updatePanel(panelPath, (p) => ({ ...p, cellControlsCollapsed: next }));
  }

  // Phase 122k F5 — Window date handlers. Both write to the bound panel's
  // own windowStart/windowEnd; when the panel has no override yet the
  // write installs one. Clearing the override (returning to global
  // default) is exposed via the "Reset to default window" button.
  function pickWindowStart(value: string) {
    if (!panelPath || !value) return;
    const iso = new Date(value).toISOString();
    if (Number.isNaN(Date.parse(iso))) return;
    updatePanel(panelPath, (p) => ({ ...p, windowStart: iso }));
  }
  function pickWindowEnd(value: string) {
    if (!panelPath || !value) return;
    const iso = new Date(value).toISOString();
    if (Number.isNaN(Date.parse(iso))) return;
    updatePanel(panelPath, (p) => ({ ...p, windowEnd: iso }));
  }
  function resetWindowToGlobal() {
    if (!panelPath) return;
    updatePanel(panelPath, (p) => {
      const out = { ...p };
      delete out.windowStart;
      delete out.windowEnd;
      return out;
    });
  }

  const activeSplitDirection = $derived<'horizontal' | 'vertical'>(
    boundPanel?.splitDirection ?? 'horizontal'
  );
  const isCollapsed = $derived(boundPanel?.cellControlsCollapsed === true);
</script>

<section
  class="cell-controls"
  aria-label="Panel controls"
  class:locked={isPanelLocked}
  class:collapsed={isCollapsed}
>
  {#if isPanelBound && boundPanel}
    <!-- Phase 122k §14 — full-header click toggles collapse (not just
         the mini chevron). Header always rendered so a collapsed strip
         can be re-opened. -->
    <button
      type="button"
      class="cell-controls-header"
      aria-expanded={!isCollapsed}
      aria-label={isCollapsed ? 'Expand panel controls' : 'Collapse panel controls'}
      onclick={toggleCollapsed}
    >
      {#if isPanelLocked && boundPanel}
        <span class="locked-banner" role="status">
          🔒 Scope locked to
          <strong>{boundPanel.lockedFunction ?? 'discourse function'}</strong>'s sources
        </span>
      {:else}
        <span class="header-eyebrow">Panel controls</span>
      {/if}
      <span
        class="collapse-toggle"
        class:expanded={!isCollapsed}
        aria-hidden="true"
        title={isCollapsed ? 'Expand panel controls' : 'Collapse panel controls'}
      >
        {isCollapsed ? '▾' : '▴'}
      </span>
    </button>
  {/if}

  {#if isPanelBound && boundPanel && !isCollapsed}
    <!-- Phase 122i revision (B1): Composition row appears whenever
         PanelControls is bound to a Panel — including locked panels.
         `locked` is scope-only; the user can toggle Merged ↔ Split on
         a DF-entry Workbench freely. The legacy global-state path
         (boundPanel === null) has no composition concept and the row
         is hidden. -->
    <!-- Phase 122k §14c finding 1 — Composition row uses TWO parallel
         labeled control groups separated by a thin vertical divider.
         Both labels (Composition / Direction) sit on the same baseline
         using the standard `ctrl-eyebrow` style. Merged is its own
         button, never glued to Vertical. -->
    <div class="ctrl-row composition-row">
      <div class="comp-group" role="radiogroup" aria-label="Composition">
        <span class="ctrl-eyebrow">Composition</span>
        <div class="ctrl-options">
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
          <button
            type="button"
            role="radio"
            aria-checked={boundPanel.composition === 'overlay'}
            class="ctrl-btn"
            class:active={boundPanel.composition === 'overlay'}
            onclick={() => pickComposition('overlay')}
            title="One Cell — sources rendered as separate viridis-coloured lines on a shared canvas"
          >
            Overlay
          </button>
          <button
            type="button"
            role="radio"
            aria-checked={boundPanel.composition === 'merged'}
            class="ctrl-btn"
            class:active={boundPanel.composition === 'merged'}
            onclick={() => pickComposition('merged')}
            title="One Cell — sources aggregated into a single joint-corpus chart"
          >
            Merged
          </button>
        </div>
      </div>

      {#if boundPanel.composition === 'split'}
        <div class="comp-divider" aria-hidden="true"></div>
        <div class="comp-group" role="radiogroup" aria-label="Split direction">
          <span class="ctrl-eyebrow">Direction</span>
          <div class="ctrl-options">
            <button
              type="button"
              role="radio"
              aria-checked={activeSplitDirection === 'horizontal'}
              class="ctrl-btn"
              class:active={activeSplitDirection === 'horizontal'}
              onclick={() => pickSplitDirection('horizontal')}
              title="Arrange split cells side-by-side"
              aria-label="Split direction: horizontal"
            >
              ↔ Horizontal
            </button>
            <button
              type="button"
              role="radio"
              aria-checked={activeSplitDirection === 'vertical'}
              class="ctrl-btn"
              class:active={activeSplitDirection === 'vertical'}
              onclick={() => pickSplitDirection('vertical')}
              title="Stack split cells vertically"
              aria-label="Split direction: vertical"
            >
              ↕ Vertical
            </button>
          </div>
        </div>
      {/if}
    </div>
  {/if}

  {#if !isCollapsed}
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

    <!-- Phase 122k §14 finding 5 — Window is more important than
         Layer/Compare so it sits above them in the row order. The date
         inputs have their click events stopped from bubbling so the
         article-level focus handler doesn't close the native date
         picker mid-interaction. -->
    {#if isPanelBound}
      <div class="ctrl-row" role="group" aria-label="Time window">
        <span class="ctrl-eyebrow">Window</span>
        <div class="window-inputs" onclick={(e) => e.stopPropagation()} role="presentation">
          <input
            type="date"
            value={dateWindow.startDate}
            onchange={(e) => pickWindowStart((e.currentTarget as HTMLInputElement).value)}
            onclick={(e) => e.stopPropagation()}
            aria-label="Window start"
          />
          <span class="window-sep" aria-hidden="true">→</span>
          <input
            type="date"
            value={dateWindow.endDate}
            onchange={(e) => pickWindowEnd((e.currentTarget as HTMLInputElement).value)}
            onclick={(e) => e.stopPropagation()}
            aria-label="Window end"
          />
          {#if dateWindow.isPanelOverride}
            <button
              type="button"
              class="ctrl-btn"
              onclick={resetWindowToGlobal}
              title="Drop this panel's window override and inherit the global default"
            >
              Reset
            </button>
          {/if}
        </div>
      </div>
    {/if}

    <!-- Layer + Compare on one row — both are low-frequency controls;
         grouped to save vertical space. -->
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
  {/if}
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

  /* Phase 122k §14b finding 4 — clickable header gets a subtle resting
     background tint plus a stronger hover state so the user reads the
     full strip as an interactive surface, not just the chevron. */
  .cell-controls-header {
    appearance: none;
    background: color-mix(in srgb, var(--color-fg) 4%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-border) 50%, transparent);
    border-radius: var(--radius-sm);
    color: inherit;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    padding: var(--space-2) var(--space-3);
    text-align: left;
    transition:
      background-color var(--motion-duration-fast) var(--motion-ease-standard),
      border-color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .cell-controls-header:hover,
  .cell-controls-header:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 10%, transparent);
    border-color: var(--color-accent);
    color: var(--color-fg);
  }

  .header-eyebrow {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
  }

  .collapse-toggle {
    margin-left: auto;
    color: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    line-height: 1;
    padding: 0 var(--space-1);
  }

  .cell-controls.collapsed {
    padding-bottom: var(--space-2);
  }

  .ctrl-row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
    padding: var(--space-2) 0;
  }
  /* Phase 122k §14 finding 5 — subtle vertical separators between rows
     so the composition / view / metric / window blocks read as discrete
     control groups. The last row gets no bottom border. */
  .ctrl-row + .ctrl-row {
    border-top: 1px dashed color-mix(in srgb, var(--color-border) 50%, transparent);
  }

  .ctrl-row-split {
    gap: var(--space-4);
  }

  /* Phase 122k §14c finding 1 — Composition row layout. Two labeled
     control groups (Composition / Direction) separated by a thin
     vertical divider. Direction appears only when Split is active. */
  .composition-row {
    align-items: flex-start;
  }
  .comp-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }
  .comp-group > .ctrl-options {
    display: flex;
    gap: 2px;
  }
  .comp-divider {
    align-self: stretch;
    width: 1px;
    background: color-mix(in srgb, var(--color-border) 60%, transparent);
    margin: 0 var(--space-2);
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

  .window-inputs {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
  }

  .window-inputs input[type='date'] {
    appearance: none;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg);
    padding: 3px var(--space-2);
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    cursor: text;
    color-scheme: dark;
  }
  .window-inputs input[type='date']:hover,
  .window-inputs input[type='date']:focus-visible {
    border-color: var(--color-accent);
  }

  .window-sep {
    color: var(--color-fg-subtle);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
  }

  @media (prefers-reduced-motion: reduce) {
    .ctrl-btn {
      transition: none;
    }
  }
</style>
