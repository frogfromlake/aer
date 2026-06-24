<script lang="ts">
  // Phase 141 — the Panel's cell grid (extracted from PanelHost). Owns everything
  // between the panel header and the panel methodology: probe-scope + facet
  // fan-out, lazy cell load, the at-scale co-occurrence branch, refusal/loading/
  // empty states, the shared-axis comparison union (Phase 124/126), and per-cell
  // config open-state. Units render via PanelCell children; soft disclosures sit
  // above in PanelDisclosureNotes; availability is threaded in as props (PanelHost).
  import type { Component } from 'svelte';
  import { m } from '$lib/paraglide/messages.js';
  import { fieldLabel } from '$lib/state/labels.svelte';
  import { createQuery } from '@tanstack/svelte-query';
  import type { PresentationDefinition, PresentationCellProps } from '$lib/presentations';
  import {
    probesQuery,
    metadataDistributionQuery,
    type FetchContext,
    type ProbeDossierDto,
    type ProbeDto,
    type QueryOutcome,
    type RefusalOutcome,
    type ScopeAvailableMetricsDto,
    type ScopeAvailableMetadataDto,
    type CategoricalDistributionResponseDto
  } from '$lib/api/queries';
  import {
    expandProbeScopeFanout,
    expandFacetFanout,
    MAX_FACET_CELLS,
    resolveCellConfig,
    selectCellRender,
    shouldRefuseMergedCrossProbe,
    type CellRenderUnit
  } from '$lib/workbench/panel-queries';
  import * as layout from '$lib/workbench/panel-host-layout';
  import type { Panel } from '$lib/state/url-internals';
  import type { PanelPath } from '$lib/workbench/panel-mutators';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import PanelCell from './PanelCell.svelte';
  import PanelDisclosureNotes from './PanelDisclosureNotes.svelte';

  interface Props {
    panel: Panel;
    dossier: ProbeDossierDto;
    ctx: FetchContext;
    presentation: PresentationDefinition;
    /** Interactive editing path (null disables per-cell config). */
    path: PanelPath | null;
    /** Inherited default window (the panel may override it per-panel). */
    windowStart: string | undefined;
    windowEnd: string | undefined;
    selection: PresentationCellProps['selection'];
    fieldDriven: boolean;
    // Shared availability (computed once by PanelHost from its queries).
    availFullData: ScopeAvailableMetricsDto | ScopeAvailableMetadataDto | null;
    dimensionSourceFilter: Set<string> | null;
    scalarMetricOptions: string[];
    droppedSources: string[];
    noSharedDimension: boolean;
  }

  let {
    panel,
    dossier,
    ctx,
    presentation,
    path,
    windowStart,
    windowEnd,
    selection,
    fieldDriven,
    availFullData,
    dimensionSourceFilter,
    scalarMetricOptions,
    droppedSources,
    noSharedDimension
  }: Props = $props();

  const cellRender = $derived(selectCellRender(panel));
  // Phase 151 — Silver only applies where the presentation supports it (else gold).
  const dataLayer = $derived<'gold' | 'silver'>(
    panel.layer === 'silver' && (presentation.supportsSilver ?? false) ? 'silver' : 'gold'
  );
  // Phase 122k F5 — the panel's own window overrides the inherited default.
  const effectiveWindowStart = $derived(panel.windowStart ?? windowStart);
  const effectiveWindowEnd = $derived(panel.windowEnd ?? windowEnd);
  // Phase 126 — per-cell config popover; holds the open cell's `unit.key` (single-open).
  let openConfigKey = $state<string | null>(null);
  function toggleCellConfig(key: string, e: MouseEvent) {
    e.stopPropagation();
    openConfigKey = openConfigKey === key ? null : key;
  }

  // Phase 123c (B) — cross-probe source resolution. A split panel whose
  // ScopeGroup spans several probes fans out EVERY probe's sources, so read the
  // app-wide probe registry (cached). The dossier still supplies the richer
  // per-source meta for its own probe; other probes' sources resolve by name.
  const probesQ = createQuery<QueryOutcome<ProbeDto[]>, Error, QueryOutcome<ProbeDto[]>>(() => {
    const o = probesQuery(ctx);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });
  const probesById = $derived.by<Record<string, ProbeDto>>(() => {
    const map: Record<string, ProbeDto> = {};
    if (probesQ.data?.kind === 'success') for (const p of probesQ.data.data) map[p.probeId] = p;
    return map;
  });
  // Source NAMES for a probe: the dossier (ordered) for its own probe, the registry
  // for the others; narrowed to dimension-carrying sources (ADR-038) — no empty cells.
  function sourceNamesForProbe(probeId: string): string[] {
    const all =
      probeId === dossier.probeId
        ? dossier.sources.map((s) => s.name)
        : (probesById[probeId]?.sources ?? []);
    return dimensionSourceFilter ? all.filter((n) => dimensionSourceFilter.has(n)) : all;
  }
  function probeLabelFor(probeId: string): string {
    const p = probesById[probeId];
    return p?.shortName ?? p?.displayName ?? probeId;
  }
  // 1-based per-probe accent index over the fan-out's stable probe order.
  function probeTintIndex(probeId: string, order: readonly string[]): number {
    const i = order.indexOf(probeId);
    return i < 0 ? 0 : i;
  }

  // Phase 130 / ADR-035 — merged-cross-probe guard (Brief §1.3): a merged cell
  // pooling >1 probe for a scaled metric would rank cross-context; refuse it.
  const crossProbeMergeRefused = $derived(shouldRefuseMergedCrossProbe(panel, cellRender));
  const crossProbeRefusal = $derived<RefusalOutcome | null>(
    crossProbeMergeRefused
      ? {
          kind: 'refusal',
          refusalKind: 'merged_cross_probe_unsupported',
          message: m.workbench_grid_merged_cross_probe_refusal({ metric: panel.metric }),
          httpStatus: 422
        }
      : null
  );

  // Phase 123c (B) — the fan-out spans EVERY in-scope probe. The pure
  // `expandProbeScopeFanout` builds per-(probe,source) units from the
  // host-resolved source names; the distinct probe order drives the accent.
  const fanout = $derived(
    expandProbeScopeFanout(panel, cellRender, sourceNamesForProbe, dossier.probeId)
  );

  // Phase 125a — faceting / small-multiples. When the panel names a facetField,
  // enumerate that categorical field's values over the panel's scope and render
  // one sub-cell per value (each restricted via a `metadataFilter`). Faceting
  // requires the panel to resolve to a SINGLE base scope unit; a panel that
  // splits into >1 base unit is NOT faceted (would silently drop sources) — we
  // disclose that instead via PanelDisclosureNotes.
  const facetField = $derived(panel.facetField?.trim() ?? '');
  const facetSupported = $derived(presentation.supportsFaceting ?? false);
  const facetSingleScope = $derived(cellRender.units.length === 1);
  const facetBaseUnit = $derived<CellRenderUnit | null>(
    facetSingleScope ? (cellRender.units[0] ?? null) : null
  );
  const facetActive = $derived(facetField.length > 0 && facetSupported && facetBaseUnit !== null);
  const facetUnavailable = $derived(facetField.length > 0 && facetSupported && !facetSingleScope);
  const facetValuesQ = createQuery<
    QueryOutcome<CategoricalDistributionResponseDto>,
    Error,
    QueryOutcome<CategoricalDistributionResponseDto>
  >(() => {
    const base = facetBaseUnit;
    const o = metadataDistributionQuery(ctx, facetField || 'section', {
      scope: base?.scope ?? 'probe',
      scopeId: base?.scopeId ?? '',
      start: panel.windowStart ?? windowStart,
      end: panel.windowEnd ?? windowEnd,
      topN: MAX_FACET_CELLS
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: facetActive
    };
  });
  const facetFanout = $derived.by(() => {
    if (!facetActive || !facetBaseUnit) return null;
    if (facetValuesQ.data?.kind !== 'success') return null;
    const data = facetValuesQ.data.data;
    const values = data.categories.map((c) => c.value);
    return expandFacetFanout(panel, values, data.distinctValues ?? values.length, facetBaseUnit);
  });

  // When faceting is active the grid is driven SOLELY by the facet fan-out — we
  // never silently fall back to the unfaceted cell. While values load (or none
  // exist) the unit list is empty and the template shows a status note instead.
  const expandedUnits = $derived<readonly CellRenderUnit[]>(
    facetActive ? (facetFanout?.units ?? []) : (fanout?.units ?? cellRender.units)
  );
  const facetPending = $derived(facetActive && facetValuesQ.isPending);
  const facetEmpty = $derived(
    facetActive && facetValuesQ.data?.kind === 'success' && facetFanout === null
  );
  const fanoutProbeOrder = $derived<readonly string[]>(fanout?.probeOrder ?? []);
  const isMultiProbeFanout = $derived(fanoutProbeOrder.length > 1);

  // Phase 148g — the WebGL (sigma) renderer is now the SINGLE co-occurrence
  // renderer for every node/edge count: it scales to 10000 nodes, carries the
  // legend + per-source/probe borders + cursor tooltip, and removes the old
  // SVG↔WebGL divergence. It engages on any single co-occurrence cell (the SVG
  // CoOccurrenceNetworkCell path is retired). Lazy-loaded.
  const atScaleActive = $derived(
    (presentation.supportsAtScale ?? false) && expandedUnits.length === 1 && !facetActive
  );
  let AtScaleComponent = $state<Component<PresentationCellProps> | null>(null);
  $effect(() => {
    if (!atScaleActive || AtScaleComponent) return;
    void import('$lib/components/presentations/CoOccurrenceNetworkAtScale.svelte').then((mod) => {
      AtScaleComponent = mod.default;
    });
  });

  // Co-occurrence redesign — the network is ONE relational graph, not a small-
  // multiple grid. A split fan-out to >1 cell is refused (Merged pools every
  // source into one graph; a single-source split is also one cell). Count-based,
  // so no cross-frame equivalence gate applies.
  const cooccurrenceMultiCellRefused = $derived(
    panel.view === 'cooccurrence_network' && expandedUnits.length > 1
  );
  const cooccurrenceRefusal = $derived<RefusalOutcome | null>(
    cooccurrenceMultiCellRefused
      ? {
          kind: 'refusal',
          refusalKind: 'unspecified',
          message: m.workbench_grid_cooccurrence_refusal(),
          httpStatus: 422
        }
      : null
  );

  // Phase 126 — per-cell overrides are offered only when the panel renders more
  // than one cell, the presentation exposes a cell-shape lever, and it is
  // interactive.
  const perCellConfig = $derived(
    path !== null && expandedUnits.length > 1 && (presentation.configurableParams?.length ?? 0) > 0
  );

  // Lazy-load the Cell component. The same instance is reused across all units.
  let CellComponent = $state<Component<PresentationCellProps> | null>(null);
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
        loadError = err instanceof Error ? err.message : m.errors_generic();
      });
  });

  // Source resolution for the at-scale branch (PanelCell resolves its own).
  function sourcesForUnit(unit: CellRenderUnit) {
    if (unit.sourceIds.length === 0) {
      return dossier.sources.map((s) => ({ name: s.name, emicDesignation: s.emicDesignation }));
    }
    return unit.sourceIds.map((id) => {
      const match = dossier.sources.find((s) => s.name === id);
      return { name: id, emicDesignation: match?.emicDesignation ?? null };
    });
  }

  // Phase 131 (bugfix) — the split LAYOUT engages whenever the panel renders >1
  // cell under `split`, including the probe-scope fan-out where selectCellRender
  // returns `merged-single` and the host expands per source. Faceting always
  // renders a small-multiples grid regardless of the composition toggle.
  const isSplitLayout = $derived(
    (panel.composition === 'split' || facetActive) && expandedUnits.length > 1
  );

  // ── Shared-axis comparison discipline (Phase 124 / 126) ──────────────────────
  // When a panel renders >1 cell of a value-axis presentation the cells must be
  // directly comparable: each reports its extent, the grid unions them and hands
  // back `sharedDomains`. Cross-context + intensive metric keeps INDEPENDENT axes
  // (needs a metric_equivalence grant — none exists yet); a panel-level 'free'
  // escape hatch overrides shared for readability.
  const SHARED_AXIS_VIEWS: ReadonlySet<string> = new Set([
    'distribution',
    'time_series',
    'metric_scatter'
  ]);
  const sharedAxisApplies = $derived(SHARED_AXIS_VIEWS.has(panel.view) && expandedUnits.length > 1);
  const renderedProbeCount = $derived(layout.renderedProbeCount(expandedUnits));
  const isIntensiveMetric = $derived(
    layout.isIntensiveMetric({
      view: panel.view,
      channels: panel.channels,
      metric: panel.metric,
      presentationUsesMetric: presentation.usesMetric
    })
  );
  const shareForbidden = $derived(sharedAxisApplies && renderedProbeCount > 1 && isIntensiveMetric);

  function effectiveCellScale(cellKey: string): 'shared' | 'free' {
    return layout.effectiveCellScale({
      cellKey,
      shareForbidden,
      cellOverrides: panel.cellOverrides,
      panelScales: panel.scales
    });
  }
  const sharedCellKeys = $derived(
    layout.computeSharedCellKeys({
      sharedAxisApplies,
      shareForbidden,
      units: expandedUnits,
      cellOverrides: panel.cellOverrides,
      panelScales: panel.scales
    })
  );
  const computeShared = $derived(sharedAxisApplies && Object.keys(sharedCellKeys).length > 0);

  // Per-rendered-cell extents, keyed `${cellKey}|${axis}`. A plain Record
  // (reassigned per update) drives reactivity without a mutable Map.
  let reportedExtents = $state<Record<string, readonly [number, number]>>({});
  function reportExtentFor(cellKey: string) {
    return (axis: 'value' | 'x', extent: readonly [number, number] | null) => {
      const k = `${cellKey}|${axis}`;
      if (extent === null) {
        if (!(k in reportedExtents)) return;
        const next = { ...reportedExtents };
        delete next[k];
        reportedExtents = next;
        return;
      }
      const prev = reportedExtents[k];
      if (prev && prev[0] === extent[0] && prev[1] === extent[1]) return;
      reportedExtents = { ...reportedExtents, [k]: [extent[0], extent[1]] };
    };
  }
  const sharedDomains = $derived(
    layout.computeSharedDomains({ computeShared, reportedExtents, sharedCellKeys })
  );

  // Prune extents for cells no longer rendered (split→merged toggle, scope
  // narrowing, fan-out change) so a removed cell's extent stops widening the
  // shared axis. Guarded so it only reassigns when something is actually stale.
  $effect(() => {
    const liveKeys: Record<string, true> = {};
    for (const u of expandedUnits) liveKeys[u.key] = true;
    let stale = false;
    for (const k of Object.keys(reportedExtents)) {
      if (!(k.slice(0, k.lastIndexOf('|')) in liveKeys)) {
        stale = true;
        break;
      }
    }
    if (!stale) return;
    const next: Record<string, readonly [number, number]> = {};
    for (const [k, v] of Object.entries(reportedExtents)) {
      if (k.slice(0, k.lastIndexOf('|')) in liveKeys) next[k] = v;
    }
    reportedExtents = next;
  });

  // Phase 125b — ISSUE 3: surface the cross-panel linked-brushing affordance on
  // the two participating views whenever a Window selection is wired.
  const showBrushHint = $derived(
    (panel.view === 'metric_scatter' || panel.view === 'parallel_coordinates') && !!selection
  );
</script>

<PanelDisclosureNotes
  {shareForbidden}
  metric={panel.metric}
  {droppedSources}
  {noSharedDimension}
  {facetUnavailable}
  {facetField}
  {showBrushHint}
/>

<div
  class="panel-body"
  class:split={isSplitLayout}
  data-split-direction={panel.splitDirection ?? 'horizontal'}
>
  {#if cooccurrenceRefusal}
    <RefusalSurface refusal={cooccurrenceRefusal} {ctx} />
  {:else if crossProbeRefusal}
    <RefusalSurface refusal={crossProbeRefusal} {ctx} />
  {:else if atScaleActive}
    <!-- Phase 125b — single-cell co-occurrence above the threshold → large-scale
         WebGL renderer (sigma.js). Handles its own loading / refusal internally. -->
    {#if AtScaleComponent && expandedUnits[0]}
      {@const AtScale = AtScaleComponent}
      {@const unit = expandedUnits[0]}
      {@const cfg = resolveCellConfig(panel, unit.key)}
      <div class="panel-cell">
        <AtScale
          {ctx}
          scopeProbeId={unit.probeId ?? dossier.probeId}
          scope={unit.scope}
          scopeId={unit.scopeId}
          windowStart={effectiveWindowStart}
          windowEnd={effectiveWindowEnd}
          metricName={cfg.metric}
          sources={sourcesForUnit(unit)}
          probeIds={unit.probeIds.length > 1 ? [...unit.probeIds] : []}
          sourceIds={[...unit.sourceIds]}
          {dataLayer}
          topN={cfg.topN}
          maxNodes={cfg.maxNodes}
          forceStrength={cfg.forceStrength}
          showEdges={cfg.showEdges}
          showLabels={cfg.showLabels}
          settleSeconds={cfg.settleSeconds}
          channels={cfg.channels}
          displayLanguage={cfg.displayLanguage}
          configOverridden={cfg.isOverridden}
        />
      </div>
    {:else}
      <p class="muted" aria-busy="true">{m.workbench_grid_loading_at_scale()}</p>
    {/if}
  {:else if loadError}
    <p class="muted">{m.workbench_grid_cell_failed({ error: loadError })}</p>
  {:else if !CellComponent}
    <p class="muted" aria-busy="true">
      {m.workbench_grid_loading_presentation({ presentation: presentation.label })}
    </p>
  {:else if noSharedDimension}
    <p class="muted">
      {m.workbench_grid_no_shared_dimension_pre()}
      <strong>{m.workbench_grid_no_shared_dimension_ways()}</strong>
      {m.workbench_grid_no_shared_dimension_action()}
      <strong>{m.workbench_grid_no_shared_dimension_show_anyway()}</strong>
      {m.workbench_grid_no_shared_dimension_post()}
    </p>
  {:else if facetPending}
    <p class="muted" aria-busy="true">
      {m.workbench_grid_loading_facet_values()} <code>{fieldLabel(facetField)}</code>…
    </p>
  {:else if facetEmpty}
    <p class="muted">
      {m.workbench_grid_facet_empty_pre()} <code>{fieldLabel(facetField)}</code>
      {m.workbench_grid_facet_empty_post()}
    </p>
  {:else if expandedUnits.length === 0}
    <p class="muted">{m.workbench_grid_no_sources()}</p>
  {:else}
    {#if facetFanout}
      {@const fp = { shown: facetFanout.units.length, total: facetFanout.totalValues }}
      {@const showing =
        facetFanout.totalValues === 1
          ? m.workbench_grid_facet_disclosure_showing_one(fp)
          : m.workbench_grid_facet_disclosure_showing_other(fp)}
      {@const cap = facetFanout.capped
        ? ` ${m.workbench_grid_facet_disclosure_capped({ max: MAX_FACET_CELLS })}`
        : ''}
      <p class="facet-disclosure" role="note">
        {m.workbench_grid_facet_disclosure_pre()}
        <code>{fieldLabel(facetFanout.field)}</code
        >{showing}{cap}{m.workbench_grid_facet_disclosure_post()}
      </p>
    {/if}
    {#each expandedUnits as unit (unit.key)}
      {@const groupNum = unit.groupIndex !== undefined ? unit.groupIndex + 1 : null}
      {@const probeNum =
        unit.probeId !== undefined && isMultiProbeFanout
          ? probeTintIndex(unit.probeId, fanoutProbeOrder) + 1
          : null}
      {@const accentNum = groupNum ?? probeNum}
      <PanelCell
        {unit}
        {panel}
        {dossier}
        {ctx}
        {presentation}
        {CellComponent}
        {path}
        {dataLayer}
        windowStart={effectiveWindowStart}
        windowEnd={effectiveWindowEnd}
        {selection}
        {accentNum}
        {groupNum}
        probeBadgeLabel={probeNum !== null && unit.probeId ? probeLabelFor(unit.probeId) : null}
        {perCellConfig}
        isConfigOpen={openConfigKey === unit.key}
        onToggleConfig={(e) => toggleCellConfig(unit.key, e)}
        onCloseConfig={() => (openConfigKey = null)}
        {availFullData}
        {fieldDriven}
        {scalarMetricOptions}
        {sharedAxisApplies}
        cellScale={effectiveCellScale(unit.key)}
        {sharedDomains}
        reportExtent={sharedAxisApplies ? reportExtentFor(unit.key) : undefined}
      />
    {/each}
  {/if}
</div>

<style>
  .panel-body {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  /* Phase 122k §14b — horizontal split forces side-by-side equal-width cells
     regardless of panel width (auto-fit/minmax collapsed to one column when the
     panel was narrower than 40rem, making horizontal look identical to vertical).
     Issue 8 — cap the horizontal split at TWO cells per row and wrap the rest as
     a 2×2 grid so each cell stays wide enough for its title and all four cross-
     probe cells stay visible at once. */
  .panel-body.split[data-split-direction='horizontal'] {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--space-4);
  }
  .panel-body.split[data-split-direction='horizontal'] > :global(.panel-cell) {
    /* Grid track already bounds the width; allow the cell to shrink to it. */
    min-width: 0;
  }

  .panel-body.split[data-split-direction='vertical'] {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  /* The at-scale branch renders its own .panel-cell wrapper inline (PanelCell
     styles the per-unit cells). */
  .panel-cell {
    min-height: 14rem;
    position: relative;
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  /* Phase 125a — faceting disclosure above the small-multiples grid. */
  .facet-disclosure {
    grid-column: 1 / -1;
    margin: 0 0 var(--space-2);
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    background: var(--color-surface-2, var(--color-surface));
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted, var(--color-fg));
    line-height: var(--line-height-loose);
  }
  .facet-disclosure code {
    font-family: var(--font-mono);
  }
</style>
