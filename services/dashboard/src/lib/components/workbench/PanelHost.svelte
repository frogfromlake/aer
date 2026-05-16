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
  import type { FetchContext, ProbeDossierDto } from '$lib/api/queries';
  import { selectCellRender, type CellRenderUnit } from '$lib/workbench/panel-queries';
  import {
    addScopeGroupToFocused,
    focusPanel,
    removePanel,
    removeScopeGroup
  } from '$lib/workbench/panel-mutators';
  import CellControls from './CellControls.svelte';

  interface Props {
    panel: Panel;
    dossier: ProbeDossierDto;
    ctx: FetchContext;
    windowStart: string;
    windowEnd: string;
    /** Phase 122i — when set, the host enables interactive editing
     *  (focus on click, CellControls, +Compare, ×Remove). Absent for
     *  legacy/preview rendering paths. */
    pillar?: ViewingMode;
    windowIndex?: number;
    panelIndex?: number;
    focused?: boolean;
    canRemove?: boolean;
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
    canRemove = false
  }: Props = $props();

  const path = $derived(
    pillar !== undefined && windowIndex !== undefined && panelIndex !== undefined
      ? { pillar, windowIndex, panelIndex }
      : null
  );
  const isInteractive = $derived(path !== null);

  function onFocusClick() {
    if (path) focusPanel(path);
  }
  function onRemove(e: MouseEvent) {
    e.stopPropagation();
    if (path) removePanel(path);
  }
  function onAddCompare(e: MouseEvent) {
    e.stopPropagation();
    if (path) addScopeGroupToFocused(path.pillar);
  }
  function onRemoveGroup(groupIndex: number) {
    if (path) removeScopeGroup(path, groupIndex);
  }

  const presentation = $derived<PresentationDefinition>(getPresentation(panel.view));
  const cellRender = $derived(selectCellRender(panel));

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
          disabled={panel.locked === true}
          title={panel.locked
            ? 'Scope locked — return to the Dossier to recombine sources'
            : 'Add a comparison ScopeGroup to this panel'}
        >
          ＋ Compare
        </button>
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

  {#if focused && path}
    <CellControls pillar={path.pillar} panelPath={path} />
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

  <div class="panel-body" class:split={cellRender.strategy === 'split'}>
    {#if loadError}
      <p class="muted">Cell failed to load: {loadError}</p>
    {:else if !CellComponent}
      <p class="muted" aria-busy="true">Loading {presentation.label}…</p>
    {:else if expandedUnits.length === 0}
      <p class="muted">No sources in the active scope.</p>
    {:else}
      {@const Cell = CellComponent}
      {#each expandedUnits as unit (unit.key)}
        <div class="panel-cell">
          <Cell
            {ctx}
            scopeProbeId={dossier.probeId}
            scope={unit.scope}
            scopeId={unit.scopeId}
            {windowStart}
            {windowEnd}
            metricName={panel.metric}
            sources={sourcesForUnit(unit)}
            {dataLayer}
            probeIds={unit.probeIds.length > 1 ? [...unit.probeIds] : []}
            composition={panel.composition}
          />
        </div>
      {/each}
    {/if}
  </div>
</article>

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

  .panel-body.split {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(20rem, 1fr));
    gap: var(--space-4);
  }

  .panel-cell {
    min-height: 14rem;
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
</style>
