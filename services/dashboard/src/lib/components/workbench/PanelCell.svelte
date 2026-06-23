<script lang="ts">
  // Phase 141 — a single rendered cell, extracted from PanelHost.svelte's
  // `{#each expandedUnits}` body. Owns one CellRenderUnit's resolved config, its
  // source resolution, the per-cell config affordance (bar + popover), the
  // group/probe/facet eyebrow, the dimension-peek banner, the lazy Cell render,
  // and the cell-scoped methodology. Cross-cell coordination (which cell is open,
  // the shared-axis union, the accent index) is decided by PanelCellGrid and
  // threaded in as props; this component is otherwise self-contained.
  import type { Component } from 'svelte';
  import { m } from '$lib/paraglide/messages.js';
  import { metricLabel } from '$lib/state/labels.svelte';
  import type { PresentationDefinition, PresentationCellProps } from '$lib/presentations';
  import type {
    FetchContext,
    ProbeDossierDto,
    ScopeAvailableMetricsDto,
    ScopeAvailableMetadataDto
  } from '$lib/api/queries';
  import { resolveCellConfig, type CellRenderUnit } from '$lib/workbench/panel-queries';
  import * as layout from '$lib/workbench/panel-host-layout';
  import type { Panel } from '$lib/state/url-internals';
  import type { PanelPath } from '$lib/workbench/panel-mutators';
  import CellConfigPopover from './CellConfigPopover.svelte';
  import ReadingGuide from '$lib/components/presentations/ReadingGuide.svelte';

  interface Props {
    unit: CellRenderUnit;
    panel: Panel;
    dossier: ProbeDossierDto;
    ctx: FetchContext;
    presentation: PresentationDefinition;
    /** The lazily-loaded cell component, shared across a panel's units. */
    CellComponent: Component<PresentationCellProps>;
    /** Interactive editing path (null disables the config popover). */
    path: PanelPath | null;
    dataLayer: 'gold' | 'silver';
    windowStart: string | undefined;
    windowEnd: string | undefined;
    selection: PresentationCellProps['selection'];
    // Cross-cell accent (PanelCellGrid resolves group vs probe vs facet).
    accentNum: number | null;
    groupNum: number | null;
    /** Probe display label for a cross-probe split fan-out cell, else null. */
    probeBadgeLabel: string | null;
    // Per-cell config affordance (offered only on multi-cell panels).
    perCellConfig: boolean;
    isConfigOpen: boolean;
    onToggleConfig: (e: MouseEvent) => void;
    onCloseConfig: () => void;
    availFullData: ScopeAvailableMetricsDto | ScopeAvailableMetadataDto | null;
    fieldDriven: boolean;
    scalarMetricOptions: string[];
    // Shared-axis comparison (Phase 124 / 126).
    sharedAxisApplies: boolean;
    cellScale: 'shared' | 'free';
    sharedDomains: { value?: readonly [number, number]; x?: readonly [number, number] } | undefined;
    reportExtent:
      | ((axis: 'value' | 'x', extent: readonly [number, number] | null) => void)
      | undefined;
  }

  let {
    unit,
    panel,
    dossier,
    ctx,
    presentation,
    CellComponent,
    path,
    dataLayer,
    windowStart,
    windowEnd,
    selection,
    accentNum,
    groupNum,
    probeBadgeLabel,
    perCellConfig,
    isConfigOpen,
    onToggleConfig,
    onCloseConfig,
    availFullData,
    fieldDriven,
    scalarMetricOptions,
    sharedAxisApplies,
    cellScale,
    sharedDomains,
    reportExtent
  }: Props = $props();

  const Cell = $derived(CellComponent);
  const cfg = $derived(resolveCellConfig(panel, unit.key));

  // Phase 148f — when a cell overrides the panel's dimension it is off-comparison,
  // so it gets its OWN "How to read" (a ReadingGuide over the cell's resolved
  // config, not the panel default) rendered below the cell.
  const cellPanel = $derived<Panel>({
    ...panel,
    metric: cfg.metric,
    ...(cfg.bins !== undefined ? { bins: cfg.bins } : {}),
    ...(cfg.topN !== undefined ? { topN: cfg.topN } : {}),
    ...(cfg.channels !== undefined ? { channels: cfg.channels } : {}),
    ...(cfg.showBand !== undefined ? { showBand: cfg.showBand } : {}),
    ...(cfg.scales !== undefined ? { scales: cfg.scales } : {})
  });

  // Resolve Source IDs back to SourceMeta for the Cell contract. Probe-scope
  // (no sourceIds) passes all dossier sources so per-source cells can iterate.
  function sourcesForUnit(u: CellRenderUnit) {
    if (u.sourceIds.length === 0) {
      return dossier.sources.map((s) => ({ name: s.name, emicDesignation: s.emicDesignation }));
    }
    return u.sourceIds.map((id) => {
      const match = dossier.sources.find((s) => s.name === id);
      return { name: id, emicDesignation: match?.emicDesignation ?? null };
    });
  }

  // The source(s) this cell covers (a single source for a per-source cell; the
  // unit's sources for a group/merged cell) — only computed while the popover is
  // open, since that is the only consumer.
  const openCellSources = $derived.by<string[]>(() => {
    if (!isConfigOpen) return [];
    if (unit.scope === 'source') return [unit.scopeId];
    return sourcesForUnit(unit).map((s) => s.name);
  });
  // ADR-038 — dimensions valid for this cell's own source(s): every intersection
  // dimension plus any partial at least one of its sources carries.
  const cellDimensionOptions = $derived(
    isConfigOpen ? layout.cellDimensionOptions(availFullData, openCellSources, fieldDriven) : []
  );
</script>

<div
  class="panel-cell"
  class:has-group={accentNum !== null}
  class:cell-overridden={cfg.isOverridden}
  data-group={accentNum}
>
  {#if perCellConfig}
    <div class="cell-config-bar">
      {#if cfg.isOverridden}
        <span class="cell-custom-badge" title={m.workbench_cell_custom_badge_title()}>
          {m.workbench_cell_custom_badge()}
        </span>
      {/if}
      <button
        type="button"
        class="cell-config-btn"
        class:active={isConfigOpen}
        aria-label={m.workbench_cell_config_btn_label()}
        aria-expanded={isConfigOpen}
        title={m.workbench_cell_config_btn_title()}
        onclick={onToggleConfig}
      >
        {m.workbench_cell_config_btn()}
      </button>
    </div>
  {/if}
  {#if perCellConfig && path && isConfigOpen}
    <CellConfigPopover
      panelPath={path}
      cellKey={unit.key}
      cellLabel={unit.scopeId}
      {panel}
      {presentation}
      {scalarMetricOptions}
      {cellDimensionOptions}
      onClose={onCloseConfig}
    />
  {/if}
  {#if groupNum !== null}
    <header class="cell-group-eyebrow">
      <span class="cell-group-badge" aria-hidden="true"
        >{m.workbench_cell_group_badge({ index: groupNum })}</span
      >
      <span class="cell-group-summary">
        {unit.probeIds.length > 0 ? unit.probeIds.join(', ') : unit.scopeId}
        {#if unit.sourceIds.length > 0}
          · {unit.sourceIds.length === 1
            ? m.workbench_cell_group_sources_one({ count: unit.sourceIds.length })
            : m.workbench_cell_group_sources_other({ count: unit.sourceIds.length })}
        {/if}
      </span>
    </header>
  {:else if probeBadgeLabel}
    <!-- Phase 123c (B) — per-probe accent for the cross-probe split fan-out. The
         badge carries the probe's display label so the reader knows which probe
         each source-cell belongs to. -->
    <header class="cell-group-eyebrow">
      <span class="cell-group-badge">{probeBadgeLabel}</span>
      <span class="cell-group-summary">{unit.scopeId}</span>
    </header>
  {:else if unit.facetField && unit.facetValue !== undefined}
    <!-- Phase 125a — faceting / small-multiples. Each sub-cell is the same view
         restricted to one value of the facet field; the badge names that value
         so the grid reads as "<field> = <value>". -->
    <header class="cell-group-eyebrow">
      <span class="cell-group-badge">{unit.facetField}</span>
      <span class="cell-group-summary">{unit.facetValue}</span>
    </header>
  {/if}
  <Cell
    {ctx}
    scopeProbeId={unit.probeId ?? dossier.probeId}
    scope={unit.scope}
    scopeId={unit.scopeId}
    {windowStart}
    {windowEnd}
    metricName={cfg.metric}
    sources={sourcesForUnit(unit)}
    {dataLayer}
    probeIds={unit.probeIds.length > 1 ? [...unit.probeIds] : []}
    composition={panel.composition}
    bins={cfg.bins}
    topN={cfg.topN}
    channels={cfg.channels}
    metricSet={panel.metricSet}
    fieldChain={panel.fieldChain}
    metadataFilter={unit.facetField && unit.facetValue !== undefined
      ? { field: unit.facetField, value: unit.facetValue }
      : undefined}
    showBand={cfg.showBand}
    resolution={panel.resolution}
    normalization={panel.normalization}
    forceStrength={cfg.forceStrength}
    displayLanguage={cfg.displayLanguage}
    cellKey={unit.key}
    reportExtent={sharedAxisApplies ? reportExtent : undefined}
    sharedDomains={cellScale === 'shared' ? sharedDomains : undefined}
    axisScaleState={sharedAxisApplies ? cellScale : undefined}
    configOverridden={cfg.isOverridden}
    {selection}
  />
  {#if cfg.dimensionOverridden}
    <!-- ADR-038 / Phase 148f — this cell peeks a DIFFERENT dimension than the
         panel, so it is off-comparison. The (desaturated) banner marks that, and
         the cell carries its OWN "How to read" over its effective config (not the
         panel default). Cells that match the panel are covered by the panel guide. -->
    <p class="cell-peek-banner" role="note">
      {m.workbench_cell_peek_banner_pre()} <code>{metricLabel(cfg.metric)}</code>
      {m.workbench_cell_peek_banner_mid()}
      <code>{metricLabel(panel.metric)}</code>{m.workbench_cell_peek_banner_post()}
    </p>
    <ReadingGuide
      panel={cellPanel}
      {presentation}
      {ctx}
      {dossier}
      {windowStart}
      {windowEnd}
      variant="cell"
    />
  {/if}
</div>

<style>
  .panel-cell {
    min-height: 14rem;
    /* Phase 126 — anchor for the per-cell config popover. */
    position: relative;
  }

  /* Issue 8 (follow-up) — the cells' own header (title + CellExport export
     buttons) was a non-wrapping flex row, so at the split's narrow width the
     export controls overflowed the cell. */
  .panel-cell :global(.cell-header) {
    flex-wrap: wrap;
    gap: var(--space-2);
  }
  .panel-cell :global(.cell-title) {
    min-width: 0;
  }

  /* Phase 126 — per-cell config affordance. A normal-flow right-aligned bar
     above the cell content (never overlapping the cell's own export row), so
     the ⚙ and the "custom" marker stay clearly visible (operator request). */
  .cell-config-bar {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--space-2);
    margin-bottom: var(--space-1);
  }
  .cell-config-btn {
    appearance: none;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 1px var(--space-2);
    color: var(--color-fg-subtle);
    font-family: var(--font-mono);
    font-size: 10.5px;
    cursor: pointer;
  }
  .cell-config-btn:hover,
  .cell-config-btn:focus-visible {
    border-color: var(--color-accent);
    color: var(--color-fg);
  }
  .cell-config-btn.active {
    border-color: var(--color-accent);
    color: var(--color-accent);
    background: color-mix(in srgb, var(--color-accent) 10%, var(--color-surface));
  }
  .cell-custom-badge {
    padding: 1px var(--space-2);
    border-radius: var(--radius-pill);
    background: color-mix(in srgb, var(--color-accent) 18%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-accent) 55%, transparent);
    color: var(--color-accent);
    font-family: var(--font-mono);
    font-size: 10px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  /* A subtle outline accent on the whole cell when it carries an override, so
     the "this one is different" signal is legible even before reading the badge. */
  .panel-cell.cell-overridden {
    outline: 1px dashed color-mix(in srgb, var(--color-accent) 45%, transparent);
    outline-offset: 2px;
    border-radius: var(--radius-sm);
  }

  /* Phase 122k §11 finding — multi-group split panels need a visual identity per
     group so the user can read which Cell belongs to which ScopeGroup. Each group
     gets a subtle border-left tint and an eyebrow header with a "Group N" badge.
     The colour cycles through a four-tone palette consistent with the
     ScopeEditor's step accents. */
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
  /* Beyond four groups, fall back to a neutral accent so the layout stays calm. */
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

  /* ADR-038 — per-cell dimension-peek banner: loud, because it marks an
     off-comparison cell (distinct from the soft `custom` shape-override badge). */
  /* Phase 148f — desaturated: the warm-neutral "unvalidated" token (a muted gold)
     instead of the loud amber, so the off-comparison cell reads as a calm
     methodological note (still visible via the solid left border), not an alarm. */
  .cell-peek-banner {
    margin: var(--space-3) 0 0;
    padding: var(--space-2) var(--space-3);
    border: 1px solid color-mix(in srgb, var(--color-status-unvalidated) 45%, var(--color-border));
    border-left: 3px solid var(--color-status-unvalidated);
    border-radius: var(--radius-sm);
    background: color-mix(in srgb, var(--color-status-unvalidated) 8%, transparent);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
    line-height: var(--line-height-loose);
  }
  .cell-peek-banner code {
    font-family: var(--font-mono);
  }
  .cell-peek-banner code {
    font-family: var(--font-mono);
  }
</style>
