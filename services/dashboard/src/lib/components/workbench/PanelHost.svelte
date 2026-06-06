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
  import { createQuery } from '@tanstack/svelte-query';
  import {
    getPresentation,
    DEFAULT_METRIC_NAME,
    type PresentationDefinition,
    type ViewModeCellProps
  } from '$lib/viewmodes';
  import type { Panel, ViewingMode } from '$lib/state/url-internals';
  import {
    probesQuery,
    scopeAvailableMetricsQuery,
    scopeAvailableMetadataQuery,
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
    availabilityScope,
    expandProbeScopeFanout,
    expandFacetFanout,
    MAX_FACET_CELLS,
    resolveCellConfig,
    selectCellRender,
    shouldRefuseMergedCrossProbe,
    type CellRenderUnit
  } from '$lib/workbench/panel-queries';
  import { isPureCountMetric } from '$lib/viewmodes/metric-presentation';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import MethodologyBanner from '$lib/components/base/MethodologyBanner.svelte';
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
  import CellConfigPopover from './CellConfigPopover.svelte';

  interface Props {
    panel: Panel;
    dossier: ProbeDossierDto;
    ctx: FetchContext;
    windowStart: string | undefined;
    windowEnd: string | undefined;
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
    /** Phase 125b — Window-level cross-panel brushing selection, threaded to
     *  every cell. Per-article cells (scatter, parallel) use it; others ignore. */
    selection?: ViewModeCellProps['selection'];
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

  // Phase 122i revision (D3). `+ Compare` now opens the ScopeEditor
  // popover where the user picks sources for the new group. Previously
  // the button silently seeded a default group with all probes and
  // empty sources — confusing UX since the user wanted to choose.
  let scopeEditorOpen = $state(false);

  // Phase 126 — per-cell configuration popover. Holds the `unit.key` of the
  // cell whose config popover is open, or null. Per-cell overrides are offered
  // only on multi-cell panels (split / small-multiples) — on a single-cell
  // panel the PanelControls already configure that one cell.
  let openConfigKey = $state<string | null>(null);
  function toggleCellConfig(key: string, e: MouseEvent) {
    e.stopPropagation();
    openConfigKey = openConfigKey === key ? null : key;
  }

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

  // Phase 123c (B) — cross-probe source resolution. The threaded `dossier`
  // covers only ONE probe (the pillar's active probe). A split panel whose
  // ScopeGroup spans several probes must fan out EVERY probe's sources, so
  // we read the app-wide probe registry (`/probes`, already cached) to
  // enumerate each in-scope probe's source names + display label. The
  // dossier still supplies the richer per-source meta (emicDesignation) for
  // its own probe; other probes' sources resolve by name (emic null).
  const probesQ = createQuery<QueryOutcome<ProbeDto[]>, Error, QueryOutcome<ProbeDto[]>>(() => {
    const o = probesQuery(ctx);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });
  const probesById = $derived.by<Record<string, ProbeDto>>(() => {
    const map: Record<string, ProbeDto> = {};
    if (probesQ.data?.kind === 'success') for (const p of probesQ.data.data) map[p.probeId] = p;
    return map;
  });
  // Source NAMES for a probe: prefer the dossier (authoritative + ordered)
  // for its own probe; fall back to the registry list for the others. When a
  // dimension-source filter is active (ADR-038 — the panel's dimension is present
  // on only some sources), the list is narrowed to the sources that carry it
  // so the fan-out never renders a known-empty cell (lacking sources are dropped
  // and disclosed in the panel note instead).
  function sourceNamesForProbe(probeId: string): string[] {
    const all =
      probeId === dossier.probeId
        ? dossier.sources.map((s) => s.name)
        : (probesById[probeId]?.sources ?? []);
    const filter = dimensionSourceFilter;
    return filter ? all.filter((n) => filter.has(n)) : all;
  }

  // Phase 123c (Issue 6) — per-metric source availability over the panel's
  // full scope, so the fan-out can drop sources that lack the active metric
  // (the "show anyway" payoff: only data-carrying cells render, no scope
  // change). Shares the `/scope/available-metrics` cache key with
  // PanelControls. Window read from props/panel directly (not the
  // `effectiveWindow*` consts, which are declared later) to avoid a TDZ on
  // this query's initial synchronous evaluation.
  // ADR-038 — FILTER semantics (a group naming specific sources scopes to those
  // only), mirroring the render fan-out, so availability/drop/note are computed
  // over exactly what renders — not the whole probe.
  const panelScopeUnion = $derived(availabilityScope(panel.scopes));
  const panelHasScope = $derived(
    panelScopeUnion.probeIds.length > 0 || panelScopeUnion.sourceIds.length > 0
  );
  // ADR-038 — the panel's dimension is a Gold metric for metric views and a
  // categorical FIELD for field-driven views; availability comes from the
  // matching endpoint. `fieldDriven` routes the filter (and the dropped-source
  // note) to the right source, so a source lacking the chosen FIELD is dropped
  // exactly like one lacking a metric — uniform behaviour, no empty essays.
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
  // The active dimension's availability split (available / partial / scopedSources)
  // from whichever endpoint matches the view. `partialField` reads the right key
  // (`field` vs `metricName`).
  const dimensionAvail = $derived.by<{
    available: string[];
    partialSources: string[] | null;
    scopedSources: string[];
  } | null>(() => {
    const q = fieldDriven ? metadataAvailQ : metricAvailQ;
    if (q.data?.kind !== 'success') return null;
    const d = q.data.data;
    const partial = fieldDriven
      ? (d as ScopeAvailableMetadataDto).partial.find((p) => p.field === panel.metric)
      : (d as ScopeAvailableMetricsDto).partial.find((p) => p.metricName === panel.metric);
    return {
      available: d.available,
      partialSources: partial ? partial.sources : null,
      scopedSources: d.scopedSources
    };
  });
  // `null` = no narrowing (dimension on every source, or data not loaded). A Set
  // = render only these sources for the active dimension.
  const dimensionSourceFilter = $derived.by<Set<string> | null>(() => {
    const a = dimensionAvail;
    if (!a) return null;
    if (a.available.includes(panel.metric)) return null;
    return a.partialSources ? new Set(a.partialSources) : null;
  });
  // ADR-038 — scoped sources dropped because they lack the active dimension,
  // named in the panel note so absence is disclosed, never silent.
  const droppedSources = $derived.by<string[]>(() => {
    const a = dimensionAvail;
    if (!a || !a.partialSources) return [];
    const have = new Set(a.partialSources);
    return a.scopedSources.filter((s) => !have.has(s));
  });
  // ADR-038 — field-driven panel whose chosen dimension is empty/unshared: there
  // is no comparable categorical field across the scope (intersection empty,
  // show-anyway off). Surface ONE panel note instead of N empty cells.
  const noSharedDimension = $derived(fieldDriven && panelHasScope && !panel.metric);
  function probeLabelFor(probeId: string): string {
    const p = probesById[probeId];
    return p?.shortName ?? p?.displayName ?? probeId;
  }

  // Phase 126 — scalar metric options for the per-cell scatter-axis pickers.
  // Same all-source-intersection set as the panel metric picker (`available`),
  // default-prepended so the picker is never empty. The popover augments this
  // with any currently-bound channel metric so a binding stays visible.
  const scalarMetricOptions = $derived.by<string[]>(() => {
    const seen: Record<string, true> = {};
    const out: string[] = [];
    const push = (n: string | undefined) => {
      if (n && !seen[n]) {
        seen[n] = true;
        out.push(n);
      }
    };
    push(DEFAULT_METRIC_NAME);
    if (metricAvailQ.data?.kind === 'success')
      for (const m of metricAvailQ.data.data.available) push(m);
    return out;
  });
  // The distinct in-scope probe ids of a single-ScopeGroup split fan-out,
  // in stable scope order — drives the per-probe accent index (1-based tint
  // cycles through the same four-tone palette as the ScopeGroup accent).
  function probeTintIndex(probeId: string, order: readonly string[]): number {
    const i = order.indexOf(probeId);
    return i < 0 ? 0 : i;
  }

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
  // selector returns `merged-single` so this host can fan out per source at
  // render time. That semantic check lives here, not in the pure mapper,
  // because only the host has the source lists.
  //
  // Phase 123c (B) — the fan-out spans EVERY in-scope probe, not just the
  // threaded dossier's probe. The pure `expandProbeScopeFanout` builds the
  // per-(probe,source) units from the host-resolved source names; the
  // distinct probe order drives the per-probe accent when >1 probe.
  const fanout = $derived(
    expandProbeScopeFanout(panel, cellRender, sourceNamesForProbe, dossier.probeId)
  );

  // Phase 125a — faceting / small-multiples. When the panel names a facetField,
  // enumerate that categorical field's distinct values over the panel's scope
  // and render one sub-cell per value, each restricted to articles carrying it
  // (a `metadataFilter`). Faceting replaces the probe-scope fan-out and is
  // orthogonal to the metric/view. Only the per-article presentations support it
  // (a facet sub-cell is just the same cell with a metadataFilter).
  //
  // Faceting requires the panel to resolve to a SINGLE base scope unit
  // (merged-single, or a split panel whose single ScopeGroup has no source
  // narrowing — both yield one unit covering the whole probe). A panel that
  // splits into MULTIPLE base units (explicit multi-source narrowing, or
  // multiple ScopeGroups) is NOT faceted: the per-article endpoints are
  // single-scope, so faceting `units[0]` would silently drop the other
  // sources/groups. In that case we keep the normal fan-out and disclose that
  // faceting is unavailable (never a silent narrowing — the supreme maxim).
  const facetField = $derived(panel.facetField?.trim() ?? '');
  const facetSupported = $derived(presentation.supportsFaceting ?? false);
  const facetSingleScope = $derived(cellRender.units.length === 1);
  const facetBaseUnit = $derived<CellRenderUnit | null>(
    facetSingleScope ? (cellRender.units[0] ?? null) : null
  );
  const facetActive = $derived(facetField.length > 0 && facetSupported && facetBaseUnit !== null);
  // facetField set + supported, but the panel splits into >1 base unit → we
  // refuse to facet (would drop sources/groups) and surface a note instead.
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
  // never silently fall back to the unfaceted cell (that would misrepresent the
  // data). While the facet values load, or if the field has none in scope, the
  // unit list is empty and the template surfaces a status note instead.
  const expandedUnits = $derived<readonly CellRenderUnit[]>(
    facetActive ? (facetFanout?.units ?? []) : (fanout?.units ?? cellRender.units)
  );
  // Faceting status for the disclosure note (honest "showing N of M facets" +
  // cap), and to distinguish "loading" from "no values" in the empty branch.
  const facetPending = $derived(facetActive && facetValuesQ.isPending);
  const facetEmpty = $derived(
    facetActive && facetValuesQ.data?.kind === 'success' && facetFanout === null
  );
  const fanoutProbeOrder = $derived<readonly string[]>(fanout?.probeOrder ?? []);
  const isMultiProbeFanout = $derived(fanoutProbeOrder.length > 1);

  // Phase 125b — at-scale (WebGL) co-occurrence renderer engages ONLY when the
  // panel is maximized AND resolves to a single cell (one big focused map,
  // bounded to one simulation). Split / multi-cell / non-maximized keep the
  // default capped SVG cell. Lazy-loaded so the sigma chunk ships only on use.
  const atScaleActive = $derived(
    isMaximized &&
      (presentation.supportsAtScale ?? false) &&
      expandedUnits.length === 1 &&
      !facetActive
  );
  let AtScaleComponent = $state<Component<ViewModeCellProps> | null>(null);
  $effect(() => {
    if (!atScaleActive || AtScaleComponent) return;
    void import('$lib/components/viewmodes/CoOccurrenceNetworkAtScale.svelte').then((m) => {
      AtScaleComponent = m.default;
    });
  });

  // Phase 126 — per-cell overrides are offered only when the panel renders more
  // than one cell (a split / small-multiple), the presentation exposes at least
  // one cell-shape lever, and the host is interactive.
  const perCellConfig = $derived(
    path !== null && expandedUnits.length > 1 && (presentation.configurableParams?.length ?? 0) > 0
  );

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

  // ADR-038 — per-cell dimension peek option list. The raw availability payload
  // (available + partial) for the active dimension KIND.
  const availFullData = $derived.by<ScopeAvailableMetricsDto | ScopeAvailableMetadataDto | null>(
    () => {
      const q = fieldDriven ? metadataAvailQ : metricAvailQ;
      return q.data?.kind === 'success' ? q.data.data : null;
    }
  );
  // The source(s) the currently-open config cell covers (a single source for a
  // per-source cell; the unit's sources for a group/merged cell).
  const openCellSources = $derived.by<string[]>(() => {
    if (!openConfigKey) return [];
    const unit = expandedUnits.find((u) => u.key === openConfigKey);
    if (!unit) return [];
    if (unit.scope === 'source') return [unit.scopeId];
    return sourcesForUnit(unit).map((s) => s.name);
  });
  // Dimensions valid for the open cell's own source(s) — same kind as the view:
  // every intersection dimension (the cell's source has it) plus any partial at
  // least one of the cell's sources carries. This is the peek menu: a cell may
  // glance at a dimension the panel does not compare, but only one its own source
  // actually emits. Fully data-driven (no probe/source-specific code).
  const cellDimensionOptions = $derived.by<string[]>(() => {
    const d = availFullData;
    if (!d || openCellSources.length === 0) return [];
    const srcSet = new Set(openCellSources);
    const names = [...d.available];
    if (fieldDriven) {
      for (const p of (d as ScopeAvailableMetadataDto).partial)
        if (p.sources.some((s) => srcSet.has(s))) names.push(p.field);
    } else {
      for (const p of (d as ScopeAvailableMetricsDto).partial)
        if (p.sources.some((s) => srcSet.has(s))) names.push(p.metricName);
    }
    return [...new Set(names)].sort();
  });

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
  // Phase 125a — faceting always renders a small-multiples grid (one sub-cell
  // per facet value), independent of the composition toggle.
  const isSplitLayout = $derived(
    (panel.composition === 'split' || facetActive) && expandedUnits.length > 1
  );

  // ---------------------------------------------------------------------------
  // Phase 124 — shared-axis comparison discipline.
  //
  // When a panel renders >1 cell of a value-axis presentation, the cells must
  // be directly comparable: identical values must plot at identical positions.
  // Each cell reports its own data extent; PanelHost unions the extents and
  // hands the union back as `sharedDomains`, which the cell applies to its
  // value (and, for scatter, x) axis instead of auto-scaling.
  //
  // Gating (Brief §1.3 / §3.2, WP-004):
  //   - within one context (same probe, N sources) → always share.
  //   - pure-count metrics (commensurable) → always share, even cross-probe.
  //   - cross-context + intensive/scaled metric → share asserts cross-cultural
  //     commensurability, which needs a metric_equivalence grant. No intensive
  //     grant exists in Phase 124, so these keep INDEPENDENT axes + a caveat
  //     (the grant-aware relaxation is a Phase-125 refinement, mirroring the
  //     merged-cross-probe guard precedent).
  //   - panel-level 'free' escape hatch overrides shared for readability.
  // ---------------------------------------------------------------------------
  const SHARED_AXIS_VIEWS: ReadonlySet<string> = new Set([
    'distribution',
    'time_series',
    'metric_scatter'
  ]);
  const sharedAxisApplies = $derived(SHARED_AXIS_VIEWS.has(panel.view) && expandedUnits.length > 1);
  const renderedProbeCount = $derived.by(() => {
    // Plain Record dedup (not Set) per the codebase's prefer-svelte-reactivity
    // convention for transient reactive computations.
    const seen: Record<string, true> = {};
    for (const u of expandedUnits) {
      if (u.probeId) seen[u.probeId] = true;
      for (const pid of u.probeIds) seen[pid] = true;
    }
    return Object.keys(seen).length;
  });
  // Intensiveness for the cross-context guard. Most presentations carry the
  // axis metric in `panel.metric`; metric_scatter (usesMetric:false) instead
  // binds its axes to channel metrics (`panel.channels` x/y, defaulting exactly
  // as ScatterCell does), so the guard must inspect those — otherwise a
  // cross-probe scatter of intensive metrics would silently share axes with no
  // equivalence grant (the very cross-cultural commensurability claim the guard
  // exists to forbid).
  const isIntensiveMetric = $derived.by(() => {
    if (panel.view === 'metric_scatter') {
      const x = panel.channels?.x ?? 'word_count';
      const y = panel.channels?.y ?? DEFAULT_METRIC_NAME;
      return !isPureCountMetric(x) || !isPureCountMetric(y);
    }
    return presentation.usesMetric !== false && !isPureCountMetric(panel.metric);
  });
  const shareForbidden = $derived(sharedAxisApplies && renderedProbeCount > 1 && isIntensiveMetric);

  // Phase 126 — a cell's EFFECTIVE axis-scale, honouring per-cell overrides:
  //   - the cross-context guard forces every cell to 'free' (shareForbidden);
  //   - a per-cell x/y channel override changes WHAT the axis measures, so the
  //     cell can no longer share an axis with its siblings → 'free' (otherwise
  //     a shared union would merge incompatible metric ranges);
  //   - otherwise the per-cell `scales` override wins over the panel default.
  function effectiveCellScale(cellKey: string): 'shared' | 'free' {
    if (shareForbidden) return 'free';
    const ov = panel.cellOverrides?.[cellKey];
    if (ov?.channels?.x !== undefined || ov?.channels?.y !== undefined) return 'free';
    return ov?.scales ?? panel.scales ?? 'shared';
  }
  // The cells that actually share the axis — only these feed AND read the union,
  // so a cell freed (by override or guard) neither stretches its siblings' axis
  // nor is distorted by theirs.
  const sharedCellKeys = $derived.by<Record<string, true>>(() => {
    const out: Record<string, true> = {};
    if (!sharedAxisApplies || shareForbidden) return out;
    for (const u of expandedUnits) if (effectiveCellScale(u.key) === 'shared') out[u.key] = true;
    return out;
  });
  // Compute the union only when at least one cell consumes it (avoids the
  // all-'free' panel recomputing a union nothing reads).
  const computeShared = $derived(sharedAxisApplies && Object.keys(sharedCellKeys).length > 0);

  // Per-rendered-cell extents, keyed `${cellKey}|${axis}`. A plain Record
  // (reassigned on each update) drives reactivity without a mutable Map, per
  // the codebase's prefer-svelte-reactivity convention.
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
  function unionExtent(axis: 'value' | 'x'): readonly [number, number] | undefined {
    let lo = Infinity;
    let hi = -Infinity;
    for (const [k, v] of Object.entries(reportedExtents)) {
      if (!k.endsWith(`|${axis}`)) continue;
      // Phase 126 — only the shared cells form the union; a freed cell still
      // reports its extent (so it stays fresh if re-shared) but is excluded here.
      if (!sharedCellKeys[k.slice(0, k.lastIndexOf('|'))]) continue;
      if (v[0] < lo) lo = v[0];
      if (v[1] > hi) hi = v[1];
    }
    return lo <= hi ? [lo, hi] : undefined;
  }
  const sharedDomains = $derived.by<
    { value?: readonly [number, number]; x?: readonly [number, number] } | undefined
  >(() => {
    if (!computeShared) return undefined;
    const value = unionExtent('value');
    const x = unionExtent('x');
    const out: { value?: readonly [number, number]; x?: readonly [number, number] } = {};
    if (value) out.value = value;
    if (x) out.x = x;
    return value || x ? out : undefined;
  });

  // Prune extents for cells no longer rendered (split→merged toggle, scope
  // narrowing, fan-out change). Without this a removed cell's extent lingers in
  // the union and widens the shared axis past the live data. Guarded so it only
  // reassigns when something is actually stale (no reactive loop).
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

  {#if shareForbidden}
    <MethodologyBanner anchorHref="/reflection/wp/wp-004?section=6.3" anchorLabel="WP-004 §6.3">
      <strong>Independent axes.</strong> Putting “{panel.metric}” on one shared axis across cultural
      contexts would assert cross-cultural commensurability, which requires a validated equivalence
      grant — none exists yet for this metric. Each cell keeps its own optimal scale; read positions
      within a cell, not across.
    </MethodologyBanner>
  {/if}

  <!-- ADR-038 — disclose sources dropped because they lack the active dimension,
       so absence is named, never silent (Tier 2 "show anyway" payoff). -->
  {#if droppedSources.length > 0 && !noSharedDimension}
    <p class="panel-drop-note" role="note">
      Not shown: <strong>{droppedSources.join(', ')}</strong> — no
      <code>{panel.metric}</code> (not emitted).
    </p>
  {/if}

  <!-- Phase 125a — faceting is set but the panel splits into >1 base scope unit
       (multi-source / multi-group). We refuse to facet rather than silently drop
       sources, and say so. -->
  {#if facetUnavailable}
    <p class="panel-drop-note" role="note">
      Faceting by <code>{facetField}</code> is unavailable for a multi-source / multi-group panel
      (it would cover only the first). Use <strong>Merged</strong> composition or a single source to facet.
    </p>
  {/if}

  <div
    class="panel-body"
    class:split={isSplitLayout}
    data-split-direction={panel.splitDirection ?? 'horizontal'}
  >
    {#if crossProbeRefusal}
      <RefusalSurface refusal={crossProbeRefusal} {ctx} />
    {:else if atScaleActive}
      <!-- Phase 125b — maximized single-cell co-occurrence → large-scale WebGL
           renderer (sigma.js). Handles its own loading / refusal internally. -->
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
            {dataLayer}
            channels={cfg.channels}
            displayLanguage={cfg.displayLanguage}
            configOverridden={cfg.isOverridden}
          />
        </div>
      {:else}
        <p class="muted" aria-busy="true">Loading large-scale renderer…</p>
      {/if}
    {:else if loadError}
      <p class="muted">Cell failed to load: {loadError}</p>
    {:else if !CellComponent}
      <p class="muted" aria-busy="true">Loading {presentation.label}…</p>
    {:else if noSharedDimension}
      <p class="muted">
        No categorical dimension is shared across the scoped sources. Enable a partial field via <strong
          >Show anyway</strong
        > in the panel config, or use a cell's config to peek at one source's own dimension.
      </p>
    {:else if facetPending}
      <p class="muted" aria-busy="true">Loading facet values for <code>{facetField}</code>…</p>
    {:else if facetEmpty}
      <p class="muted">
        No values for <code>{facetField}</code> in the active scope — nothing to facet by.
      </p>
    {:else if expandedUnits.length === 0}
      <p class="muted">No sources in the active scope.</p>
    {:else}
      {@const Cell = CellComponent}
      {#if facetFanout}
        <p class="facet-disclosure" role="note">
          Faceted by <code>{facetFanout.field}</code> — showing {facetFanout.units.length} of {facetFanout.totalValues}
          value{facetFanout.totalValues === 1 ? '' : 's'}{facetFanout.capped
            ? ` (top ${MAX_FACET_CELLS} by article count; the rest are omitted)`
            : ''}. Each sub-cell holds only articles carrying that value.
        </p>
      {/if}
      {#each expandedUnits as unit (unit.key)}
        {@const groupNum = unit.groupIndex !== undefined ? unit.groupIndex + 1 : null}
        {@const probeNum =
          unit.probeId !== undefined && isMultiProbeFanout
            ? probeTintIndex(unit.probeId, fanoutProbeOrder) + 1
            : null}
        {@const accentNum = groupNum ?? probeNum}
        {@const cfg = resolveCellConfig(panel, unit.key)}
        {@const cellScale = effectiveCellScale(unit.key)}
        <div
          class="panel-cell"
          class:has-group={accentNum !== null}
          class:cell-overridden={cfg.isOverridden}
          data-group={accentNum}
        >
          {#if perCellConfig}
            <div class="cell-config-bar">
              {#if cfg.isOverridden}
                <span
                  class="cell-custom-badge"
                  title="This cell is on a custom configuration — not directly comparable to its sibling cells."
                >
                  custom
                </span>
              {/if}
              <button
                type="button"
                class="cell-config-btn"
                class:active={openConfigKey === unit.key}
                aria-label="Configure this cell"
                aria-expanded={openConfigKey === unit.key}
                title="Per-cell configuration — override the panel default for this cell only"
                onclick={(e) => toggleCellConfig(unit.key, e)}
              >
                ⚙ Cell
              </button>
            </div>
          {/if}
          {#if perCellConfig && path && openConfigKey === unit.key}
            <CellConfigPopover
              panelPath={path}
              cellKey={unit.key}
              cellLabel={unit.scopeId}
              {panel}
              {presentation}
              {scalarMetricOptions}
              {cellDimensionOptions}
              onClose={() => (openConfigKey = null)}
            />
          {/if}
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
          {:else if probeNum !== null && unit.probeId}
            <!-- Phase 123c (B) — per-probe accent for the cross-probe split
                 fan-out. The badge carries the probe's display label so the
                 reader knows which probe each source-cell belongs to. -->
            <header class="cell-group-eyebrow">
              <span class="cell-group-badge">{probeLabelFor(unit.probeId)}</span>
              <span class="cell-group-summary">{unit.scopeId}</span>
            </header>
          {:else if unit.facetField && unit.facetValue !== undefined}
            <!-- Phase 125a — faceting / small-multiples. Each sub-cell is the
                 same view restricted to one value of the facet field; the badge
                 names that value so the grid reads as "<field> = <value>". -->
            <header class="cell-group-eyebrow">
              <span class="cell-group-badge">{unit.facetField}</span>
              <span class="cell-group-summary">{unit.facetValue}</span>
            </header>
          {/if}
          {#if cfg.dimensionOverridden}
            <!-- ADR-038 — per-cell dimension peek. This cell shows a DIFFERENT
                 dimension than the panel, so it is deliberately off-comparison.
                 A loud banner makes that unmistakable (cf. the soft `custom`
                 badge for shape-only overrides). -->
            <p class="cell-peek-banner" role="note">
              ⚠ Showing <code>{cfg.metric}</code> — a different dimension than this panel's
              <code>{panel.metric}</code>. Not comparable to the sibling cells.
            </p>
          {/if}
          <Cell
            {ctx}
            scopeProbeId={unit.probeId ?? dossier.probeId}
            scope={unit.scope}
            scopeId={unit.scopeId}
            windowStart={effectiveWindowStart}
            windowEnd={effectiveWindowEnd}
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
            reportExtent={sharedAxisApplies ? reportExtentFor(unit.key) : undefined}
            sharedDomains={cellScale === 'shared' ? sharedDomains : undefined}
            axisScaleState={sharedAxisApplies ? cellScale : undefined}
            configOverridden={cfg.isOverridden}
            {selection}
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
    /* Issue 8 (revision) — cap the horizontal split at TWO cells per row and
       wrap the rest beneath, instead of a single ever-narrowing row that
       either crushes cells or scrolls the 4th out of view. Two columns keep
       each cell wide enough that the cell title (e.g.
       `sentiment_score_bert_multilingual — distribution · bundesregierung`)
       fits on one line, while all four cross-probe cells stay visible at once
       as a 2×2 grid. */
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--space-4);
  }
  .panel-body.split[data-split-direction='horizontal'] > .panel-cell {
    /* Grid track already bounds the width; allow the cell to shrink to it. */
    min-width: 0;
  }

  /* Issue 8 (follow-up) — the cells' own header (title + CellExport export
     buttons) was a non-wrapping flex row, so at the split's narrow width the
     export controls overflowed the cell. Scoped to panel cells so the shells'
     and reflection InlineChart `.cell-header`s elsewhere are untouched. */
  .panel-cell :global(.cell-header) {
    flex-wrap: wrap;
    gap: var(--space-2);
  }
  .panel-cell :global(.cell-title) {
    min-width: 0;
  }

  .panel-body.split[data-split-direction='vertical'] {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .panel-cell {
    min-height: 14rem;
    /* Phase 126 — anchor for the per-cell config popover. */
    position: relative;
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
  /* ADR-038 — per-cell dimension-peek banner: loud, because it marks an
     off-comparison cell (distinct from the soft `custom` shape-override badge). */
  .cell-peek-banner {
    margin: 0 0 var(--space-2);
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--color-warning, #e8a850);
    border-left-width: 3px;
    border-radius: var(--radius-sm);
    background: color-mix(in srgb, var(--color-warning, #e8a850) 12%, transparent);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
    line-height: var(--line-height-loose);
  }
  .cell-peek-banner code {
    font-family: var(--font-mono);
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
  /* ADR-038 — dropped-source disclosure note above the cell grid. */
  .panel-drop-note {
    margin: 0 0 var(--space-2);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
  }
  .panel-drop-note code {
    font-family: var(--font-mono);
    color: var(--color-fg);
  }
</style>
