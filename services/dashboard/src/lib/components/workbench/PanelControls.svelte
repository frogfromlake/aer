<script lang="ts">
  // PanelControls — the per-Panel control strip (the lever surface of the
  // Presentation × Metric matrix). Exposes Composition, View, Metric, Group-by,
  // Resolution, per-cell Config, Window, Layer, and Compare.
  //
  // Phase 141 — decomposed into per-lever child components under ./levers/. This
  // parent is the thin orchestrator: it owns the three shared availability
  // queries (/metrics/available, /scope/available-metrics,
  // /scope/available-metadata) and the derivations every lever shares, and
  // passes them down as props. Each child carries its own state reads, handlers,
  // markup, and CSS; the shared row/button styling lives in ./levers/LeverRow +
  // LeverButton. The strip is panel-bound — PanelHost mounts it only for the
  // focused panel, so an unbound (legacy) render shows no levers.
  import { createQuery } from '@tanstack/svelte-query';
  import { m } from '$lib/paraglide/messages.js';
  import {
    metricsAvailableQuery,
    scopeAvailableMetricsQuery,
    scopeAvailableMetadataQuery,
    type AvailableMetricDto,
    type FetchContext,
    type QueryOutcome,
    type ScopeAvailableMetricsDto,
    type ScopeAvailableMetadataDto
  } from '$lib/api/queries';
  import { urlState } from '$lib/state/url.svelte';
  import { presentationsForPillar, resolvePresentation } from '$lib/presentations';
  import { DEFAULT_LOOKBACK_MS, type PillarId } from '$lib/state/url-internals';
  import { updatePanel, type PanelPath } from '$lib/workbench/panel-mutators';
  import { availabilityScope } from '$lib/workbench/panel-queries';
  // Phase 141 — shared pure derivations (unit-tested in panel-controls-derive).
  import {
    computeWindowBounds,
    toDateWindow,
    toWindowIso,
    offerableMetadataFields as offerableMetadataFieldsOf,
    buildScalarMetricOptions,
    type ScopeGate
  } from '$lib/workbench/panel-controls-derive';
  // Phase 141 — per-lever child components.
  import CompositionControls from './levers/CompositionControls.svelte';
  import ViewControls from './levers/ViewControls.svelte';
  import MetricControls from './levers/MetricControls.svelte';
  import MetricHints from './levers/MetricHints.svelte';
  import ResolutionControls from './levers/ResolutionControls.svelte';
  import ConfigValueLevers from './levers/ConfigValueLevers.svelte';
  import ConfigChannelLevers from './levers/ConfigChannelLevers.svelte';
  import WindowControls from './levers/WindowControls.svelte';
  import LayerCompareControls from './levers/LayerCompareControls.svelte';

  interface Props {
    pillar: PillarId;
    /** Binds the controls to the addressed Panel; the pillar prop must match. */
    panelPath?: PanelPath;
  }

  let { pillar, panelPath }: Props = $props();

  const ctx: FetchContext = { baseUrl: '/api/v1' };
  const url = $derived(urlState());

  // Resolve the addressed Panel; null when the path is stale (e.g. just removed)
  // so the strip renders nothing for one frame instead of crashing.
  const boundPanel = $derived.by(() => {
    if (!panelPath) return null;
    return (
      url.pillars?.[panelPath.pillar]?.windows[panelPath.windowIndex]?.panels[
        panelPath.panelIndex
      ] ?? null
    );
  });
  const isPanelLocked = $derived(boundPanel?.locked === true);
  const isCollapsed = $derived(boundPanel?.cellControlsCollapsed === true);

  // ---- Window bounds (Phase 122k F5) -------------------------------------
  // Snapshot "now" ONCE per mount. Reading `Date.now()` inside the derived made
  // the Episteme default window's `end` advance on EVERY reactive re-eval — and
  // `windowBounds` re-runs on any `url` change (opening a global overlay, or any
  // panel-control write to the URL). That produced fresh windowIso strings →
  // new scope-availability query keys → those queries refetched → `partialMetrics`
  // / `partialMetadataFields` briefly emptied → the WITHHELD hint unmounted and
  // remounted, jumping the panel controls + cell up and down. A stable snapshot
  // keeps the window strings (and thus the query keys) fixed across re-evals.
  // (Mirrors the same fix in EpistemeShell; Aleph/Rhizome are unbounded so they
  // never read `now` and were unaffected — hence the bug was Episteme-only.)
  const nowAtInit = Date.now();
  // Per-Panel window override, else the global default; Episteme defaults to a
  // disclosed recent window, Aleph/Rhizome stay unbounded.
  const windowBounds = $derived.by(() =>
    computeWindowBounds({
      panelStart: boundPanel?.windowStart,
      panelEnd: boundPanel?.windowEnd,
      urlFrom: url.from,
      urlTo: url.to,
      isEpisteme: pillar === 'episteme',
      now: nowAtInit,
      lookbackMs: DEFAULT_LOOKBACK_MS
    })
  );
  const dateWindow = $derived(toDateWindow(windowBounds));
  const windowIso = $derived(toWindowIso(windowBounds));
  const todayStr = new Date().toISOString().slice(0, 10);

  // ---- Metric availability (/metrics/available) --------------------------
  const availQ = createQuery<
    QueryOutcome<AvailableMetricDto[]>,
    Error,
    QueryOutcome<AvailableMetricDto[]>
  >(() => {
    const o = metricsAvailableQuery(ctx, dateWindow);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });
  const availableMetricNames = $derived<string[]>(
    availQ.data?.kind === 'success' ? availQ.data.data.map((md) => md.metricName) : []
  );

  // ---- Scope metric availability (Phase 123c C1 / ADR-038) ---------------
  // The metric pickers offer only metrics present for EVERY scoped source (the
  // intersection); partials are surfaced as a hint. FILTER semantics (a group
  // naming sources scopes to those only), mirroring the render fan-out.
  const panelScope = $derived(availabilityScope(boundPanel?.scopes ?? []));
  const hasScope = $derived(panelScope.probeIds.length > 0 || panelScope.sourceIds.length > 0);
  const scopeAvailQ = createQuery<
    QueryOutcome<ScopeAvailableMetricsDto>,
    Error,
    QueryOutcome<ScopeAvailableMetricsDto>
  >(() => {
    const o = scopeAvailableMetricsQuery(ctx, {
      probeIds: panelScope.probeIds,
      sourceIds: panelScope.sourceIds,
      start: windowIso.start,
      end: windowIso.end
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: hasScope
    };
  });
  const scopeAvail = $derived<ScopeAvailableMetricsDto | null>(
    hasScope && scopeAvailQ.data?.kind === 'success' ? scopeAvailQ.data.data : null
  );
  const scopeAvailableSet = $derived<Set<string> | null>(
    scopeAvail ? new Set(scopeAvail.available) : null
  );
  const partialMetrics = $derived<ScopeAvailableMetricsDto['partial']>(scopeAvail?.partial ?? []);
  const partialMetricSet = $derived<Set<string>>(new Set(partialMetrics.map((p) => p.metricName)));
  const scopedSourceNames = $derived<readonly string[]>(scopeAvail?.scopedSources ?? []);
  const scopedSourceCount = $derived(scopedSourceNames.length);
  const activeShowWithheld = $derived(boundPanel?.showWithheld === true);
  // The scope-availability gate (ADR-038): no scope yet → all; else the all-source
  // intersection, plus partials only under "show anyway".
  const scopeGate = $derived<ScopeGate>({
    scopeAvailableSet,
    partialMetricSet,
    showWithheld: activeShowWithheld
  });

  // ---- Scope metadata-field availability (Phase 133) ---------------------
  const metadataAvailQ = createQuery<
    QueryOutcome<ScopeAvailableMetadataDto>,
    Error,
    QueryOutcome<ScopeAvailableMetadataDto>
  >(() => {
    const o = scopeAvailableMetadataQuery(ctx, {
      probeIds: panelScope.probeIds,
      sourceIds: panelScope.sourceIds,
      start: windowIso.start,
      end: windowIso.end
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: hasScope
    };
  });
  const metadataAvail = $derived<ScopeAvailableMetadataDto | null>(
    hasScope && metadataAvailQ.data?.kind === 'success' ? metadataAvailQ.data.data : null
  );
  const availableMetadataFields = $derived<readonly string[]>(metadataAvail?.available ?? []);
  const partialMetadataFields = $derived<ScopeAvailableMetadataDto['partial']>(
    metadataAvail?.partial ?? []
  );
  // Offerable categorical fields (shared across Metric / View / Config levers).
  const offerableFields = $derived<string[]>(
    offerableMetadataFieldsOf({
      availableMetadataFields,
      partialMetadataFields,
      showWithheld: activeShowWithheld
    })
  );

  // ---- Presentation + per-view capability flags --------------------------
  const presentations = $derived(presentationsForPillar(pillar));
  const activePresentation = $derived(resolvePresentation(boundPanel?.view ?? null, pillar));
  const viewUsesMetric = $derived(activePresentation.usesMetric ?? true);
  const viewUsesMetadataField = $derived(activePresentation.usesMetadataField ?? false);
  const viewUsesResolution = $derived(activePresentation.usesResolution ?? false);
  const viewUsesNormalization = $derived(activePresentation.usesNormalization ?? false);
  const viewSupportsFaceting = $derived(activePresentation.supportsFaceting ?? false);
  // Phase 151 — the Au-Gold / Ag-Silver layer lever only does anything where the
  // presentation has a Silver query path (distribution); hide it elsewhere.
  const viewSupportsSilver = $derived(activePresentation.supportsSilver ?? false);
  const configParams = $derived(activePresentation.configurableParams ?? []);

  // Compare gate (Phase 131 BUG1): deviation/percentile need a deviation/absolute
  // equivalence grant; read it from /metrics/available so the buttons disable.
  const metricEquivalenceLevel = $derived.by<string | null>(() => {
    if (availQ.data?.kind !== 'success') return null;
    const md = availQ.data.data.find((x) => x.metricName === (boundPanel?.metric ?? ''));
    return md?.equivalenceStatus?.level ?? md?.equivalenceLevel ?? null;
  });
  const canNormalize = $derived(
    metricEquivalenceLevel === 'deviation' || metricEquivalenceLevel === 'absolute'
  );

  // Scalar-metric options for the channel / scatter pickers (shared).
  const scalarMetricOptions = $derived.by<string[]>(() =>
    buildScalarMetricOptions({
      availableMetricNames,
      gate: scopeGate,
      activeChannels: boundPanel?.channels ?? {}
    })
  );

  function toggleCollapsed() {
    if (!panelPath || !boundPanel) return;
    const next = !(boundPanel.cellControlsCollapsed === true);
    updatePanel(panelPath, (p) => ({ ...p, cellControlsCollapsed: next }));
  }
</script>

<section
  class="cell-controls"
  aria-label={m.workbench_controls_aria_label()}
  class:locked={isPanelLocked}
  class:collapsed={isCollapsed}
>
  {#if panelPath && boundPanel}
    <!-- Full-header click toggles collapse; always rendered so a collapsed
         strip can be re-opened. -->
    <button
      type="button"
      class="cell-controls-header"
      aria-expanded={!isCollapsed}
      aria-label={isCollapsed ? m.workbench_controls_expand() : m.workbench_controls_collapse()}
      onclick={toggleCollapsed}
    >
      {#if isPanelLocked}
        <span class="locked-banner" role="status">
          {m.workbench_controls_locked_pre()}
          <strong>{boundPanel.lockedFunction ?? m.workbench_controls_locked_fallback()}</strong
          >{m.workbench_controls_locked_post()}
        </span>
      {:else}
        <span class="header-eyebrow">{m.workbench_controls_header_eyebrow()}</span>
      {/if}
      <span
        class="collapse-toggle"
        class:expanded={!isCollapsed}
        aria-hidden="true"
        title={isCollapsed ? m.workbench_controls_expand() : m.workbench_controls_collapse()}
      >
        {isCollapsed ? '▾' : '▴'}
      </span>
    </button>

    {#if !isCollapsed}
      <CompositionControls
        {panelPath}
        {boundPanel}
        supportsOverlay={activePresentation.supportsOverlay ?? false}
        showScale={configParams.includes('scales')}
      />

      <!-- Phase 151 — View + Metric dropdowns side by side on one row to save
           vertical space; the withheld hints render as their own rows below. -->
      <div class="ctrl-row ctrl-row-split vm-row">
        <ViewControls
          {panelPath}
          {presentations}
          {activePresentation}
          {scalarMetricOptions}
          {offerableFields}
          {availableMetricNames}
          {availableMetadataFields}
        />
        <MetricControls
          {panelPath}
          {boundPanel}
          view={activePresentation.id}
          {viewUsesMetric}
          {viewUsesMetadataField}
          {availableMetricNames}
          {scopeGate}
          {scopeAvailableSet}
          {offerableFields}
          metadataResolved={metadataAvail !== null}
        />
      </div>

      <MetricHints
        {panelPath}
        {boundPanel}
        view={activePresentation.id}
        {viewUsesMetric}
        {viewUsesMetadataField}
        {availableMetricNames}
        {partialMetrics}
        {partialMetadataFields}
        {scopedSourceNames}
        {scopedSourceCount}
        {scopeAvailableSet}
        {configParams}
      />

      {#if viewUsesResolution}
        <ResolutionControls {panelPath} {boundPanel} />
      {/if}

      {#if configParams.length > 0}
        <ConfigValueLevers {panelPath} {boundPanel} {configParams} {viewUsesMetadataField} />
        <ConfigChannelLevers
          {panelPath}
          {boundPanel}
          {configParams}
          {scalarMetricOptions}
          {offerableFields}
          {viewUsesMetadataField}
          {viewSupportsFaceting}
        />
      {/if}

      <WindowControls {panelPath} {dateWindow} {todayStr} {boundPanel} {viewSupportsSilver} />

      <LayerCompareControls {panelPath} {boundPanel} {viewUsesNormalization} {canNormalize} />
    {/if}
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

  /* Phase 122k §14b finding 4 — clickable header reads as an interactive
     surface, not just the chevron. */
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

  /* Phase 151 — two labelled groups on one row (e.g. View · Metric). */
  .ctrl-row-split {
    gap: var(--space-4);
  }
</style>
