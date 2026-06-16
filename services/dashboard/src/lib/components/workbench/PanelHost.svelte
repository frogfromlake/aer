<script lang="ts">
  // Phase 122i / ADR-034 — Multi-Panel Workbench: Panel renderer.
  //
  // A Panel is a `(scopes[], composition, view, metric, layer, …)` tuple from the
  // URL state. PanelHost is the orchestrator: it owns the panel chrome (header,
  // meta strip, scope chips, the focused PanelControls strip, the scope editor)
  // and the SHARED dimension-availability queries, then hands the actual cell
  // rendering to PanelCellGrid. The cell grid, each rendered cell, the soft
  // disclosures, the header, and the scope chips each live in their own child
  // component (Phase 141 decomposition).
  import { createQuery } from '@tanstack/svelte-query';
  import {
    getPresentation,
    type PresentationDefinition,
    type PresentationCellProps
  } from '$lib/presentations';
  import type { Panel, PillarId, ScopeGroup } from '$lib/state/url-internals';
  import {
    scopeAvailableMetricsQuery,
    scopeAvailableMetadataQuery,
    type FetchContext,
    type ProbeDossierDto,
    type QueryOutcome,
    type ScopeAvailableMetricsDto,
    type ScopeAvailableMetadataDto
  } from '$lib/api/queries';
  import { availabilityScope } from '$lib/workbench/panel-queries';
  // Phase 141 — pure availability/layout logic (unit-tested in
  // tests/unit/panel-host-layout.test.ts). The component reads the reactive
  // inputs (query data, panel) and passes them in.
  import * as layout from '$lib/workbench/panel-host-layout';
  import {
    focusPanel,
    removePanel,
    toggleMaximizedPanel,
    updatePanel
  } from '$lib/workbench/panel-mutators';
  import type { DiscourseFunction } from '$lib/discourse-function';
  import CellMethodology from './CellMethodology.svelte';
  import PanelControls from './PanelControls.svelte';
  import PanelMetaStrip from './PanelMetaStrip.svelte';
  import PanelToolbar from './PanelToolbar.svelte';
  import PanelScopeChips from './PanelScopeChips.svelte';
  import PanelCellGrid from './PanelCellGrid.svelte';
  import ScopeEditor from './ScopeEditor.svelte';

  interface Props {
    panel: Panel;
    dossier: ProbeDossierDto;
    ctx: FetchContext;
    windowStart: string | undefined;
    windowEnd: string | undefined;
    /** Phase 122i — when set, the host enables interactive editing
     *  (focus on click, PanelControls, +Compare, ×Remove). Absent for
     *  legacy/preview rendering paths. */
    pillar?: PillarId;
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
     *  panel (nothing to compare against, no minimised tray would appear). */
    canMaximize?: boolean;
    /** Phase 125b — Window-level cross-panel brushing selection, threaded to
     *  every cell. Per-article cells (scatter, parallel) use it; others ignore. */
    selection?: PresentationCellProps['selection'];
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
    canMaximize = false,
    selection
  }: Props = $props();

  const path = $derived(
    pillar !== undefined && windowIndex !== undefined && panelIndex !== undefined
      ? { pillar, windowIndex, panelIndex }
      : null
  );
  const isInteractive = $derived(path !== null);

  // Phase 122i revision (D3). `⚙ Edit scope` opens the ScopeEditor where the user
  // picks sources for the new group.
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
  function onToggleMaximize(e: MouseEvent) {
    e.stopPropagation();
    if (!path) return;
    toggleMaximizedPanel(path.pillar, path.windowIndex, path.panelIndex);
  }

  const presentation = $derived<PresentationDefinition>(getPresentation(panel.view));

  // ── Shared dimension availability (ADR-038) ──────────────────────────────────
  // PanelHost owns the availability queries (TanStack must live in a component)
  // and the derived availability split; PanelCellGrid + PanelCell consume the
  // resolved data as props. The panel's dimension is a Gold metric for metric
  // views and a categorical FIELD for field-driven views; `fieldDriven` routes
  // the filter (and the dropped-source note) to the matching endpoint, so a
  // source lacking the chosen FIELD is dropped exactly like one lacking a metric.
  const panelScopeUnion = $derived(availabilityScope(panel.scopes));
  const panelHasScope = $derived(
    panelScopeUnion.probeIds.length > 0 || panelScopeUnion.sourceIds.length > 0
  );
  const fieldDriven = $derived(presentation.usesMetadataField ?? false);
  const metricAvailQ = createQuery<
    QueryOutcome<ScopeAvailableMetricsDto>,
    Error,
    QueryOutcome<ScopeAvailableMetricsDto>
  >(() => {
    const o = scopeAvailableMetricsQuery(ctx, {
      probeIds: panelScopeUnion.probeIds,
      sourceIds: panelScopeUnion.sourceIds,
      start: panel.windowStart ?? windowStart,
      end: panel.windowEnd ?? windowEnd
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: panelHasScope && !fieldDriven
    };
  });
  const metadataAvailQ = createQuery<
    QueryOutcome<ScopeAvailableMetadataDto>,
    Error,
    QueryOutcome<ScopeAvailableMetadataDto>
  >(() => {
    const o = scopeAvailableMetadataQuery(ctx, {
      probeIds: panelScopeUnion.probeIds,
      sourceIds: panelScopeUnion.sourceIds,
      start: panel.windowStart ?? windowStart,
      end: panel.windowEnd ?? windowEnd
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: panelHasScope && fieldDriven
    };
  });
  // The active dimension's availability split (available / partial / scoped) from
  // whichever endpoint matches the view.
  const dimensionAvail = $derived.by(() => {
    const q = fieldDriven ? metadataAvailQ : metricAvailQ;
    const d = q.data?.kind === 'success' ? q.data.data : null;
    return layout.extractDimensionAvail(d, fieldDriven, panel.metric);
  });
  // `null` = no narrowing; a Set = render only these sources for the dimension.
  const dimensionSourceFilter = $derived(
    layout.dimensionSourceFilter(dimensionAvail, panel.metric)
  );
  // ADR-038 — scoped sources dropped because they lack the active dimension.
  const droppedSources = $derived(layout.droppedSources(dimensionAvail));
  // ADR-038 — field-driven panel whose chosen dimension is empty/unshared.
  const noSharedDimension = $derived(fieldDriven && panelHasScope && !panel.metric);
  // ADR-038 — the raw availability payload for the active dimension KIND, for the
  // per-cell dimension-peek menu.
  const availFullData = $derived.by<ScopeAvailableMetricsDto | ScopeAvailableMetadataDto | null>(
    () => {
      const q = fieldDriven ? metadataAvailQ : metricAvailQ;
      return q.data?.kind === 'success' ? q.data.data : null;
    }
  );
  // Scalar metric options for the per-cell scatter-axis pickers (all-source
  // intersection, default-prepended).
  const scalarMetricOptions = $derived(
    layout.scalarMetricOptionsFromAvailable(
      metricAvailQ.data?.kind === 'success' ? metricAvailQ.data.data.available : []
    )
  );
</script>

<!--
  Phase 122i — the Panel-host is an `<article>` for semantic structure (each
  Panel is a self-contained analytical unit). Click-anywhere-to-focus is
  implemented on the article itself; the svelte-ignore is intentional —
  switching to a `<div>`/wrapper `<button>` would lose the semantic, and the
  keyboard handler satisfies the actual a11y requirement.
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
  <PanelToolbar
    {presentation}
    {panel}
    {isInteractive}
    {canMaximize}
    {isMaximized}
    {canRemove}
    onEditScope={onAddCompare}
    {onToggleMaximize}
    {onRemove}
  />

  {#if path}
    <PanelMetaStrip
      {panel}
      {dossier}
      panelPath={path}
      {ctx}
      onEditScope={() => (scopeEditorOpen = true)}
    />
  {/if}

  {#if focused && path}
    <PanelControls pillar={path.pillar} panelPath={path} />
  {/if}

  {#if path}
    <PanelScopeChips {panel} panelPath={path} />
  {/if}

  <PanelCellGrid
    {panel}
    {dossier}
    {ctx}
    {presentation}
    {path}
    {windowStart}
    {windowEnd}
    {selection}
    {fieldDriven}
    {availFullData}
    {dimensionSourceFilter}
    {scalarMetricOptions}
    {droppedSources}
    {noSharedDimension}
  />

  <!-- Phase 135 — panel-level methodology (provenance + view-mode notes) for the
       panel's own metric + presentation. One per panel; cells that override the
       metric carry their own. Collapsed by default. -->
  <CellMethodology
    metricName={panel.metric}
    viewMode={presentation.id}
    viewLabel={presentation.label}
  />
</article>

{#if scopeEditorOpen && path}
  <ScopeEditor
    {panel}
    {dossier}
    {ctx}
    onApply={(scopes: ScopeGroup[], lockedFunction: DiscourseFunction | null) => {
      // Commit draft state to the Panel via the mutator. The mutator respects the
      // `locked` guard (scope edits are gated when the panel is DF-locked), but in
      // 122k the lock is set BY the editor itself via the DF-lock dropdown, so we
      // update both scopes and lockedFunction atomically.
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
</style>
