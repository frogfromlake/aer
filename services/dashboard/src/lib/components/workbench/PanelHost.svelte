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
  import type { Panel } from '$lib/state/url-internals';
  import type { FetchContext, ProbeDossierDto } from '$lib/api/queries';
  import { selectCellRender, type CellRenderUnit } from '$lib/workbench/panel-queries';

  interface Props {
    panel: Panel;
    dossier: ProbeDossierDto;
    ctx: FetchContext;
    windowStart: string;
    windowEnd: string;
  }

  let { panel, dossier, ctx, windowStart, windowEnd }: Props = $props();

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

<article class="panel-host" data-composition={panel.composition} data-view={panel.view}>
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
  </header>

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
