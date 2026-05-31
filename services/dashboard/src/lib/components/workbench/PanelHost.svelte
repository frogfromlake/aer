<script lang="ts">
  // Phase 122i / ADR-034 — Multi-Panel Workbench: Panel renderer.
  //
  // A Panel is a `(scopes[], composition, view, metric, layer, …)` tuple
  // from the URL state. PanelHost translates it into one or more Cell
  // renderings:
  //   - merged-single → 1 Cell with the panel's single ScopeGroup.
  //   - merged-multi  → 1 Cell with the union of N ScopeGroups (CSV lists
  //                     for endpoints that consume them today; Slice 6
  //                     wires the CoOccurrence multi-scope POST path).
  //   - split         → N Cells, one per source or per ScopeGroup.
  //
  // Cells keep their Phase-107 `ViewModeCellProps` contract. The host
  // resolves Source-name → SourceMeta via the Dossier so cells receive
  // the same `sources` shape they always have.
  import type { Component } from 'svelte';
  import {
    getPresentation,
    type PresentationDefinition,
    type ViewModeCellProps
  } from '$lib/viewmodes';
  import type { Panel, ViewingMode } from '$lib/state/url-internals';
  import type { FetchContext, ProbeDossierDto, RefusalOutcome } from '$lib/api/queries';
  import {
    selectCellRender,
    shouldRefuseMergedCrossProbe,
    type CellRenderUnit
  } from '$lib/workbench/panel-queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import {
    focusPanel,
    removePanel,
    removeScopeGroup,
    toggleMaximizedPanel,
    updatePanel
  } from '$lib/workbench/panel-mutators';
  import type { DiscourseFunction } from '$lib/discourse-function';
  import type { ScopeGroup } from '$lib/state/url-internals';
  import PanelControls from './PanelControls.svelte';
  import PanelMetaStrip from './PanelMetaStrip.svelte';
  import ScopeEditor from './ScopeEditor.svelte';

  interface Props {
    panel: Panel;
    dossier: ProbeDossierDto;
    ctx: FetchContext;
    windowStart: string;
    windowEnd: string;
    /** Phase 122i — when set, the host enables interactive editing
     *  (focus on click, PanelControls, +Compare, ×Remove). Absent for
     *  legacy/preview rendering paths. */
    pillar?: ViewingMode;
    windowIndex?: number;
    panelIndex?: number;
    focused?: boolean;
    canRemove?: boolean;
    /** Phase 122i revision (C3) — whether this panel is the maximized
     *  member of its window. Drives the icon on the maximize button
     *  (⤢ to maximize, ⤡ to restore). */
    isMaximized?: boolean;
    /** Phase 122i revision (C3) — whether a Maximize button should
     *  appear on this panel. Suppressed when the window has only one
     *  panel (nothing to compare against, no minimised tray would
     *  appear). */
    canMaximize?: boolean;
  }

  let {
    panel,
    dossier,
    ctx,
    windowStart,
    windowEnd,
    pillar,
    windowIndex,
    panelIndex,
    focused = false,
    canRemove = false,
    isMaximized = false,
    canMaximize = false
  }: Props = $props();

  const path = $derived(
    pillar !== undefined && windowIndex !== undefined && panelIndex !== undefined
      ? { pillar, windowIndex, panelIndex }
      : null
  );
  const isInteractive = $derived(path !== null);

  // Phase 122i revision (D3). `+ Compare` now opens the ScopeEditor
  // popover where the user picks sources for the new group. Previously
  // the button silently seeded a default group with all probes and
  // empty sources — confusing UX since the user wanted to choose.
  let scopeEditorOpen = $state(false);

  function onFocusClick() {
    if (path) focusPanel(path);
  }
  function onRemove(e: MouseEvent) {
    e.stopPropagation();
    if (path) removePanel(path);
  }
  function onAddCompare(e: MouseEvent) {
    e.stopPropagation();
    if (!path) return;
    scopeEditorOpen = true;
  }
  function onRemoveGroup(groupIndex: number) {
    if (path) removeScopeGroup(path, groupIndex);
  }
  function onToggleMaximize(e: MouseEvent) {
    e.stopPropagation();
    if (!path) return;
    toggleMaximizedPanel(path.pillar, path.windowIndex, path.panelIndex);
  }

  const presentation = $derived<PresentationDefinition>(getPresentation(panel.view));
  const cellRender = $derived(selectCellRender(panel));

  // Phase 130 / ADR-035 — merged-cross-probe guard (Brief §1.3). A merged
  // Cell that pools >1 probe for a scaled/intensive metric would render a
  // cross-context ranking; refuse it via the standard refusal surface.
  // `split`/`overlay` keep each probe on its own axis and are unaffected.
  const crossProbeMergeRefused = $derived(shouldRefuseMergedCrossProbe(panel, cellRender));
  const crossProbeRefusal = $derived<RefusalOutcome | null>(
    crossProbeMergeRefused
      ? {
          kind: 'refusal',
          refusalKind: 'merged_cross_probe_unsupported',
          message: `Merged composition pools more than one probe onto a single shared axis for "${panel.metric}", a scaled metric — that reads as a cross-context ranking, which AĒR does not render (Brief §1.3). Switch this Panel to split or overlay, narrow it to one probe, or choose a pure-count metric.`,
          httpStatus: 422
        }
      : null
  );

  // When `split + single group without source narrowing`, the panel-queries
  // selector returns `merged-single` so this host can fan out across the
  // dossier's sources at render time. That semantic check lives here, not
  // in the pure mapper, because only the host has the Dossier.
  const expandedUnits = $derived.by<readonly CellRenderUnit[]>(() => {
    if (
      panel.composition === 'split' &&
      panel.scopes.length === 1 &&
      panel.scopes[0]!.sourceIds.length === 0 &&
      cellRender.strategy === 'merged-single'
    ) {
      // The dossier carries the resolved source list for the panel's
      // probe; fan out one Cell per source.
      return dossier.sources.map((s, i) => ({
        key: `dossier-${i}`,
        scope: 'source' as const,
        scopeId: s.name,
        probeIds: [],
        sourceIds: [s.name]
      }));
    }
    return cellRender.units;
  });

  // Lazy-load the Cell component. Each presentation lives in its own
  // chunk; the same Component instance is reused across all units of a
  // panel (the contract is per-unit-props, not per-unit-bundle).
  let CellComponent = $state<Component<ViewModeCellProps> | null>(null);
  let loadError = $state<string | null>(null);
  let loadToken = 0;

  $effect(() => {
    const t = ++loadToken;
    loadError = null;
    presentation
      .loadComponent()
      .then((Comp) => {
        if (t !== loadToken) return;
        CellComponent = Comp;
      })
      .catch((err: unknown) => {
        if (t !== loadToken) return;
        CellComponent = null;
        loadError = err instanceof Error ? err.message : 'Cell failed to load';
      });
  });

  // Resolve Source IDs back to SourceMeta for the Cell contract.
  function sourcesForUnit(unit: CellRenderUnit) {
    if (unit.sourceIds.length === 0) {
      // Probe-scope: pass all dossier sources so per-source cells
      // (time_series) can iterate; per-scope cells ignore the list.
      return dossier.sources.map((s) => ({
        name: s.name,
        emicDesignation: s.emicDesignation
      }));
    }
    return unit.sourceIds.map((id) => {
      const match = dossier.sources.find((s) => s.name === id);
      return {
        name: id,
        emicDesignation: match?.emicDesignation ?? null
      };
    });
  }

  const dataLayer = $derived<'gold' | 'silver'>(panel.layer === 'silver' ? 'silver' : 'gold');

  // Phase 122k F5 — effective per-panel window: the panel's own
  // windowStart/windowEnd override the inherited defaults when set.
  const effectiveWindowStart = $derived(panel.windowStart ?? windowStart);
  const effectiveWindowEnd = $derived(panel.windowEnd ?? windowEnd);

  // Phase 131 (bugfix) — the split LAYOUT (side-by-side / stacked + the
  // horizontal/vertical direction) must engage whenever the panel renders
  // more than one cell under `split` composition, INCLUDING the common
  // probe-scope fan-out where `selectCellRender` returns `merged-single`
  // and the host expands it per dossier source via `expandedUnits`. Keying
  // the layout off `cellRender.strategy === 'split'` alone missed that path,
  // so per-scope split cells always stacked vertically and the direction
  // toggle appeared dead. Drive it off the composition + rendered-cell count.
  const isSplitLayout = $derived(panel.composition === 'split' && expandedUnits.length > 1);
</script>

<!--
  Phase 122i — the Panel-host is an `<article>` for semantic structure
  (each Panel is a self-contained analytical unit). Click-anywhere-to-
  focus is implemented on the article itself; Svelte's `<article>` is
  considered noninteractive by default but we layer a `role="button"`
  + tabindex on top. The svelte-ignore below is intentional: switching
  to a `<div>` or wrapper `<button>` would lose the semantic, and the
  keyboard handler below satisfies the actual a11y requirement.
-->
<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
<article
  class="panel-host"
  class:focused
  class:interactive={isInteractive}
  data-composition={panel.composition}
  data-view={panel.view}
  onclick={onFocusClick}
  onkeydown={(e) => {
    if (e.key === 'Enter' || e.key === ' ') onFocusClick();
  }}
  role={isInteractive ? 'button' : undefined}
  tabindex={isInteractive ? 0 : undefined}
>
  <header class="panel-header">
    <span class="panel-eyebrow">{presentation.label}</span>
    {#if presentation.usesMetric !== false}
      <span class="panel-sep" aria-hidden="true">·</span>
      <code class="panel-metric">{panel.metric}</code>
    {/if}
    {#if panel.locked === true && panel.lockedFunction}
      <span class="panel-lock" title="Locked from Probe Dossier — return to Dossier to recombine">
        🔒 Locked to {panel.lockedFunction}
      </span>
    {/if}
    {#if isInteractive}
      <!-- Each action button stops click + keydown propagation in its own
           handler so the surrounding `<article>`'s focus handler does
           not also fire. No wrapping role needed — this is just a
           visual grouping. Phase 122i revision (B1): `locked` is
           scope-only; `+Compare` (scope mutation) is disabled when
           locked, `×Remove` and all other panel-level actions remain
           available. -->
      <div class="panel-actions">
        <button
          type="button"
          class="panel-action"
          onclick={onAddCompare}
          title="Configure this panel's scope (probes, sources, discourse-function restriction)"
        >
          ⚙ Edit scope
        </button>
        {#if canMaximize || isMaximized}
          <!-- Phase 122i revision (C3). Maximize button. Always enabled
               on locked panels too — maximize is UI state, not scope
               editing. Disappears when there's nothing else in the
               window (lone panel + not maximized = nothing to maximize
               against). -->
          <button
            type="button"
            class="panel-action"
            onclick={onToggleMaximize}
            title={isMaximized ? 'Restore (un-maximize) — Esc also works' : 'Maximize this panel'}
            aria-pressed={isMaximized}
          >
            {isMaximized ? '⤡ Restore' : '⤢ Maximize'}
          </button>
        {/if}
        {#if canRemove}
          <button
            type="button"
            class="panel-action panel-action-remove"
            onclick={onRemove}
            title="Remove this panel"
          >
            ×
          </button>
        {/if}
      </div>
    {/if}
  </header>

  {#if path}
    <PanelMetaStrip {panel} {dossier} panelPath={path} {ctx} />
  {/if}

  {#if focused && path}
    <PanelControls pillar={path.pillar} panelPath={path} />
  {/if}

  {#if isInteractive && panel.scopes.length > 1}
    <ul class="scope-groups" role="list" aria-label="Scope groups in this panel">
      {#each panel.scopes as group, i (i)}
        <li class="scope-group-chip">
          <span class="scope-group-eyebrow">Group {i + 1}</span>
          <span class="scope-group-detail">
            {group.probeIds.join(', ') || '—'}
            {#if group.sourceIds.length > 0}
              · {group.sourceIds.join(', ')}
            {/if}
          </span>
          {#if !panel.locked}
            <button
              type="button"
              class="scope-group-remove"
              onclick={(e) => {
                e.stopPropagation();
                onRemoveGroup(i);
              }}
              title="Remove this ScopeGroup"
              aria-label="Remove ScopeGroup {i + 1}"
            >
              ×
            </button>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}

  <div
    class="panel-body"
    class:split={isSplitLayout}
    data-split-direction={panel.splitDirection ?? 'horizontal'}
  >
    {#if crossProbeRefusal}
      <RefusalSurface refusal={crossProbeRefusal} {ctx} />
    {:else if loadError}
      <p class="muted">Cell failed to load: {loadError}</p>
    {:else if !CellComponent}
      <p class="muted" aria-busy="true">Loading {presentation.label}…</p>
    {:else if expandedUnits.length === 0}
      <p class="muted">No sources in the active scope.</p>
    {:else}
      {@const Cell = CellComponent}
      {#each expandedUnits as unit (unit.key)}
        {@const groupNum = unit.groupIndex !== undefined ? unit.groupIndex + 1 : null}
        <div class="panel-cell" class:has-group={groupNum !== null} data-group={groupNum}>
          {#if groupNum !== null}
            <header class="cell-group-eyebrow">
              <span class="cell-group-badge" aria-hidden="true">Group {groupNum}</span>
              <span class="cell-group-summary">
                {unit.probeIds.length > 0 ? unit.probeIds.join(', ') : unit.scopeId}
                {#if unit.sourceIds.length > 0}
                  · {unit.sourceIds.length} source{unit.sourceIds.length === 1 ? '' : 's'}
                {/if}
              </span>
            </header>
          {/if}
          <Cell
            {ctx}
            scopeProbeId={dossier.probeId}
            scope={unit.scope}
            scopeId={unit.scopeId}
            windowStart={effectiveWindowStart}
            windowEnd={effectiveWindowEnd}
            metricName={panel.metric}
            sources={sourcesForUnit(unit)}
            {dataLayer}
            probeIds={unit.probeIds.length > 1 ? [...unit.probeIds] : []}
            composition={panel.composition}
            bins={panel.bins}
            topN={panel.topN}
            channels={panel.channels}
            showBand={panel.showBand}
            resolution={panel.resolution}
            normalization={panel.normalization}
            forceStrength={panel.forceStrength}
          />
        </div>
      {/each}
    {/if}
  </div>
</article>

{#if scopeEditorOpen && path}
  <ScopeEditor
    {panel}
    {dossier}
    {ctx}
    onApply={(scopes: ScopeGroup[], lockedFunction: DiscourseFunction | null) => {
      // Commit draft state to the Panel via the mutator. The mutator
      // respects the `locked` guard (scope edits are gated when the panel
      // is DF-locked), but in 122k the lock is set BY the editor itself
      // via the DF-lock dropdown, so we update both scopes and lockedFunction
      // atomically.
      if (path) {
        updatePanel(path, (p) => {
          const next: Panel = { ...p, scopes };
          if (lockedFunction) {
            next.locked = true;
            next.lockedFunction = lockedFunction;
            next.lockedReason = 'df_entry';
          } else {
            delete next.locked;
            delete next.lockedFunction;
            delete next.lockedReason;
          }
          return next;
        });
      }
      scopeEditorOpen = false;
    }}
    onCancel={() => (scopeEditorOpen = false)}
  />
{/if}

<style>
  .panel-host {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-3);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    min-width: 28rem;
    transition: border-color var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .panel-host.interactive {
    cursor: pointer;
  }

  .panel-host.interactive:hover {
    border-color: color-mix(in srgb, var(--color-accent) 50%, var(--color-border));
  }

  .panel-host.focused {
    border-color: var(--color-accent);
    box-shadow: 0 0 0 1px var(--color-accent);
  }

  .panel-actions {
    display: flex;
    gap: var(--space-1);
    margin-left: auto;
  }

  .panel-action {
    appearance: none;
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 2px var(--space-2);
    color: var(--color-fg);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    cursor: pointer;
  }

  .panel-action:hover,
  .panel-action:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 10%, var(--color-surface));
    border-color: var(--color-accent);
  }

  .panel-action-remove {
    color: var(--color-status-expired);
  }

  .panel-action-remove:hover,
  .panel-action-remove:focus-visible {
    background: color-mix(in srgb, var(--color-status-expired) 12%, var(--color-surface));
    border-color: var(--color-status-expired);
  }

  .scope-groups {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
  }

  .scope-group-chip {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    padding: 2px var(--space-2);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-pill);
    font-size: var(--font-size-xs);
  }

  .scope-group-eyebrow {
    font-family: var(--font-mono);
    color: var(--color-fg-subtle);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .scope-group-detail {
    font-family: var(--font-mono);
    color: var(--color-fg);
  }

  .scope-group-remove {
    appearance: none;
    background: transparent;
    border: none;
    color: var(--color-fg-subtle);
    cursor: pointer;
    font-size: var(--font-size-sm);
    line-height: 1;
    padding: 0 4px;
  }

  .scope-group-remove:hover,
  .scope-group-remove:focus-visible {
    color: var(--color-status-expired);
  }

  .panel-header {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    flex-wrap: wrap;
  }

  .panel-eyebrow {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    font-weight: var(--font-weight-semibold);
  }

  .panel-sep {
    color: var(--color-fg-subtle);
  }

  .panel-metric {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
  }

  .panel-lock {
    margin-left: auto;
    font-size: var(--font-size-xs);
    color: var(--color-accent);
    font-style: italic;
  }

  .panel-body {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  /* Phase 122k §14b finding 1 — horizontal split forces side-by-side
     equal-width cells regardless of panel width. The previous
     auto-fit/minmax(20rem, 1fr) collapsed to a single column when the
     panel was narrower than 40rem (e.g. with panels-per-row=2 or 4),
     making horizontal look identical to vertical. Flex with explicit
     row/column directions is unambiguous. */
  .panel-body.split[data-split-direction='horizontal'] {
    display: flex;
    flex-direction: row;
    gap: var(--space-4);
  }
  .panel-body.split[data-split-direction='horizontal'] > .panel-cell {
    flex: 1 1 0;
    min-width: 0;
  }

  .panel-body.split[data-split-direction='vertical'] {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .panel-cell {
    min-height: 14rem;
  }

  /* Phase 122k §11 finding — multi-group split panels need a visual
     identity per group so the user can read which Cell belongs to
     which ScopeGroup. Each group gets a subtle border-left tint and
     an eyebrow header with a "Group N" badge. The colour cycles
     through a four-tone palette consistent with the ScopeEditor's
     step accents. */
  .panel-cell.has-group {
    position: relative;
    padding: var(--space-2) var(--space-3);
    background: color-mix(in srgb, var(--group-color) 4%, transparent);
    border-left: 2px solid color-mix(in srgb, var(--group-color) 50%, var(--color-border));
    border-radius: var(--radius-sm);
  }
  .panel-cell.has-group[data-group='1'] {
    --group-color: #7dc7e5;
  }
  .panel-cell.has-group[data-group='2'] {
    --group-color: #e8a25c;
  }
  .panel-cell.has-group[data-group='3'] {
    --group-color: #a3c984;
  }
  .panel-cell.has-group[data-group='4'] {
    --group-color: #d97a7a;
  }
  /* Beyond four groups, fall back to a neutral accent so the layout
     stays calm. */
  .panel-cell.has-group:not([data-group='1']):not([data-group='2']):not([data-group='3']):not(
      [data-group='4']
    ) {
    --group-color: var(--color-fg-subtle);
  }

  .cell-group-eyebrow {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    flex-wrap: wrap;
    padding: 0 0 var(--space-1) 0;
    border-bottom: 1px dashed color-mix(in srgb, var(--group-color) 30%, var(--color-border));
    margin-bottom: var(--space-2);
  }

  .cell-group-badge {
    display: inline-block;
    padding: 1px var(--space-2);
    border-radius: var(--radius-pill);
    background: color-mix(in srgb, var(--group-color) 18%, transparent);
    border: 1px solid color-mix(in srgb, var(--group-color) 50%, transparent);
    color: var(--group-color);
    font-family: var(--font-mono);
    font-size: 10.5px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .cell-group-summary {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
</style>
